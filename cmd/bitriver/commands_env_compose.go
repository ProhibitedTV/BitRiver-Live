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
	"runtime"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/config"
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

// defaultComposeLimitsFile returns the optional resource-limits overlay file.
func defaultComposeLimitsFile() string {
	return filepath.Join(repoRoot(), "deploy", "docker-compose.limits.yml")
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
		"BITRIVER_TRANSCODER_TOKEN",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD",
	}

	forbiddenPlaceholders = defaultForbiddenPlaceholders()
	placeholderLoadErr    error
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
		fmt.Fprintln(os.Stderr, "Usage: env <init|validate|admin> [flags]")
		return errors.New("env subcommand required")
	}

	switch args[0] {
	case "init":
		return runEnvInit(args[1:])
	case "validate":
		return runEnvValidate(args[1:])
	case "admin":
		return runEnvAdmin(args[1:])
	default:
		return fmt.Errorf("unknown env subcommand: %s", args[0])
	}
}

// runEnvInit runs env init and exits when the work completes or a dependency fails.
func runEnvInit(args []string) error {
	fs := flag.NewFlagSet("env init", flag.ContinueOnError)
	envPath := fs.String("env-file", defaultEnvFile(), "path to write the environment file")
	examplePath := fs.String("example", defaultExampleEnv(), "path to the example env file")
	wizard := fs.Bool("wizard", false, "prompt for guided first-run quickstart settings before writing the env file")
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

	if *wizard {
		if err := promptForQuickstartWizard(existingValues, *envPath); err != nil {
			return err
		}
	} else {
		promptForAdminEmail(existingValues)
	}

	generated, _, err := generateEnvValues(existingValues)
	if err != nil {
		return fmt.Errorf("generate env values: %w", err)
	}
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

	result := validateEnv(values)
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

	modeCfg, err := resolveDeployImageSource("", values, config.LoadEnvironment())
	if err != nil {
		return err
	}
	if err := validateProductionDeploymentContract(*envPath, values, modeCfg.mode); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Environment file %s looks ready.\n", *envPath)
	return nil
}

func runEnvAdmin(args []string) error {
	fs := flag.NewFlagSet("env admin", flag.ContinueOnError)
	envPath := fs.String("env-file", defaultEnvFile(), "path to the deployment environment file")
	showPassword := fs.Bool("show-password", false, "print the env-backed bootstrap admin password")
	if err := fs.Parse(args); err != nil {
		return err
	}

	values, err := loadEnvValues(*envPath, false)
	if err != nil {
		return err
	}

	return printBootstrapAdminAccessSummary(os.Stdout, *envPath, values, *showPassword)
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
var lookPathRunner = executil.LookPath
var manifestInspectRunner = runDockerManifestInspect
var deployImageSourcePreflightRunner = runDeployImageSourcePreflight
var quickstartWaiter = waitForAPIReadiness
var quickstartComposeHealthWaiter = waitForComposeServiceHealth
var bootstrapAdminRunner = runBootstrapAdmin
var migrationsRunner = runMigrations
var composeUpRunner = runComposeUp
var envInitRunner = runEnvInit
var envValidateRunner = runEnvValidate
var omeRunner = runOME
var omeVerifyHealthTokenRunner = runOMEVerifyHealthToken
var quickstartOMEAuthPreflightRunner = runQuickstartOMEAuthPreflight
var doctorRunner = runDoctor
var dockerVersionRunner = runDockerVersionPreflight
var dockerComposeVersionRunner = runDockerComposeVersionPreflight
var quickstartHostPortPreflightRunner = runQuickstartHostPortPreflight
var quickstartHostPortAvailabilityChecker = checkHostPortAvailable

const (
	deployImageSourcePull  = "pull"
	deployImageSourceBuild = "build"
)

type deployImageSourceConfig struct {
	mode        string
	composeArgs []string
	description string
}

// composeArgsWithEnv performs compose args with env and propagates validation or dependency failures to the caller.
func composeArgsWithEnv(composeFile, envFile string) []string {
	return composeArgsWithEnvAndFiles([]string{composeFile}, envFile)
}

func composeArgsWithEnvAndFiles(composeFiles []string, envFile string) []string {
	args := []string{"compose"}
	if strings.TrimSpace(envFile) != "" {
		args = append(args, "--env-file", envFile)
	} else {
		args = append(args, "--project-directory", repoRoot())
	}
	for _, composeFile := range composeFiles {
		if strings.TrimSpace(composeFile) != "" {
			args = append(args, "--file", composeFile)
		}
	}
	return args
}

// runComposeUp runs compose up and exits when the work completes or a dependency fails.
func runComposeUp(args []string) error {
	fs := flag.NewFlagSet("compose up", flag.ContinueOnError)
	composeFile := fs.String("file", defaultComposeFile(), "compose file to use")
	limits := fs.Bool("limits", false, "include deploy/docker-compose.limits.yml resource overlay")
	envFile := fs.String("env-file", "", "env file to use for compose interpolation")
	detach := fs.Bool("detached", true, "run docker compose in detached mode")
	build := fs.Bool("build", false, "build images before starting (development-only; rejected in production mode)")
	imageSource := fs.String("image-source", "", "image source mode: pull (default, production) or build (development-only)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	resolvedEnvFile := strings.TrimSpace(*envFile)
	if resolvedEnvFile == "" {
		resolvedEnvFile = defaultEnvFile()
	}

	envValues, err := loadEnvValues(resolvedEnvFile, true)
	if err != nil {
		return fmt.Errorf("load env values: %w", err)
	}

	modeCfg, err := resolveDeployImageSource(*imageSource, envValues, config.LoadEnvironment())
	if err != nil {
		return err
	}
	if *build && modeCfg.mode == deployImageSourcePull {
		return errors.New("conflicting compose options: --build cannot be used with image source mode 'pull'; set BITRIVER_DEPLOY_IMAGE_SOURCE=build or pass --image-source build")
	}

	if err := validateComposeEffectiveEnvironment(*envFile); err != nil {
		return err
	}

	if err := validateProductionDeploymentContract(resolvedEnvFile, envValues, modeCfg.mode); err != nil {
		return err
	}

	if _, err := lookPathRunner("docker"); err != nil {
		return err
	}

	if err := deployImageSourcePreflightRunner(modeCfg.mode, envValues, resolvedEnvFile); err != nil {
		return err
	}

	composeFiles := []string{*composeFile}
	if *limits {
		composeFiles = append(composeFiles, defaultComposeLimitsFile())
	}

	composeArgs := append(composeArgsWithEnvAndFiles(composeFiles, *envFile), "up")
	composeArgs = append(composeArgs, modeCfg.composeArgs...)
	if *build && modeCfg.mode == deployImageSourceBuild {
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

	result := validateEnv(effectiveValues)
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

	if _, err := lookPathRunner("docker"); err != nil {
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
	limits := fs.Bool("limits", false, "include deploy/docker-compose.limits.yml resource overlay")
	envFile := fs.String("env-file", defaultEnvFile(), "environment file path")
	wizard := fs.Bool("wizard", false, "run the guided first-run setup wizard before env init/validate")
	build := fs.Bool("build", false, "build images from the local source tree before starting (development-only; rejected in production mode)")
	imageSource := fs.String("image-source", "", "image source mode: pull (default, production) or build (development-only)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	printQuickstartStageHeader("Doctor")
	if !doctorRunner(nil) {
		return quickstartStageFailure("Doctor", errors.New("some local requirements are missing"), "Run `go run ./cmd/bitriver doctor` and follow the suggested fixes, then run quickstart again.")
	}

	preExisting := map[string]string{}
	if existingValues, err := loadEnvValues(*envFile, true); err == nil {
		preExisting = existingValues
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read env file before init: %w", err)
	}
	preExistingCopy := copyEnvValues(preExisting)
	_, generatedSecrets, err := generateEnvValues(preExistingCopy)
	if err != nil {
		return quickstartStageFailure("Env init", fmt.Errorf("generate env values: %w", err), fmt.Sprintf("Retry quickstart; if this persists, verify system entropy and rerun with %s.", *envFile))
	}
	if err := validateQuickstartOMEAuthMode(preExisting["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"], *envFile); err != nil {
		return quickstartStageFailure("Env validate", err, fmt.Sprintf("Review OME auth values in %s, then rerun quickstart.", *envFile))
	}

	printQuickstartStageHeader("Env init")
	envInitArgs := []string{"--env-file", *envFile}
	if *wizard {
		envInitArgs = append(envInitArgs, "--wizard")
	}
	if err := envInitRunner(envInitArgs); err != nil {
		return quickstartStageFailure("Env init", err, fmt.Sprintf("Check that %s is writable, then run quickstart again.", *envFile))
	}

	printQuickstartStageHeader("Env validate")
	envValues, err := loadEnvValues(*envFile, false)
	if err != nil {
		return quickstartStageFailure("Env validate", err, fmt.Sprintf("Open %s, fix the highlighted values, then rerun quickstart.", *envFile))
	}

	modeCfg, err := resolveDeployImageSource(*imageSource, envValues, config.LoadEnvironment())
	if err != nil {
		return quickstartStageFailure("Image preflight (pull/build)", err, "Choose image source pull or build and rerun quickstart.")
	}
	if *build && modeCfg.mode == deployImageSourcePull {
		return errors.New("conflicting quickstart options: --build cannot be used with image source mode 'pull'; set BITRIVER_DEPLOY_IMAGE_SOURCE=build or pass --image-source build")
	}

	if err := validateProductionDeploymentContract(*envFile, envValues, modeCfg.mode); err != nil {
		return quickstartStageFailure("Env validate", err, fmt.Sprintf("Update %s to match production requirements, then run quickstart again.", *envFile))
	}

	if err := envValidateRunner([]string{"--env-file", *envFile}); err != nil {
		return quickstartStageFailure("Env validate", err, fmt.Sprintf("Fix the env validation issues in %s and rerun quickstart.", *envFile))
	}

	if err := quickstartOMEAuthPreflightRunner(*envFile, envValues); err != nil {
		return quickstartStageFailure("Env validate", err, fmt.Sprintf("Review OME auth values in %s, then rerun quickstart.", *envFile))
	}

	if err := validateComposeEffectiveEnvironment(*envFile); err != nil {
		return quickstartStageFailure("Env validate", err, "Resolve the compose environment mismatch and run quickstart again.")
	}

	printQuickstartStageHeader("Deployment preflight")
	if err := dockerVersionRunner(); err != nil {
		return quickstartStageFailure("Deployment preflight", err, "Start Docker Desktop/Engine, then rerun quickstart.")
	}
	if err := dockerComposeVersionRunner(); err != nil {
		return quickstartStageFailure("Deployment preflight", err, "Install/enable Docker Compose v2 (`docker compose`), then rerun quickstart.")
	}
	if err := quickstartHostPortPreflightRunner(envValues); err != nil {
		return quickstartStageFailure("Deployment preflight", err, "Free the listed host ports by stopping conflicting services or updating .env port values, then rerun quickstart.")
	}

	printQuickstartStageHeader("Image preflight (pull/build)")
	if err := deployImageSourcePreflightRunner(modeCfg.mode, envValues, *envFile); err != nil {
		return quickstartStageFailure("Image preflight (pull/build)", err, "Confirm image tags/access settings, then rerun quickstart.")
	}

	printQuickstartStageHeader("Migrations")
	if err := migrationsRunner(*composeFile, *envFile); err != nil {
		return quickstartStageFailure("Migrations", err, "Check database container logs and rerun quickstart.")
	}

	composeUpArgs := []string{"--file", *composeFile, "--env-file", *envFile}
	if *limits {
		composeUpArgs = append(composeUpArgs, "--limits")
	}
	composeUpArgs = append(composeUpArgs, "--image-source", modeCfg.mode)
	if *build && modeCfg.mode == deployImageSourceBuild {
		composeUpArgs = append(composeUpArgs, "--build")
	}

	printQuickstartStageHeader("Compose up")
	if err := composeUpRunner(composeUpArgs); err != nil {
		return quickstartStageFailure("Compose up", err, "Run `docker compose --file deploy/docker-compose.yml --env-file .env logs --tail=120` to inspect startup issues, then rerun quickstart.")
	}

	printQuickstartStageHeader("Wait for readiness")
	if err := quickstartWaiter(envValues, *composeFile, *envFile); err != nil {
		return quickstartStageFailure("Wait for readiness", err, "Give the stack a bit more time, check service logs, then rerun quickstart.")
	}

	printQuickstartStageHeader("Health checks")
	if err := quickstartComposeHealthWaiter(*composeFile, *envFile); err != nil {
		return quickstartStageFailure("Health checks", err, "Run compose logs for unhealthy services, fix the root cause, then rerun quickstart.")
	}

	printQuickstartStageHeader("Admin bootstrap")
	if err := bootstrapAdminRunner(*composeFile, *envFile, envValues); err != nil {
		return quickstartStageFailure("Admin bootstrap", err, "Confirm admin env values and database readiness, then rerun quickstart.")
	}

	printGeneratedSecrets(generatedSecrets)
	printQuickstartSuccessSummary(*composeFile, *envFile, envValues)
	return nil
}

func runDockerVersionPreflight() error {
	if _, err := lookPathRunner("docker"); err != nil {
		return fmt.Errorf("docker CLI not found: %w\nNext: install Docker Desktop/Engine and ensure `docker` is on PATH", err)
	}
	if err := commandRunner("docker", "version"); err != nil {
		return fmt.Errorf("docker daemon check failed (`docker version`): %w\nNext: start Docker Desktop/Engine and verify `docker version` succeeds", err)
	}
	return nil
}

func runDockerComposeVersionPreflight() error {
	if err := commandRunner("docker", "compose", "version"); err != nil {
		return fmt.Errorf("docker compose v2 check failed (`docker compose version`): %w\nNext: enable/install Docker Compose v2 and verify `docker compose version` succeeds", err)
	}
	return nil
}

type quickstartPortRequirement struct {
	name     string
	protocol string
	ports    []int
}

type quickstartPortConflict struct {
	name     string
	protocol string
	port     int
	err      error
}

func runQuickstartHostPortPreflight(values map[string]string) error {
	requirements, err := quickstartRequiredHostPorts(values)
	if err != nil {
		return err
	}

	var conflicts []quickstartPortConflict
	for _, req := range requirements {
		for _, port := range req.ports {
			if err := quickstartHostPortAvailabilityChecker(req.protocol, port); err != nil {
				conflicts = append(conflicts, quickstartPortConflict{name: req.name, protocol: req.protocol, port: port, err: err})
			}
		}
	}

	if len(conflicts) == 0 {
		return nil
	}

	lines := []string{"host port conflicts detected:"}
	for _, conflict := range conflicts {
		lines = append(lines, fmt.Sprintf("- %s %d (%s): %v", strings.ToUpper(conflict.protocol), conflict.port, conflict.name, conflict.err))
	}
	lines = append(lines, "Next: stop the conflicting local service, or change the matching .env port value and rerun quickstart")
	return errors.New(strings.Join(lines, "\n"))
}

func quickstartRequiredHostPorts(values map[string]string) ([]quickstartPortRequirement, error) {
	tcp := "tcp"
	udp := "udp"
	requirements := []quickstartPortRequirement{}

	addSingle := func(name, key, fallback, protocol string) error {
		port, err := parseRequiredPort(values, key, fallback)
		if err != nil {
			return err
		}
		requirements = append(requirements, quickstartPortRequirement{name: name, protocol: protocol, ports: []int{port}})
		return nil
	}

	addPrefer := func(name, primaryKey, primaryFallback, secondaryKey, secondaryFallback, protocol string) error {
		port, err := parsePreferredPort(values, primaryKey, primaryFallback, secondaryKey, secondaryFallback)
		if err != nil {
			return err
		}
		requirements = append(requirements, quickstartPortRequirement{name: name, protocol: protocol, ports: []int{port}})
		return nil
	}

	if err := addSingle("BITRIVER_LIVE_PORT", "BITRIVER_LIVE_PORT", "8080", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_SRS_CONTROLLER_PORT", "BITRIVER_SRS_CONTROLLER_PORT", "1986", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_SRS_RTMP_PORT", "BITRIVER_SRS_RTMP_PORT", "1935", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_OME_HTTP_PORT", "BITRIVER_OME_HTTP_PORT", "8081", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_OME_HTTP_TLS_PORT", "BITRIVER_OME_HTTP_TLS_PORT", "8082", tcp); err != nil {
		return nil, err
	}
	if err := addPrefer("BITRIVER_OME_LLHLS_HOST_PORT/BITRIVER_OME_LLHLS_PORT", "BITRIVER_OME_LLHLS_HOST_PORT", "", "BITRIVER_OME_LLHLS_PORT", "8080", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_OME_LLHLS_TLS_PORT", "BITRIVER_OME_LLHLS_TLS_PORT", "8443", tcp); err != nil {
		return nil, err
	}
	if err := addPrefer("BITRIVER_OME_SIGNALLING_PORT/BITRIVER_OME_SERVER_PORT", "BITRIVER_OME_SIGNALLING_PORT", "", "BITRIVER_OME_SERVER_PORT", "9000", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_OME_SERVER_TLS_PORT", "BITRIVER_OME_SERVER_TLS_PORT", "9443", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_OME_RELAY_PORT", "BITRIVER_OME_RELAY_PORT", "3478", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_OME_RELAY_PORT", "BITRIVER_OME_RELAY_PORT", "3478", udp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_TRANSCODER_HOST_PORT", "BITRIVER_TRANSCODER_HOST_PORT", "9001", tcp); err != nil {
		return nil, err
	}
	if err := addSingle("BITRIVER_TRANSCODER_PUBLIC_PORT", "BITRIVER_TRANSCODER_PUBLIC_PORT", "9080", tcp); err != nil {
		return nil, err
	}

	icePorts, err := parseRequiredPortRange(values, "BITRIVER_OME_ICE_PORT_RANGE", "10000-10009")
	if err != nil {
		return nil, err
	}
	requirements = append(requirements, quickstartPortRequirement{name: "BITRIVER_OME_ICE_PORT_RANGE", protocol: udp, ports: icePorts})

	return requirements, nil
}

func parseRequiredPort(values map[string]string, key, fallback string) (int, error) {
	value := strings.TrimSpace(values[key])
	if value == "" {
		value = fallback
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return 0, fmt.Errorf("invalid %s value %q: expected TCP/UDP port 1-65535", key, value)
	}
	return port, nil
}

func parsePreferredPort(values map[string]string, primaryKey, primaryFallback, secondaryKey, secondaryFallback string) (int, error) {
	if strings.TrimSpace(values[primaryKey]) != "" {
		return parseRequiredPort(values, primaryKey, primaryFallback)
	}
	return parseRequiredPort(values, secondaryKey, secondaryFallback)
}

func parseRequiredPortRange(values map[string]string, key, fallback string) ([]int, error) {
	value := strings.TrimSpace(values[key])
	if value == "" {
		value = fallback
	}
	parts := strings.Split(value, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid %s value %q: expected range start-end", key, value)
	}
	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q: expected numeric start", key, value)
	}
	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid %s value %q: expected numeric end", key, value)
	}
	if start < 1 || end > 65535 || start > end {
		return nil, fmt.Errorf("invalid %s value %q: expected 1 <= start <= end <= 65535", key, value)
	}

	ports := make([]int, 0, end-start+1)
	for p := start; p <= end; p++ {
		ports = append(ports, p)
	}
	return ports, nil
}

func printQuickstartStageHeader(stage string) {
	fmt.Fprintf(os.Stdout, "\n[%s]\n", stage)
}

func quickstartStageFailure(stage string, err error, next string) error {
	return fmt.Errorf("quickstart stopped during %s\n\nWhat happened:\n%s\n\nWhat to do next:\n%s", stage, strings.TrimSpace(err.Error()), strings.TrimSpace(next))
}

func printQuickstartSuccessSummary(composeFile, envFile string, values map[string]string) {
	apiPort := resolveAPIPort(values)
	apiURL := fmt.Sprintf("http://localhost:%s", apiPort)
	viewerURL := strings.TrimSpace(values["NEXT_PUBLIC_VIEWER_URL"])
	if viewerURL == "" {
		viewerURL = fmt.Sprintf("http://localhost:%s/viewer", apiPort)
	}
	adminEmail := strings.TrimSpace(values["BITRIVER_LIVE_ADMIN_EMAIL"])
	if adminEmail == "" {
		adminEmail = "(not set)"
	}

	fmt.Fprintln(os.Stdout, "\nBitRiver Live is running")
	fmt.Fprintf(os.Stdout, "- Control/API URL: %s\n", apiURL)
	fmt.Fprintf(os.Stdout, "- Viewer URL: %s\n", viewerURL)
	fmt.Fprintf(os.Stdout, "- Admin sign-in URL: %s\n", resolveAdminSignInURL(values))
	fmt.Fprintf(os.Stdout, "- Admin email: %s\n", adminEmail)
	fmt.Fprintf(os.Stdout, "- Env file: %s\n", envFile)
	fmt.Fprintf(os.Stdout, "- Bootstrap credentials: stored in %s (use the CLI's `env admin` subcommand later if you need this summary again)\n", envFile)
	fmt.Fprintln(os.Stdout, "- Password note: if you rotate the admin password later in /admin, the env-backed bootstrap password becomes a historical seed value.")
	fmt.Fprintln(os.Stdout, "- Useful next commands:")
	fmt.Fprintf(os.Stdout, "  - docker compose --file %s --env-file %s ps\n", composeFile, envFile)
	fmt.Fprintf(os.Stdout, "  - docker compose --file %s --env-file %s logs -f\n", composeFile, envFile)
	fmt.Fprintf(os.Stdout, "  - docker compose --file %s --env-file %s down\n", composeFile, envFile)
}

func printBootstrapAdminAccessSummary(out io.Writer, envPath string, values map[string]string, showPassword bool) error {
	adminEmail := strings.TrimSpace(values["BITRIVER_LIVE_ADMIN_EMAIL"])
	if adminEmail == "" {
		return errors.New("BITRIVER_LIVE_ADMIN_EMAIL missing from environment")
	}

	password := strings.TrimSpace(values["BITRIVER_LIVE_ADMIN_PASSWORD"])

	fmt.Fprintln(out, "Bootstrap admin access")
	fmt.Fprintf(out, "- Admin sign-in URL: %s\n", resolveAdminSignInURL(values))
	fmt.Fprintf(out, "- Admin email: %s\n", adminEmail)
	fmt.Fprintf(out, "- Env file: %s\n", envPath)
	switch {
	case password == "":
		fmt.Fprintln(out, "- Bootstrap password: (not set in env)")
	case showPassword:
		fmt.Fprintf(out, "- Bootstrap password: %s\n", password)
	default:
		fmt.Fprintln(out, "- Bootstrap password: hidden by default; rerun with --show-password to reveal the env-backed seed credential")
	}
	fmt.Fprintln(out, "- Note: the env-backed bootstrap password is only the original seed credential. If you changed it later in /admin, use the newer password instead.")

	return nil
}

func resolveAdminSignInURL(values map[string]string) string {
	if baseURL := resolveControlBaseURL(values); baseURL != "" {
		return strings.TrimRight(baseURL, "/") + "/admin"
	}
	return fmt.Sprintf("http://localhost:%s/admin", resolveAPIPort(values))
}

func resolveControlBaseURL(values map[string]string) string {
	for _, candidate := range []struct {
		rawValue   string
		trimSuffix string
	}{
		{rawValue: values["NEXT_PUBLIC_API_BASE_URL"], trimSuffix: "/api"},
		{rawValue: values["NEXT_PUBLIC_VIEWER_URL"], trimSuffix: "/viewer"},
	} {
		if normalized := normalizePublicBaseURL(candidate.rawValue, candidate.trimSuffix); normalized != "" {
			return normalized
		}
	}

	return fmt.Sprintf("http://localhost:%s", resolveAPIPort(values))
}

func normalizePublicBaseURL(rawValue, trimSuffix string) string {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	cleanPath := strings.TrimRight(parsed.EscapedPath(), "/")
	lowerPath := strings.ToLower(cleanPath)
	lowerSuffix := strings.ToLower(strings.TrimSpace(trimSuffix))
	if lowerSuffix != "" && strings.HasSuffix(lowerPath, lowerSuffix) {
		cleanPath = cleanPath[:len(cleanPath)-len(lowerSuffix)]
	}
	cleanPath = strings.TrimRight(cleanPath, "/")

	parsed.Path = cleanPath
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return strings.TrimRight(parsed.String(), "/")
}

func resolveDeployImageSource(explicitMode string, envValues map[string]string, runtimeEnv config.Environment) (deployImageSourceConfig, error) {
	mode := strings.ToLower(strings.TrimSpace(explicitMode))
	if mode == "" {
		mode = config.LoadDeployImageSourceFromEnv(runtimeEnv).Mode
	}
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(envValues["BITRIVER_DEPLOY_IMAGE_SOURCE"]))
	}
	if mode == "" {
		mode = deployImageSourcePull
	}

	switch mode {
	case deployImageSourcePull:
		return deployImageSourceConfig{mode: mode, composeArgs: []string{"--pull", "always", "--no-build"}, description: "registry pulls only"}, nil
	case deployImageSourceBuild:
		return deployImageSourceConfig{mode: mode, composeArgs: []string{"--build", "--pull", "never"}, description: "local source builds"}, nil
	default:
		return deployImageSourceConfig{}, fmt.Errorf("invalid BITRIVER_DEPLOY_IMAGE_SOURCE value %q (expected pull or build)", mode)
	}
}

func runDeployImageSourcePreflight(mode string, envValues map[string]string, envFile string) error {
	switch mode {
	case deployImageSourcePull:
		return runPullImagePreflight(envValues)
	case deployImageSourceBuild:
		return runBuildImagePreflight(envFile)
	default:
		return fmt.Errorf("unsupported deploy image source mode %q", mode)
	}
}

func runPullImagePreflight(envValues map[string]string) error {
	imageRefs, err := requiredGHCRImageRefs(envValues)
	if err != nil {
		return err
	}
	for _, imageRef := range imageRefs {
		if err := manifestInspectRunner(imageRef); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "denied") || strings.Contains(msg, "unauthorized") || strings.Contains(msg, "forbidden") {
				return fmt.Errorf("GHCR pull preflight failed for %s: access denied.\nRun `docker login ghcr.io` with a token that has read access to the package, then retry", imageRef)
			}
			if strings.Contains(msg, "no such manifest") || strings.Contains(msg, "manifest unknown") || strings.Contains(msg, "not found") {
				return fmt.Errorf("GHCR pull preflight failed for %s: manifest not found.\nVerify tag/digest values in BITRIVER_*_IMAGE_TAG and BITRIVER_*_IMAGE_DIGEST", imageRef)
			}
			return fmt.Errorf("GHCR pull preflight failed for %s: %w", imageRef, err)
		}
	}
	fmt.Fprintf(os.Stdout, "Image preflight ok: verified %d GHCR manifests for pull mode.\n", len(imageRefs))
	return nil
}

func requiredGHCRImageRefs(values map[string]string) ([]string, error) {
	refs := []struct {
		name      string
		tagKey    string
		digestKey string
	}{
		{name: "ghcr.io/bitriver-live/bitriver-live", tagKey: "BITRIVER_LIVE_IMAGE_TAG", digestKey: "BITRIVER_LIVE_IMAGE_DIGEST"},
		{name: "ghcr.io/bitriver-live/bitriver-viewer", tagKey: "BITRIVER_VIEWER_IMAGE_TAG", digestKey: "BITRIVER_VIEWER_IMAGE_DIGEST"},
		{name: "ghcr.io/bitriver-live/bitriver-srs-controller", tagKey: "BITRIVER_SRS_CONTROLLER_IMAGE_TAG", digestKey: "BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST"},
		{name: "ghcr.io/bitriver-live/bitriver-transcoder", tagKey: "BITRIVER_TRANSCODER_IMAGE_TAG", digestKey: "BITRIVER_TRANSCODER_IMAGE_DIGEST"},
	}

	resolved := make([]string, 0, len(refs))
	for _, ref := range refs {
		tag := strings.TrimSpace(values[ref.tagKey])
		if tag == "" {
			return nil, fmt.Errorf("GHCR preflight requires %s to be set", ref.tagKey)
		}
		digest := strings.TrimSpace(values[ref.digestKey])
		if digest != "" && !strings.HasPrefix(digest, "@") {
			return nil, fmt.Errorf("%s must start with @ when set (current: %s)", ref.digestKey, digest)
		}
		resolved = append(resolved, fmt.Sprintf("%s:%s%s", ref.name, tag, digest))
	}
	return resolved, nil
}

func runDockerManifestInspect(imageRef string) error {
	cmd := exec.Command("docker", "manifest", "inspect", imageRef)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return err
		}
		return errors.New(trimmed)
	}
	return nil
}

func runBuildImagePreflight(envFile string) error {
	paths := []string{
		filepath.Join(repoRoot(), "Dockerfile"),
		filepath.Join(repoRoot(), "web", "viewer", "Dockerfile"),
		filepath.Join(repoRoot(), "cmd", "srs-controller", "Dockerfile"),
		filepath.Join(repoRoot(), "cmd", "transcoder", "Dockerfile"),
		filepath.Join(repoRoot(), "deploy", "ome-config", "Dockerfile"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("build preflight failed: required local source file is missing: %s", path)
			}
			return fmt.Errorf("build preflight failed while checking %s: %w", path, err)
		}
	}
	fmt.Fprintf(os.Stdout, "Image preflight ok: local build prerequisites found for build mode (env: %s).\n", envFile)
	return nil
}

func validateProductionDeploymentContract(envFile string, values map[string]string, imageSourceMode string) error {
	if !strings.EqualFold(strings.TrimSpace(values["BITRIVER_LIVE_MODE"]), "production") {
		return nil
	}

	if !strings.EqualFold(strings.TrimSpace(values["BITRIVER_LIVE_STORAGE_DRIVER"]), "postgres") {
		return fmt.Errorf("production deployment contract failed for %s: BITRIVER_LIVE_STORAGE_DRIVER must be postgres", envFile)
	}

	if imageSourceMode != deployImageSourcePull {
		return fmt.Errorf("production deployment contract failed for %s: image source mode must be %q (current: %q)", envFile, deployImageSourcePull, imageSourceMode)
	}

	if err := validateQuickstartProductionRequirements(envFile, values); err != nil {
		return err
	}

	if err := validatePinnedGHCRImageDigests(values); err != nil {
		return fmt.Errorf("production deployment contract failed for %s: %w", envFile, err)
	}

	return nil
}

func validatePinnedGHCRImageDigests(values map[string]string) error {
	refs := []string{
		"BITRIVER_LIVE_IMAGE_DIGEST",
		"BITRIVER_VIEWER_IMAGE_DIGEST",
		"BITRIVER_SRS_CONTROLLER_IMAGE_DIGEST",
		"BITRIVER_TRANSCODER_IMAGE_DIGEST",
	}

	missing := make([]string, 0, len(refs))
	for _, key := range refs {
		digest := strings.TrimSpace(values[key])
		if digest == "" {
			missing = append(missing, key)
			continue
		}
		if !strings.HasPrefix(digest, "@") {
			return fmt.Errorf("%s must start with @ when set (current: %s)", key, digest)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("pinned image digests are required in production; set %s", strings.Join(missing, ", "))
	}

	return nil
}

func validateQuickstartProductionRequirements(envFile string, values map[string]string) error {
	if !strings.EqualFold(strings.TrimSpace(values["BITRIVER_LIVE_MODE"]), "production") {
		return nil
	}

	issues := []string{}
	loopbackURL := func(value string) bool {
		lower := strings.ToLower(strings.TrimSpace(value))
		return strings.HasPrefix(lower, "http://localhost") ||
			strings.HasPrefix(lower, "https://localhost") ||
			strings.HasPrefix(lower, "http://127.") ||
			strings.HasPrefix(lower, "https://127.") ||
			strings.HasPrefix(lower, "http://0.0.0.0") ||
			strings.HasPrefix(lower, "https://0.0.0.0") ||
			strings.HasPrefix(lower, "http://[::1]") ||
			strings.HasPrefix(lower, "https://[::1]")
	}
	loopbackHost := func(value string) bool {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		return trimmed == "localhost" || trimmed == "0.0.0.0" || trimmed == "::" || trimmed == "::1" || strings.HasPrefix(trimmed, "127.")
	}

	if val := strings.TrimSpace(values["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"]); val == "" || loopbackURL(val) {
		issues = append(issues, fmt.Sprintf("- BITRIVER_TRANSCODER_PUBLIC_BASE_URL=%q must be a public/routable URL (example: https://cdn.example.com/hls)", valueOrDefault(val, "<empty>")))
	}
	if val := strings.TrimSpace(values["NEXT_PUBLIC_VIEWER_URL"]); val == "" || loopbackURL(val) {
		issues = append(issues, fmt.Sprintf("- NEXT_PUBLIC_VIEWER_URL=%q must be a public/routable viewer URL (example: https://viewer.example.com)", valueOrDefault(val, "<empty>")))
	}
	if val := strings.TrimSpace(values["BITRIVER_OME_BIND"]); val == "" || loopbackHost(val) {
		issues = append(issues, fmt.Sprintf("- BITRIVER_OME_BIND=%q must be a routable bind/interface value for production", valueOrDefault(val, "<empty>")))
	}
	if val := strings.TrimSpace(values["BITRIVER_OME_IP"]); val == "" || loopbackHost(val) {
		issues = append(issues, fmt.Sprintf("- BITRIVER_OME_IP=%q must be the public/routable OME host or IP", valueOrDefault(val, "<empty>")))
	}

	if len(issues) == 0 {
		return nil
	}

	return fmt.Errorf("quickstart-prod validation failed for %s:\nBITRIVER_LIVE_MODE=production requires explicit production values.\nFix these entries, then rerun go run ./cmd/bitriver quickstart ...\n%s", envFile, strings.Join(issues, "\n"))
}

func validateQuickstartOMEAuthMode(rawAuthMode, envFile string) error {
	rawAuthMode = strings.TrimSpace(rawAuthMode)
	authMode := strings.ToLower(rawAuthMode)
	if authMode == "" {
		authMode = "accesstoken"
	}

	switch authMode {
	case "accesstoken", "basic", "none", "off", "disabled":
		return nil
	default:
		return fmt.Errorf("OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE must be accesstoken, basic, or none/off/disabled (current: %s).\nSet BITRIVER_OME_HEALTHCHECK_AUTH_MODE=accesstoken for token probes, or:\n  BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic\n  BITRIVER_OME_USERNAME=ome-operator\n  BITRIVER_OME_PASSWORD=replace-with-strong-password\nin %s before running quickstart", valueOrDefault(rawAuthMode, "<empty>"), envFile)
	}
}

func runQuickstartOMEAuthPreflight(envFile string, values map[string]string) error {
	rawAuthMode := strings.TrimSpace(values["BITRIVER_OME_HEALTHCHECK_AUTH_MODE"])
	authMode := strings.ToLower(rawAuthMode)
	if authMode == "" {
		authMode = "accesstoken"
	}

	if err := validateQuickstartOMEAuthMode(rawAuthMode, envFile); err != nil {
		return err
	}

	if strings.TrimSpace(values["BITRIVER_OME_API_TOKEN"]) == "" {
		return fmt.Errorf("OME auth preflight failed: BITRIVER_OME_API_TOKEN is empty in %s.\nExpected BITRIVER_OME_API_TOKEN=<non-empty token> so OME render and healthchecks share one canonical token contract", envFile)
	}

	if authMode == "basic" {
		if strings.TrimSpace(values["BITRIVER_OME_USERNAME"]) == "" || strings.TrimSpace(values["BITRIVER_OME_PASSWORD"]) == "" {
			return fmt.Errorf("OME auth preflight failed: BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic requires BITRIVER_OME_USERNAME and BITRIVER_OME_PASSWORD in %s.\nExample:\n  BITRIVER_OME_HEALTHCHECK_AUTH_MODE=basic\n  BITRIVER_OME_USERNAME=ome-operator\n  BITRIVER_OME_PASSWORD=replace-with-strong-password", envFile)
		}
	}

	fmt.Fprintln(os.Stdout, "Running OME auth preflight: rendering config and validating token consistency ...")
	if err := omeRunner([]string{"render", "--env-file", envFile, "--force"}); err != nil {
		return fmt.Errorf("render OME config: %w", err)
	}

	configPath := filepath.Join(repoRoot(), "deploy", "ome", "Server.generated.xml")
	if err := omeVerifyHealthTokenRunner([]string{"--env-file", envFile, "--config", configPath}); err != nil {
		return err
	}

	return nil
}

func valueOrDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

// runMigrations runs migrations and exits when the work completes or a dependency fails.
func runMigrations(composeFile, envFile string) error {
	if _, err := lookPathRunner("docker"); err != nil {
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
var readinessWaitTimeout = 2 * time.Minute
var readinessPollInterval = 3 * time.Second
var readinessDiagnosticsRunner = gatherReadinessDiagnostics
var dockerComposeCommandRunner = runDockerComposeCommand

func pollUntil(ctx context.Context, timeout, interval time.Duration, poll func(context.Context) (bool, error)) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return false, err
		}

		done, err := poll(ctx)
		if err != nil {
			return false, err
		}
		if done {
			return true, nil
		}

		wait := interval
		remaining := time.Until(deadline)
		if remaining < wait {
			wait = remaining
		}
		if wait <= 0 {
			break
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return false, ctx.Err()
		case <-timer.C:
		}
	}

	if err := ctx.Err(); err != nil {
		return false, err
	}

	return false, nil
}

func waitForAPIReadiness(values map[string]string, composeFile, envFile string) error {
	readyzURL := fmt.Sprintf("http://127.0.0.1:%s/readyz", resolveAPIPort(values))
	fmt.Fprintf(os.Stdout, "Waiting for API readiness at %s...\n", readyzURL)

	ready, err := pollUntil(context.Background(), readinessWaitTimeout, readinessPollInterval, func(ctx context.Context) (bool, error) {
		requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		req, reqErr := http.NewRequestWithContext(requestCtx, http.MethodGet, readyzURL, nil)
		if reqErr != nil {
			return false, fmt.Errorf("build readiness request: %w", reqErr)
		}

		resp, reqErr := http.DefaultClient.Do(req)
		if reqErr == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return true, nil
			}
		}

		return false, nil
	})
	if err != nil {
		return err
	}
	if ready {
		fmt.Fprintln(os.Stdout, "API is ready.")
		return nil
	}

	diagnostics := strings.TrimSpace(readinessDiagnosticsRunner(composeFile, envFile))
	if diagnostics == "" {
		return errors.New("API did not become ready before timeout; run `docker compose ps` and `docker compose logs --tail=80 bitriver-live` for diagnostics")
	}

	return fmt.Errorf("API did not become ready before timeout.\n%s", diagnostics)
}

func gatherReadinessDiagnostics(composeFile, envFile string) string {
	var sections []string

	psOutput, psErr := dockerComposeCommandRunner(composeFile, envFile, "ps")
	if psErr != nil {
		sections = append(sections, fmt.Sprintf("docker compose ps failed: %v", psErr))
	} else if trimmed := strings.TrimSpace(psOutput); trimmed != "" {
		sections = append(sections, "docker compose ps:\n"+trimmed)
	}

	if criticalStatus := gatherCriticalServiceStatusSection(composeFile, envFile); criticalStatus != "" {
		sections = append(sections, criticalStatus)
	}

	logsOutput, logsErr := dockerComposeCommandRunner(composeFile, envFile, "logs", "--tail=80", "bitriver-live")
	if logsErr != nil {
		sections = append(sections, fmt.Sprintf("docker compose logs --tail=80 bitriver-live failed: %v", logsErr))
	} else {
		keyLines := extractKeyLogLines(logsOutput)
		if len(keyLines) > 0 {
			sections = append(sections, "bitriver-live key log lines:\n"+strings.Join(keyLines, "\n"))
		}
		if hint := detectPostgresStubHint(logsOutput); hint != "" {
			sections = append(sections, hint)
		}
	}

	return strings.Join(sections, "\n\n")
}

func gatherCriticalServiceStatusSection(composeFile, envFile string) string {
	output, err := composePSRunner(composeFile, envFile)
	if err != nil {
		return fmt.Sprintf("critical service status unavailable: %v", err)
	}

	serviceStates, err := parseComposeServiceStates(output)
	if err != nil {
		return fmt.Sprintf("critical service status unavailable: parse docker compose ps output: %v", err)
	}

	statusLines, nextSteps := summarizeCriticalServiceStates(serviceStates, composeFile, envFile)
	if len(statusLines) == 0 {
		return ""
	}

	section := "critical service status:\n" + strings.Join(statusLines, "\n")
	if len(nextSteps) > 0 {
		section += "\nnext commands:\n" + strings.Join(nextSteps, "\n")
	}
	return section
}

func runDockerComposeCommand(composeFile, envFile string, extraArgs ...string) (string, error) {
	args := append(composeArgsWithEnv(composeFile, envFile), extraArgs...)
	cmd := exec.Command("docker", args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed != "" {
			return "", fmt.Errorf("%w: %s", err, trimmed)
		}
		return "", err
	}
	return trimmed, nil
}

func extractKeyLogLines(logs string) []string {
	keywords := []string{"error", "fatal", "panic", "postgres repository unavailable", "pgx driver stubbed in this build"}
	lines := strings.Split(logs, "\n")
	var keyLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				keyLines = append(keyLines, trimmed)
				break
			}
		}
		if len(keyLines) >= 12 {
			break
		}
	}
	return keyLines
}

func detectPostgresStubHint(logs string) string {
	lower := strings.ToLower(logs)
	if strings.Contains(lower, "pgx driver stubbed in this build") || strings.Contains(lower, "postgres repository unavailable") {
		return "Hint: bitriver-live appears to be running without the Postgres module wired in. Rebuild the image/binary with the real pgx-backed storage implementation included and verify module wiring in go.mod/build tags."
	}
	return ""
}

type quickstartComposeServiceStatus struct {
	Service string `json:"Service"`
	Name    string `json:"Name"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Health  string `json:"Health"`
}

type quickstartCriticalService struct {
	name    string
	compose string
}

var criticalComposeServices = []quickstartCriticalService{
	{name: "api", compose: "bitriver-live"},
	{name: "postgres", compose: "postgres"},
	{name: "redis", compose: "redis"},
	{name: "srs", compose: "srs"},
	{name: "ome", compose: "ome"},
	{name: "transcoder", compose: "transcoder"},
}

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

func summarizeCriticalServiceStates(serviceStates map[string]quickstartComposeServiceStatus, composeFile, envFile string) ([]string, []string) {
	statusLines := make([]string, 0, len(criticalComposeServices))
	nextSteps := []string{}

	for _, svc := range criticalComposeServices {
		state, ok := serviceStates[svc.compose]
		if !ok {
			statusLines = append(statusLines, fmt.Sprintf("- %s: missing", svc.name))
			continue
		}

		currentState := strings.ToLower(strings.TrimSpace(state.State))
		health := strings.ToLower(strings.TrimSpace(state.Health))
		currentStatus := strings.TrimSpace(state.Status)
		if currentStatus == "" {
			currentStatus = currentState
		}

		if health != "" {
			statusLines = append(statusLines, fmt.Sprintf("- %s: %s (health=%s)", svc.name, currentStatus, health))
		} else {
			statusLines = append(statusLines, fmt.Sprintf("- %s: %s", svc.name, currentStatus))
		}

		if health == "unhealthy" || currentState == "exited" || currentState == "dead" {
			nextSteps = append(nextSteps, fmt.Sprintf("- %s: run `%s`", svc.name, composeServiceLogCommand(composeFile, envFile, svc.compose)))
		}
	}

	return statusLines, nextSteps
}

func composeServiceLogCommand(composeFile, envFile, service string) string {
	return fmt.Sprintf("docker compose --file %s --env-file %s logs --tail=80 %s", composeFile, envFile, service)
}

func waitForComposeServiceHealth(composeFile, envFile string) error {
	fmt.Fprintln(os.Stdout, "Waiting for critical Docker Compose services to report healthy...")
	lastSummary := map[string]string{}
	lastStates := map[string]quickstartComposeServiceStatus{}

	allHealthy, err := pollUntil(context.Background(), composeHealthWaitTimeout, composeHealthPollInterval, func(context.Context) (bool, error) {
		output, err := composePSRunner(composeFile, envFile)
		if err != nil {
			return false, err
		}

		serviceStates, err := parseComposeServiceStates(output)
		if err != nil {
			return false, fmt.Errorf("parse docker compose ps output: %w", err)
		}

		allHealthy := true
		failingDetails := []string{}
		nextSteps := []string{}
		for _, svc := range criticalComposeServices {
			state, ok := serviceStates[svc.compose]
			if !ok {
				allHealthy = false
				lastSummary[svc.name] = "missing"
				continue
			}
			lastStates[svc.name] = state

			health := strings.ToLower(strings.TrimSpace(state.Health))
			currentState := strings.ToLower(strings.TrimSpace(state.State))
			currentStatus := strings.TrimSpace(state.Status)
			if currentStatus == "" {
				currentStatus = currentState
			}
			lastSummary[svc.name] = currentStatus

			if health == "unhealthy" || currentState == "exited" || currentState == "dead" {
				failingDetails = append(failingDetails, fmt.Sprintf("%s=%s (state=%s, health=%s)", svc.name, currentStatus, currentState, health))
				nextSteps = append(nextSteps, fmt.Sprintf("- %s: run `%s`", svc.name, composeServiceLogCommand(composeFile, envFile, svc.compose)))
			}

			if health != "healthy" {
				allHealthy = false
			}
		}

		if len(failingDetails) > 0 {
			return false, fmt.Errorf("critical service failure detected: %s\nnext commands:\n%s", strings.Join(failingDetails, ", "), strings.Join(nextSteps, "\n"))
		}

		if allHealthy {
			return true, nil
		}

		return false, nil
	})
	if err != nil {
		return err
	}
	if allHealthy {
		fmt.Fprintln(os.Stdout, "Critical services are healthy.")
		return nil
	}

	var statusParts []string
	failingDetails := []string{}
	nextSteps := []string{}
	for _, svc := range criticalComposeServices {
		status := lastSummary[svc.name]
		if status == "" {
			status = "unknown"
		}
		statusParts = append(statusParts, fmt.Sprintf("%s=%s", svc.name, status))

		state, ok := lastStates[svc.name]
		if !ok {
			failingDetails = append(failingDetails, fmt.Sprintf("%s=missing", svc.name))
			continue
		}
		health := strings.ToLower(strings.TrimSpace(state.Health))
		currentState := strings.ToLower(strings.TrimSpace(state.State))
		if health != "healthy" {
			failingDetails = append(failingDetails, fmt.Sprintf("%s=%s (state=%s, health=%s)", svc.name, status, currentState, health))
			nextSteps = append(nextSteps, fmt.Sprintf("- %s: run `%s`", svc.name, composeServiceLogCommand(composeFile, envFile, svc.compose)))
		}
	}

	message := fmt.Sprintf("critical services did not become healthy before timeout; last known states: %s", strings.Join(statusParts, ", "))
	if len(failingDetails) > 0 {
		message += fmt.Sprintf("\nfailing services: %s", strings.Join(failingDetails, ", "))
	}
	if len(nextSteps) > 0 {
		message += "\nnext commands:\n" + strings.Join(nextSteps, "\n")
	}
	return errors.New(message)
}

func parseComposeServiceStates(raw []byte) (map[string]quickstartComposeServiceStatus, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil, errors.New("empty JSON payload")
	}

	var services []quickstartComposeServiceStatus
	if err := json.Unmarshal([]byte(trimmed), &services); err != nil {
		decoder := json.NewDecoder(strings.NewReader(trimmed))
		for {
			var svc quickstartComposeServiceStatus
			if decodeErr := decoder.Decode(&svc); decodeErr != nil {
				if errors.Is(decodeErr, io.EOF) {
					break
				}
				return nil, err
			}
			services = append(services, svc)
		}
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
	if storageDriver != "" && storageDriver != "postgres" {
		return fmt.Errorf("unsupported storage driver %q for bootstrap-admin", storageDriver)
	}

	args := append(composeArgsWithEnv(composeFile, envFile), "exec", "-T", "bitriver-live", "/app/bootstrap-admin")
	dsn, err := buildPostgresDSN(values)
	if err != nil {
		return err
	}
	args = append(args, "--postgres-dsn", dsn)

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
