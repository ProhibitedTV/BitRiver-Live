package platform

import (
	"errors"
	"os/exec"
)

// FindPythonExecutable attempts to locate a Python interpreter in PATH.
// It prefers `python` and falls back to the Windows launcher `py` when available.
func FindPythonExecutable() (string, error) {
	candidates := []string{"python", "py"}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}

	return "", errors.New("python executable not found; install python or ensure it is in PATH")
}
