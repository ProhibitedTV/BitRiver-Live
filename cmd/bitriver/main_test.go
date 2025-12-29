package main

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"bitriver-live/internal/envutil"
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

	envPath := filepath.Join(tempDir, ".env")
	var output strings.Builder
	if err := initEnvFile(envPath, tempDir, &output); err != nil {
		t.Fatalf("initEnvFile returned error: %v", err)
	}

	values, err := envutil.LoadFile(envPath, nil)
	if err != nil {
		t.Fatalf("failed to parse .env: %v", err)
	}

	if values["FOO"] != "bar" {
		t.Fatalf("expected template value to persist, got %q", values["FOO"])
	}

	required := []string{
		"BITRIVER_POSTGRES_PASSWORD",
		"BITRIVER_REDIS_PASSWORD",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD",
		"BITRIVER_LIVE_ADMIN_PASSWORD",
		"BITRIVER_SRS_TOKEN",
		"BITRIVER_OME_PASSWORD",
		"BITRIVER_OME_API_TOKEN",
		"BITRIVER_OME_ACCESS_TOKEN",
		"BITRIVER_TRANSCODER_TOKEN",
	}

	for _, key := range required {
		if values[key] == "" {
			t.Fatalf("expected %s to be generated", key)
		}
	}

	if values["BITRIVER_REDIS_PASSWORD"] != values["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] {
		t.Fatalf("redis password and chat queue password should match")
	}

	if values["BITRIVER_OME_API_TOKEN"] != values["BITRIVER_OME_ACCESS_TOKEN"] {
		t.Fatalf("OME access token should default to API token")
	}

	if !strings.Contains(output.String(), "Created .env") {
		t.Fatalf("expected creation message, got: %q", output.String())
	}
}

func TestInitEnvFilePrefersDeployTemplate(t *testing.T) {
	tempDir := t.TempDir()

	templateDir := filepath.Join(tempDir, "deploy")
	if err := os.MkdirAll(templateDir, 0o755); err != nil {
		t.Fatalf("failed to create template dir: %v", err)
	}

	deployTemplate := filepath.Join(templateDir, ".env.example")
	rootTemplate := filepath.Join(tempDir, ".env")

	if err := os.WriteFile(deployTemplate, []byte("FROM_DEPLOY=1\n"), 0o644); err != nil {
		t.Fatalf("failed to write deploy template: %v", err)
	}
	if err := os.WriteFile(rootTemplate, []byte("FROM_ROOT=1\n"), 0o644); err != nil {
		t.Fatalf("failed to write root template: %v", err)
	}

	envPath := filepath.Join(tempDir, "generated.env")
	var output strings.Builder
	if err := initEnvFile(envPath, tempDir, &output); err != nil {
		t.Fatalf("initEnvFile returned error: %v", err)
	}

	values, err := envutil.LoadFile(envPath, nil)
	if err != nil {
		t.Fatalf("failed to parse generated env: %v", err)
	}

	if values["FROM_DEPLOY"] != "1" {
		t.Fatalf("expected deploy template content, got %v", values["FROM_DEPLOY"])
	}

	if _, ok := values["FROM_ROOT"]; ok {
		t.Fatalf("expected deploy template to take precedence over root template")
	}

	if !strings.Contains(output.String(), "Created .env from .env.example") {
		t.Fatalf("expected message to mention deploy template, got: %q", output.String())
	}
}

func TestInitEnvFileFallsBackToOtherTemplates(t *testing.T) {
	tempDir := t.TempDir()

	otherTemplateDir := filepath.Join(tempDir, "configs")
	if err := os.MkdirAll(otherTemplateDir, 0o755); err != nil {
		t.Fatalf("failed to create alternate template dir: %v", err)
	}

	otherTemplate := filepath.Join(otherTemplateDir, ".env.example")
	if err := os.WriteFile(otherTemplate, []byte("FROM_OTHER=1\n"), 0o644); err != nil {
		t.Fatalf("failed to write alternate template: %v", err)
	}

	rootEnv := filepath.Join(tempDir, ".env")
	if err := os.WriteFile(rootEnv, []byte("FROM_ROOT=1\n"), 0o644); err != nil {
		t.Fatalf("failed to write root env template: %v", err)
	}

	envPath := filepath.Join(tempDir, "generated.env")
	if err := initEnvFile(envPath, tempDir, io.Discard); err != nil {
		t.Fatalf("initEnvFile returned error: %v", err)
	}

	values, err := envutil.LoadFile(envPath, nil)
	if err != nil {
		t.Fatalf("failed to parse generated env: %v", err)
	}

	if values["FROM_OTHER"] != "1" {
		t.Fatalf("expected alternate template content, got %v", values["FROM_OTHER"])
	}

	if _, ok := values["FROM_ROOT"]; ok {
		t.Fatalf("expected alternate template to take precedence over root env")
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
	if err := initEnvFile(envPath, tempDir, &output); err != nil {
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

func TestBuildComposeArgs(t *testing.T) {
	tests := []struct {
		name    string
		action  string
		file    string
		want    []string
		wantErr bool
	}{
		{
			name:   "up action",
			action: "up",
			file:   "deploy/docker-compose.yml",
			want: []string{
				"compose", "-f", "deploy/docker-compose.yml", "up", "-d", "--remove-orphans",
			},
		},
		{
			name:   "down action",
			action: "down",
			file:   "custom.yml",
			want: []string{
				"compose", "-f", "custom.yml", "down", "--remove-orphans",
			},
		},
		{
			name:    "invalid action",
			action:  "restart",
			file:    "deploy/docker-compose.yml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildComposeArgs(tt.action, tt.file)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected args: %v", got)
			}
		})
	}
}

func TestParseComposeFlags(t *testing.T) {
	defaultFile := filepath.Join("deploy", "docker-compose.yml")
	tests := []struct {
		name      string
		args      []string
		want      composeConfig
		wantError string
	}{
		{
			name: "up default file",
			args: []string{"up"},
			want: composeConfig{action: "up", composeFile: defaultFile},
		},
		{
			name: "down custom file before action",
			args: []string{"--file", "custom.yml", "down"},
			want: composeConfig{action: "down", composeFile: "custom.yml"},
		},
		{
			name: "up custom file after action",
			args: []string{"up", "--file", "custom.yml"},
			want: composeConfig{action: "up", composeFile: "custom.yml"},
		},
		{
			name: "up custom file via equals",
			args: []string{"--file=custom.yml", "up"},
			want: composeConfig{action: "up", composeFile: "custom.yml"},
		},
		{
			name:      "missing action",
			args:      []string{},
			wantError: "compose action is required",
		},
		{
			name:      "unknown flag",
			args:      []string{"--unknown"},
			wantError: "unknown flag",
		},
		{
			name:      "extra positional",
			args:      []string{"up", "down"},
			wantError: "unexpected argument",
		},
		{
			name:      "missing file value",
			args:      []string{"--file"},
			wantError: "requires a value",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseComposeFlags(tt.args, defaultFile)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("unexpected config: %+v", got)
			}
		})
	}
}
