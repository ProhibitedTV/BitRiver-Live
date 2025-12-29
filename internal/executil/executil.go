package executil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

const stderrTailLimit = 8 * 1024

type CommandError struct {
	Command    []string
	ExitCode   int
	StderrTail string
}

func (e *CommandError) Error() string {
	command := strings.Join(e.Command, " ")
	if e.StderrTail == "" {
		return fmt.Sprintf("%s exited with code %d", command, e.ExitCode)
	}

	tail := strings.TrimSpace(e.StderrTail)
	return fmt.Sprintf("%s exited with code %d: %s", command, e.ExitCode, tail)
}

// Run executes a command with live stdout/stderr streaming.
func Run(name string, args ...string) error {
	return runWithWriters(os.Stdout, os.Stderr, name, args...)
}

func runWithWriters(stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout

	stderrTail := &tailBuffer{limit: stderrTailLimit}
	switch {
	case stderr == nil:
		cmd.Stderr = stderrTail
	default:
		cmd.Stderr = io.MultiWriter(stderr, stderrTail)
	}

	if err := cmd.Run(); err != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}

		return &CommandError{
			Command:    cmd.Args,
			ExitCode:   exitCode,
			StderrTail: stderrTail.String(),
		}
	}

	return nil
}

// LookPath resolves a binary on PATH with a user-friendly error.
func LookPath(file string) (string, error) {
	path, err := exec.LookPath(file)
	if err == nil {
		return path, nil
	}

	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("%s not found in PATH (PATH=%s)", file, os.Getenv("PATH"))
	}

	return "", fmt.Errorf("failed to find %s in PATH: %w", file, err)
}

type tailBuffer struct {
	buf   []byte
	limit int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.limit {
		t.buf = t.buf[len(t.buf)-t.limit:]
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	return string(t.buf)
}
