package scripts_test

import (
	"bytes"
	"os"
	"os/exec"
	"testing"
)

// Keep the Python upgrade-pair parser/preflight tests in the existing Go gate,
// including CI runs that do not execute the Docker-dependent upgrade rehearsal.
func TestStatefulComposeUpgradePreparationPythonSuite(t *testing.T) {
	t.Parallel()

	root := offhostRepoRoot(t)
	python, prefix := offhostPython(t)
	args := append(prefix, "-m", "unittest", "scripts.stateful_compose_upgrade_test")
	cmd := exec.Command(python, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		t.Fatalf("stateful Compose upgrade preparation tests failed: %v\n%s", err, output.String())
	}
}
