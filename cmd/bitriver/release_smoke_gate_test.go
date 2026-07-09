package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseSmokeGateFastWritesArtifacts(t *testing.T) {
	dir := t.TempDir()
	envPath, composePath, migrationsDir := writeReleaseGateFixture(t, dir)
	artifactDir := filepath.Join(dir, "artifacts")

	restore := stubReleaseGateRunners(t)
	defer restore()

	err := runReleaseSmokeGate([]string{
		"--tier", "fast",
		"--env-file", envPath,
		"--contract-env-file", envPath,
		"--compose-file", composePath,
		"--migrations-dir", migrationsDir,
		"--artifact-dir", artifactDir,
		"--target", "v1.2.3",
	})
	if err != nil {
		t.Fatalf("smoke-gate fast failed: %v", err)
	}

	for _, name := range []string{
		"version.txt",
		"env-redaction-summary.json",
		"contract-snapshot.json",
		"compose-config.yml",
		"upgrade-plan.txt",
		"release-gate-report.json",
	} {
		if _, err := os.Stat(filepath.Join(artifactDir, name)); err != nil {
			t.Fatalf("expected artifact %s: %v", name, err)
		}
	}

	report := readReleaseGateReport(t, filepath.Join(artifactDir, "release-gate-report.json"))
	if report.Status != "passed" {
		t.Fatalf("expected report to pass, got %#v", report)
	}
	if !releaseGatePhaseStatus(report, "compose config", "passed") {
		t.Fatalf("expected compose config phase to pass, got %#v", report.Phases)
	}
}

func TestReleaseSmokeGateFullRunsQuickstartSmokeAndDiagnostics(t *testing.T) {
	dir := t.TempDir()
	envPath, composePath, migrationsDir := writeReleaseGateFixture(t, dir)
	artifactDir := filepath.Join(dir, "artifacts")

	restore := stubReleaseGateRunners(t)
	defer restore()

	var calls []string
	releaseGateQuickstartRunner = func(args []string) error {
		calls = append(calls, "quickstart:"+strings.Join(args, " "))
		return nil
	}
	releaseGateSmokeRunner = func(args []string) error {
		calls = append(calls, "smoke:"+strings.Join(args, " "))
		return nil
	}

	err := runReleaseSmokeGate([]string{
		"--tier", "full",
		"--env-file", envPath,
		"--contract-env-file", envPath,
		"--compose-file", composePath,
		"--migrations-dir", migrationsDir,
		"--artifact-dir", artifactDir,
	})
	if err != nil {
		t.Fatalf("smoke-gate full failed: %v", err)
	}
	if len(calls) != 2 || !strings.HasPrefix(calls[0], "quickstart:") || !strings.HasPrefix(calls[1], "smoke:") {
		t.Fatalf("expected quickstart then smoke calls, got %#v", calls)
	}
	report := readReleaseGateReport(t, filepath.Join(artifactDir, "release-gate-report.json"))
	if !releaseGatePhaseStatus(report, "compose ps diagnostics", "passed") {
		t.Fatalf("expected compose ps diagnostics, got %#v", report.Phases)
	}
	if !releaseGatePhaseStatus(report, "compose logs diagnostics", "passed") {
		t.Fatalf("expected compose logs diagnostics, got %#v", report.Phases)
	}
}

func TestReleaseSmokeGateFailureNamesPhaseAndArtifact(t *testing.T) {
	dir := t.TempDir()
	envPath, composePath, migrationsDir := writeReleaseGateFixture(t, dir)
	artifactDir := filepath.Join(dir, "artifacts")

	restore := stubReleaseGateRunners(t)
	defer restore()
	releaseGateCommandRunner = func(name string, args []string, output string) error {
		if len(args) > 0 && args[len(args)-1] == "config" {
			return errors.New("compose exploded")
		}
		return os.WriteFile(output, []byte("ok\n"), 0o644)
	}

	err := runReleaseSmokeGate([]string{
		"--tier", "fast",
		"--env-file", envPath,
		"--contract-env-file", envPath,
		"--compose-file", composePath,
		"--migrations-dir", migrationsDir,
		"--artifact-dir", artifactDir,
	})
	if err == nil {
		t.Fatal("expected smoke-gate to fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "compose config") || !strings.Contains(msg, "compose-config.yml") {
		t.Fatalf("expected actionable phase/artifact error, got %q", msg)
	}
	report := readReleaseGateReport(t, filepath.Join(artifactDir, "release-gate-report.json"))
	if report.Status != "failed" {
		t.Fatalf("expected failed report, got %#v", report)
	}
	if !releaseGatePhaseStatus(report, "compose config", "failed") {
		t.Fatalf("expected failed compose phase, got %#v", report.Phases)
	}
}

func writeReleaseGateFixture(t *testing.T, dir string) (string, string, string) {
	t.Helper()
	envPath := filepath.Join(dir, ".env.example")
	composePath := filepath.Join(dir, "docker-compose.yml")
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		t.Fatalf("create migrations: %v", err)
	}
	writeTestFile(t, envPath, `
# API port exposed to operators.
BITRIVER_LIVE_PORT=18080
# Admin password.
BITRIVER_LIVE_ADMIN_PASSWORD=secret
BITRIVER_LIVE_IMAGE_TAG=bitriver-live:test
`)
	writeTestFile(t, composePath, `
services:
  bitriver-live:
    image: ${BITRIVER_LIVE_IMAGE_TAG:-bitriver-live:dev}
    ports:
      - "${BITRIVER_LIVE_PORT:-8080}:8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/healthz"]
`)
	writeTestFile(t, filepath.Join(migrationsDir, "0001_initial.sql"), "create table test (id text);\n")
	return envPath, composePath, migrationsDir
}

func stubReleaseGateRunners(t *testing.T) func() {
	t.Helper()
	origQuickstart := releaseGateQuickstartRunner
	origSmoke := releaseGateSmokeRunner
	origUpgrade := releaseGateUpgradePlanRunner
	origCommand := releaseGateCommandRunner
	origComposeAvailable := releaseGateComposeAvailable

	releaseGateQuickstartRunner = func([]string) error { return nil }
	releaseGateSmokeRunner = func([]string) error { return nil }
	releaseGateUpgradePlanRunner = func([]string) error {
		printVersionInfo(os.Stdout)
		return nil
	}
	releaseGateComposeAvailable = func() error { return nil }
	releaseGateCommandRunner = func(_ string, args []string, output string) error {
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}
		return os.WriteFile(output, []byte(strings.Join(args, " ")+"\n"), 0o644)
	}

	return func() {
		releaseGateQuickstartRunner = origQuickstart
		releaseGateSmokeRunner = origSmoke
		releaseGateUpgradePlanRunner = origUpgrade
		releaseGateCommandRunner = origCommand
		releaseGateComposeAvailable = origComposeAvailable
	}
}

func readReleaseGateReport(t *testing.T, path string) releaseSmokeGateReport {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report releaseSmokeGateReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	return report
}

func releaseGatePhaseStatus(report releaseSmokeGateReport, name, status string) bool {
	for _, phase := range report.Phases {
		if phase.Name == name && phase.Status == status {
			return true
		}
	}
	return false
}
