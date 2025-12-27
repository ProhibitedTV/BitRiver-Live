package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestValidateEnvFileReportsMissing(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	values := allRequiredEnvValues()
	delete(values, "BITRIVER_OME_BIND")
	delete(values, "BITRIVER_LIVE_IMAGE_TAG")
	writeEnvFile(t, envPath, values)

	var output strings.Builder
	err := validateEnvFile(envPath, nil, &output)
	if err == nil {
		t.Fatalf("expected validation to fail")
	}

	if !strings.Contains(output.String(), "BITRIVER_OME_BIND") || !strings.Contains(output.String(), "BITRIVER_LIVE_IMAGE_TAG") {
		t.Fatalf("missing variables were not reported: %s", output.String())
	}
}

func TestValidateEnvFileMergesEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	values := allRequiredEnvValues()
	delete(values, "BITRIVER_POSTGRES_PASSWORD")
	delete(values, "BITRIVER_LIVE_IMAGE_TAG")
	writeEnvFile(t, envPath, values)

	env := []string{
		"BITRIVER_POSTGRES_PASSWORD=from-environment",
		"BITRIVER_LIVE_IMAGE_TAG=v1.2.3",
	}

	var output strings.Builder
	if err := validateEnvFile(envPath, env, &output); err != nil {
		t.Fatalf("expected validation to succeed: %v", err)
	}

	if !strings.Contains(output.String(), "looks ready") {
		t.Fatalf("expected success message, got: %s", output.String())
	}
}

func TestValidateEnvFileSuccess(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	writeEnvFile(t, envPath, allRequiredEnvValues())

	var output strings.Builder
	if err := validateEnvFile(envPath, nil, &output); err != nil {
		t.Fatalf("expected validation to pass: %v", err)
	}

	if !strings.Contains(output.String(), "looks ready") {
		t.Fatalf("expected ready message, got: %s", output.String())
	}
}

func allRequiredEnvValues() map[string]string {
	values := make(map[string]string)

	for _, key := range requiredEnvKeys {
		values[key] = "value"
	}
	for _, key := range requiredImageTagKeys {
		values[key] = "value"
	}

	return values
}

func writeEnvFile(t *testing.T, path string, values map[string]string) {
	t.Helper()

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var lines []string
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", key, values[key]))
	}

	content := strings.Join(lines, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}
}
