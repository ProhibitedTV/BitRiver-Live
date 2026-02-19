package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"bitriver-live/internal/executil"
)

type verifyCheckResult struct {
	name    string
	passed  bool
	details string
}

var verifyDoctorRunner = runDoctor
var verifySmokeRunner = runSmoke
var verifyCommandRunner = commandRunner
var verifyLookPath = executil.LookPath

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	composeFile := fs.String("compose-file", defaultComposeFile(), "compose file to validate and test")
	envFile := fs.String("env-file", defaultEnvFile(), "environment file for smoke checks")
	if err := fs.Parse(args); err != nil {
		return err
	}

	results := make([]verifyCheckResult, 0, 3)

	fmt.Fprintln(os.Stdout, "BitRiver Live verify")
	fmt.Fprintf(os.Stdout, "Compose file: %s\n", *composeFile)
	fmt.Fprintf(os.Stdout, "Env file: %s\n", *envFile)

	if verifyDoctorRunner(nil) {
		results = append(results, verifyCheckResult{
			name:    "Doctor checks",
			passed:  true,
			details: "docker and docker compose are available.",
		})
	} else {
		results = append(results, verifyCheckResult{
			name:    "Doctor checks",
			passed:  false,
			details: "doctor checks failed. Fix: start Docker Desktop/Engine and verify `docker version` and `docker compose version` both succeed.",
		})
		printVerifySummary(results)
		return errors.New("verification failed")
	}

	if _, err := verifyLookPath("docker"); err != nil {
		results = append(results, verifyCheckResult{
			name:    "Docker Compose config validation",
			passed:  true,
			details: "skipped: docker is not installed or not on PATH.",
		})
	} else {
		if err := verifyCommandRunner("docker", "compose", "-f", *composeFile, "config"); err != nil {
			results = append(results, verifyCheckResult{
				name:    "Docker Compose config validation",
				passed:  false,
				details: fmt.Sprintf("docker compose config failed: %v. Fix: run `docker compose -f %s config` and resolve interpolation/compose errors.", err, *composeFile),
			})
			printVerifySummary(results)
			return errors.New("verification failed")
		}
		results = append(results, verifyCheckResult{
			name:    "Docker Compose config validation",
			passed:  true,
			details: "docker compose config rendered successfully.",
		})
	}

	if err := verifySmokeRunner([]string{"--compose-file", *composeFile, "--env-file", *envFile}); err != nil {
		results = append(results, verifyCheckResult{
			name:    "Smoke checks",
			passed:  false,
			details: "smoke checks failed. Fix: review the smoke output above and run the suggested compose/log commands.",
		})
		printVerifySummary(results)
		return errors.New("verification failed")
	}
	results = append(results, verifyCheckResult{
		name:    "Smoke checks",
		passed:  true,
		details: "smoke checks passed.",
	})

	printVerifySummary(results)
	return nil
}

func printVerifySummary(results []verifyCheckResult) {
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Verify results:")
	passed := 0
	for _, r := range results {
		status := "FAIL"
		if r.passed {
			status = "PASS"
			passed++
		}
		fmt.Fprintf(os.Stdout, "[%s] %s\n", status, r.name)
		fmt.Fprintf(os.Stdout, "       %s\n", strings.TrimSpace(r.details))
	}
	fmt.Fprintln(os.Stdout)
	if passed == len(results) {
		fmt.Fprintf(os.Stdout, "SUMMARY: PASS (%d/%d checks passed)\n", passed, len(results))
		return
	}
	fmt.Fprintf(os.Stdout, "SUMMARY: FAIL (%d/%d checks passed)\n", passed, len(results))
}
