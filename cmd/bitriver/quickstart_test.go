package main

import (
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseQuickstartFlags(t *testing.T) {
	config, err := parseQuickstartFlags([]string{"--env-file", "custom.env", "--compose-file", "stack.yml"}, io.Discard)
	if err != nil {
		t.Fatalf("parseQuickstartFlags returned error: %v", err)
	}

	if config.envFile != "custom.env" {
		t.Fatalf("unexpected env file: %s", config.envFile)
	}

	if config.composeFile != "stack.yml" {
		t.Fatalf("unexpected compose file: %s", config.composeFile)
	}
}

func TestParseQuickstartFlagsRejectsExtraArgs(t *testing.T) {
	if _, err := parseQuickstartFlags([]string{"--env-file", "a", "extra"}, io.Discard); err == nil {
		t.Fatalf("expected error for extra arguments")
	}
}

func TestExecuteQuickstartRunsStepsInOrder(t *testing.T) {
	workDir := t.TempDir()
	expectedEnv := filepath.Join(workDir, "custom.env")
	expectedCompose := filepath.Join(workDir, "compose.yml")

	var calls []string
	deps := quickstartDeps{
		doctor: func(io.Writer) doctorResult {
			calls = append(calls, "doctor")
			return doctorResult{}
		},
		envInit: func(envPath string, templateRoot string, out io.Writer) error {
			calls = append(calls, "env")
			if envPath != expectedEnv {
				t.Fatalf("env path mismatch: got %s", envPath)
			}
			if templateRoot != workDir {
				t.Fatalf("unexpected template root: %s", templateRoot)
			}
			if out != io.Discard {
				t.Fatalf("unexpected stdout writer")
			}
			return nil
		},
		omeRender: func(args []string) error {
			calls = append(calls, "ome")
			expectedArgs := []string{"--env-file", expectedEnv}
			if !reflect.DeepEqual(args, expectedArgs) {
				t.Fatalf("unexpected ome args: %v", args)
			}
			return nil
		},
		composeUp: func(composeFile string) error {
			calls = append(calls, "compose")
			if composeFile != expectedCompose {
				t.Fatalf("unexpected compose file: %s", composeFile)
			}
			return nil
		},
		getwd:  func() (string, error) { return workDir, nil },
		stdout: io.Discard,
	}

	config := quickstartConfig{envFile: "custom.env", composeFile: "compose.yml"}
	if err := executeQuickstart(config, deps); err != nil {
		t.Fatalf("executeQuickstart returned error: %v", err)
	}

	expectedCalls := []string{"doctor", "env", "ome", "compose"}
	if !reflect.DeepEqual(calls, expectedCalls) {
		t.Fatalf("unexpected call order: %v", calls)
	}
}

func TestExecuteQuickstartStopsWhenDockerMissing(t *testing.T) {
	deps := quickstartDeps{
		doctor: func(io.Writer) doctorResult {
			return doctorResult{dockerErr: errors.New("docker missing")}
		},
		envInit: func(string, string, io.Writer) error {
			t.Fatalf("env init should not run when doctor fails")
			return nil
		},
		omeRender: func([]string) error { t.Fatalf("ome render should not run when doctor fails"); return nil },
		composeUp: func(string) error { t.Fatalf("compose should not run when doctor fails"); return nil },
		getwd:     func() (string, error) { return t.TempDir(), nil },
		stdout:    io.Discard,
	}

	config := quickstartConfig{envFile: ".env", composeFile: "compose.yml"}
	if err := executeQuickstart(config, deps); err == nil || !strings.Contains(err.Error(), "docker is required") {
		t.Fatalf("expected docker error, got %v", err)
	}
}

func TestExecuteQuickstartFailsWhenPythonUnavailable(t *testing.T) {
	composeCalled := false
	deps := quickstartDeps{
		doctor:  func(io.Writer) doctorResult { return doctorResult{} },
		envInit: func(string, string, io.Writer) error { return nil },
		omeRender: func([]string) error {
			return errors.New("python executable not found; install python or ensure it is in PATH")
		},
		composeUp: func(string) error {
			composeCalled = true
			return nil
		},
		getwd:  func() (string, error) { return t.TempDir(), nil },
		stdout: io.Discard,
	}

	config := quickstartConfig{envFile: ".env", composeFile: "compose.yml"}
	err := executeQuickstart(config, deps)
	if err == nil || !strings.Contains(err.Error(), "python 3 is required") {
		t.Fatalf("expected python error, got %v", err)
	}

	if composeCalled {
		t.Fatalf("compose should not run when OME render fails")
	}
}
