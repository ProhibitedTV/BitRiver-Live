package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitEnvFileCreatesFromTemplate(t *testing.T) {
	tempDir := t.TempDir()

	templateDir := filepath.Join(tempDir, "deploy")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("failed to create template dir: %v", err)
	}

	templatePath := filepath.Join(templateDir, ".env.example")
	templateContent := "FOO=bar\n"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0o644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	var output strings.Builder
	if err := initEnvFile(tempDir, &output); err != nil {
		t.Fatalf("initEnvFile returned error: %v", err)
	}

	envPath := filepath.Join(tempDir, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	if string(data) != templateContent {
		t.Fatalf("unexpected .env content: %q", string(data))
	}

	if !strings.Contains(output.String(), "Created .env") {
		t.Fatalf("expected creation message, got: %q", output.String())
	}
}

func TestInitEnvFileSkipsExisting(t *testing.T) {
	tempDir := t.TempDir()

	envPath := filepath.Join(tempDir, ".env")
	original := "EXISTING=1\n"
	if err := os.WriteFile(envPath, []byte(original), 0o644); err != nil {
		t.Fatalf("failed to seed .env: %v", err)
	}

	templateDir := filepath.Join(tempDir, "deploy")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("failed to create template dir: %v", err)
	}

	templatePath := filepath.Join(templateDir, ".env.example")
	if err := os.WriteFile(templatePath, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	var output strings.Builder
	if err := initEnvFile(tempDir, &output); err != nil {
		t.Fatalf("initEnvFile returned error: %v", err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}

	if string(data) != original {
		t.Fatalf(".env was modified: %q", string(data))
	}

	if !strings.Contains(output.String(), "already exists") {
		t.Fatalf("expected already exists message, got: %q", output.String())
	}
}
