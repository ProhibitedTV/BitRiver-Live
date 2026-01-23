package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"bitriver-live/internal/executil"
)

var (
	Version = "dev"
	Commit  = "dev"
	Date    = "unknown"
)

var cachedRepoRoot string

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		runVersion(os.Args[2:])
	case "doctor":
		if !runDoctor(os.Args[2:]) {
			os.Exit(1)
		}
	case "env":
		if err := runEnv(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "compose":
		if err := runCompose(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "desktop":
		if err := runDesktop(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "install":
		if err := runInstall(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "quickstart":
		if err := runQuickstart(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "ome":
		if err := runOME(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command>\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  version   Show BitRiver Live version information")
	fmt.Fprintln(os.Stderr, "  doctor    Check local environment for BitRiver Live")
	fmt.Fprintln(os.Stderr, "  env       Initialize or validate environment files")
	fmt.Fprintln(os.Stderr, "  compose   Run docker compose up/down with defaults")
	fmt.Fprintln(os.Stderr, "  desktop   Launch the Docker Compose control panel with tray shortcuts")
	fmt.Fprintln(os.Stderr, "  install   Stage binaries/configs and emit service templates")
	fmt.Fprintln(os.Stderr, "  quickstart  Run doctor, env init/validate, render OME config, migrations, and compose up")
	fmt.Fprintln(os.Stderr, "  ome       Render OME configuration from .env")
}

func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s version\n", os.Args[0])
	}
	_ = fs.Parse(args)

	printVersionInfo(os.Stdout)
}

func printVersionInfo(out io.Writer) {
	fmt.Fprintf(out, "Version: %s\n", valueOrFallback(Version, "dev"))
	fmt.Fprintf(out, "Commit: %s\n", valueOrFallback(Commit, "unknown"))
	fmt.Fprintf(out, "Date: %s\n", valueOrFallback(Date, "unknown"))
}

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

func valueOrFallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

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

func defaultEnvFile() string {
	return filepath.Join(repoRoot(), ".env")
}

func defaultComposeFile() string {
	return filepath.Join(repoRoot(), "deploy", "docker-compose.yml")
}

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

func init() {
	placeholders, err := loadSampleCredentialValues(defaultExampleEnv(), sampleCredentialKeys)
	if err != nil {
		placeholderLoadErr = err
		return
	}
	forbiddenPlaceholders = placeholders
}

func defaultForbiddenPlaceholders() map[string]string {
	return map[string]string{
		"BITRIVER_LIVE_METRICS_TOKEN":             "metrics-collector-token",
		"BITRIVER_POSTGRES_PASSWORD":              "P0stgres-Example!",
		"BITRIVER_REDIS_PASSWORD":                 "R3dis-Example!",
		"BITRIVER_LIVE_ADMIN_EMAIL":               "admin@stream.example.com",
		"BITRIVER_LIVE_ADMIN_PASSWORD":            "Sup3rSecureAdmin!",
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

	existingValues, err := readEnvFile(*envPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	generated, _ := generateEnvValues(existingValues)
	content := mergeEnv(templateLines, existingValues, generated)
	if err := os.WriteFile(*envPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}

	fmt.Fprintf(os.Stdout, "Wrote environment file to %s\n", *envPath)
	return nil
}

func runEnvValidate(args []string) error {
	fs := flag.NewFlagSet("env validate", flag.ContinueOnError)
	envPath := fs.String("env-file", defaultEnvFile(), "path to validate")
	if err := fs.Parse(args); err != nil {
		return err
	}

	values, err := readEnvFile(*envPath)
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

	fmt.Fprintf(os.Stdout, "Environment file %s looks ready.\n", *envPath)
	return nil
}

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
var bootstrapAdminRunner = runBootstrapAdmin
var migrationsRunner = runMigrations
var composeUpRunner = runComposeUp
var envInitRunner = runEnvInit
var envValidateRunner = runEnvValidate
var omeRunner = runOME
var doctorRunner = runDoctor

func runComposeUp(args []string) error {
	fs := flag.NewFlagSet("compose up", flag.ContinueOnError)
	composeFile := fs.String("file", defaultComposeFile(), "compose file to use")
	detach := fs.Bool("detached", true, "run docker compose in detached mode")
	build := fs.Bool("build", true, "build images before starting")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := executil.LookPath("docker"); err != nil {
		return err
	}

	composeArgs := []string{"compose", "--file", *composeFile, "up"}
	if *build {
		composeArgs = append(composeArgs, "--build")
	}
	if *detach {
		composeArgs = append(composeArgs, "-d")
	}

	return commandRunner("docker", composeArgs...)
}

func runComposeDown(args []string) error {
	fs := flag.NewFlagSet("compose down", flag.ContinueOnError)
	composeFile := fs.String("file", defaultComposeFile(), "compose file to use")
	volumes := fs.Bool("volumes", false, "remove named volumes")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := executil.LookPath("docker"); err != nil {
		return err
	}

	composeArgs := []string{"compose", "--file", *composeFile, "down"}
	if *volumes {
		composeArgs = append(composeArgs, "-v")
	}
	return commandRunner("docker", composeArgs...)
}

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
	if existingValues, err := readEnvFile(*envFile); err == nil {
		preExisting = existingValues
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read env file before init: %w", err)
	}
	preExistingCopy := copyEnvValues(preExisting)
	_, generatedSecrets := generateEnvValues(preExistingCopy)

	if err := envInitRunner([]string{"--env-file", *envFile}); err != nil {
		return fmt.Errorf("env init: %w", err)
	}
	if err := envValidateRunner([]string{"--env-file", *envFile}); err != nil {
		return fmt.Errorf("env validate: %w", err)
	}

	envValues, err := readEnvFile(*envFile)
	if err != nil {
		return fmt.Errorf("read env file: %w", err)
	}

	if err := omeRunner([]string{"render", "--env-file", *envFile, "--force"}); err != nil {
		return fmt.Errorf("render OME config: %w", err)
	}

	if err := migrationsRunner(*composeFile); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if err := composeUpRunner([]string{"--file", *composeFile}); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}

	if err := quickstartWaiter(envValues); err != nil {
		return fmt.Errorf("wait for API readiness: %w", err)
	}

	if err := bootstrapAdminRunner(*composeFile, envValues); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}

	printGeneratedSecrets(generatedSecrets)
	return nil
}

func runMigrations(composeFile string) error {
	if _, err := executil.LookPath("docker"); err != nil {
		return err
	}

	args := []string{"compose", "--file", composeFile, "run", "--rm", "postgres-migrations"}
	return commandRunner("docker", args...)
}

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

func runBootstrapAdmin(composeFile string, values map[string]string) error {
	email := strings.TrimSpace(values["BITRIVER_LIVE_ADMIN_EMAIL"])
	password := strings.TrimSpace(values["BITRIVER_LIVE_ADMIN_PASSWORD"])
	if email == "" || password == "" {
		return errors.New("admin email/password missing from environment")
	}

	storageDriver := strings.ToLower(strings.TrimSpace(values["BITRIVER_LIVE_STORAGE_DRIVER"]))
	if storageDriver == "" {
		storageDriver = "postgres"
	}

	args := []string{"compose", "--file", composeFile, "exec", "-T", "bitriver-live", "/app/bootstrap-admin"}
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

func runOME(args []string) error {
	if len(args) == 0 {
		return errors.New("ome subcommand required")
	}

	switch args[0] {
	case "render":
		return runOMERender(args[1:])
	default:
		return fmt.Errorf("unknown ome subcommand: %s", args[0])
	}
}

func runOMERender(args []string) error {
	fs := flag.NewFlagSet("ome render", flag.ContinueOnError)
	envPath := fs.String("env-file", defaultEnvFile(), "path to env file")
	force := fs.Bool("force", false, "force regeneration")
	checkOnly := fs.Bool("check", false, "only verify the file exists")
	quiet := fs.Bool("quiet", false, "suppress informational output")
	if err := fs.Parse(args); err != nil {
		return err
	}

	return renderOMEFromEnv(*envPath, *force, *checkOnly, *quiet)
}

func renderOMEFromEnv(envPath string, force, checkOnly, quiet bool) error {
	templatePath := filepath.Join(repoRoot(), "deploy", "ome", "Server.xml")
	outputPath := filepath.Join(repoRoot(), "deploy", "ome", "Server.generated.xml")

	if checkOnly {
		if err := validateOMEGeneratedConfig(outputPath); err != nil {
			return err
		}
		if !quiet {
			fmt.Fprintf(os.Stdout, "OME config found at %s.\n", outputPath)
		}
		return nil
	}

	values, err := readEnvFile(envPath)
	if err != nil {
		return err
	}

	cfg, err := buildOMERenderConfig(values, templatePath, outputPath)
	if err != nil {
		return err
	}

	if !force {
		if _, err := os.Stat(outputPath); err == nil {
			if err := validateOMEGeneratedConfig(outputPath); err != nil {
				return fmt.Errorf("invalid OME config at %s (re-render with --force): %w", outputPath, err)
			}
			if !quiet {
				fmt.Fprintf(os.Stdout, "OME config already exists at %s (use --force to regenerate).\n", outputPath)
			}
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("failed to check generated config: %w", err)
		}
	}

	if !quiet {
		if force {
			fmt.Fprintln(os.Stdout, "Rendering OME config (--force requested)...")
		} else {
			fmt.Fprintln(os.Stdout, "Rendering OME config...")
		}
	}

	if err := renderOMEConfig(cfg); err != nil {
		return fmt.Errorf("render deploy/ome/Server.generated.xml: %w", err)
	}

	if err := validateOMEGeneratedConfig(outputPath); err != nil {
		return fmt.Errorf("validate generated OME config: %w", err)
	}

	if !quiet {
		fmt.Fprintf(os.Stdout, "Rendered OME configuration to %s\n", outputPath)
	}

	return nil
}

type omeRenderConfig struct {
	TemplatePath string
	OutputPath   string
	Bind         string
	ServerIP     string
	Port         string
	TLSPort      string
	Username     string
	Password     string
	APIToken     string
	AccessToken  string
	ImageTag     string
	TCPRelay     string
	ICECandidate string
}

var omeTestDefaults = map[string]string{
	"BITRIVER_OME_USERNAME":  "ome-test-user",
	"BITRIVER_OME_PASSWORD":  "ome-test-pass",
	"BITRIVER_OME_API_TOKEN": "ome-test-access-token",
	// BITRIVER_OME_ACCESS_TOKEN falls back to BITRIVER_OME_API_TOKEN when unset.
	"BITRIVER_OME_ACCESS_TOKEN": "ome-test-access-token",
}

func buildOMERenderConfig(values map[string]string, templatePath, outputPath string) (omeRenderConfig, error) {
	if _, err := os.Stat(templatePath); err != nil {
		return omeRenderConfig{}, fmt.Errorf("OME template missing at %s: %w", templatePath, err)
	}

	bind := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_BIND"]), "0.0.0.0")
	port := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_SERVER_PORT"]), "9000")
	tlsPort := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_SERVER_TLS_PORT"]), "9443")
	ip := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_IP"]), bind)
	imageTag := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_IMAGE_TAG"]), "0.16.0")
	icePortRange := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_ICE_PORT_RANGE"]), "10000-10009")
	tcpRelay := firstNonEmpty(strings.TrimSpace(values["BITRIVER_OME_TCP_RELAY"]), strings.TrimSpace(values["BITRIVER_OME_RELAY_PORT"]), "3478")
	if !strings.Contains(tcpRelay, ":") {
		tcpRelay = "*:" + strings.Trim(tcpRelay, "*:")
	}
	iceCandidate := strings.TrimSpace(values["BITRIVER_OME_ICE_CANDIDATE"])
	if iceCandidate == "" {
		iceCandidate = fmt.Sprintf("*:%s/udp", icePortRange)
	}

	username := strings.TrimSpace(values["BITRIVER_OME_USERNAME"])
	password := strings.TrimSpace(values["BITRIVER_OME_PASSWORD"])
	apiToken := strings.TrimSpace(values["BITRIVER_OME_API_TOKEN"])
	accessToken := strings.TrimSpace(values["BITRIVER_OME_ACCESS_TOKEN"])
	if accessToken == "" {
		accessToken = apiToken
	}

	missing := make([]string, 0)
	for key, value := range map[string]string{
		"BITRIVER_OME_USERNAME":        username,
		"BITRIVER_OME_PASSWORD":        password,
		"BITRIVER_OME_API_TOKEN":       apiToken,
		"BITRIVER_OME_SERVER_PORT":     port,
		"BITRIVER_OME_SERVER_TLS_PORT": tlsPort,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return omeRenderConfig{}, fmt.Errorf("missing required OME variables: %s", strings.Join(missing, ", "))
	}

	for key, forbidden := range omeTestDefaults {
		switch key {
		case "BITRIVER_OME_ACCESS_TOKEN":
			if strings.TrimSpace(accessToken) == forbidden {
				return omeRenderConfig{}, fmt.Errorf("%s is set to ome-test-* default; provide deployment credentials before rendering", key)
			}
		default:
			if strings.TrimSpace(values[key]) == forbidden {
				return omeRenderConfig{}, fmt.Errorf("%s is set to ome-test-* default; provide deployment credentials before rendering", key)
			}
		}
	}

	return omeRenderConfig{
		TemplatePath: templatePath,
		OutputPath:   outputPath,
		Bind:         bind,
		ServerIP:     ip,
		Port:         port,
		TLSPort:      tlsPort,
		Username:     username,
		Password:     password,
		APIToken:     apiToken,
		AccessToken:  accessToken,
		ImageTag:     imageTag,
		TCPRelay:     tcpRelay,
		ICECandidate: iceCandidate,
	}, nil
}

func renderOMEConfig(cfg omeRenderConfig) error {
	data, err := os.ReadFile(cfg.TemplatePath)
	if err != nil {
		return fmt.Errorf("read template: %w", err)
	}

	text := string(data)
	text = regexp.MustCompile(`<\s*Server\.bind\s*>`).ReplaceAllString(text, "<Bind>")
	text = regexp.MustCompile(`</\s*Server\.bind\s*>`).ReplaceAllString(text, "</Bind>")

	text, err = replaceRootBindings(text, xmlEscape(cfg.Bind), xmlEscape(cfg.Port), xmlEscape(cfg.TLSPort))
	if err != nil {
		return err
	}

	text, err = replaceRootIP(text, xmlEscape(cfg.ServerIP))
	if err != nil {
		return err
	}

	text, err = scopedReplaceControlBindings(text, xmlEscape(cfg.Bind))
	if err != nil {
		return err
	}

	text, err = ensureIceCandidatesTag(text, "TcpRelay", xmlEscape(cfg.TCPRelay), cfg.TemplatePath)
	if err != nil {
		return err
	}

	text, err = ensureIceCandidatesTag(text, "IceCandidate", xmlEscape(cfg.ICECandidate), cfg.TemplatePath)
	if err != nil {
		return err
	}

	text, err = replaceAccessToken(text, cfg.AccessToken)
	if err != nil {
		return err
	}

	text, err = replaceAuthentication(text, cfg.Username, cfg.Password)
	if err != nil {
		return err
	}

	text = stampImageTag(text, cfg.ImageTag)

	text = regexp.MustCompile("\\n{3,}").ReplaceAllString(text, "\n\n")

	if err := os.WriteFile(cfg.OutputPath, []byte(text), 0o644); err != nil {
		return fmt.Errorf("write generated config: %w", err)
	}

	return nil
}

func validateOMEGeneratedConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("OME config missing at %s: %w", path, err)
	}

	contents := string(data)
	for key, forbidden := range omeTestDefaults {
		if strings.Contains(contents, forbidden) {
			return fmt.Errorf("%s still set to ome-test-* default in %s", key, path)
		}
	}

	return nil
}

func replaceTagContent(data, tag, value string) (string, error) {
	openTag := fmt.Sprintf("<%s>", tag)
	closeTag := fmt.Sprintf("</%s>", tag)

	start := strings.Index(data, openTag)
	if start == -1 {
		return "", fmt.Errorf("missing %s in template", openTag)
	}

	end := strings.Index(data[start:], closeTag)
	if end == -1 {
		return "", fmt.Errorf("missing %s in template", closeTag)
	}

	end += start
	return data[:start+len(openTag)] + value + data[end:], nil
}

func replaceAllTagContent(data, tag, value string, required bool) (string, error) {
	pattern := regexp.MustCompile(fmt.Sprintf(`(<%s>)([^<]*)(</%s>)`, tag, tag))
	replaced := pattern.ReplaceAllString(data, fmt.Sprintf(`$1%s$3`, value))
	if required && replaced == data {
		return "", fmt.Errorf("missing <%s> in template", tag)
	}
	return replaced, nil
}

func ensureIceCandidatesTag(text, tag, value, templatePath string) (string, error) {
	iceRe := regexp.MustCompile(`(?s)<IceCandidates>(.*?)</IceCandidates>`)
	matches := 0
	rewriteErr := error(nil)
	updated := iceRe.ReplaceAllStringFunc(text, func(section string) string {
		matches++
		if strings.Contains(section, "<"+tag+">") {
			replaced, err := replaceAllTagContent(section, tag, value, false)
			if err != nil {
				rewriteErr = err
				return section
			}
			return replaced
		}

		closing := "</IceCandidates>"
		insertPos := strings.LastIndex(section, closing)
		if insertPos == -1 {
			return section
		}

		indent := "    "
		if indentMatch := regexp.MustCompile(`\n([ \t]*)</IceCandidates>`).FindStringSubmatch(section); indentMatch != nil {
			indent = indentMatch[1]
		}
		childIndent := indent + "    "
		insertion := fmt.Sprintf("\n%s<%s>%s</%s>", childIndent, tag, value, tag)
		return section[:insertPos] + insertion + section[insertPos:]
	})

	if rewriteErr != nil {
		return "", rewriteErr
	}
	if matches == 0 {
		return "", fmt.Errorf("missing <IceCandidates> block in %s while injecting <%s>", templatePath, tag)
	}
	return updated, nil
}

func replaceRootBindings(text, address, port, tlsPort string) (string, error) {
	serverRe := regexp.MustCompile(`(?s)<Server[^>]*>(.*)</Server>`)
	serverLoc := serverRe.FindStringSubmatchIndex(text)
	if serverLoc == nil {
		return "", errors.New("missing <Server> root element in template")
	}

	serverBody := text[serverLoc[2]:serverLoc[3]]
	bindRe := regexp.MustCompile(`(?s)<Bind>(.*?)</Bind>`)
	bindLoc := bindRe.FindStringSubmatchIndex(serverBody)
	if bindLoc == nil {
		return "", errors.New("missing <Bind> section under <Server> in template")
	}

	bindBody := serverBody[bindLoc[2]:bindLoc[3]]
	var err error
	if strings.Contains(bindBody, "<Address>") {
		bindBody, err = replaceTagContent(bindBody, "Address", address)
	} else if strings.Contains(bindBody, "<IP>") {
		bindBody, err = replaceTagContent(bindBody, "IP", address)
	}
	if err != nil {
		return "", err
	}

	signallingRe := regexp.MustCompile(`(?s)<Signalling>(.*?)</Signalling>`)
	rewriteErr := error(nil)
	signallingCount := 0
	bindBody = signallingRe.ReplaceAllStringFunc(bindBody, func(section string) string {
		signallingCount++
		match := signallingRe.FindStringSubmatch(section)
		inner := match[1]
		updated, errPort := replaceTagContent(inner, "Port", port)
		if errPort != nil {
			rewriteErr = errPort
			return section
		}
		updated, errPort = replaceTagContent(updated, "TLSPort", tlsPort)
		if errPort != nil {
			rewriteErr = errPort
			return section
		}
		return "<Signalling>" + updated + "</Signalling>"
	})
	if rewriteErr != nil {
		return "", rewriteErr
	}

	if signallingCount == 0 {
		bindBody, err = replaceTagContent(bindBody, "Port", port)
		if err != nil {
			return "", err
		}
		bindBody, err = replaceTagContent(bindBody, "TLSPort", tlsPort)
		if err != nil {
			return "", err
		}
	}

	serverBody = serverBody[:bindLoc[2]] + bindBody + serverBody[bindLoc[3]:]
	return text[:serverLoc[2]] + serverBody + text[serverLoc[3]:], nil
}

func replaceRootIP(text, ip string) (string, error) {
	serverRe := regexp.MustCompile(`(?s)<Server[^>]*>(.*)</Server>`)
	serverLoc := serverRe.FindStringSubmatchIndex(text)
	if serverLoc == nil {
		return "", errors.New("missing <Server> root element in template")
	}

	serverBody := text[serverLoc[2]:serverLoc[3]]
	ipRe := regexp.MustCompile(`(?s)<IP>(.*?)</IP>`)
	matches := ipRe.FindAllStringSubmatchIndex(serverBody, -1)
	for _, loc := range matches {
		start, end := loc[2], loc[3]
		bindOpen := strings.LastIndex(serverBody[:start], "<Bind>")
		bindClose := strings.LastIndex(serverBody[:start], "</Bind>")
		if bindOpen != -1 && (bindClose == -1 || bindClose < bindOpen) {
			continue
		}

		vhostOpen := strings.LastIndex(serverBody[:start], "<VirtualHosts>")
		vhostClose := strings.LastIndex(serverBody[:start], "</VirtualHosts>")
		if vhostOpen != -1 && (vhostClose == -1 || vhostClose < vhostOpen) {
			continue
		}

		serverBody = serverBody[:start] + ip + serverBody[end:]
		return text[:serverLoc[2]] + serverBody + text[serverLoc[3]:], nil
	}

	return text, nil
}

func scopedReplaceControlBindings(text, bind string) (string, error) {
	controlRe := regexp.MustCompile(`(?s)<Control>(.*?)</Control>`)
	controlLoc := controlRe.FindStringSubmatchIndex(text)
	if controlLoc == nil {
		return text, nil
	}

	controlBody := text[controlLoc[0]:controlLoc[1]]
	serverRe := regexp.MustCompile(`(?s)<Server>(.*?)</Server>`)
	serverLoc := serverRe.FindStringSubmatchIndex(controlBody)
	if serverLoc == nil {
		return text, nil
	}

	serverBody := controlBody[serverLoc[0]:serverLoc[1]]
	inner := serverLoc[2] - serverLoc[0]
	outer := serverLoc[3] - serverLoc[0]
	content := serverBody[inner:outer]

	var err error
	if strings.Contains(content, "<Bind>") {
		content, err = replaceAllTagContent(content, "Bind", bind, false)
		if err != nil {
			return "", err
		}
	}
	if strings.Contains(content, "<IP>") {
		content, err = replaceAllTagContent(content, "IP", bind, false)
		if err != nil {
			return "", err
		}
	}
	if strings.Contains(content, "<Address>") {
		content, err = replaceAllTagContent(content, "Address", bind, false)
		if err != nil {
			return "", err
		}
	}

	serverBody = serverBody[:inner] + content + serverBody[outer:]
	controlBody = controlBody[:serverLoc[0]] + serverBody + controlBody[serverLoc[1]:]
	return text[:controlLoc[0]] + controlBody + text[controlLoc[1]:], nil
}

func replaceAccessToken(text, token string) (string, error) {
	token = xmlEscape(token)
	accessTokensRe := regexp.MustCompile(`(?s)<AccessTokens>(.*?)</AccessTokens>`)
	loc := accessTokensRe.FindStringSubmatchIndex(text)
	if loc != nil {
		inner := text[loc[2]:loc[3]]
		replaced, err := replaceTagContent(inner, "AccessToken", token)
		if err != nil {
			return "", err
		}
		return text[:loc[2]] + replaced + text[loc[3]:], nil
	}

	if strings.Contains(text, "<AccessToken>") {
		replaced, err := replaceTagContent(text, "AccessToken", token)
		if err != nil {
			return "", err
		}
		return replaced, nil
	}

	return "", errors.New("missing <AccessTokens> or <AccessToken> in template")
}

func replaceAuthentication(text, username, password string) (string, error) {
	authRe := regexp.MustCompile(`(?s)<Authentication>(.*?)</Authentication>`)
	loc := authRe.FindStringSubmatchIndex(text)
	if loc == nil {
		return "", errors.New("missing <Authentication> block in template")
	}

	inner := text[loc[2]:loc[3]]
	var err error
	inner, err = replaceTagContent(inner, "ID", xmlEscape(username))
	if err != nil {
		return "", err
	}
	inner, err = replaceTagContent(inner, "Password", xmlEscape(password))
	if err != nil {
		return "", err
	}

	return text[:loc[2]] + inner + text[loc[3]:], nil
}

func stampImageTag(text, imageTag string) string {
	if strings.TrimSpace(imageTag) == "" {
		return text
	}

	marker := fmt.Sprintf("<!-- Rendered for BITRIVER_OME_IMAGE_TAG=%s -->", xmlEscape(imageTag))
	pattern := regexp.MustCompile(`<!--\s*Rendered for BITRIVER_OME_IMAGE_TAG=.*?-->`)
	if pattern.MatchString(text) {
		return pattern.ReplaceAllString(text, marker)
	}

	return strings.Replace(text, "<Server version=\"10\">", "<Server version=\"10\">\n    "+marker, 1)
}

func xmlEscape(value string) string {
	return html.EscapeString(value)
}

func loadSampleCredentialValues(path string, keys []string) (map[string]string, error) {
	templateLines, err := readEnvTemplate(path)
	if err != nil {
		return nil, fmt.Errorf("load sample credentials: %w", err)
	}

	allowed := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		allowed[key] = struct{}{}
	}

	placeholders := make(map[string]string)
	for _, line := range templateLines {
		if line.Key == "" {
			continue
		}
		if _, ok := allowed[line.Key]; ok {
			placeholders[line.Key] = line.Value
		}
	}

	var missing []string
	for _, key := range keys {
		if _, ok := placeholders[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing sample credentials for %s in %s", strings.Join(missing, ", "), path)
	}

	return placeholders, nil
}

func readEnvTemplate(path string) ([]templateLine, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}
	defer file.Close()

	var lines []templateLine
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		raw := scanner.Text()
		key, value, ok := parseEnvLine(raw)
		if ok {
			lines = append(lines, templateLine{Key: key, Value: value, Raw: raw})
			continue
		}
		lines = append(lines, templateLine{Raw: raw})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan template: %w", err)
	}
	return lines, nil
}

func readEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseEnvLine(scanner.Text())
		if ok {
			values[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan env: %w", err)
	}
	return values, nil
}

type templateLine struct {
	Key   string
	Value string
	Raw   string
}

func parseEnvLine(line string) (string, string, bool) {
	if strings.HasPrefix(strings.TrimSpace(line), "#") || !strings.Contains(line, "=") {
		return "", "", false
	}
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func mergeEnv(template []templateLine, existing, generated map[string]string) string {
	seen := make(map[string]struct{})
	var out []string

	for _, line := range template {
		if line.Key == "" {
			out = append(out, line.Raw)
			continue
		}

		seen[line.Key] = struct{}{}

		value := line.Value
		if v, ok := existing[line.Key]; ok && strings.TrimSpace(v) != "" {
			value = v
		} else if v, ok := generated[line.Key]; ok {
			value = v
		}
		out = append(out, fmt.Sprintf("%s=%s", line.Key, value))
	}

	extraKeys := make([]string, 0, len(existing))
	for k := range existing {
		if _, ok := seen[k]; !ok {
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)
	for _, k := range extraKeys {
		out = append(out, fmt.Sprintf("%s=%s", k, existing[k]))
	}

	return strings.Join(out, "\n") + "\n"
}

func generateEnvValues(existing map[string]string) (map[string]string, map[string]string) {
	generated := make(map[string]string)
	newlyGenerated := make(map[string]string)

	generated["BITRIVER_LIVE_MODE"] = firstNonEmpty(existing["BITRIVER_LIVE_MODE"], "development")
	generated["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"] = defaultIfPlaceholder("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", existing, "http://localhost:9001/hls")
	generated["NEXT_PUBLIC_VIEWER_URL"] = defaultIfPlaceholder("NEXT_PUBLIC_VIEWER_URL", existing, "http://localhost:8080/viewer")
	generated["BITRIVER_OME_ACCESS_TOKEN"] = existing["BITRIVER_OME_ACCESS_TOKEN"]

	for key := range defaultEnvSecrets.secrets {
		current := existing[key]
		if current == "" || isForbiddenValue(key, current) {
			secret := randomSecret()
			generated[key] = secret
			newlyGenerated[key] = secret
		}
	}

	if current := existing["BITRIVER_LIVE_METRICS_TOKEN"]; current == "" || isForbiddenValue("BITRIVER_LIVE_METRICS_TOKEN", current) {
		if generated["BITRIVER_LIVE_METRICS_TOKEN"] == "" {
			secret := randomSecret()
			generated["BITRIVER_LIVE_METRICS_TOKEN"] = secret
			newlyGenerated["BITRIVER_LIVE_METRICS_TOKEN"] = secret
		}
		existing["BITRIVER_LIVE_METRICS_TOKEN"] = ""
	}

	if current := existing["BITRIVER_OME_USERNAME"]; current == "" || isForbiddenValue("BITRIVER_OME_USERNAME", current) {
		generated["BITRIVER_OME_USERNAME"] = fmt.Sprintf("ome-operator-%s", randomSuffix())
		existing["BITRIVER_OME_USERNAME"] = ""
	}

	if v := existing["BITRIVER_OME_API_TOKEN"]; generated["BITRIVER_OME_ACCESS_TOKEN"] == "" && v != "" {
		generated["BITRIVER_OME_ACCESS_TOKEN"] = v
	}

	if val := existing["BITRIVER_REDIS_PASSWORD"]; val != "" && !isForbiddenValue("BITRIVER_REDIS_PASSWORD", val) {
		generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = firstNonEmpty(existing["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"], val)
		delete(newlyGenerated, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD")
	} else {
		generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = generated["BITRIVER_REDIS_PASSWORD"]
		if _, ok := newlyGenerated["BITRIVER_REDIS_PASSWORD"]; ok {
			newlyGenerated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"]
		}
	}

	if existing["BITRIVER_LIVE_ADMIN_EMAIL"] == "" || isForbiddenValue("BITRIVER_LIVE_ADMIN_EMAIL", existing["BITRIVER_LIVE_ADMIN_EMAIL"]) {
		generated["BITRIVER_LIVE_ADMIN_EMAIL"] = defaultEnvSecrets.adminEmail
	}

	return generated, newlyGenerated
}

func copyEnvValues(values map[string]string) map[string]string {
	clone := make(map[string]string, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func printGeneratedSecrets(values map[string]string) {
	if len(values) == 0 {
		return
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Generated credentials (store these securely):")
	for _, key := range keys {
		fmt.Fprintf(os.Stdout, "  %s=%s\n", key, values[key])
	}
}

func defaultIfPlaceholder(key string, existing map[string]string, defaultValue string) string {
	if val, ok := existing[key]; ok && val != "" && !isForbiddenValue(key, val) {
		return val
	}
	return defaultValue
}

func isForbiddenValue(key, value string) bool {
	if placeholder, ok := forbiddenPlaceholders[key]; ok && strings.TrimSpace(value) == placeholder {
		return true
	}
	return false
}

func randomSecret() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("failed to generate secret: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func randomSuffix() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("failed to generate suffix: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func postgresSSLModeDisable(dsn string) bool {
	return sslModeDisablePattern.MatchString(dsn)
}

func isComposePostgresDSN(dsn string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(dsn))
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "host=postgres") {
		return true
	}
	if strings.HasPrefix(trimmed, "postgres://") || strings.HasPrefix(trimmed, "postgresql://") {
		parsed, err := url.Parse(trimmed)
		if err == nil && strings.EqualFold(parsed.Hostname(), "postgres") {
			return true
		}
	}
	return false
}

func validateEnv(values map[string]string) envValidatorResult {
	requiredVars := []string{
		"BITRIVER_POSTGRES_USER",
		"BITRIVER_POSTGRES_PASSWORD",
		"BITRIVER_REDIS_PASSWORD",
		"BITRIVER_OME_API",
		"BITRIVER_OME_BIND",
		"BITRIVER_OME_IP",
		"BITRIVER_OME_SERVER_PORT",
		"BITRIVER_OME_SERVER_TLS_PORT",
		"BITRIVER_LIVE_ADMIN_EMAIL",
		"BITRIVER_LIVE_ADMIN_PASSWORD",
		"BITRIVER_LIVE_SESSION_TTL",
		"BITRIVER_LIVE_ALLOW_SELF_SIGNUP",
		"BITRIVER_SRS_TOKEN",
		"BITRIVER_OME_USERNAME",
		"BITRIVER_OME_PASSWORD",
		"BITRIVER_OME_API_TOKEN",
		"BITRIVER_OME_ACCESS_TOKEN",
		"BITRIVER_TRANSCODER_TOKEN",
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD",
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL",
		"NEXT_PUBLIC_VIEWER_URL",
	}

	imageTags := []string{
		"BITRIVER_LIVE_IMAGE_TAG",
		"BITRIVER_VIEWER_IMAGE_TAG",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG",
		"BITRIVER_TRANSCODER_IMAGE_TAG",
		"BITRIVER_SRS_IMAGE_TAG",
		"BITRIVER_OME_IMAGE_TAG",
	}

	mode := strings.ToLower(strings.TrimSpace(values["BITRIVER_LIVE_MODE"]))
	production := mode == "" || mode == "production"

	res := envValidatorResult{}

	if placeholderLoadErr != nil {
		res.Errors = append(res.Errors, fmt.Sprintf("failed to load sample credentials from %s: %v", defaultExampleEnv(), placeholderLoadErr))
	}

	for _, key := range requiredVars {
		if strings.TrimSpace(values[key]) == "" {
			res.Missing = append(res.Missing, key)
		}
	}

	for _, key := range imageTags {
		if strings.TrimSpace(values[key]) == "" {
			res.Missing = append(res.Missing, key)
		}
	}

	for key, placeholder := range forbiddenPlaceholders {
		if strings.TrimSpace(values[key]) == placeholder {
			res.Blocked = append(res.Blocked, key)
		}
	}

	if values["BITRIVER_REDIS_PASSWORD"] != "" && values["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] != "" &&
		values["BITRIVER_REDIS_PASSWORD"] != values["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] {
		res.Warnings = append(res.Warnings, "BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD does not match BITRIVER_REDIS_PASSWORD. Ensure Redis credentials stay in sync unless intentionally different.")
	}

	tlsCert := strings.TrimSpace(values["BITRIVER_LIVE_TLS_CERT"])
	tlsKey := strings.TrimSpace(values["BITRIVER_LIVE_TLS_KEY"])

	if (tlsCert == "") != (tlsKey == "") {
		res.Errors = append(res.Errors, "BITRIVER_LIVE_TLS_CERT and BITRIVER_LIVE_TLS_KEY must both be set to enable HTTPS.")
	}

	httpsRequested := false
	for _, candidate := range []string{values["NEXT_PUBLIC_API_BASE_URL"], values["NEXT_PUBLIC_VIEWER_URL"]} {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(candidate)), "https://") {
			httpsRequested = true
			break
		}
	}

	if tlsCert != "" && tlsKey != "" {
		if _, err := os.Stat(tlsCert); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_TLS_CERT is set to %s but is not readable: %v", tlsCert, err))
		}
		if _, err := os.Stat(tlsKey); err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_TLS_KEY is set to %s but is not readable: %v", tlsKey, err))
		}
	} else if httpsRequested {
		res.Errors = append(res.Errors, "HTTPS URLs are configured for the viewer or API, but BITRIVER_LIVE_TLS_CERT/BITRIVER_LIVE_TLS_KEY are empty. Provide TLS files or terminate HTTPS in front of the service and update the URLs accordingly.")
	}

	metricsToken := strings.TrimSpace(values["BITRIVER_LIVE_METRICS_TOKEN"])
	metricsAllowNetworks := strings.TrimSpace(values["BITRIVER_LIVE_METRICS_ALLOW_NETWORKS"])

	if metricsToken == "" && metricsAllowNetworks == "" {
		message := "production mode requires protecting /metrics with BITRIVER_LIVE_METRICS_TOKEN or BITRIVER_LIVE_METRICS_ALLOW_NETWORKS"
		if production {
			res.Errors = append(res.Errors, message)
		} else {
			res.Warnings = append(res.Warnings, message)
		}
	}

	loginLimitRaw := strings.TrimSpace(values["BITRIVER_LIVE_RATE_LOGIN_LIMIT"])
	loginLimit := 0
	if loginLimitRaw != "" {
		parsed, err := strconv.Atoi(loginLimitRaw)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_RATE_LOGIN_LIMIT must be an integer (current: %s)", loginLimitRaw))
		} else {
			loginLimit = parsed
		}
	}

	if production && (loginLimit <= 0 || loginLimitRaw == "") {
		res.Errors = append(res.Errors, "production mode requires non-zero login throttling; set BITRIVER_LIVE_RATE_LOGIN_LIMIT")
	}

	for _, key := range []string{"BITRIVER_LIVE_POSTGRES_DSN", "BITRIVER_LIVE_SESSION_POSTGRES_DSN"} {
		if val := strings.TrimSpace(values[key]); val != "" && postgresSSLModeDisable(val) && !isComposePostgresDSN(val) {
			message := fmt.Sprintf("%s disables TLS. Use sslmode=require or verify-full for external Postgres; sslmode=disable is only allowed for the local Compose postgres service.", key)
			if production {
				res.Errors = append(res.Errors, message)
			} else {
				res.Warnings = append(res.Warnings, message)
			}
		}
	}

	if profiles := strings.TrimSpace(values["COMPOSE_PROFILES"]); profiles != "" {
		for _, profile := range strings.FieldsFunc(profiles, func(r rune) bool { return r == ',' || r == ':' }) {
			if profile == "postgres-host" {
				res.Warnings = append(res.Warnings, "COMPOSE_PROFILES includes postgres-host, which publishes PostgreSQL to the host.")
				break
			}
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_IMAGE_TAG"]); val != "" {
		parts := strings.Split(val, ".")
		if len(parts) != 3 {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_OME_IMAGE_TAG must be MAJOR.MINOR.PATCH so the renderer can stamp the config (current: %s)", val))
		} else {
			if parts[0] == "0" {
				minor, _ := strconv.Atoi(parts[1])
				if minor < 16 {
					res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_OME_IMAGE_TAG must be 0.16.0 or newer to match the rendered Server.xml schema (current: %s).", val))
				}
			}
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_SRS_IMAGE_TAG"]); val != "" && val != "v5.0.185" {
		res.Warnings = append(res.Warnings, fmt.Sprintf("BITRIVER_SRS_IMAGE_TAG is set to %s. Update systemd docs or units before upgrading.", val))
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_SERVER_PORT"]); val != "" {
		if portErr := validatePort(val, "BITRIVER_OME_SERVER_PORT"); portErr != "" {
			res.Errors = append(res.Errors, portErr)
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_SERVER_TLS_PORT"]); val != "" {
		if portErr := validatePort(val, "BITRIVER_OME_SERVER_TLS_PORT"); portErr != "" {
			res.Errors = append(res.Errors, portErr)
		}
	}

	if v := strings.TrimSpace(values["BITRIVER_LIVE_ALLOW_SELF_SIGNUP"]); v != "" {
		lower := strings.ToLower(v)
		if lower != "true" && lower != "false" {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_LIVE_ALLOW_SELF_SIGNUP must be true or false (current: %s)", v))
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_LIVE_POSTGRES_DSN"]); strings.Contains(val, "bitriver:bitriver") {
		res.Warnings = append(res.Warnings, "BITRIVER_LIVE_POSTGRES_DSN still references bitriver:bitriver. Update or unset it to match the Postgres credentials.")
	}

	loopback := regexp.MustCompile(`^https?://(localhost|127\\.[0-9.]*|0\\.0\\.0\\.0|::1|\\[::1\\])([:/]|$)`)
	loopbackHost := regexp.MustCompile(`^(localhost|127\\.[0-9.]*|::1|\\[::1\\]|0\\.0\\.0\\.0|::)$`)

	flagEnvIssue := func(message string) {
		if production {
			res.Errors = append(res.Errors, message)
		} else {
			res.Warnings = append(res.Warnings, message)
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"]); val != "" {
		switch {
		case val == "https://cdn.example.com/hls":
			res.Errors = append(res.Errors, "BITRIVER_TRANSCODER_PUBLIC_BASE_URL still uses the sample CDN URL (https://cdn.example.com/hls). Replace it with the public origin end users can reach.")
		case loopback.MatchString(val):
			flagEnvIssue(fmt.Sprintf("BITRIVER_TRANSCODER_PUBLIC_BASE_URL points at loopback (%s). Configure a CDN, reverse proxy, or routable origin instead.", val))
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_API"]); val != "" && loopback.MatchString(val) {
		flagEnvIssue(fmt.Sprintf("BITRIVER_OME_API points at loopback (%s). Use the ome hostname from docker-compose.yml or another reachable host/IP.", val))
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_BIND"]); val != "" && loopbackHost.MatchString(val) {
		flagEnvIssue(fmt.Sprintf("BITRIVER_OME_BIND is set to %s. Bind OvenMediaEngine to a routable interface instead of loopback.", val))
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_IP"]); val != "" && loopbackHost.MatchString(val) {
		flagEnvIssue(fmt.Sprintf("BITRIVER_OME_IP is set to %s. Configure the public IP or hostname for OvenMediaEngine instead of a placeholder or loopback value.", val))
	}

	if val := strings.TrimSpace(values["NEXT_PUBLIC_API_BASE_URL"]); val != "" {
		switch {
		case loopback.MatchString(val):
			flagEnvIssue(fmt.Sprintf("NEXT_PUBLIC_API_BASE_URL points at loopback (%s). Point it at the API hostname end users reach.", val))
		case strings.Contains(val, "example.com"):
			res.Errors = append(res.Errors, fmt.Sprintf("NEXT_PUBLIC_API_BASE_URL still uses an example.com placeholder (%s). Replace it with the production API origin.", val))
		}
	} else {
		viewerBasePath := values["NEXT_VIEWER_BASE_PATH"]
		if viewerBasePath == "" {
			viewerBasePath = "/viewer"
		}
		res.Warnings = append(res.Warnings, fmt.Sprintf("NEXT_PUBLIC_API_BASE_URL is empty; the viewer will fall back to the API origin when proxied at NEXT_VIEWER_BASE_PATH=%s.", viewerBasePath))
	}

	if val := strings.TrimSpace(values["NEXT_PUBLIC_VIEWER_URL"]); val != "" {
		switch {
		case loopback.MatchString(val):
			flagEnvIssue(fmt.Sprintf("NEXT_PUBLIC_VIEWER_URL points at loopback (%s). Point it at the viewer hostname end users reach.", val))
		case strings.Contains(val, "example.com"):
			res.Errors = append(res.Errors, fmt.Sprintf("NEXT_PUBLIC_VIEWER_URL still uses an example.com placeholder (%s). Replace it with the production viewer origin.", val))
		}
	}

	return res
}

func validatePort(value, name string) string {
	port, err := strconv.Atoi(value)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Sprintf("%s must be a valid TCP port number (current: %s)", name, value)
	}

	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
