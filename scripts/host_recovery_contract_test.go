package scripts_test

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// Exercise the standalone packaged-host recovery helper's Python regressions
// in every supported Go scripts test gate, without altering CI workflows.
func TestHostRecoveryPythonSuite(t *testing.T) {
	t.Parallel()

	root := offhostRepoRoot(t)
	python, prefix := offhostPython(t)
	args := append(prefix, "-m", "unittest", "scripts.host_recovery_test")
	cmd := exec.Command(python, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("packaged-host recovery Python suite failed: %v\n%s", err, output.String())
	}
}
