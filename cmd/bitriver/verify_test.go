package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunVerifyRunsDoctorComposeAndSmoke(t *testing.T) {
	originalDoctor := verifyDoctorRunner
	originalLookPath := verifyLookPath
	originalCommand := verifyCommandRunner
	originalSmoke := verifySmokeRunner
	originalStdout := os.Stdout
	defer func() {
		verifyDoctorRunner = originalDoctor
		verifyLookPath = originalLookPath
		verifyCommandRunner = originalCommand
		verifySmokeRunner = originalSmoke
		os.Stdout = originalStdout
	}()

	calls := []string{}
	verifyDoctorRunner = func([]string) bool {
		calls = append(calls, "doctor")
		return true
	}
	verifyLookPath = func(string) (string, error) { return "docker", nil }
	verifyCommandRunner = func(name string, args ...string) error {
		calls = append(calls, "compose-config")
		if name != "docker" {
			t.Fatalf("expected docker command, got %s", name)
		}
		if len(args) != 4 || args[0] != "compose" || args[1] != "-f" || args[3] != "config" {
			t.Fatalf("unexpected args: %v", args)
		}
		return nil
	}
	verifySmokeRunner = func(args []string) error {
		calls = append(calls, "smoke")
		if len(args) != 4 || args[0] != "--compose-file" || args[2] != "--env-file" {
			t.Fatalf("unexpected smoke args: %v", args)
		}
		return nil
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	err = runVerify([]string{"--compose-file", "deploy/docker-compose.yml", "--env-file", ".env"})
	w.Close()
	if err != nil {
		t.Fatalf("runVerify failed: %v", err)
	}

	var out bytes.Buffer
	if _, err := io.Copy(&out, r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}

	expected := []string{"doctor", "compose-config", "smoke"}
	if strings.Join(calls, ",") != strings.Join(expected, ",") {
		t.Fatalf("expected call order %v, got %v", expected, calls)
	}
	if !strings.Contains(out.String(), "SUMMARY: PASS") {
		t.Fatalf("expected pass summary, got %q", out.String())
	}
}

func TestRunVerifySkipsComposeConfigWhenDockerMissing(t *testing.T) {
	originalDoctor := verifyDoctorRunner
	originalLookPath := verifyLookPath
	originalSmoke := verifySmokeRunner
	defer func() {
		verifyDoctorRunner = originalDoctor
		verifyLookPath = originalLookPath
		verifySmokeRunner = originalSmoke
	}()

	verifyDoctorRunner = func([]string) bool { return true }
	verifyLookPath = func(string) (string, error) { return "", errors.New("not found") }
	verifySmokeRunner = func([]string) error { return nil }

	if err := runVerify(nil); err != nil {
		t.Fatalf("runVerify failed: %v", err)
	}
}

func TestRunVerifyFailsWhenSmokeFails(t *testing.T) {
	originalDoctor := verifyDoctorRunner
	originalLookPath := verifyLookPath
	originalSmoke := verifySmokeRunner
	defer func() {
		verifyDoctorRunner = originalDoctor
		verifyLookPath = originalLookPath
		verifySmokeRunner = originalSmoke
	}()

	verifyDoctorRunner = func([]string) bool { return true }
	verifyLookPath = func(string) (string, error) { return "", errors.New("not found") }
	verifySmokeRunner = func([]string) error { return errors.New("smoke checks failed") }

	if err := runVerify(nil); err == nil {
		t.Fatal("expected runVerify to fail")
	}
}
