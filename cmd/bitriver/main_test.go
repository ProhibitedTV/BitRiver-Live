package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionOutputIncludesVersionLabel(t *testing.T) {
var buf bytes.Buffer
Version = "test-version"
Commit = "test-commit"
Date = "2024-01-01"

	printVersionInfo(&buf)

	output := buf.String()
	if !strings.Contains(output, "Version:") {
		t.Fatalf("expected output to contain Version:, got %q", output)
	}
	if !strings.Contains(output, "Commit:") {
		t.Fatalf("expected output to contain Commit:, got %q", output)
	}
	if !strings.Contains(output, "Date:") {
		t.Fatalf("expected output to contain Date:, got %q", output)
	}
}

func TestEnvInitWritesGeneratedValues(t *testing.T) {
envPath := filepath.Join(t.TempDir(), ".env")
examplePath := defaultExampleEnv()

	if err := runEnvInit([]string{"--env-file", envPath, "--example", examplePath}); err != nil {
		t.Fatalf("env init failed: %v", err)
	}

	values, err := readEnvFile(envPath)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}

	if values["BITRIVER_POSTGRES_PASSWORD"] == "P0stgres-Example!" || values["BITRIVER_POSTGRES_PASSWORD"] == "" {
		t.Fatalf("expected postgres password to be generated, got %q", values["BITRIVER_POSTGRES_PASSWORD"])
	}

	if values["BITRIVER_LIVE_ADMIN_EMAIL"] == "" || values["BITRIVER_LIVE_ADMIN_EMAIL"] == "admin@stream.example.com" {
		t.Fatalf("expected admin email to be set, got %q", values["BITRIVER_LIVE_ADMIN_EMAIL"])
	}
}

func TestEnvValidateFailsForMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.env")
	if err := runEnvValidate([]string{"--env-file", missing}); err == nil {
		t.Fatal("expected validation to fail for missing env file")
	}
}

func TestEnvValidateBlocksPlaceholders(t *testing.T) {
	root := t.TempDir()
	envPath := filepath.Join(root, ".env")
	content := strings.Join([]string{
		"BITRIVER_LIVE_IMAGE_TAG=v1.2.3",
		"BITRIVER_VIEWER_IMAGE_TAG=v1.2.3",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG=v1.2.3",
		"BITRIVER_TRANSCODER_IMAGE_TAG=v1.2.3",
		"BITRIVER_SRS_IMAGE_TAG=v5.0.185",
		"BITRIVER_OME_IMAGE_TAG=0.16.0",
		"BITRIVER_POSTGRES_USER=brlive_app",
		"BITRIVER_POSTGRES_PASSWORD=P0stgres-Example!",
		"BITRIVER_REDIS_PASSWORD=secret",
		"BITRIVER_OME_API=http://ome:8081",
		"BITRIVER_OME_BIND=0.0.0.0",
		"BITRIVER_OME_IP=0.0.0.0",
		"BITRIVER_OME_SERVER_PORT=9000",
		"BITRIVER_OME_SERVER_TLS_PORT=9443",
		"BITRIVER_LIVE_ADMIN_EMAIL=admin@bitriver.local",
		"BITRIVER_LIVE_ADMIN_PASSWORD=password",
		"BITRIVER_LIVE_SESSION_TTL=168h",
		"BITRIVER_LIVE_ALLOW_SELF_SIGNUP=false",
		"BITRIVER_SRS_TOKEN=token",
		"BITRIVER_OME_USERNAME=ome-operator",
		"BITRIVER_OME_PASSWORD=ome-password",
		"BITRIVER_OME_API_TOKEN=api-token",
		"BITRIVER_OME_ACCESS_TOKEN=access-token",
		"BITRIVER_TRANSCODER_TOKEN=transcoder-token",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD=secret",
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL=http://localhost:9001/hls",
		"NEXT_PUBLIC_VIEWER_URL=http://localhost:8080/viewer",
		"BITRIVER_LIVE_MODE=development",
	}, "\n") + "\n"

	if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	err := runEnvValidate([]string{"--env-file", envPath})
	if err == nil {
		t.Fatal("expected validation to fail due to placeholder password")
	}
}

func TestComposeRejectsUnknownSubcommand(t *testing.T) {
	if err := runCompose([]string{"noop"}); err == nil {
		t.Fatal("expected compose to error on unknown subcommand")
	}
}
