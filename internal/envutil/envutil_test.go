package envutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileMissingReturnsBase(t *testing.T) {
	base := map[string]string{"EXISTING": "1"}

	values, err := LoadFile(filepath.Join(t.TempDir(), ".env"), base)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}

	if values["EXISTING"] != "1" {
		t.Fatalf("expected base value preserved, got %q", values["EXISTING"])
	}

	if _, ok := values["NEW"]; ok {
		t.Fatalf("unexpected value NEW present")
	}

	if base["EXISTING"] != "1" {
		t.Fatalf("base map was mutated")
	}
}

func TestLoadFileParsesValues(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	content := "" +
		"# comment\n" +
		"\n" +
		"KEY=value\n" +
		"QUOTED=\"quoted value with spaces\"\n" +
		"TRAILING=trim   \n"

	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	base := map[string]string{"KEY": "base"}

	values, err := LoadFile(envPath, base)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}

	if values["KEY"] != "value" {
		t.Fatalf("expected KEY from file, got %q", values["KEY"])
	}

	if values["QUOTED"] != "quoted value with spaces" {
		t.Fatalf("expected QUOTED parsed, got %q", values["QUOTED"])
	}

	if values["TRAILING"] != "trim" {
		t.Fatalf("expected TRAILING trimmed, got %q", values["TRAILING"])
	}
}

func TestFromEnviron(t *testing.T) {
	environ := []string{"KEY=value", "NOEQUAL", "ANOTHER=two"}

	values := FromEnviron(environ)

	if values["KEY"] != "value" {
		t.Fatalf("expected KEY, got %q", values["KEY"])
	}

	if values["ANOTHER"] != "two" {
		t.Fatalf("expected ANOTHER, got %q", values["ANOTHER"])
	}

	if _, ok := values["NOEQUAL"]; ok {
		t.Fatalf("unexpected key for entry without equals")
	}
}

func TestFirstExistingPath(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "first")
	second := filepath.Join(tempDir, "second")

	if err := os.WriteFile(second, []byte("second"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	path, err := FirstExistingPath([]string{first, second})
	if err != nil {
		t.Fatalf("FirstExistingPath returned error: %v", err)
	}

	if path != second {
		t.Fatalf("expected second to be selected, got %s", path)
	}
}
