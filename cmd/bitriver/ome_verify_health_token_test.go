package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOMEVerifyHealthTokenPassesWithCanonicalHealthcheckToken(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	configPath := filepath.Join(tempDir, "Server.generated.xml")

	envContents := strings.Join([]string{
		"BITRIVER_OME_API_TOKEN=api-token",
		"BITRIVER_OME_HEALTHCHECK_TOKEN=healthcheck-token",
	}, "\n") + "\n"
	if err := os.WriteFile(envPath, []byte(envContents), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	config := `<Server><Managers><API><AccessToken>healthcheck-token</AccessToken></API></Managers></Server>`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := runOMEVerifyHealthToken([]string{"--env-file", envPath, "--config", configPath}); err != nil {
		t.Fatalf("expected verification success, got %v", err)
	}
}

func TestOMEVerifyHealthTokenFailsOnMismatch(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	configPath := filepath.Join(tempDir, "Server.generated.xml")

	if err := os.WriteFile(envPath, []byte("BITRIVER_OME_API_TOKEN=expected-token\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	config := `<Server><Managers><API><AccessToken>rendered-token</AccessToken></API></Managers></Server>`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := runOMEVerifyHealthToken([]string{"--env-file", envPath, "--config", configPath})
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	if !strings.Contains(err.Error(), "rendered and runtime tokens differ") {
		t.Fatalf("expected mismatch detail, got %v", err)
	}
	if !strings.Contains(err.Error(), "BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_API_TOKEN") {
		t.Fatalf("expected canonical precedence guidance, got %v", err)
	}
}
