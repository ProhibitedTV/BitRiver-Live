package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"bitriver-live/internal/executil"
)

type stubRunner struct {
	calls []runnerCall
	err   error
}

type runnerCall struct {
	name string
	args []string
}

func (r *stubRunner) Run(name string, args []string, _ ...executil.RunOption) error {
	copiedArgs := append([]string(nil), args...)
	r.calls = append(r.calls, runnerCall{name: name, args: copiedArgs})
	if r.err != nil {
		return r.err
	}
	return nil
}

func TestRunDoctorChecksSuccess(t *testing.T) {
	runner := &stubRunner{}
	deps := doctorDeps{
		lookPath: func(binary string) (string, error) {
			if binary != "docker" {
				t.Fatalf("unexpected binary lookup: %s", binary)
			}
			return "/usr/bin/docker", nil
		},
		runner: runner,
		getwd:  func() (string, error) { return "/workspace/project", nil },
	}

	result := runDoctorChecks(io.Discard, deps)

	if result.dockerPath != "/usr/bin/docker" {
		t.Fatalf("unexpected docker path: %s", result.dockerPath)
	}
	if result.dockerErr != nil {
		t.Fatalf("expected dockerErr to be nil: %v", result.dockerErr)
	}
	if result.dockerVersionErr != nil {
		t.Fatalf("expected dockerVersionErr to be nil: %v", result.dockerVersionErr)
	}
	if result.composeVersionErr != nil {
		t.Fatalf("expected composeVersionErr to be nil: %v", result.composeVersionErr)
	}
	if result.workDir != "/workspace/project" {
		t.Fatalf("unexpected workdir: %s", result.workDir)
	}

	expectedCalls := []runnerCall{
		{name: "/usr/bin/docker", args: []string{"version"}},
		{name: "/usr/bin/docker", args: []string{"compose", "version"}},
	}
	if len(runner.calls) != len(expectedCalls) {
		t.Fatalf("unexpected number of calls: %d", len(runner.calls))
	}
	if !reflect.DeepEqual(runner.calls[0], expectedCalls[0]) {
		t.Fatalf("unexpected docker version call: %+v", runner.calls[0])
	}
	if !reflect.DeepEqual(runner.calls[1], expectedCalls[1]) {
		t.Fatalf("unexpected compose version call: %+v", runner.calls[1])
	}
}

func TestRunDoctorChecksHandlesMissingDocker(t *testing.T) {
	expectedErr := errors.New("not found")
	runner := &stubRunner{}
	deps := doctorDeps{
		lookPath: func(string) (string, error) { return "", expectedErr },
		runner:   runner,
		getwd:    func() (string, error) { return "", errors.New("workdir failed") },
	}

	result := runDoctorChecks(io.Discard, deps)

	if result.dockerErr != expectedErr {
		t.Fatalf("expected dockerErr to propagate lookup error")
	}
	if result.dockerVersionErr != expectedErr {
		t.Fatalf("expected dockerVersionErr to propagate lookup error")
	}
	if result.composeVersionErr != expectedErr {
		t.Fatalf("expected composeVersionErr to propagate lookup error")
	}
	if result.workDirErr == nil || !strings.Contains(result.workDirErr.Error(), "workdir failed") {
		t.Fatalf("expected workdir error, got %v", result.workDirErr)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("runner should not be invoked when docker lookup fails")
	}
}

func TestPrintDoctorResult(t *testing.T) {
	var buf bytes.Buffer
	result := doctorResult{
		dockerPath:        "/usr/bin/docker",
		dockerErr:         nil,
		dockerVersionOut:  "daemon not running",
		dockerVersionErr:  errors.New("version failed"),
		composeVersionOut: "compose output",
		composeVersionErr: nil,
		workDir:           "/workspace/project",
		workDirErr:        nil,
	}

	printDoctorResult(&buf, result)
	output := buf.String()

	checks := []string{
		"- docker in PATH: yes (/usr/bin/docker)",
		"- docker version: failed (version failed)",
		"    daemon not running",
		"- docker compose version: ok",
		"- Working directory: /workspace/project",
		fmt.Sprintf("- OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH),
	}

	for _, check := range checks {
		if !strings.Contains(output, check) {
			t.Fatalf("expected output to contain %q, got: %s", check, output)
		}
	}
}
