package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSystemdStagesArtifacts(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	dataDir := filepath.Join(root, "data")

	serverBin := filepath.Join(root, "bitriver-live")
	bootstrapBin := filepath.Join(root, "bootstrap-admin")
	if err := os.WriteFile(serverBin, []byte("#!/bin/sh\necho server\n"), 0o755); err != nil {
		t.Fatalf("write server: %v", err)
	}
	if err := os.WriteFile(bootstrapBin, []byte("#!/bin/sh\necho bootstrap\n"), 0o755); err != nil {
		t.Fatalf("write bootstrap: %v", err)
	}

	args := []string{
		"--install-dir", installDir,
		"--data-dir", dataDir,
		"--service-user", "bitriver",
		"--addr", ":8081",
		"--server-binary", serverBin,
		"--bootstrap-admin-binary", bootstrapBin,
		"--build-from-source=false",
		"--enable-logs=true",
		"--log-dir", filepath.Join(dataDir, "logs"),
	}

	if err := runInstallSystemd(args); err != nil {
		t.Fatalf("install systemd failed: %v", err)
	}

	logDir := filepath.Join(dataDir, "logs")
	if _, err := os.Stat(logDir); err != nil {
		t.Fatalf("log directory not created: %v", err)
	}

	envPath := filepath.Join(installDir, ".env")
	envContent, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	if !strings.Contains(string(envContent), "BITRIVER_LIVE_ADDR=:8081") {
		t.Fatalf("env missing addr: %s", envContent)
	}

	unitPath := filepath.Join(installDir, "bitriver-live.service")
	unitContent, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("read unit: %v", err)
	}
	if !strings.Contains(string(unitContent), "ExecStart="+filepath.Join(installDir, "bitriver-live")) {
		t.Fatalf("unit missing ExecStart: %s", unitContent)
	}
	if !strings.Contains(string(unitContent), filepath.Join(dataDir, "logs", "server.log")) {
		t.Fatalf("unit missing log redirect: %s", unitContent)
	}
}

func TestInstallSystemdPrintsBootstrapRecoveryGuidance(t *testing.T) {
	root := t.TempDir()
	installDir := filepath.Join(root, "install")
	dataDir := filepath.Join(root, "data")

	serverBin := filepath.Join(root, "bitriver-live")
	bootstrapBin := filepath.Join(root, "bootstrap-admin")
	if err := os.WriteFile(serverBin, []byte("#!/bin/sh\necho server\n"), 0o755); err != nil {
		t.Fatalf("write server: %v", err)
	}
	if err := os.WriteFile(bootstrapBin, []byte("#!/bin/sh\necho bootstrap\n"), 0o755); err != nil {
		t.Fatalf("write bootstrap: %v", err)
	}

	args := []string{
		"--install-dir", installDir,
		"--data-dir", dataDir,
		"--service-user", "bitriver",
		"--addr", ":8081",
		"--viewer-url", "https://stream.example.com/viewer",
		"--bootstrap-admin-email", "admin@example.com",
		"--bootstrap-admin-password", "SeedPassword123",
		"--server-binary", serverBin,
		"--bootstrap-admin-binary", bootstrapBin,
		"--build-from-source=false",
	}

	output := captureStdout(t, func() {
		if err := runInstallSystemd(args); err != nil {
			t.Fatalf("install systemd failed: %v", err)
		}
	})

	checks := []string{
		"Bootstrap admin sign-in URL: https://stream.example.com/admin",
		"Bootstrap admin email: admin@example.com",
		"Bootstrap credentials are stored in " + filepath.Join(installDir, ".env"),
		"env admin",
		"historical seed value",
	}
	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got %q", check, output)
		}
	}
}

func TestRenderWindowsServiceScript(t *testing.T) {
	cfg := windowsServiceConfig{
		ServiceName: "BitRiverLive",
		DisplayName: "BitRiver Live",
		InstallDir:  `C:\\BitRiver`,
		EnvPath:     `C:\\BitRiver\\.env`,
	}

	script := renderWindowsServiceScript(cfg)
	if !strings.Contains(script, "New-Service -Name $serviceName") {
		t.Fatalf("expected service creation command in script: %s", script)
	}
	if !strings.Contains(script, "run-bitriver-live.ps1") {
		t.Fatalf("expected wrapper script reference: %s", script)
	}
}

func TestGenerateStrongPasswordHasRequiredClasses(t *testing.T) {
	password, err := generateStrongPassword()
	if err != nil {
		t.Fatalf("generateStrongPassword error: %v", err)
	}

	if len(password) < 16 {
		t.Fatalf("password too short: %d", len(password))
	}

	if !passwordHasClasses([]byte(password)) {
		t.Fatalf("password missing required character classes: %s", password)
	}
}
