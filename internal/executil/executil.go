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

// CommandError wraps failures from external commands with exit code and stderr tail information.
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

// RunOption configures the behaviour of Run.
type RunOption func(*runConfig)

type runConfig struct {
	Stdout        io.Writer
	Stderr        io.Writer
	CommandOutput io.Writer
	PrintCommand  bool
}

// WithStdout overrides the destination for stdout streaming.
func WithStdout(w io.Writer) RunOption {
	return func(cfg *runConfig) {
		cfg.Stdout = w
	}
}

// WithStderr overrides the destination for stderr streaming.
func WithStderr(w io.Writer) RunOption {
	return func(cfg *runConfig) {
		cfg.Stderr = w
	}
}

// WithCommandOutput overrides where the executed command line is printed.
func WithCommandOutput(w io.Writer) RunOption {
	return func(cfg *runConfig) {
		cfg.CommandOutput = w
	}
}

// WithPrintCommand enables printing the command before execution.
func WithPrintCommand() RunOption {
	return func(cfg *runConfig) {
		cfg.PrintCommand = true
	}
}

// Run executes an external command with streaming stdout/stderr and rich errors.
func Run(name string, args []string, opts ...RunOption) error {
	cfg := runConfig{Stdout: os.Stdout, Stderr: os.Stderr, CommandOutput: os.Stdout}
	for _, opt := range opts {
		opt(&cfg)
	}

	cmd := exec.Command(name, args...)
	if cfg.PrintCommand {
		fmt.Fprintf(cfg.CommandOutput, "Executing: %s\n", strings.Join(cmd.Args, " "))
	}

	cmd.Stdout = cfg.Stdout

	stderrTail := &tailBuffer{limit: stderrTailLimit}
	switch {
	case cfg.Stderr == nil:
		cmd.Stderr = stderrTail
	default:
		cmd.Stderr = io.MultiWriter(cfg.Stderr, stderrTail)
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
