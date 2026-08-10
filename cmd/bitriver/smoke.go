package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type smokeCheckResult struct {
	name    string
	passed  bool
	details string
}

var smokePrerequisiteRunner = runSmokePrerequisites
var smokeComposePSRunner = runComposePS
var smokeHTTPClient = &http.Client{Timeout: 4 * time.Second}

func runSmokePrerequisites() error {
	opts, err := parseDoctorArgs(nil)
	if err != nil {
		return fmt.Errorf("load Docker prerequisite policy: %w", err)
	}

	for _, result := range []doctorResult{
		checkRequiredBinaries(),
		checkDockerAndCompose(opts),
	} {
		if result.Status != doctorStatusFail {
			continue
		}
		if strings.TrimSpace(result.Remediation) != "" {
			return fmt.Errorf("%s: %s; %s", result.Name, result.Details, result.Remediation)
		}
		return fmt.Errorf("%s: %s", result.Name, result.Details)
	}

	return nil
}

func runSmoke(args []string) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	composeFile := fs.String("compose-file", defaultComposeFile(), "path to docker compose file")
	envFile := fs.String("env-file", defaultEnvFile(), "path to environment file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	results := make([]smokeCheckResult, 0, 6)

	fmt.Fprintln(os.Stdout, "BitRiver Live smoke")
	fmt.Fprintf(os.Stdout, "Compose file: %s\n", *composeFile)
	fmt.Fprintf(os.Stdout, "Env file: %s\n", *envFile)

	if err := smokePrerequisiteRunner(); err == nil {
		results = append(results, smokeCheckResult{
			name:    "Docker + Docker Compose availability",
			passed:  true,
			details: "docker and docker compose are available.",
		})
	} else {
		results = append(results, smokeCheckResult{
			name:    "Docker + Docker Compose availability",
			passed:  false,
			details: fmt.Sprintf("Docker prerequisite checks failed: %v", err),
		})
		printSmokeSummary(results)
		return errors.New("smoke checks failed")
	}

	composeState, err := smokeCheckComposeState(*composeFile, *envFile)
	if err != nil {
		results = append(results, smokeCheckResult{
			name:    "Compose stack reachability",
			passed:  false,
			details: fmt.Sprintf("%v", err),
		})
		printSmokeSummary(results)
		return errors.New("smoke checks failed")
	}
	results = append(results, smokeCheckResult{
		name:    "Compose stack reachability",
		passed:  true,
		details: composeState,
	})

	values, err := loadEnvValues(*envFile, false)
	if err != nil {
		results = append(results, smokeCheckResult{
			name:    "Load environment config",
			passed:  false,
			details: fmt.Sprintf("unable to read env file %s: %v. Fix: run `go run ./cmd/bitriver env init --env-file %s` then rerun smoke.", *envFile, err, *envFile),
		})
		printSmokeSummary(results)
		return errors.New("smoke checks failed")
	}

	endpointChecks := []smokeEndpointCheck{
		{name: "API readiness", url: fmt.Sprintf("http://127.0.0.1:%s/readyz", resolveAPIPort(values)), expected2xx: true},
		{name: "API health", url: fmt.Sprintf("http://127.0.0.1:%s/healthz", resolveAPIPort(values)), expected2xx: true},
		{name: "SRS controller health", url: fmt.Sprintf("http://127.0.0.1:%s/healthz", portOrDefault(values, "BITRIVER_SRS_CONTROLLER_PORT", "1986")), expected2xx: true},
		{name: "Transcoder health", url: fmt.Sprintf("http://127.0.0.1:%s/healthz", portOrDefault(values, "BITRIVER_TRANSCODER_HOST_PORT", "9001")), expected2xx: true},
		{name: "OME HTTP endpoint", url: fmt.Sprintf("http://127.0.0.1:%s/", portOrDefault(values, "BITRIVER_OME_HTTP_PORT", "8081")), expected2xx: false},
	}

	for _, check := range endpointChecks {
		res := runSmokeEndpointCheck(check)
		results = append(results, res)
	}

	printSmokeSummary(results)
	for _, result := range results {
		if !result.passed {
			return errors.New("smoke checks failed")
		}
	}
	return nil
}

type smokeEndpointCheck struct {
	name        string
	url         string
	expected2xx bool
}

func smokeCheckComposeState(composeFile, envFile string) (string, error) {
	output, err := smokeComposePSRunner(composeFile, envFile)
	if err != nil {
		return "", fmt.Errorf("unable to query compose stack: %w. Fix: run `go run ./cmd/bitriver compose up --file %s --env-file %s` or `go run ./cmd/bitriver quickstart --compose-file %s`", err, composeFile, envFile, composeFile)
	}

	states, err := parseComposeServiceStates(output)
	if err != nil {
		return "", fmt.Errorf("unable to parse compose state output: %w. Fix: run `docker compose --file %s --env-file %s ps --format json` and inspect the output", err, composeFile, envFile)
	}

	if len(states) == 0 {
		return "", fmt.Errorf("compose stack is reachable but no services are listed. Fix: run `go run ./cmd/bitriver compose up --file %s --env-file %s`", composeFile, envFile)
	}

	live, ok := states["bitriver-live"]
	if !ok {
		return "", fmt.Errorf("compose stack is missing service \"bitriver-live\". Fix: ensure you are using deploy/docker-compose.yml and run `go run ./cmd/bitriver compose up --file %s --env-file %s`", composeFile, envFile)
	}

	state := strings.ToLower(strings.TrimSpace(live.State))
	if state == "" {
		state = strings.ToLower(strings.TrimSpace(live.Status))
	}
	if state == "exited" || state == "dead" {
		return "", fmt.Errorf("bitriver-live is not running (state=%s). Fix: run `docker compose --file %s --env-file %s logs --tail=80 bitriver-live` then restart with `go run ./cmd/bitriver compose up --file %s --env-file %s`", state, composeFile, envFile, composeFile, envFile)
	}

	return fmt.Sprintf("compose stack is reachable with %d listed services.", len(states)), nil
}

func runSmokeEndpointCheck(check smokeEndpointCheck) smokeCheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, check.url, nil)
	if err != nil {
		return smokeCheckResult{
			name:    check.name,
			passed:  false,
			details: fmt.Sprintf("invalid endpoint %s: %v. Fix: verify port-related values in .env and rerun smoke.", check.url, err),
		}
	}

	resp, err := smokeHTTPClient.Do(req)
	if err != nil {
		return smokeCheckResult{
			name:    check.name,
			passed:  false,
			details: fmt.Sprintf("request to %s failed: %v. Fix: ensure the compose stack is running (`docker compose ps`) and no host firewall/port conflict blocks this endpoint.", check.url, err),
		}
	}
	defer resp.Body.Close()

	if check.expected2xx {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return smokeCheckResult{
				name:    check.name,
				passed:  false,
				details: fmt.Sprintf("%s returned HTTP %d (expected 2xx). Fix: inspect service logs with `docker compose logs --tail=80` and verify service credentials/config in .env.", check.url, resp.StatusCode),
			}
		}
	} else {
		if resp.StatusCode >= 500 || resp.StatusCode == 0 {
			return smokeCheckResult{
				name:    check.name,
				passed:  false,
				details: fmt.Sprintf("%s returned HTTP %d (expected <500). Fix: inspect OME logs with `docker compose logs --tail=80 ome` and verify BITRIVER_OME_HTTP_PORT/BITRIVER_OME_API_TOKEN settings.", check.url, resp.StatusCode),
			}
		}
	}

	return smokeCheckResult{name: check.name, passed: true, details: fmt.Sprintf("%s responded with HTTP %d.", check.url, resp.StatusCode)}
}

func printSmokeSummary(results []smokeCheckResult) {
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Smoke check results:")
	passed := 0
	for _, r := range results {
		status := "FAIL"
		if r.passed {
			status = "PASS"
			passed++
		}
		fmt.Fprintf(os.Stdout, "[%s] %s\n", status, r.name)
		fmt.Fprintf(os.Stdout, "       %s\n", r.details)
	}
	fmt.Fprintln(os.Stdout)
	if passed == len(results) {
		fmt.Fprintf(os.Stdout, "SUMMARY: PASS (%d/%d checks passed)\n", passed, len(results))
		return
	}
	fmt.Fprintf(os.Stdout, "SUMMARY: FAIL (%d/%d checks passed)\n", passed, len(results))
}

func portOrDefault(values map[string]string, key, fallback string) string {
	candidate := strings.TrimSpace(values[key])
	if candidate == "" {
		return fallback
	}
	if _, err := strconv.Atoi(candidate); err != nil {
		return fallback
	}
	return candidate
}
