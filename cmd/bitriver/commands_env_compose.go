package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/executil"
)

// This file groups command handlers for version/doctor/env/compose/quickstart.
// Keeping these operational flows together makes command orchestration easier
// to follow without scrolling through OME rendering internals.

func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s version\n", os.Args[0])
	}
	_ = fs.Parse(args)

	printVersionInfo(os.Stdout)
}

// printVersionInfo performs print version info and propagates validation or dependency failures to the caller.
func printVersionInfo(out io.Writer) {
	fmt.Fprintf(out, "Version: %s\n", valueOrFallback(Version, "dev"))
	fmt.Fprintf(out, "Commit: %s\n", valueOrFallback(Commit, "unknown"))
	fmt.Fprintf(out, "Date: %s\n", valueOrFallback(Date, "unknown"))
}

// runDoctor runs doctor and exits when the work completes or a dependency fails.
func runDoctor(args []string) bool {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s doctor\n", os.Args[0])
	}
	_ = fs.Parse(args)

	fmt.Println("BitRiver Live doctor")
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	if cwd, err := os.Getwd(); err == nil {
		fmt.Printf("Working directory: %s\n", cwd)
	} else {
		fmt.Printf("Working directory: (error: %v)\n", err)
	}

	dockerPath, err := executil.LookPath("docker")
	if err != nil {
		fmt.Printf("Docker: not found (%v)\n", err)
		return false
	}
	fmt.Printf("Docker: %s\n", dockerPath)

	fmt.Println()
	fmt.Println("Checking docker version...")
	if err := executil.Run(dockerPath, "version"); err != nil {
		fmt.Printf("docker version failed: %v\n", err)
		return false
	}
	fmt.Println("docker version: ok")

	fmt.Println()
	fmt.Println("Checking docker compose version...")
	if err := executil.Run(dockerPath, "compose", "version"); err != nil {
		fmt.Printf("docker compose version failed: %v\n", err)
		return false
	}
	fmt.Println("docker compose version: ok")

	fmt.Println()
	fmt.Println("All checks passed! You are ready to run BitRiver Live.")
	return true
}

// valueOrFallback performs value or fallback and propagates validation or dependency failures to the caller.
func valueOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// repoRoot performs repo root and propagates validation or dependency failures to the caller.
func repoRoot() string {
	if cachedRepoRoot != "" {
		return cachedRepoRoot
	}

	dir, err := os.Getwd()
	if err != nil {
		cachedRepoRoot = "."
		return cachedRepoRoot
	}

	current := dir
	for {
		if _, statErr := os.Stat(filepath.Join(current, "go.mod")); statErr == nil {
			cachedRepoRoot = current
			return cachedRepoRoot
		}
		parent := filepath.Dir(current)
		if parent == current {
			cachedRepoRoot = dir
			return cachedRepoRoot
		}
		current = parent
	}
}

// defaultEnvFile returns the default env file for the current runtime mode.
func defaultEnvFile() string {
	return filepath.Join(repoRoot(), ".env")
}

// defaultComposeFile returns the default compose file for the current runtime mode.
func defaultComposeFile() string {
	return filepath.Join(repoRoot(), "deploy", "docker-compose.yml")
}

// defaultExampleEnv returns the default example env for the current runtime mode.
func defaultExampleEnv() string {
	return filepath.Join(repoRoot(), "deploy", ".env.example")
}

type envSecrets struct {
	adminEmail string
	secrets    map[string]string
}

var defaultEnvSecrets = envSecrets{
	adminEmail: "admin@bitriver.local",
	secrets: map[string]string{
		"BITRIVER_LIVE_METRICS_TOKEN":             "",
		"BITRIVER_POSTGRES_PASSWORD":              "",
		"BITRIVER_REDIS_PASSWORD":                 "",
		"BITRIVER_LIVE_ADMIN_PASSWORD":            "",
		"BITRIVER_SRS_TOKEN":                      "",
		"BITRIVER_OME_PASSWORD":                   "",
		"BITRIVER_OME_API_TOKEN":                  "",
		"BITRIVER_OME_ACCESS_TOKEN":               "",
		"BITRIVER_TRANSCODER_TOKEN":               "",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD": "",
	},
}

var (
	sampleCredentialKeys = []string{
		"BITRIVER_LIVE_METRICS_TOKEN",
		"BITRIVER_POSTGRES_PASSWORD",
		"BITRIVER_REDIS_PASSWORD",
		"BITRIVER_LIVE_ADMIN_EMAIL",
		"BITRIVER_LIVE_ADMIN_PASSWORD",
		"BITRIVER_SRS_TOKEN",
		"BITRIVER_OME_USERNAME",
		"BITRIVER_OME_PASSWORD",
		"BITRIVER_OME_API_TOKEN",
		"BITRIVER_OME_ACCESS_TOKEN",
		"BITRIVER_TRANSCODER_TOKEN",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD",
	}

	forbiddenPlaceholders = defaultForbiddenPlaceholders()
	placeholderLoadErr    error
	sslModeDisablePattern = regexp.MustCompile(`(?i)(^|[?&\s;])sslmode=disable([&#;\s]|$)`)
)

// init performs init and propagates validation or dependency failures to the caller.
func init() {
	placeholders, err := loadSampleCredentialValues(defaultExampleEnv(), sampleCredentialKeys)
	if err != nil {
		placeholderLoadErr = err
		return
	}
	forbiddenPlaceholders = placeholders
}

// defaultForbiddenPlaceholders returns the default forbidden placeholders for the current runtime mode.
func defaultForbiddenPlaceholders() map[string]string {
	return map[string]string{
		"BITRIVER_LIVE_METRICS_TOKEN":             "metrics-collector-token",
		"BITRIVER_POSTGRES_PASSWORD":              "P0stgres-Example!",
		"BITRIVER_REDIS_PASSWORD":                 "R3dis-Example!",
		"BITRIVER_LIVE_ADMIN_EMAIL":               "admin@stream.example.com",
		"BITRIVER_LIVE_ADMIN_PASSWORD":            "Sup3rSecureAdmin!",
		"BITRIVER_LIVE_MODE":                      "development",
		"BITRIVER_SRS_TOKEN":                      "srs-secure-token-example",
		"BITRIVER_OME_USERNAME":                   "ome-operator",
		"BITRIVER_OME_PASSWORD":                   "OME-Example-Pass!",
		"BITRIVER_OME_API_TOKEN":                  "OME-Example-Access-Token",
		"BITRIVER_OME_ACCESS_TOKEN":               "OME-Example-Access-Token",
		"BITRIVER_TRANSCODER_TOKEN":               "transcoder-secure-token-example",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD": "R3dis-Example!",
	}
}

type envValidatorResult struct {
	Missing  []string
	Blocked  []string
	Errors   []string
	Warnings []string
}

// runEnv runs env and exits when the work completes or a dependency fails.
func runEnv(args []string) error {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: env <init|validate> [flags]")
		return errors.New("env subcommand required")
	}

	switch args[0] {
	case "init":
		return runEnvInit(args[1:])
	case "validate":
		return runEnvValidate(args[1:])
	default:
		return fmt.Errorf("unknown env subcommand: %s", args[0])
	}
}

// runEnvInit runs env init and exits when the work completes or a dependency fails.
func runEnvInit(args []string) error {
	fs := flag.NewFlagSet("env init", flag.ContinueOnError)
	envPath := fs.String("env-file", defaultEnvFile(), "path to write the environment file")
	examplePath := fs.String("example", defaultExampleEnv(), "path to the example env file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	templateLines, err := readEnvTemplate(*examplePath)
	if err != nil {
		return err
	}

	existingValues, err := loadEnvValues(*envPath, true)
	if err != nil {
		return err
	}

	if warning := omeHealthcheckAuthModeDeprecationWarning(existingValues["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"]); warning != "" {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
	}

	promptForAdminEmail(existingValues)

	generated, _ := generateEnvValues(existingValues)
	content := mergeEnv(templateLines, existingValues, generated)
	if err := os.WriteFile(*envPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Wrote environment file to %s\n", *envPath)
	return nil
}

// runEnvValidate runs env validate and exits when the work completes or a dependency fails.
func runEnvValidate(args []string) error {
	fs := flag.NewFlagSet("env validate", flag.ContinueOnError)
	envPath := fs.String("env-file", defaultEnvFile(), "path to validate")
	if err := fs.Parse(args); err != nil {
		return err
	}

	values, err := loadEnvValues(*envPath, false)
	if err != nil {
		return err
	}

	result := validateEnvironmentValues(values)
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}

	for _, msg := range result.Errors {
		fmt.Fprintln(os.Stderr, msg)
	}
	if len(result.Missing) > 0 {
		fmt.Fprintln(os.Stderr, "The following required variables are unset or empty:")
		for _, v := range result.Missing {
			fmt.Fprintf(os.Stderr, "  - %s\n", v)
		}
	}
	if len(result.Blocked) > 0 {
		fmt.Fprintln(os.Stderr, "The following variables still use example placeholders:")
		for _, v := range result.Blocked {
			fmt.Fprintf(os.Stderr, "  - %s\n", v)
		}
	}

	if len(result.Errors) > 0 || len(result.Missing) > 0 || len(result.Blocked) > 0 {
		return errors.New("environment validation failed")
	}

	fmt.Fprintf(os.Stdout, "Environment file %s looks ready.\n", *envPath)
	return nil
}

func validateEnvironmentValues(values map[string]string) envValidatorResult {
	return validateEnv(values)
}

// runCompose runs compose and exits when the work completes or a dependency fails.
func runCompose(args []string) error {
	if len(args) == 0 {
		return errors.New("compose requires a subcommand (up/down)")
	}

	switch args[0] {
	case "up":
		return runComposeUp(args[1:])
	case "down":
		return runComposeDown(args[1:])
	default:
		return fmt.Errorf("unknown compose subcommand: %s", args[0])
	}
}

var commandRunner = executil.Run
var quickstartWaiter = waitForAPIReadiness
var quickstartComposeHealthWaiter = waitForComposeServiceHealth
var bootstrapAdminRunner = runBootstrapAdmin
var migrationsRunner = runMigrations
var composeUpRunner = runComposeUp
var envInitRunner = runEnvInit
var envValidateRunner = runEnvValidate
var omeRunner = runOME
var doctorRunner = runDoctor

// composeArgsWithEnv performs compose args with env and propagates validation or dependency failures to the caller.
func composeArgsWithEnv(composeFile, envFile string) []string {
	args := []string{"compose"}
	if strings.TrimSpace(envFile) != "" {
		args = append(args, "--env-file", envFile)
	} else {
		args = append(args, "--project-directory", repoRoot())
	}
	if strings.TrimSpace(composeFile) != "" {
		args = append(args, "--file", composeFile)
	}
	return args
}

// runComposeUp runs compose up and exits when the work completes or a dependency fails.
func runComposeUp(args []string) error {
	fs := flag.NewFlagSet("compose up", flag.ContinueOnError)
	composeFile := fs.String("file", defaultComposeFile(), "compose file to use")
	envFile := fs.String("env-file", "", "env file to use for compose interpolation")
	detach := fs.Bool("detached", true, "run docker compose in detached mode")
	build := fs.Bool("build", true, "build images before starting")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validateComposeEffectiveEnvironment(*envFile); err != nil {
		return err
	}

	if _, err := executil.LookPath("docker"); err != nil {
		return err
	}

	composeArgs := append(composeArgsWithEnv(*composeFile, *envFile), "up")
	if *build {
		composeArgs = append(composeArgs, "--build")
	}
	if *detach {
		composeArgs = append(composeArgs, "-d")
	}

	return commandRunner("docker", composeArgs...)
}

func validateComposeEffectiveEnvironment(envFile string) error {
	resolvedEnvFile := strings.TrimSpace(envFile)
	if resolvedEnvFile == "" {
		resolvedEnvFile = defaultEnvFile()
	}

	fileValues, err := loadEnvValues(resolvedEnvFile, true)
	if err != nil {
		return fmt.Errorf("load env values: %w", err)
	}

	effectiveValues := copyEnvValues(fileValues)
	overrides := make(map[string][2]string)
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(key, "BITRIVER_") {
			continue
		}
		value := parts[1]
		if fileValue, ok := fileValues[key]; ok && fileValue != value {
			overrides[key] = [2]string{fileValue, value}
		}
		effectiveValues[key] = value
	}

	for _, key := range criticalDeployEnvironmentKeys() {
		if _, ok := overrides[key]; ok {
			return composeEnvOverrideError(resolvedEnvFile, key)
		}
	}

	result := validateEnvironmentValues(effectiveValues)
	if len(result.Errors) > 0 || len(result.Missing) > 0 || len(result.Blocked) > 0 {
		for _, key := range criticalDeployEnvironmentKeys() {
			if _, ok := overrides[key]; ok {
				return composeEnvOverrideError(resolvedEnvFile, key)
			}
		}
		return errors.New("effective environment validation failed after applying process environment overrides")
	}

	return nil
}

func criticalDeployEnvironmentKeys() []string {
	return []string{
		"BITRIVER_OME_HEALTHCHECK_AUTH_MODE",
		"BITRIVER_OME_USERNAME",
		"BITRIVER_OME_PASSWORD",
		"BITRIVER_OME_API_TOKEN",
		"BITRIVER_OME_ACCESS_TOKEN",
		"BITRIVER_OME_HEALTHCHECK_TOKEN",
		"BITRIVER_LIVE_ADMIN_EMAIL",
		"BITRIVER_LIVE_ADMIN_PASSWORD",
		"BITRIVER_POSTGRES_DB",
		"BITRIVER_POSTGRES_USER",
		"BITRIVER_POSTGRES_PASSWORD",
		"BITRIVER_REDIS_PASSWORD",
		"BITRIVER_LIVE_STORAGE_DRIVER",
	}
}

func composeEnvOverrideError(envFile, key string) error {
	return fmt.Errorf("%s from the current process overrides %s and is not allowed for critical deploy settings; run `unset %s` (bash/zsh) or `Remove-Item Env:%s` (PowerShell), then retry", key, envFile, key, key)
}

// runComposeDown runs compose down and exits when the work completes or a dependency fails.
func runComposeDown(args []string) error {
	fs := flag.NewFlagSet("compose down", flag.ContinueOnError)
	composeFile := fs.String("file", defaultComposeFile(), "compose file to use")
	envFile := fs.String("env-file", "", "env file to use for compose interpolation")
	volumes := fs.Bool("volumes", false, "remove named volumes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := executil.LookPath("docker"); err != nil {
		return err
	}

	composeArgs := append(composeArgsWithEnv(*composeFile, *envFile), "down")
	if *volumes {
		composeArgs = append(composeArgs, "-v")
	}
	return commandRunner("docker", composeArgs...)
}

// runQuickstart runs quickstart and exits when the work completes or a dependency fails.
func runQuickstart(args []string) error {
	fs := flag.NewFlagSet("quickstart", flag.ContinueOnError)
	composeFile := fs.String("compose-file", defaultComposeFile(), "compose file to use")
	envFile := fs.String("env-file", defaultEnvFile(), "environment file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !doctorRunner(nil) {
		return errors.New("doctor checks failed")
	}

	preExisting := map[string]string{}
	if existingValues, err := loadEnvValues(*envFile, true); err == nil {
		preExisting = existingValues
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read env file before init: %w", err)
	}
	preExistingCopy := copyEnvValues(preExisting)
	_, generatedSecrets := generateEnvValues(preExistingCopy)

	if err := validateQuickstartOMEHealthcheckAuthMode(preExisting["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"]); err != nil {
		return err
	}

	if err := envInitRunner([]string{"--env-file", *envFile}); err != nil {
		return fmt.Errorf("env init: %w", err)
	}
	if err := envValidateRunner([]string{"--env-file", *envFile}); err != nil {
		return fmt.Errorf("env validate: %w", err)
	}

	envValues, err := loadEnvValues(*envFile, false)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}

	if err := omeRunner([]string{"render", "--env-file", *envFile, "--force"}); err != nil {
		return fmt.Errorf("render OME config: %w", err)
	}

	if err := validateComposeEffectiveEnvironment(*envFile); err != nil {
		return err
	}

	if err := migrationsRunner(*composeFile, *envFile); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if err := composeUpRunner([]string{"--file", *composeFile, "--env-file", *envFile}); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}

	if err := quickstartWaiter(envValues); err != nil {
		return fmt.Errorf("wait for API readiness: %w", err)
	}

	if err := quickstartComposeHealthWaiter(*composeFile, *envFile); err != nil {
		return fmt.Errorf("wait for critical service health: %w", err)
	}

	if err := bootstrapAdminRunner(*composeFile, *envFile, envValues); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}

	printGeneratedSecrets(generatedSecrets)
	return nil
}

func validateQuickstartOMEHealthcheckAuthMode(raw string) error {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" || mode == "accesstoken" || mode == "basic" || mode == "token" || mode == "token+basic" {
		return nil
	}

	return fmt.Errorf("BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be one of [accesstoken, basic] before quickstart can continue (current: %s)", strings.TrimSpace(raw))
}

// runMigrations runs migrations and exits when the work completes or a dependency fails.
func runMigrations(composeFile, envFile string) error {
	if _, err := executil.LookPath("docker"); err != nil {
		return err
	}

	args := append(composeArgsWithEnv(composeFile, envFile), "run", "--rm")
	if runtime.GOOS == "windows" || !stdinIsTerminal() {
		args = append(args, "-T")
	}
	args = append(args, "postgres-migrations")
	return commandRunner("docker", args...)
}

// waitForAPIReadiness performs wait for apireadiness and propagates validation or dependency failures to the caller.
func waitForAPIReadiness(values map[string]string) error {
	readyzURL := fmt.Sprintf("http://127.0.0.1:%s/readyz", resolveAPIPort(values))
	fmt.Fprintf(os.Stdout, "Waiting for API readiness at %s...\n", readyzURL)

	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, readyzURL, nil)
		if reqErr != nil {
			cancel()
			return fmt.Errorf("build readiness request: %w", reqErr)
		}

		resp, err := http.DefaultClient.Do(req)
		cancel()
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				fmt.Fprintln(os.Stdout, "API is ready.")
				return nil
			}
		}

		time.Sleep(3 * time.Second)
	}

	return errors.New("API did not become ready before timeout")
}

type quickstartComposeServiceStatus struct {
	Service string `json:"Service"`
	Name    string `json:"Name"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Health  string `json:"Health"`
}

var criticalComposeServices = []string{"bitriver-live", "ome", "srs", "srs-controller", "transcoder", "postgres", "redis"}

var composePSRunner = runComposePS
var composeHealthWaitTimeout = 2 * time.Minute
var composeHealthPollInterval = 3 * time.Second

func runComposePS(composeFile, envFile string) ([]byte, error) {
	args := append(composeArgsWithEnv(composeFile, envFile), "ps", "--format", "json")
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return nil, fmt.Errorf("docker compose ps failed: %w: %s", err, trimmed)
		}
		return nil, fmt.Errorf("docker compose ps failed: %w", err)
	}
	return output, nil
}

func waitForComposeServiceHealth(composeFile, envFile string) error {
	fmt.Fprintln(os.Stdout, "Waiting for critical Docker Compose services to report healthy...")
	deadline := time.Now().Add(composeHealthWaitTimeout)
	lastSummary := map[string]string{}

	for time.Now().Before(deadline) {
		output, err := composePSRunner(composeFile, envFile)
		if err != nil {
			return err
		}

		serviceStates, err := parseComposeServiceStates(output)
		if err != nil {
			return fmt.Errorf("parse docker compose ps output: %w", err)
		}

		allHealthy := true
		for _, svc := range criticalComposeServices {
			state, ok := serviceStates[svc]
			if !ok {
				allHealthy = false
				lastSummary[svc] = "missing"
				continue
			}

			health := strings.ToLower(strings.TrimSpace(state.Health))
			currentState := strings.ToLower(strings.TrimSpace(state.State))
			currentStatus := strings.TrimSpace(state.Status)
			if currentStatus == "" {
				currentStatus = currentState
			}
			lastSummary[svc] = currentStatus

			if health == "unhealthy" || currentState == "exited" || currentState == "dead" {
				return fmt.Errorf("critical service %q is %s (state=%s, health=%s); run `docker compose logs %s` for details", svc, currentStatus, currentState, health, svc)
			}

			if health != "healthy" {
				allHealthy = false
			}
		}

		if allHealthy {
			fmt.Fprintln(os.Stdout, "Critical services are healthy.")
			return nil
		}

		time.Sleep(composeHealthPollInterval)
	}

	var statusParts []string
	for _, svc := range criticalComposeServices {
		status := lastSummary[svc]
		if status == "" {
			status = "unknown"
		}
		statusParts = append(statusParts, fmt.Sprintf("%s=%s", svc, status))
	}

	return fmt.Errorf("critical services did not become healthy before timeout; last known states: %s", strings.Join(statusParts, ", "))
}

func parseComposeServiceStates(raw []byte) (map[string]quickstartComposeServiceStatus, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, errors.New("empty JSON payload")
	}

	var services []quickstartComposeServiceStatus
	if err := json.Unmarshal([]byte(trimmed), &services); err != nil {
		return nil, err
	}

	states := make(map[string]quickstartComposeServiceStatus, len(services))
	for _, svc := range services {
		key := strings.ToLower(strings.TrimSpace(svc.Service))
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(svc.Name))
		}
		if key == "" {
			continue
		}
		states[key] = svc
	}

	return states, nil
}

// resolveAPIPort resolves apiport from flags and environment values, returning validation errors when incompatible settings are provided.
func resolveAPIPort(values map[string]string) string {
	if port := strings.TrimSpace(values["BITRIVER_LIVE_PORT"]); port != "" {
		return port
	}

	addr := strings.TrimSpace(values["BITRIVER_LIVE_ADDR"])
	if p := extractPort(addr); p != 0 {
		return strconv.Itoa(p)
	}

	return "8080"
}

// runBootstrapAdmin runs bootstrap admin and exits when the work completes or a dependency fails.
func runBootstrapAdmin(composeFile, envFile string, values map[string]string) error {
	email := strings.TrimSpace(values["BITRIVER_LIVE_ADMIN_EMAIL"])
	password := strings.TrimSpace(values["BITRIVER_LIVE_ADMIN_PASSWORD"])
	if email == "" || password == "" {
		return errors.New("admin email/password missing from environment")
	}

	storageDriver := strings.ToLower(strings.TrimSpace(values["BITRIVER_LIVE_STORAGE_DRIVER"]))
	if storageDriver == "" {
		storageDriver = "postgres"
	}

	args := append(composeArgsWithEnv(composeFile, envFile), "exec", "-T", "bitriver-live", "/app/bootstrap-admin")
	switch storageDriver {
	case "postgres":
		dsn, err := buildPostgresDSN(values)
		if err != nil {
			return err
		}
		args = append(args, "--postgres-dsn", dsn)
	case "json":
		dataPath := strings.TrimSpace(values["BITRIVER_LIVE_DATA"])
		if dataPath == "" {
			dataPath = "/var/lib/bitriver-live/store.json"
		}
		args = append(args, "--json", dataPath)
	default:
		return fmt.Errorf("unsupported storage driver %q for bootstrap-admin", storageDriver)
	}

	args = append(args, "--email", email, "--password", password)

	fmt.Fprintln(os.Stdout, "Seeding administrator account via bootstrap-admin...")
	if err := commandRunner("docker", args...); err != nil {
		return err
	}
	return nil
}

// buildPostgresDSN builds postgres dsn from runtime state used by downstream handlers.
func buildPostgresDSN(values map[string]string) (string, error) {
	if dsn := strings.TrimSpace(values["BITRIVER_LIVE_POSTGRES_DSN"]); dsn != "" {
		return dsn, nil
	}

	user := strings.TrimSpace(values["BITRIVER_POSTGRES_USER"])
	password := strings.TrimSpace(values["BITRIVER_POSTGRES_PASSWORD"])
	db := strings.TrimSpace(values["BITRIVER_POSTGRES_DB"])
	if db == "" {
		db = "bitriver"
	}
	if user == "" || password == "" {
		return "", errors.New("postgres credentials missing for bootstrap-admin")
	}

	host := "postgres"
	port := "5432"
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   "/" + db,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()

	return u.String(), nil
}
