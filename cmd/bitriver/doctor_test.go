package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDoctorReturnsFalseWhenThresholdFails(t *testing.T) {
	if runDoctor([]string{"--min-cpu", "999"}) {
		t.Fatal("expected doctor to fail with intentionally impossible cpu threshold")
	}
}

func TestRunDoctorJSONOutput(t *testing.T) {
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = origStdout
	}()

	_ = runDoctor([]string{"--json", "--min-cpu", "999"})

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read output: %v", err)
	}

	var report doctorReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal doctor json: %v\noutput=%s", err, buf.String())
	}
	if report.Status != doctorStatusFail {
		t.Fatalf("expected fail status, got %s", report.Status)
	}
	if len(report.Checks) == 0 {
		t.Fatal("expected checks in report")
	}
}

func TestRunDoctorChecksPassWithInjectedDependencies(t *testing.T) {
	originalLookPath := doctorLookPath
	originalOutput := doctorCommandOutput
	originalPortLoader := doctorPortRequirementsLoader
	originalPortChecker := doctorHostPortChecker
	originalGPU := doctorIsGPUAvailable
	defer func() {
		doctorLookPath = originalLookPath
		doctorCommandOutput = originalOutput
		doctorPortRequirementsLoader = originalPortLoader
		doctorHostPortChecker = originalPortChecker
		doctorIsGPUAvailable = originalGPU
	}()

	doctorLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	doctorCommandOutput = func(name string, args ...string) (string, error) {
		cmd := strings.Join(append([]string{name}, args...), " ")
		switch cmd {
		case "docker version --format {{.Client.Version}}":
			return "25.0.1", nil
		case "docker compose version --short":
			return "2.27.0", nil
		default:
			return "", nil
		}
	}
	doctorPortRequirementsLoader = func(map[string]string) ([]quickstartPortRequirement, error) {
		return []quickstartPortRequirement{{name: "test", protocol: "tcp", ports: []int{65530}}}, nil
	}
	doctorHostPortChecker = func(protocol string, port int) error { return nil }
	doctorIsGPUAvailable = func() bool { return false }

	report := runDoctorChecks(doctorOptions{EnvFile: filepath.Join("testdata", "missing.env"), MinCPU: 1, MinRAMGB: 0, MinDiskGB: 0, MinDocker: "24.0.0", MinCompose: "2.20.0"})
	if report.Status == doctorStatusFail {
		t.Fatalf("expected non-fail report with injected dependencies, got %+v", report)
	}
}
