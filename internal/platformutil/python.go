package platformutil

import (
	"errors"
	"os/exec"
	"runtime"
)

// Command represents an executable plus its fixed arguments.
type Command struct {
	Executable string
	Args       []string
}

// LookPathFunc abstracts exec.LookPath for testing.
type LookPathFunc func(file string) (string, error)

// FindPythonCommands returns one or more candidate commands for invoking Python.
// It prefers `python` when present and, on Windows, falls back to the launcher
// (`py -3`, then `py`).
func FindPythonCommands() ([]Command, error) {
	return findPythonCommands(exec.LookPath, runtime.GOOS)
}

func findPythonCommands(lookPath LookPathFunc, goos string) ([]Command, error) {
	if path, err := lookPath("python"); err == nil {
		return []Command{{Executable: path}}, nil
	}

	if goos == "windows" {
		if launcherPath, err := lookPath("py"); err == nil {
			return []Command{
				{Executable: launcherPath, Args: []string{"-3"}},
				{Executable: launcherPath},
			}, nil
		}
	}

	return nil, errors.New("python executable not found; install python or ensure it is in PATH")
}
