package main

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

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

var forbiddenPlaceholders = map[string]string{
	"BITRIVER_POSTGRES_PASSWORD":              "P0stgres-Example!",
	"BITRIVER_REDIS_PASSWORD":                 "R3dis-Example!",
	"BITRIVER_LIVE_ADMIN_EMAIL":               "admin@stream.example.com",
	"BITRIVER_LIVE_ADMIN_PASSWORD":            "Sup3rSecureAdmin!",
	"BITRIVER_SRS_TOKEN":                      "srs-secure-token-example",
	"BITRIVER_OME_PASSWORD":                   "OME-Example-Pass!",
	"BITRIVER_OME_API_TOKEN":                  "OME-Example-Access-Token",
	"BITRIVER_OME_ACCESS_TOKEN":               "OME-Example-Access-Token",
	"BITRIVER_TRANSCODER_TOKEN":               "transcoder-secure-token-example",
	"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD": "R3dis-Example!",
	"BITRIVER_TRANSCODER_PUBLIC_BASE_URL":     "https://cdn.example.com/hls",
	"NEXT_PUBLIC_VIEWER_URL":                  "https://stream.example.com/viewer",
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

	generated := generateEnvValues(existingValues)
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

	if !runDoctor(nil) {
		return errors.New("doctor checks failed")
	}

	if err := runEnvInit([]string{"--env-file", *envFile}); err != nil {
		return fmt.Errorf("env init: %w", err)
	}
	if err := runEnvValidate([]string{"--env-file", *envFile}); err != nil {
		return fmt.Errorf("env validate: %w", err)
	}

	if err := runOME([]string{"render", "--env-file", *envFile, "--force"}); err != nil {
		return fmt.Errorf("render OME config: %w", err)
	}

	if err := runMigrations(*composeFile); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	if err := runComposeUp([]string{"--file", *composeFile}); err != nil {
		return fmt.Errorf("docker compose up: %w", err)
	}

	return nil
}

func runMigrations(composeFile string) error {
	if _, err := executil.LookPath("docker"); err != nil {
		return err
	}

	args := []string{"compose", "--file", composeFile, "run", "--rm", "postgres-migrations"}
	return commandRunner("docker", args...)
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
	if err := fs.Parse(args); err != nil {
		return err
	}

	scriptPath := filepath.Join(repoRoot(), "scripts", "render-ome-config.sh")
	if _, err := executil.LookPath("python3"); err != nil {
		return fmt.Errorf("python3 is required to render OME config: %w", err)
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return fmt.Errorf("renderer script missing at %s: %w", scriptPath, err)
	}

	renderArgs := []string{scriptPath}
	if *checkOnly {
		renderArgs = append(renderArgs, "--check")
	}
	if *force {
		renderArgs = append(renderArgs, "--force")
	}
	renderArgs = append(renderArgs, "--env-file", *envPath)

	return commandRunner("bash", renderArgs...)
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

func generateEnvValues(existing map[string]string) map[string]string {
	generated := make(map[string]string)

	generated["BITRIVER_LIVE_MODE"] = firstNonEmpty(existing["BITRIVER_LIVE_MODE"], "development")
	generated["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"] = defaultIfPlaceholder("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", existing, "http://localhost:9001/hls")
	generated["NEXT_PUBLIC_VIEWER_URL"] = defaultIfPlaceholder("NEXT_PUBLIC_VIEWER_URL", existing, "http://localhost:8080/viewer")
	generated["BITRIVER_OME_ACCESS_TOKEN"] = existing["BITRIVER_OME_ACCESS_TOKEN"]

	for key := range defaultEnvSecrets.secrets {
		current := existing[key]
		if current == "" || isForbiddenValue(key, current) {
			generated[key] = randomSecret()
		}
	}

	if v := existing["BITRIVER_OME_API_TOKEN"]; generated["BITRIVER_OME_ACCESS_TOKEN"] == "" && v != "" {
		generated["BITRIVER_OME_ACCESS_TOKEN"] = v
	}

	if val := existing["BITRIVER_REDIS_PASSWORD"]; val != "" && !isForbiddenValue("BITRIVER_REDIS_PASSWORD", val) {
		generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = firstNonEmpty(existing["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"], val)
	} else {
		generated["BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD"] = generated["BITRIVER_REDIS_PASSWORD"]
	}

	if existing["BITRIVER_LIVE_ADMIN_EMAIL"] == "" || isForbiddenValue("BITRIVER_LIVE_ADMIN_EMAIL", existing["BITRIVER_LIVE_ADMIN_EMAIL"]) {
		generated["BITRIVER_LIVE_ADMIN_EMAIL"] = defaultEnvSecrets.adminEmail
	}

	return generated
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

func validateEnv(values map[string]string) envValidatorResult {
	requiredVars := []string{
		"BITRIVER_LIVE_IMAGE_TAG",
		"BITRIVER_VIEWER_IMAGE_TAG",
		"BITRIVER_SRS_CONTROLLER_IMAGE_TAG",
		"BITRIVER_TRANSCODER_IMAGE_TAG",
		"BITRIVER_SRS_IMAGE_TAG",
		"BITRIVER_OME_IMAGE_TAG",
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

	res := envValidatorResult{}

	for _, key := range requiredVars {
		if strings.TrimSpace(values[key]) == "" {
			res.Missing = append(res.Missing, key)
		}
	}

	for key, placeholder := range forbiddenPlaceholders {
		if strings.TrimSpace(values[key]) == placeholder {
			res.Blocked = append(res.Blocked, key)
		}
	}

	if val := strings.TrimSpace(values["BITRIVER_OME_IMAGE_TAG"]); val != "" {
		if parts := strings.Split(val, "."); len(parts) != 3 {
			res.Errors = append(res.Errors, fmt.Sprintf("BITRIVER_OME_IMAGE_TAG must be MAJOR.MINOR.PATCH (current: %s)", val))
		}
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

	mode := strings.ToLower(strings.TrimSpace(values["BITRIVER_LIVE_MODE"]))
	production := mode == "" || mode == "production"

	if val := strings.TrimSpace(values["BITRIVER_TRANSCODER_PUBLIC_BASE_URL"]); val != "" {
		if strings.Contains(val, "localhost") || strings.Contains(val, "127.0.0.1") {
			warn := fmt.Sprintf("BITRIVER_TRANSCODER_PUBLIC_BASE_URL points at loopback (%s). Configure a routable origin before production.", val)
			if production {
				res.Errors = append(res.Errors, warn)
			} else {
				res.Warnings = append(res.Warnings, warn)
			}
		}
	}

	if val := strings.TrimSpace(values["NEXT_PUBLIC_VIEWER_URL"]); val != "" {
		if strings.Contains(val, "example.com") {
			res.Errors = append(res.Errors, fmt.Sprintf("NEXT_PUBLIC_VIEWER_URL still uses an example.com placeholder (%s)", val))
		} else if strings.Contains(val, "localhost") || strings.HasPrefix(val, "http://127.") || strings.HasPrefix(val, "https://127.") {
			warn := fmt.Sprintf("NEXT_PUBLIC_VIEWER_URL points at loopback (%s).", val)
			if production {
				res.Errors = append(res.Errors, warn)
			} else {
				res.Warnings = append(res.Warnings, warn)
			}
		}
	}

	if val := strings.TrimSpace(values["NEXT_PUBLIC_API_BASE_URL"]); val != "" {
		if strings.Contains(val, "example.com") {
			res.Errors = append(res.Errors, fmt.Sprintf("NEXT_PUBLIC_API_BASE_URL still uses an example.com placeholder (%s)", val))
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
