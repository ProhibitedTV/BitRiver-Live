package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"bitriver-live/internal/executil"
)

type doctorStatus string

const (
	doctorStatusPass doctorStatus = "PASS"
	doctorStatusWarn doctorStatus = "WARN"
	doctorStatusFail doctorStatus = "FAIL"
)

type doctorResult struct {
	Name        string       `json:"name"`
	Status      doctorStatus `json:"status"`
	Details     string       `json:"details"`
	Remediation string       `json:"remediation,omitempty"`
}

type doctorReport struct {
	Status      doctorStatus   `json:"status"`
	Checks      []doctorResult `json:"checks"`
	EnvFile     string         `json:"env_file"`
	ComposeFile string         `json:"compose_file"`
}

type doctorOptions struct {
	JSON        bool
	EnvFile     string
	ComposeFile string
	MinCPU      int
	MinRAMGB    float64
	MinDiskGB   float64
	MinDocker   string
	MinCompose  string
}

type composeBindMount struct {
	source   string
	readOnly bool
}

var (
	doctorLookPath      = executil.LookPath
	doctorCommandOutput = func(name string, args ...string) (string, error) {
		cmd := exec.Command(name, args...)
		out, err := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), err
	}
	doctorPortRequirementsLoader = quickstartRequiredHostPorts
	doctorHostPortChecker        = checkHostPortAvailable
)

func runDoctor(args []string) bool {
	opts, err := parseDoctorArgs(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true
		}
		fmt.Fprintf(os.Stderr, "doctor: %v\n", err)
		return false
	}

	report := runDoctorChecks(opts)
	if opts.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(report)
	} else {
		printDoctorHuman(report)
	}
	return report.Status != doctorStatusFail
}

func parseDoctorArgs(args []string) (doctorOptions, error) {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	opts := doctorOptions{}
	fs.BoolVar(&opts.JSON, "json", false, "emit machine-readable JSON report")
	fs.StringVar(&opts.EnvFile, "env-file", defaultEnvFile(), "env file for profile/port checks")
	fs.StringVar(&opts.ComposeFile, "compose-file", defaultComposeFile(), "compose file for port/path checks")
	fs.IntVar(&opts.MinCPU, "min-cpu", 4, "minimum recommended logical CPUs")
	fs.Float64Var(&opts.MinRAMGB, "min-ram-gb", 8, "minimum recommended host RAM in GiB")
	fs.Float64Var(&opts.MinDiskGB, "min-free-disk-gb", 20, "minimum recommended free disk in GiB")
	fs.StringVar(&opts.MinDocker, "min-docker-version", "24.0.0", "minimum Docker version")
	fs.StringVar(&opts.MinCompose, "min-compose-version", "2.20.0", "minimum Docker Compose v2 version")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s doctor [flags]\n", os.Args[0])
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return doctorOptions{}, err
	}
	return opts, nil
}

func runDoctorChecks(opts doctorOptions) doctorReport {
	envValues, _ := loadEnvValues(opts.EnvFile, true)
	checks := []doctorResult{
		checkRequiredBinaries(),
		checkDockerAndCompose(opts),
		checkHostResources(opts),
		checkPortConflicts(opts, envValues),
		checkWritablePaths(opts, envValues),
	}
	status := doctorStatusPass
	for _, c := range checks {
		if c.Status == doctorStatusFail {
			status = doctorStatusFail
			break
		}
		if c.Status == doctorStatusWarn {
			status = doctorStatusWarn
		}
	}
	return doctorReport{Status: status, Checks: checks, EnvFile: opts.EnvFile, ComposeFile: opts.ComposeFile}
}

func printDoctorHuman(report doctorReport) {
	fmt.Fprintln(os.Stdout, "BitRiver Live preflight (doctor)")
	fmt.Fprintf(os.Stdout, "OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(os.Stdout, "Compose file: %s\n", report.ComposeFile)
	fmt.Fprintf(os.Stdout, "Env file: %s\n\n", report.EnvFile)
	for _, result := range report.Checks {
		fmt.Fprintf(os.Stdout, "%-6s %-30s %s\n", result.Status, result.Name, result.Details)
		if strings.TrimSpace(result.Remediation) != "" {
			fmt.Fprintf(os.Stdout, "       fix: %s\n", result.Remediation)
		}
	}
	fmt.Fprintf(os.Stdout, "\nSUMMARY: %s\n", report.Status)
}

func checkRequiredBinaries() doctorResult {
	if _, err := doctorLookPath("docker"); err != nil {
		return doctorResult{Name: "Required binaries", Status: doctorStatusFail, Details: "docker CLI not found on PATH", Remediation: "Install Docker Desktop/Engine and ensure `docker` is on PATH."}
	}
	if _, err := doctorLookPath("git"); err != nil {
		return doctorResult{Name: "Required binaries", Status: doctorStatusWarn, Details: "docker found; git missing (optional)", Remediation: "Install git for release/debug workflows that inspect repo state."}
	}
	return doctorResult{Name: "Required binaries", Status: doctorStatusPass, Details: "docker and git detected"}
}

func checkDockerAndCompose(opts doctorOptions) doctorResult {
	dockerVersionOut, err := doctorCommandOutput("docker", "version", "--format", "{{.Client.Version}}")
	if err != nil {
		return doctorResult{Name: "Docker/Compose versions", Status: doctorStatusFail, Details: fmt.Sprintf("docker version check failed: %v", err), Remediation: "Start Docker daemon and verify `docker version` succeeds."}
	}
	composeVersionOut, err := doctorCommandOutput("docker", "compose", "version", "--short")
	if err != nil {
		return doctorResult{Name: "Docker/Compose versions", Status: doctorStatusFail, Details: fmt.Sprintf("docker compose check failed: %v", err), Remediation: "Install/enable Docker Compose v2 and verify `docker compose version` succeeds."}
	}

	dockerVersion := extractVersion(dockerVersionOut)
	composeVersion := extractVersion(composeVersionOut)
	if dockerVersion == "" || composeVersion == "" {
		return doctorResult{Name: "Docker/Compose versions", Status: doctorStatusWarn, Details: fmt.Sprintf("could not parse versions (docker=%q compose=%q)", dockerVersionOut, composeVersionOut), Remediation: fmt.Sprintf("Confirm Docker >= %s and Compose >= %s manually.", opts.MinDocker, opts.MinCompose)}
	}
	if compareSemver(dockerVersion, opts.MinDocker) < 0 {
		return doctorResult{Name: "Docker/Compose versions", Status: doctorStatusFail, Details: fmt.Sprintf("docker %s < required %s", dockerVersion, opts.MinDocker), Remediation: "Upgrade Docker Desktop/Engine to a supported release."}
	}
	if compareSemver(composeVersion, opts.MinCompose) < 0 {
		return doctorResult{Name: "Docker/Compose versions", Status: doctorStatusFail, Details: fmt.Sprintf("compose %s < required %s", composeVersion, opts.MinCompose), Remediation: "Upgrade Docker Compose v2 plugin to a supported release."}
	}
	return doctorResult{Name: "Docker/Compose versions", Status: doctorStatusPass, Details: fmt.Sprintf("docker=%s compose=%s (minimums: %s/%s)", dockerVersion, composeVersion, opts.MinDocker, opts.MinCompose)}
}

func checkHostResources(opts doctorOptions) doctorResult {
	cpus := runtime.NumCPU()
	ramBytes, ramErr := detectTotalMemoryBytes()
	diskPath := detectDiskPath(repoRoot())
	diskBytes, diskErr := detectFreeDiskBytes(diskPath)

	issues := []string{}
	warnings := []string{}
	if cpus < opts.MinCPU {
		issues = append(issues, fmt.Sprintf("CPU %d < %d", cpus, opts.MinCPU))
	}
	if ramErr != nil {
		warnings = append(warnings, fmt.Sprintf("RAM detection unavailable (%v)", ramErr))
	} else if float64(ramBytes)/(1024*1024*1024) < opts.MinRAMGB {
		issues = append(issues, fmt.Sprintf("RAM %.1f GiB < %.1f GiB", float64(ramBytes)/(1024*1024*1024), opts.MinRAMGB))
	}
	if diskErr != nil {
		warnings = append(warnings, fmt.Sprintf("disk detection unavailable (%v)", diskErr))
	} else if float64(diskBytes)/(1024*1024*1024) < opts.MinDiskGB {
		issues = append(issues, fmt.Sprintf("free disk %.1f GiB < %.1f GiB", float64(diskBytes)/(1024*1024*1024), opts.MinDiskGB))
	}

	details := fmt.Sprintf("cpu=%d", cpus)
	if ramErr == nil {
		details += fmt.Sprintf(", ram=%.1fGiB", float64(ramBytes)/(1024*1024*1024))
	}
	if diskErr == nil {
		details += fmt.Sprintf(", free_disk=%.1fGiB@%s", float64(diskBytes)/(1024*1024*1024), diskPath)
	}
	if len(issues) > 0 {
		return doctorResult{Name: "Host resources", Status: doctorStatusFail, Details: details + " (" + strings.Join(issues, "; ") + ")", Remediation: "Scale host CPU/RAM/disk or reduce expected workload before production."}
	}
	if len(warnings) > 0 {
		return doctorResult{Name: "Host resources", Status: doctorStatusWarn, Details: details + " (" + strings.Join(warnings, "; ") + ")", Remediation: "Validate host sizing manually for this OS before production rollout."}
	}
	return doctorResult{Name: "Host resources", Status: doctorStatusPass, Details: details}
}

func checkPortConflicts(opts doctorOptions, envValues map[string]string) doctorResult {
	if _, err := os.Stat(opts.ComposeFile); err != nil {
		return doctorResult{Name: "Host port conflicts", Status: doctorStatusFail, Details: fmt.Sprintf("compose file unavailable: %v", err), Remediation: "Pass a valid --compose-file path and rerun doctor."}
	}
	if envValues == nil {
		envValues = map[string]string{}
	}
	requirements, err := doctorPortRequirementsLoader(envValues)
	if err != nil {
		return doctorResult{Name: "Host port conflicts", Status: doctorStatusFail, Details: fmt.Sprintf("invalid env port values: %v", err), Remediation: "Fix .env port values (1-65535, valid ranges) and rerun doctor."}
	}
	composePorts, parseErr := parseComposeHostPorts(opts.ComposeFile, envValues)
	if parseErr == nil {
		requirements = append(requirements, composePorts...)
	}
	conflicts := []string{}
	seen := map[string]struct{}{}
	for _, req := range requirements {
		for _, port := range req.ports {
			key := req.protocol + ":" + strconv.Itoa(port)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if err := doctorHostPortChecker(req.protocol, port); err != nil {
				conflicts = append(conflicts, fmt.Sprintf("%s/%d (%s)", strings.ToUpper(req.protocol), port, req.name))
			}
		}
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return doctorResult{Name: "Host port conflicts", Status: doctorStatusFail, Details: fmt.Sprintf("%d required port(s) unavailable", len(conflicts)), Remediation: "Stop conflicting services or change host port mappings: " + strings.Join(conflicts, ", ")}
	}
	if parseErr != nil {
		return doctorResult{Name: "Host port conflicts", Status: doctorStatusWarn, Details: "ports from env are available (compose parse incomplete)", Remediation: fmt.Sprintf("Review host port mappings in %s manually: %v", opts.ComposeFile, parseErr)}
	}
	return doctorResult{Name: "Host port conflicts", Status: doctorStatusPass, Details: "required host ports appear available"}
}

func checkWritablePaths(opts doctorOptions, envValues map[string]string) doctorResult {
	mounts, err := parseComposeBindMounts(opts.ComposeFile, envValues)
	if err != nil {
		return doctorResult{Name: "Compose bind-mount paths", Status: doctorStatusFail, Details: fmt.Sprintf("unable to read compose file: %v", err), Remediation: "Pass a valid --compose-file and ensure it is readable."}
	}
	if len(mounts) == 0 {
		return doctorResult{Name: "Compose bind-mount paths", Status: doctorStatusWarn, Details: "no bind mounts detected in compose file", Remediation: "Confirm compose file still uses expected host bind mounts for state/config."}
	}
	failures := []string{}
	warnings := []string{}
	for _, m := range mounts {
		fi, statErr := os.Stat(m.source)
		if statErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s missing/unreadable", m.source))
			continue
		}
		if fi.IsDir() {
			if _, openErr := os.ReadDir(m.source); openErr != nil {
				warnings = append(warnings, fmt.Sprintf("%s not readable", m.source))
				continue
			}
			if !m.readOnly && fi.Mode().Perm()&0200 == 0 {
				failures = append(failures, fmt.Sprintf("%s not writable", m.source))
			}
			continue
		}
		if fh, openErr := os.Open(m.source); openErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s not readable", m.source))
		} else {
			_ = fh.Close()
		}
		if !m.readOnly && fi.Mode().Perm()&0200 == 0 {
			failures = append(failures, fmt.Sprintf("%s not writable", m.source))
		}
	}
	if len(failures) > 0 {
		return doctorResult{Name: "Compose bind-mount paths", Status: doctorStatusFail, Details: fmt.Sprintf("%d path permission issue(s)", len(failures)), Remediation: strings.Join(failures, "; ")}
	}
	if len(warnings) > 0 {
		return doctorResult{Name: "Compose bind-mount paths", Status: doctorStatusWarn, Details: fmt.Sprintf("validated with %d warning(s)", len(warnings)), Remediation: "Create or fix bind-mount paths: " + strings.Join(warnings, "; ")}
	}
	return doctorResult{Name: "Compose bind-mount paths", Status: doctorStatusPass, Details: fmt.Sprintf("validated %d bind-mount host path(s)", len(mounts))}
}

func detectDiskPath(fallback string) string {
	out, err := doctorCommandOutput("docker", "info", "--format", "{{.DockerRootDir}}")
	if err == nil && strings.TrimSpace(out) != "" {
		return strings.TrimSpace(out)
	}
	return fallback
}

func parseComposeHostPorts(composeFile string, values map[string]string) ([]quickstartPortRequirement, error) {
	f, err := os.Open(composeFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	ports := []quickstartPortRequirement{}
	inPorts := false
	portsIndent := 0
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if strings.HasSuffix(trim, "ports:") {
			inPorts = true
			portsIndent = indent
			continue
		}
		if inPorts && indent <= portsIndent && !strings.HasPrefix(trim, "-") {
			inPorts = false
		}
		if !inPorts || !strings.HasPrefix(trim, "-") {
			continue
		}
		item := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trim, "-")), "\"")
		parts := strings.Split(item, ":")
		if len(parts) < 2 {
			continue
		}
		hostPart := parts[len(parts)-2]
		protocol := "tcp"
		if strings.Contains(parts[len(parts)-1], "/udp") {
			protocol = "udp"
		}
		hostPart = strings.Split(hostPart, "/")[0]
		hostPart = strings.TrimSpace(resolveTemplate(hostPart, values))
		if hostPart == "" {
			continue
		}
		p, convErr := strconv.Atoi(hostPart)
		if convErr != nil || p < 1 || p > 65535 {
			continue
		}
		ports = append(ports, quickstartPortRequirement{name: "compose-file", protocol: protocol, ports: []int{p}})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return ports, nil
}

func parseComposeBindMounts(composeFile string, values map[string]string) ([]composeBindMount, error) {
	f, err := os.Open(composeFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	baseDir := filepath.Dir(composeFile)
	mounts := []composeBindMount{}
	inVolumes := false
	volumesIndent := 0

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if strings.HasSuffix(trim, "volumes:") {
			inVolumes = true
			volumesIndent = indent
			continue
		}
		if inVolumes && indent <= volumesIndent && !strings.HasPrefix(trim, "-") {
			inVolumes = false
		}
		if !inVolumes || !strings.HasPrefix(trim, "-") {
			continue
		}
		item := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
		item = strings.Trim(item, "\"")
		parts := strings.Split(item, ":")
		if len(parts) < 2 {
			continue
		}
		host := strings.TrimSpace(resolveTemplate(parts[0], values))
		mode := ""
		if len(parts) > 2 {
			mode = strings.TrimSpace(parts[len(parts)-1])
		}
		if host == "" || (!strings.HasPrefix(host, ".") && !strings.HasPrefix(host, "/")) {
			continue
		}
		if !filepath.IsAbs(host) {
			host = filepath.Clean(filepath.Join(baseDir, host))
		}
		mounts = append(mounts, composeBindMount{source: host, readOnly: strings.Contains(mode, "ro")})
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return mounts, nil
}

func resolveTemplate(in string, values map[string]string) string {
	re := regexp.MustCompile(`\$\{([^}]+)\}`)
	return re.ReplaceAllStringFunc(in, func(token string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(token, "${"), "}")
		if strings.Contains(inner, ":-") {
			parts := strings.SplitN(inner, ":-", 2)
			if v := strings.TrimSpace(values[parts[0]]); v != "" {
				return v
			}
			return parts[1]
		}
		if strings.Contains(inner, "-") {
			parts := strings.SplitN(inner, "-", 2)
			if _, ok := values[parts[0]]; ok {
				return values[parts[0]]
			}
			return parts[1]
		}
		if v := strings.TrimSpace(values[inner]); v != "" {
			return v
		}
		if v := strings.TrimSpace(os.Getenv(inner)); v != "" {
			return v
		}
		return ""
	})
}

func extractVersion(raw string) string {
	re := regexp.MustCompile(`\d+\.\d+(?:\.\d+)?`)
	return re.FindString(raw)
}

func compareSemver(a, b string) int {
	pa := parseSemverParts(a)
	pb := parseSemverParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseSemverParts(v string) [3]int {
	parts := strings.Split(v, ".")
	out := [3]int{}
	for i := 0; i < 3 && i < len(parts); i++ {
		n, _ := strconv.Atoi(parts[i])
		out[i] = n
	}
	return out
}

func detectTotalMemoryBytes() (uint64, error) {
	if runtime.GOOS != "linux" {
		return 0, fmt.Errorf("unsupported OS %s", runtime.GOOS)
	}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, fmt.Errorf("unexpected MemTotal format")
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, err
		}
		return kb * 1024, nil
	}
	return 0, fmt.Errorf("MemTotal not found")
}

func checkHostPortAvailable(protocol string, port int) error {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	switch protocol {
	case "tcp":
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		_ = ln.Close()
		return nil
	case "udp":
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			return err
		}
		_ = pc.Close()
		return nil
	default:
		return fmt.Errorf("unsupported protocol %q", protocol)
	}
}
