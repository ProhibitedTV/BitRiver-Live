package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureUpgradePlanStdout(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })

	fn()
	_ = w.Close()

	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("read output: %v", err)
	}
	return out.String()
}

func TestDetectRunningComposeImageTagsParsesTags(t *testing.T) {
	origRunner := upgradePlanComposePSRunner
	upgradePlanComposePSRunner = func(_, _ string) ([]byte, error) {
		return []byte(`[
			{"Service":"bitriver-live","Image":"ghcr.io/bitriver-live/bitriver-live:v1.2.3"},
			{"Service":"viewer","Image":"ghcr.io/bitriver-live/bitriver-viewer:v1.2.3"}
		]`), nil
	}
	t.Cleanup(func() { upgradePlanComposePSRunner = origRunner })

	tags, err := detectRunningComposeImageTags("compose.yml", ".env")
	if err != nil {
		t.Fatalf("detectRunningComposeImageTags returned error: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}
	if tags[0].Tag != "v1.2.3" || tags[0].Source != "compose-ps" {
		t.Fatalf("unexpected first tag: %#v", tags[0])
	}
}

func TestRunUpgradePlanFallsBackToEnvTags(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	envData := "BITRIVER_LIVE_IMAGE_TAG=v1.2.3\nBITRIVER_VIEWER_IMAGE_TAG=v1.2.3\n"
	if err := os.WriteFile(envPath, []byte(envData), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	origRunner := upgradePlanComposePSRunner
	upgradePlanComposePSRunner = func(_, _ string) ([]byte, error) {
		return nil, errors.New("docker unavailable")
	}
	t.Cleanup(func() { upgradePlanComposePSRunner = origRunner })

	output := captureUpgradePlanStdout(t, func() {
		if err := runUpgradePlan([]string{"--env-file", envPath, "--target", "v1.3.0"}); err != nil {
			t.Fatalf("runUpgradePlan returned error: %v", err)
		}
	})

	if !strings.Contains(output, "source=env-file") {
		t.Fatalf("expected env fallback source in output, got %q", output)
	}
	if !strings.Contains(output, "WARN:") {
		t.Fatalf("expected warning output, got %q", output)
	}
	if !strings.Contains(output, "Operator checklist:") {
		t.Fatalf("expected checklist output, got %q", output)
	}
	if !strings.Contains(output, "migrations --mode plan") {
		t.Fatalf("expected read-only migration preflight guidance, got %q", output)
	}
}

func TestRunUpgradePlanWarnsWhenCurrentVersionUnknown(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(envPath, []byte("# no image tags\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	origRunner := upgradePlanComposePSRunner
	upgradePlanComposePSRunner = func(_, _ string) ([]byte, error) {
		return nil, errors.New("compose not running")
	}
	t.Cleanup(func() { upgradePlanComposePSRunner = origRunner })

	output := captureUpgradePlanStdout(t, func() {
		if err := runUpgradePlan([]string{"--env-file", envPath, "--target", "v1.3.0"}); err != nil {
			t.Fatalf("runUpgradePlan returned error: %v", err)
		}
	})

	if !strings.Contains(output, "Current image tags (best-effort):") {
		t.Fatalf("expected current image tags section, got %q", output)
	}
	if !strings.Contains(output, "- none detected") {
		t.Fatalf("expected none detected message, got %q", output)
	}
	if !strings.Contains(output, "current version could not be determined") {
		t.Fatalf("expected current-version warning guidance, got %q", output)
	}
	if !strings.Contains(output, "Rollback caveats:") {
		t.Fatalf("expected rollback caveats section, got %q", output)
	}
}
