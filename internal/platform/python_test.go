package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPythonExecutableUsesPath(t *testing.T) {
	tempDir := t.TempDir()
	fakePython := filepath.Join(tempDir, "python")
	if err := os.WriteFile(fakePython, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("failed to write fake python: %v", err)
	}

	t.Setenv("PATH", tempDir)

	path, err := FindPythonExecutable()
	if err != nil {
		t.Fatalf("FindPythonExecutable returned error: %v", err)
	}

	if path != fakePython {
		t.Fatalf("unexpected python path: %s", path)
	}
}

func TestFindPythonExecutableFailsWhenMissing(t *testing.T) {
	t.Setenv("PATH", "")

	if _, err := FindPythonExecutable(); err == nil {
		t.Fatal("expected error when python is not in PATH")
	}
}
