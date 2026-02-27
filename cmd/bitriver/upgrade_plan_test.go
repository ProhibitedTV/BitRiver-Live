package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildUpgradePlanRejectsSkippedMinor(t *testing.T) {
	plan, err := buildUpgradePlan("v1.1.0", "v1.3.0")
	if err != nil {
		t.Fatalf("buildUpgradePlan returned error: %v", err)
	}
	if plan.Supported {
		t.Fatal("expected unsupported plan for skipped minor hop")
	}
	if !strings.Contains(strings.Join(plan.Warnings, "\n"), "N-1 minor") {
		t.Fatalf("expected N-1 warning, got %v", plan.Warnings)
	}
}

func TestBuildUpgradePlanMajorHopWarns(t *testing.T) {
	plan, err := buildUpgradePlan("v1.9.0", "v2.0.0")
	if err != nil {
		t.Fatalf("buildUpgradePlan returned error: %v", err)
	}
	if !plan.Supported {
		t.Fatal("expected adjacent major hop to remain supported with warnings")
	}
	if !strings.Contains(strings.Join(plan.Warnings, "\n"), "major upgrade") {
		t.Fatalf("expected major upgrade warning, got %v", plan.Warnings)
	}
}

func TestRunUpgradePlanReadsCurrentFromEnv(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	envData := strings.Join([]string{
		"BITRIVER_LIVE_IMAGE_TAG=v1.2.3",
		"BITRIVER_VIEWER_IMAGE_TAG=v1.2.3",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(envData), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	if err := runUpgradePlan([]string{"--env-file", envPath, "--to", "v1.3.0", "--check-schema", "--current-schema", "0010", "--expected-schema", "0010"}); err != nil {
		t.Fatalf("runUpgradePlan: %v", err)
	}
	w.Close()

	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("read output: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "Current: v1.2.3") {
		t.Fatalf("expected current version in output, got %q", text)
	}
	if !strings.Contains(text, "Schema check: PASS") {
		t.Fatalf("expected schema pass in output, got %q", text)
	}
}
