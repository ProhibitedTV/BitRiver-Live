package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const releaseSmokeGateSchemaVersion = "bitriver.releaseSmokeGate.v1"

type releaseSmokeGateReport struct {
	SchemaVersion  string                  `json:"schemaVersion"`
	Tier           string                  `json:"tier"`
	Status         string                  `json:"status"`
	ComposeFile    string                  `json:"composeFile"`
	EnvFile        string                  `json:"envFile"`
	EffectiveEnv   string                  `json:"effectiveEnvFile,omitempty"`
	ArtifactDir    string                  `json:"artifactDir"`
	Phases         []releaseSmokeGatePhase `json:"phases"`
	StagedFollowUp []string                `json:"stagedFollowUp,omitempty"`
}

type releaseSmokeGatePhase struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Command  []string `json:"command,omitempty"`
	Artifact string   `json:"artifact,omitempty"`
	Details  string   `json:"details"`
}

type releaseSmokeGateConfig struct {
	Tier            string
	ComposeFile     string
	EnvFile         string
	ContractEnvFile string
	MigrationsDir   string
	ArtifactDir     string
	Target          string
	ImageSource     string
	LogsTail        string
}

var (
	releaseGateQuickstartRunner  = runQuickstart
	releaseGateSmokeRunner       = runSmoke
	releaseGateUpgradePlanRunner = runUpgradePlan
	releaseGateCommandRunner     = runReleaseGateCommandToFile
	releaseGateComposeAvailable  = defaultReleaseGateComposeAvailable
)

func runReleaseSmokeGate(args []string) error {
	fs := flag.NewFlagSet("release smoke-gate", flag.ContinueOnError)
	tier := fs.String("tier", "fast", "gate tier to run: fast or full")
	composeFile := fs.String("compose-file", defaultComposeFile(), "compose file for release-gate checks")
	envFile := fs.String("env-file", defaultEnvFile(), "runtime environment file")
	contractEnvFile := fs.String("contract-env-file", defaultExampleEnv(), "env template used for contract snapshot evidence")
	migrationsDir := fs.String("migrations-dir", filepath.Join(repoRoot(), "deploy", "migrations"), "migration directory used for contract snapshot evidence")
	artifactDir := fs.String("artifact-dir", filepath.Join(repoRoot(), ".artifacts", "release-gate"), "directory for release-gate evidence artifacts")
	target := fs.String("target", "", "optional target release tag for upgrade-plan evidence")
	imageSource := fs.String("image-source", "build", "image source passed to full-tier quickstart")
	logsTail := fs.String("logs-tail", "120", "docker compose logs --tail value for full-tier diagnostics")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg := releaseSmokeGateConfig{
		Tier:            strings.ToLower(strings.TrimSpace(*tier)),
		ComposeFile:     *composeFile,
		EnvFile:         *envFile,
		ContractEnvFile: *contractEnvFile,
		MigrationsDir:   *migrationsDir,
		ArtifactDir:     *artifactDir,
		Target:          strings.TrimSpace(*target),
		ImageSource:     strings.TrimSpace(*imageSource),
		LogsTail:        strings.TrimSpace(*logsTail),
	}
	if cfg.Tier == "" {
		cfg.Tier = "fast"
	}
	if cfg.ImageSource == "" {
		cfg.ImageSource = "build"
	}
	if cfg.LogsTail == "" {
		cfg.LogsTail = "120"
	}
	if cfg.Tier != "fast" && cfg.Tier != "full" {
		return fmt.Errorf("unsupported smoke-gate tier %q", cfg.Tier)
	}

	if err := os.MkdirAll(cfg.ArtifactDir, 0o755); err != nil {
		return fmt.Errorf("create artifact dir: %w", err)
	}

	report := newReleaseSmokeGateReport(cfg)
	err := executeReleaseSmokeGate(cfg, &report)
	if err != nil {
		report.Status = "failed"
	} else if report.Status == "" {
		report.Status = "passed"
	}
	if writeErr := writeReleaseSmokeGateReport(report, filepath.Join(cfg.ArtifactDir, "release-gate-report.json")); writeErr != nil {
		if err != nil {
			return fmt.Errorf("%w; additionally failed to write release-gate report: %v", err, writeErr)
		}
		return writeErr
	}
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Release smoke gate passed (%s tier). Artifacts: %s\n", cfg.Tier, cfg.ArtifactDir)
	return nil
}

func newReleaseSmokeGateReport(cfg releaseSmokeGateConfig) releaseSmokeGateReport {
	effectiveEnv, _ := resolveReleaseGateEvidenceEnv(cfg.EnvFile)
	return releaseSmokeGateReport{
		SchemaVersion: releaseSmokeGateSchemaVersion,
		Tier:          cfg.Tier,
		ComposeFile:   filepath.ToSlash(cfg.ComposeFile),
		EnvFile:       filepath.ToSlash(cfg.EnvFile),
		EffectiveEnv:  filepath.ToSlash(effectiveEnv),
		ArtifactDir:   filepath.ToSlash(cfg.ArtifactDir),
		StagedFollowUp: []string{
			"Packaged launcher smoke: install or unpack the release artifact, run the launcher against a clean env, run smoke, and attach launcher logs plus version output.",
			"Upgrade smoke: start the previous release or baseline, preserve stateful data, apply the target release, run migrations if applicable, run smoke, and attach upgrade notes.",
		},
	}
}

func executeReleaseSmokeGate(cfg releaseSmokeGateConfig, report *releaseSmokeGateReport) error {
	effectiveEnv, envDetails := resolveReleaseGateEvidenceEnv(cfg.EnvFile)
	if effectiveEnv != cfg.EnvFile {
		report.EffectiveEnv = filepath.ToSlash(effectiveEnv)
	}
	composeEnv, composeEnvDetails, cleanupComposeEnv, err := prepareReleaseGateComposeEnv(cfg.EnvFile, effectiveEnv)
	if err != nil {
		return err
	}
	defer cleanupComposeEnv()

	if err := runReleaseGatePhase(report, "version evidence", []string{"bitriver", "version"}, filepath.Join(cfg.ArtifactDir, "version.txt"), "records CLI version metadata for release evidence", func(artifact string) error {
		return captureStdoutToFile(artifact, func() error {
			printVersionInfo(os.Stdout)
			return nil
		})
	}); err != nil {
		return err
	}

	if err := runReleaseGatePhase(report, "environment redaction summary", []string{"bitriver", "release", "smoke-gate", "redact-env"}, filepath.Join(cfg.ArtifactDir, "env-redaction-summary.json"), envDetails, func(artifact string) error {
		return writeEnvRedactionSummary(effectiveEnv, artifact)
	}); err != nil {
		return err
	}

	if err := runReleaseGatePhase(report, "contract snapshot", []string{"bitriver", "release", "contract-snapshot", "--env-file", cfg.ContractEnvFile, "--compose-file", cfg.ComposeFile}, filepath.Join(cfg.ArtifactDir, "contract-snapshot.json"), "captures env, compose, migration, generated-artifact, endpoint, and release input contract evidence", func(artifact string) error {
		snapshot, err := buildContractSnapshot(cfg.ContractEnvFile, cfg.ComposeFile, cfg.MigrationsDir)
		if err != nil {
			return err
		}
		return writeJSONFile(snapshot, artifact)
	}); err != nil {
		return err
	}

	composeConfigArtifact := filepath.Join(cfg.ArtifactDir, "compose-config.yml")
	if err := releaseGateComposeAvailable(); err != nil {
		details := fmt.Sprintf("skipped: docker compose is unavailable: %v", err)
		if writeErr := os.WriteFile(composeConfigArtifact, []byte(details+"\n"), 0o644); writeErr != nil {
			return writeErr
		}
		report.Phases = append(report.Phases, releaseSmokeGatePhase{
			Name:     "compose config",
			Status:   "skipped",
			Command:  []string{"docker", "compose", "--env-file", composeEnv, "-f", cfg.ComposeFile, "config"},
			Artifact: filepath.ToSlash(composeConfigArtifact),
			Details:  details,
		})
		if cfg.Tier == "full" {
			return fmt.Errorf("release smoke gate phase %q failed (artifact: %s): %w", "compose config", filepath.ToSlash(composeConfigArtifact), err)
		}
	} else {
		if err := runReleaseGatePhase(report, "compose config", []string{"docker", "compose", "--env-file", composeEnv, "-f", cfg.ComposeFile, "config"}, composeConfigArtifact, "renders the canonical compose contract for the selected env file. "+composeEnvDetails, func(artifact string) error {
			return releaseGateCommandRunner("docker", []string{"compose", "--env-file", composeEnv, "-f", cfg.ComposeFile, "config"}, artifact)
		}); err != nil {
			return err
		}
	}

	if strings.TrimSpace(cfg.Target) == "" {
		report.Phases = append(report.Phases, releaseSmokeGatePhase{
			Name:    "upgrade plan",
			Status:  "skipped",
			Details: "skipped: pass --target vX.Y.Z to capture upgrade-plan evidence for a release candidate.",
		})
	} else if err := runReleaseGatePhase(report, "upgrade plan", []string{"bitriver", "upgrade-plan", "--compose-file", cfg.ComposeFile, "--env-file", composeEnv, "--target", cfg.Target}, filepath.Join(cfg.ArtifactDir, "upgrade-plan.txt"), "records migration and operator upgrade checklist evidence", func(artifact string) error {
		return captureStdoutToFile(artifact, func() error {
			return releaseGateUpgradePlanRunner([]string{"--compose-file", cfg.ComposeFile, "--env-file", composeEnv, "--target", cfg.Target})
		})
	}); err != nil {
		return err
	}

	if cfg.Tier == "fast" {
		return nil
	}

	if err := runReleaseGatePhase(report, "source quickstart", []string{"bitriver", "quickstart", "--compose-file", cfg.ComposeFile, "--env-file", cfg.EnvFile, "--image-source", cfg.ImageSource}, filepath.Join(cfg.ArtifactDir, "quickstart.txt"), "runs the source checkout quickstart path for release-candidate evidence", func(artifact string) error {
		return captureStdoutToFile(artifact, func() error {
			return releaseGateQuickstartRunner([]string{"--compose-file", cfg.ComposeFile, "--env-file", cfg.EnvFile, "--image-source", cfg.ImageSource})
		})
	}); err != nil {
		collectReleaseGateDiagnostics(cfg, report)
		return err
	}

	if err := runReleaseGatePhase(report, "smoke command", []string{"bitriver", "smoke", "--compose-file", cfg.ComposeFile, "--env-file", cfg.EnvFile}, filepath.Join(cfg.ArtifactDir, "smoke.txt"), "runs post-start endpoint and service smoke checks", func(artifact string) error {
		return captureStdoutToFile(artifact, func() error {
			return releaseGateSmokeRunner([]string{"--compose-file", cfg.ComposeFile, "--env-file", cfg.EnvFile})
		})
	}); err != nil {
		collectReleaseGateDiagnostics(cfg, report)
		return err
	}

	collectReleaseGateDiagnostics(cfg, report)
	return nil
}

func runReleaseGatePhase(report *releaseSmokeGateReport, name string, command []string, artifact, details string, run func(string) error) error {
	phase := releaseSmokeGatePhase{
		Name:     name,
		Command:  append([]string(nil), command...),
		Artifact: filepath.ToSlash(artifact),
		Details:  details,
	}
	if err := run(artifact); err != nil {
		phase.Status = "failed"
		phase.Details = fmt.Sprintf("%s: %v", details, err)
		report.Phases = append(report.Phases, phase)
		return fmt.Errorf("release smoke gate phase %q failed (artifact: %s): %w", name, filepath.ToSlash(artifact), err)
	}
	phase.Status = "passed"
	report.Phases = append(report.Phases, phase)
	return nil
}

func collectReleaseGateDiagnostics(cfg releaseSmokeGateConfig, report *releaseSmokeGateReport) {
	psArtifact := filepath.Join(cfg.ArtifactDir, "compose-ps.json")
	if err := releaseGateCommandRunner("docker", []string{"compose", "--env-file", cfg.EnvFile, "-f", cfg.ComposeFile, "ps", "--format", "json"}, psArtifact); err != nil {
		report.Phases = append(report.Phases, releaseSmokeGatePhase{Name: "compose ps diagnostics", Status: "failed", Command: []string{"docker", "compose", "ps", "--format", "json"}, Artifact: filepath.ToSlash(psArtifact), Details: err.Error()})
	} else {
		report.Phases = append(report.Phases, releaseSmokeGatePhase{Name: "compose ps diagnostics", Status: "passed", Command: []string{"docker", "compose", "ps", "--format", "json"}, Artifact: filepath.ToSlash(psArtifact), Details: "captured docker compose service state."})
	}

	logArtifact := filepath.Join(cfg.ArtifactDir, "compose-logs.txt")
	args := []string{"compose", "--env-file", cfg.EnvFile, "-f", cfg.ComposeFile, "logs", "--tail", cfg.LogsTail, "bitriver-live", "viewer", "srs-controller", "transcoder", "postgres-migrations", "srs-config", "ome-config", "ome-health-token-check", "postgres", "redis"}
	if err := releaseGateCommandRunner("docker", args, logArtifact); err != nil {
		report.Phases = append(report.Phases, releaseSmokeGatePhase{Name: "compose logs diagnostics", Status: "failed", Command: append([]string{"docker"}, args...), Artifact: filepath.ToSlash(logArtifact), Details: err.Error()})
	} else {
		report.Phases = append(report.Phases, releaseSmokeGatePhase{Name: "compose logs diagnostics", Status: "passed", Command: append([]string{"docker"}, args...), Artifact: filepath.ToSlash(logArtifact), Details: "captured relevant service logs."})
	}
}

func resolveReleaseGateEvidenceEnv(envFile string) (string, string) {
	if _, err := os.Stat(envFile); err == nil {
		return envFile, "records environment keys only; values are redacted."
	}
	example := defaultExampleEnv()
	if _, err := os.Stat(example); err == nil {
		return example, fmt.Sprintf("runtime env %s was not present; used %s for non-secret evidence.", envFile, example)
	}
	return envFile, "records environment keys only; values are redacted."
}

func prepareReleaseGateComposeEnv(envFile, fallbackEnv string) (string, string, func(), error) {
	cleanup := func() {}
	if _, err := os.Stat(envFile); err == nil {
		return envFile, "Using requested env file for Compose evidence.", cleanup, nil
	}
	if !sameCleanPath(envFile, defaultEnvFile()) {
		return fallbackEnv, fmt.Sprintf("Requested env file %s is absent; using %s for Compose evidence.", envFile, fallbackEnv), cleanup, nil
	}
	if _, err := os.Stat(fallbackEnv); err != nil {
		return envFile, "", cleanup, err
	}
	data, err := os.ReadFile(fallbackEnv)
	if err != nil {
		return envFile, "", cleanup, err
	}
	if err := os.WriteFile(envFile, data, 0o600); err != nil {
		return envFile, "", cleanup, err
	}
	cleanup = func() {
		_ = os.Remove(envFile)
	}
	return envFile, fmt.Sprintf("Root .env was absent; created a temporary copy from %s for Compose env_file compatibility and removed it after the gate.", fallbackEnv), cleanup, nil
}

func sameCleanPath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA == nil {
		a = absA
	}
	if errB == nil {
		b = absB
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

type envRedactionSummary struct {
	EnvFile string                 `json:"envFile"`
	Keys    []envRedactionKeyEntry `json:"keys"`
	Summary envRedactionCounts     `json:"summary"`
}

type envRedactionKeyEntry struct {
	Key               string `json:"key"`
	Redacted          bool   `json:"redacted"`
	SecuritySensitive bool   `json:"securitySensitive"`
}

type envRedactionCounts struct {
	TotalKeys         int `json:"totalKeys"`
	RedactedKeys      int `json:"redactedKeys"`
	SecuritySensitive int `json:"securitySensitiveKeys"`
}

func writeEnvRedactionSummary(envFile, output string) error {
	values, err := loadEnvValues(envFile, false)
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	summary := envRedactionSummary{EnvFile: filepath.ToSlash(envFile)}
	for _, key := range keys {
		sensitive := securitySensitiveEnvKey(key)
		entry := envRedactionKeyEntry{Key: key, Redacted: true, SecuritySensitive: sensitive}
		summary.Keys = append(summary.Keys, entry)
		summary.Summary.TotalKeys++
		summary.Summary.RedactedKeys++
		if sensitive {
			summary.Summary.SecuritySensitive++
		}
	}
	return writeJSONFile(summary, output)
}

func captureStdoutToFile(output string, run func() error) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil && filepath.Dir(output) != "." {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	oldStdout := os.Stdout
	os.Stdout = file
	runErr := run()
	os.Stdout = oldStdout
	closeErr := file.Close()
	if runErr != nil {
		return runErr
	}
	return closeErr
}

func runReleaseGateCommandToFile(name string, args []string, output string) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil && filepath.Dir(output) != "." {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()

	cmd := exec.Command(name, args...)
	cmd.Stdout = file
	cmd.Stderr = file
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w%s", name, strings.Join(args, " "), err, commandOutputTail(output))
	}
	return nil
}

func commandOutputTail(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	const limit = 4096
	if len(data) > limit {
		data = data[len(data)-limit:]
	}
	tail := strings.TrimSpace(string(data))
	if tail == "" {
		return ""
	}
	return ": " + tail
}

func defaultReleaseGateComposeAvailable() error {
	cmd := exec.Command("docker", "compose", "version")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func writeReleaseSmokeGateReport(report releaseSmokeGateReport, output string) error {
	return writeJSONFile(report, output)
}

func writeJSONFile(value any, output string) error {
	if strings.TrimSpace(output) == "" {
		return errors.New("missing JSON output path")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil && filepath.Dir(output) != "." {
		return err
	}
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return err
	}
	_, err = io.WriteString(file, "\n")
	return err
}
