package scripts

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOffhostBackupQualificationPythonSuite(t *testing.T) {
	t.Parallel()

	repoRoot := offhostRepoRoot(t)
	name, prefix := offhostPython(t)
	args := append(prefix, "-m", "unittest", "scripts.qualify_offhost_backup_test")
	cmd := exec.Command(name, args...)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("off-host backup qualification tests failed: %v\n%s", err, output.String())
	}
}

func offhostRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if filepath.Base(cwd) == "scripts" {
		return filepath.Dir(cwd)
	}
	if _, err := os.Stat(filepath.Join(cwd, "scripts", "qualify_offhost_backup.py")); err == nil {
		return cwd
	}
	t.Fatalf("could not locate repository root from %s", cwd)
	return ""
}

func offhostPython(t *testing.T) (string, []string) {
	t.Helper()
	candidates := []struct {
		name   string
		prefix []string
	}{
		{name: "python3"},
		{name: "python"},
	}
	if runtime.GOOS == "windows" {
		candidates = append([]struct {
			name   string
			prefix []string
		}{{name: "py", prefix: []string{"-3"}}}, candidates...)
	}

	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate.name)
		if err == nil {
			return path, candidate.prefix
		}
	}
	t.Fatalf("Python 3 is required for repository contract checks; tried %s", formatPythonCandidates(candidates))
	return "", nil
}

func formatPythonCandidates(candidates []struct {
	name   string
	prefix []string
}) string {
	var result string
	for index, candidate := range candidates {
		if index > 0 {
			result += ", "
		}
		result += candidate.name
		for _, argument := range candidate.prefix {
			result += " " + argument
		}
	}
	return fmt.Sprintf("[%s]", result)
}
