package executil

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestRunReturnsCommandErrorWithStderrTail(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := Run("sh", []string{"-c", "echo out; echo err >&2; exit 3"}, WithStdout(&stdout), WithStderr(&stderr))
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
	command := fmt.Sprintf("head -c %d /dev/zero 1>&2; exit 2", len(largeMessage))
	err := Run("sh", []string{"-c", command}, WithStderr(&stderr))
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

func TestLookPathFriendlyError(t *testing.T) {
	_, err := LookPath("binary-that-does-not-exist-123")
	if err == nil {
		t.Fatalf("expected error")
	}

	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
