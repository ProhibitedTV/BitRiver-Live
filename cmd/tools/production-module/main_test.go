package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareProductionModuleDropsAllThirdPartyReplacements(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "go.mod")
	output := filepath.Join(dir, "go.production.mod")
	contents := `module example.com/test

go 1.26.0

require (
	example.com/local v1.0.0
	example.com/remote v1.0.0
)

replace example.com/local => ./third_party/example.com/local
replace example.com/remote v1.0.0 => example.com/remote v1.0.1
`
	mustWriteFile(t, source, contents)
	mustWriteFile(t, filepath.Join(dir, "go.sum"), "example checksum fixture\n")

	if err := prepareProductionModule(source, output); err != nil {
		t.Fatalf("prepare production module: %v", err)
	}

	production := mustReadFile(t, output)
	if strings.Contains(production, "third_party") || strings.Contains(production, "replace example.com/local") {
		t.Fatalf("production module retained local replacement:\n%s", production)
	}
	if !strings.Contains(production, "example.com/remote v1.0.0 => example.com/remote v1.0.1") {
		t.Fatalf("production module removed versioned upstream replacement:\n%s", production)
	}
	if got := mustReadFile(t, filepath.Join(dir, "go.production.sum")); got != "example checksum fixture\n" {
		t.Fatalf("production sum mismatch: %q", got)
	}
	if got := mustReadFile(t, source); got != contents {
		t.Fatal("source go.mod was modified")
	}
}

func TestPrepareProductionModuleRejectsLocalReplacementOutsideThirdParty(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "go.mod")
	output := filepath.Join(dir, "go.production.mod")
	mustWriteFile(t, source, "module example.com/test\n\ngo 1.26.0\n\nreplace example.com/local => ./local\n")
	mustWriteFile(t, filepath.Join(dir, "go.sum"), "")

	err := prepareProductionModule(source, output)
	if err == nil || !strings.Contains(err.Error(), "outside third_party") {
		t.Fatalf("expected outside-third_party error, got %v", err)
	}
	if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
		t.Fatalf("failed preparation should not leave output, stat error=%v", statErr)
	}
}

func TestPrepareProductionModuleRejectsSourceOverwrite(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "go.mod")
	mustWriteFile(t, source, "module example.com/test\n\ngo 1.26.0\n")

	err := prepareProductionModule(source, source)
	if err == nil || !strings.Contains(err.Error(), "must not overwrite") {
		t.Fatalf("expected overwrite error, got %v", err)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}
