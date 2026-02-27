package main

import (
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
	"syscall"

	"bitriver-live/internal/executil"
)

type doctorStatus string

const (
	doctorStatusPass doctorStatus = "PASS"
	doctorStatusWarn doctorStatus = "WARN"
	doctorStatusFail doctorStatus = "FAIL"
)

type doctorCheckResult struct {
	Name       string       `json:"name"`
	Status     doctorStatus `json:"status"`
	Summary    string       `json:"summary"`
	Mitigation string       `json:"mitigation,omitempty"`
}

type doctorReport struct {
	Status   doctorStatus        `json:"status"`
	Checks   []doctorCheckResult `json:"checks"`
	EnvFile  string              `json:"env_file"`
	JSONMode bool                `json:"json"`
}

type doctorOptions struct {
	JSON       bool
	EnvFile    string
	MinCPU     int
	MinRAMGB   float64
	MinDiskGB  float64
	MinDocker  string
	MinCompose string
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
	doctorIsGPUAvailable         = detectGPUAvailable
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
	fs.StringVar(&opts.EnvFile, "env-file", defaultEnvFile(), "env file for port/profile checks")
	fs.IntVar(&opts.MinCPU, "min-cpu", 2, "minimum recommended logical CPUs")
	fs.Float64Var(&opts.MinRAMGB, "min-ram-gb", 4, "minimum recommended host RAM in GiB")
	fs.Float64Var(&opts.MinDiskGB, "min-free-disk-gb", 10, "minimum recommended free disk in GiB")
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
	results := []doctorCheckResult{
		checkHostResources(opts),
		checkDockerAndCompose(opts),
		checkPortConflicts(opts),
		checkWritablePaths(),
		checkGPUProfile(opts),
	}
	status := doctorStatusPass
	for _, r := range results {
		if r.Status == doctorStatusFail {
			status = doctorStatusFail
			break
		}
		if r.Status == doctorStatusWarn {
			status = doctorStatusWarn
		}
	}
	return doctorReport{Status: status, Checks: results, EnvFile: opts.EnvFile, JSONMode: opts.JSON}
}

func printDoctorHuman(report doctorReport) {
	fmt.Fprintln(os.Stdout, "BitRiver Live doctor")
	fmt.Fprintf(os.Stdout, "OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(os.Stdout, "Env file: %s\n\n", report.EnvFile)
	for _, result := range report.Checks {
		fmt.Fprintf(os.Stdout, "[%s] %s\n", result.Status, result.Name)
		fmt.Fprintf(os.Stdout, "  %s\n", result.Summary)
		if strings.TrimSpace(result.Mitigation) != "" {
			fmt.Fprintf(os.Stdout, "  Mitigation: %s\n", result.Mitigation)
		}
	}
	fmt.Fprintf(os.Stdout, "\nSUMMARY: %s\n", report.Status)
	if report.Status == doctorStatusFail {
		fmt.Fprintln(os.Stdout, "One or more required preflight checks failed. Fix the FAIL items and rerun `go run ./cmd/bitriver doctor`.")
	} else if report.Status == doctorStatusWarn {
		fmt.Fprintln(os.Stdout, "Preflight can continue with WARN items, but review mitigations before production use.")
	}
}

func checkHostResources(opts doctorOptions) doctorCheckResult {
	cpus := runtime.NumCPU()
	ramBytes, ramErr := detectTotalMemoryBytes()
	diskBytes, diskErr := detectFreeDiskBytes(repoRoot())

	issues := []string{}
	warns := []string{}
	if cpus < opts.MinCPU {
		issues = append(issues, fmt.Sprintf("CPU %d < recommended %d", cpus, opts.MinCPU))
	}
	if ramErr != nil {
		warns = append(warns, fmt.Sprintf("RAM detection unavailable (%v)", ramErr))
	} else if float64(ramBytes)/(1024*1024*1024) < opts.MinRAMGB {
		issues = append(issues, fmt.Sprintf("RAM %.1f GiB < recommended %.1f GiB", float64(ramBytes)/(1024*1024*1024), opts.MinRAMGB))
	}
	if diskErr != nil {
		warns = append(warns, fmt.Sprintf("disk free-space detection unavailable (%v)", diskErr))
	} else if float64(diskBytes)/(1024*1024*1024) < opts.MinDiskGB {
		issues = append(issues, fmt.Sprintf("free disk %.1f GiB < recommended %.1f GiB", float64(diskBytes)/(1024*1024*1024), opts.MinDiskGB))
	}

	summaryParts := []string{fmt.Sprintf("logical CPU=%d", cpus)}
	if ramErr == nil {
		summaryParts = append(summaryParts, fmt.Sprintf("RAM=%.1f GiB", float64(ramBytes)/(1024*1024*1024)))
	}
	if diskErr == nil {
		summaryParts = append(summaryParts, fmt.Sprintf("free disk=%.1f GiB", float64(diskBytes)/(1024*1024*1024)))
	}
	summary := strings.Join(summaryParts, ", ")

	if len(issues) > 0 {
		return doctorCheckResult{Name: "Host resources", Status: doctorStatusFail, Summary: summary + " (" + strings.Join(issues, "; ") + ")", Mitigation: "Add host resources or lower load (fewer streams/transcodes), then rerun doctor."}
	}
	if len(warns) > 0 {
		return doctorCheckResult{Name: "Host resources", Status: doctorStatusWarn, Summary: summary + " (" + strings.Join(warns, "; ") + ")", Mitigation: "Validate RAM/disk manually for your OS before production rollout."}
	}
	return doctorCheckResult{Name: "Host resources", Status: doctorStatusPass, Summary: summary}
}

func checkDockerAndCompose(opts doctorOptions) doctorCheckResult {
	if _, err := doctorLookPath("docker"); err != nil {
		return doctorCheckResult{Name: "Docker + Docker Compose", Status: doctorStatusFail, Summary: fmt.Sprintf("docker CLI not found: %v", err), Mitigation: "Install Docker Desktop/Engine and ensure `docker` is on PATH."}
	}
	dockerVersionOut, err := doctorCommandOutput("docker", "version", "--format", "{{.Client.Version}}")
	if err != nil {
		return doctorCheckResult{Name: "Docker + Docker Compose", Status: doctorStatusFail, Summary: fmt.Sprintf("docker version check failed: %v", err), Mitigation: "Start Docker daemon and verify `docker version` succeeds."}
	}
	composeVersionOut, err := doctorCommandOutput("docker", "compose", "version", "--short")
	if err != nil {
		return doctorCheckResult{Name: "Docker + Docker Compose", Status: doctorStatusFail, Summary: fmt.Sprintf("docker compose check failed: %v", err), Mitigation: "Install/enable Docker Compose v2 and verify `docker compose version` succeeds."}
	}

	dockerVersion := extractVersion(dockerVersionOut)
	composeVersion := extractVersion(composeVersionOut)
	if dockerVersion == "" || composeVersion == "" {
		return doctorCheckResult{Name: "Docker + Docker Compose", Status: doctorStatusWarn, Summary: fmt.Sprintf("unable to parse versions (docker=%q compose=%q)", dockerVersionOut, composeVersionOut), Mitigation: fmt.Sprintf("Confirm Docker >= %s and Compose >= %s manually.", opts.MinDocker, opts.MinCompose)}
	}
	if compareSemver(dockerVersion, opts.MinDocker) < 0 {
		return doctorCheckResult{Name: "Docker + Docker Compose", Status: doctorStatusFail, Summary: fmt.Sprintf("docker %s is older than required %s", dockerVersion, opts.MinDocker), Mitigation: "Upgrade Docker Desktop/Engine to a supported release."}
	}
	if compareSemver(composeVersion, opts.MinCompose) < 0 {
		return doctorCheckResult{Name: "Docker + Docker Compose", Status: doctorStatusFail, Summary: fmt.Sprintf("docker compose %s is older than required %s", composeVersion, opts.MinCompose), Mitigation: "Upgrade Docker Compose v2 plugin to a supported release."}
	}

	return doctorCheckResult{Name: "Docker + Docker Compose", Status: doctorStatusPass, Summary: fmt.Sprintf("docker=%s compose=%s", dockerVersion, composeVersion)}
}

func checkPortConflicts(opts doctorOptions) doctorCheckResult {
	values, err := loadEnvValues(opts.EnvFile, true)
	if err != nil {
		return doctorCheckResult{Name: "Host port conflicts", Status: doctorStatusWarn, Summary: fmt.Sprintf("could not load env file %s: %v", opts.EnvFile, err), Mitigation: "Ensure .env exists and rerun doctor for full port conflict checks."}
	}
	requirements, err := doctorPortRequirementsLoader(values)
	if err != nil {
		return doctorCheckResult{Name: "Host port conflicts", Status: doctorStatusFail, Summary: fmt.Sprintf("port requirements invalid: %v", err), Mitigation: "Fix port variables in .env (port range 1-65535, valid ranges), then rerun doctor."}
	}

	conflicts := []string{}
	for _, req := range requirements {
		for _, port := range req.ports {
			if err := doctorHostPortChecker(req.protocol, port); err != nil {
				conflicts = append(conflicts, fmt.Sprintf("%s/%d (%s)", strings.ToUpper(req.protocol), port, req.name))
			}
		}
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		return doctorCheckResult{Name: "Host port conflicts", Status: doctorStatusFail, Summary: fmt.Sprintf("%d required port(s) unavailable", len(conflicts)), Mitigation: "Stop conflicting services or adjust .env port values: " + strings.Join(conflicts, ", ")}
	}
	return doctorCheckResult{Name: "Host port conflicts", Status: doctorStatusPass, Summary: "all required host ports appear available"}
}

func checkWritablePaths() doctorCheckResult {
	paths := []string{
		filepath.Join(repoRoot(), "deploy", "data"),
		filepath.Join(repoRoot(), "deploy", "transcoder-data"),
		filepath.Join(repoRoot(), "deploy", "ome"),
		filepath.Join(repoRoot(), "deploy", "srs", "conf"),
	}
	unwritable := []string{}
	warnings := []string{}
	for _, path := range paths {
		fi, err := os.Stat(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s (%v)", path, err))
			continue
		}
		if !fi.IsDir() {
			unwritable = append(unwritable, fmt.Sprintf("%s is not a directory", path))
			continue
		}
		if fi.Mode().Perm()&0200 == 0 {
			unwritable = append(unwritable, fmt.Sprintf("%s lacks owner write bit", path))
		}
	}
	if len(unwritable) > 0 {
		return doctorCheckResult{Name: "Writable runtime paths", Status: doctorStatusFail, Summary: "required runtime directories are not writable", Mitigation: strings.Join(unwritable, "; ")}
	}
	if len(warnings) > 0 {
		return doctorCheckResult{Name: "Writable runtime paths", Status: doctorStatusWarn, Summary: "some runtime directories are missing/unreadable", Mitigation: "Create/permission-check these paths: " + strings.Join(warnings, "; ")}
	}
	return doctorCheckResult{Name: "Writable runtime paths", Status: doctorStatusPass, Summary: "checked deploy runtime directories"}
}

func checkGPUProfile(opts doctorOptions) doctorCheckResult {
	values, err := loadEnvValues(opts.EnvFile, true)
	if err != nil {
		return doctorCheckResult{Name: "GPU profile (optional)", Status: doctorStatusWarn, Summary: "skipped GPU profile detection (env unavailable)", Mitigation: "If you run GPU workloads, verify host GPU drivers/runtime manually."}
	}
	profiles := strings.ToLower(strings.TrimSpace(values["COMPOSE_PROFILES"]))
	if profiles == "" {
		profiles = strings.ToLower(strings.TrimSpace(os.Getenv("COMPOSE_PROFILES")))
	}
	if !strings.Contains(profiles, "gpu") && !strings.Contains(profiles, "nvidia") {
		return doctorCheckResult{Name: "GPU profile (optional)", Status: doctorStatusPass, Summary: "no GPU profile selected"}
	}
	if doctorIsGPUAvailable() {
		return doctorCheckResult{Name: "GPU profile (optional)", Status: doctorStatusPass, Summary: "GPU profile selected and GPU tooling detected"}
	}
	return doctorCheckResult{Name: "GPU profile (optional)", Status: doctorStatusWarn, Summary: "GPU profile appears selected but no GPU runtime detected", Mitigation: "Install/enable NVIDIA drivers + container runtime, or remove GPU profile from COMPOSE_PROFILES."}
}

func detectGPUAvailable() bool {
	if _, err := doctorLookPath("nvidia-smi"); err == nil {
		return true
	}
	if out, err := doctorCommandOutput("docker", "info", "--format", "{{json .Runtimes}}"); err == nil {
		lower := strings.ToLower(out)
		return strings.Contains(lower, "nvidia")
	}
	return false
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

func detectFreeDiskBytes(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
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
