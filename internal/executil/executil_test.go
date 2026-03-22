package executil

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestRunReturnsCommandErrorWithStderrTail(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := helperCommand(t, "stdout-stderr-exit", "3")
	err := runWithWriters(&stdout, &stderr, cmd[0], cmd[1:]...)
	if err == nil {
		t.Fatalf("expected error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T", err)
	}

	if cmdErr.ExitCode != 3 {
		t.Fatalf("expected exit code 3, got %d", cmdErr.ExitCode)
	}

	if got := strings.TrimSpace(cmdErr.StderrTail); got != "err" {
		t.Fatalf("unexpected stderr tail: %q", got)
	}

	if !strings.Contains(stdout.String(), "out") {
		t.Fatalf("stdout not captured: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "err") {
		t.Fatalf("stderr not captured: %q", stderr.String())
	}
}

func TestRunTrimsStderrTail(t *testing.T) {
	var stderr bytes.Buffer
	largeMessage := strings.Repeat("e", stderrTailLimit+1024)

	cmd := helperCommand(t, "stderr-exit", "2", largeMessage)
	err := runWithWriters(nil, &stderr, cmd[0], cmd[1:]...)
	if err == nil {
		t.Fatalf("expected error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T", err)
	}

	if len(cmdErr.StderrTail) != stderrTailLimit {
		t.Fatalf("unexpected tail length: %d", len(cmdErr.StderrTail))
	}
}

func TestExecutilHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_EXECUTIL_HELPER_PROCESS") != "1" {
		return
	}

	args := helperArgs(t)
	switch args[0] {
	case "stdout-stderr-exit":
		if _, err := io.WriteString(os.Stdout, "out\n"); err != nil {
			t.Fatalf("write stdout: %v", err)
		}
		if _, err := io.WriteString(os.Stderr, "err\n"); err != nil {
			t.Fatalf("write stderr: %v", err)
		}
		os.Exit(parseExitCode(t, args[1]))
	case "stderr-exit":
		if _, err := io.WriteString(os.Stderr, args[2]); err != nil {
			t.Fatalf("write stderr: %v", err)
		}
		os.Exit(parseExitCode(t, args[1]))
	default:
		t.Fatalf("unknown helper mode %q", args[0])
	}
}

func TestLookPathFriendlyError(t *testing.T) {
	_, err := LookPath("binary-that-does-not-exist-123")
	if err == nil {
		t.Fatalf("expected error")
	}

	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func helperCommand(t *testing.T, args ...string) []string {
	t.Helper()
	t.Setenv("GO_WANT_EXECUTIL_HELPER_PROCESS", "1")
	return append([]string{os.Args[0], "-test.run=^TestExecutilHelperProcess$", "--"}, args...)
}

func helperArgs(t *testing.T) []string {
	t.Helper()
	for i, arg := range os.Args {
		if arg == "--" {
			if i+1 >= len(os.Args) {
				t.Fatal("expected helper args after --")
			}
			return os.Args[i+1:]
		}
	}
	t.Fatal("expected helper delimiter --")
	return nil
}

func parseExitCode(t *testing.T, raw string) int {
	t.Helper()
	code, err := strconv.Atoi(raw)
	if err != nil {
		t.Fatalf("parse exit code %q: %v", raw, err)
	}
	return code
}
