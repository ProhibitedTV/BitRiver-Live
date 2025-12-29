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
// It prefers `python3`, then `python`, and on Windows falls back to the launcher
// (`py -3`, then `py`).
func FindPythonCommands() ([]Command, error) {
	return findPythonCommands(exec.LookPath, runtime.GOOS)
}

func findPythonCommands(lookPath LookPathFunc, goos string) ([]Command, error) {
	var commands []Command

	addIfPresent := func(binary string, args ...string) {
		if path, err := lookPath(binary); err == nil {
			commands = append(commands, Command{Executable: path, Args: args})
		}
	}

	addIfPresent("python3")
	addIfPresent("python")

	if goos == "windows" {
		addIfPresent("py", "-3")
		addIfPresent("py")
	}

	if len(commands) == 0 {
		return nil, errors.New("python executable not found; install python or ensure it is in PATH")
	}

	return commands, nil
}
