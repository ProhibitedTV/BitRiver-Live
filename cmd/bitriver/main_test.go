package main

import (
	"bytes"
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
