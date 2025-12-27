package main

import (
	"bytes"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"bitriver-live/internal/envutil"
	"bitriver-live/internal/executil"
)

var (
	version = "dev"
	commit  = "dev"
	date    = "dev"
)

type doctorDeps struct {
	lookPath func(string) (string, error)
	runner   processRunner
	getwd    func() (string, error)
}

type doctorResult struct {
	dockerPath        string
	dockerErr         error
	dockerVersionOut  string
	dockerVersionErr  error
	composeVersionOut string
	composeVersionErr error
	workDir           string
	workDirErr        error
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "version":
		runVersion(os.Args[2:])
	case "doctor":
		runDoctor(os.Args[2:])
	case "env":
		runEnv(os.Args[2:])
	case "ome":
		runOME(os.Args[2:])
	case "compose":
		runCompose(os.Args[2:])
	case "quickstart":
		runQuickstart(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  version   Show BitRiver Live version information")
	fmt.Fprintln(os.Stderr, "  doctor    Run environment diagnostics")
	fmt.Fprintln(os.Stderr, "  env       Manage environment files")
	fmt.Fprintln(os.Stderr, "  ome       Manage OvenMediaEngine configuration")
	fmt.Fprintln(os.Stderr, "  compose   Manage Docker Compose stack")
	fmt.Fprintln(os.Stderr, "  quickstart    Run doctor, env init, OME render, and docker compose up")
}

func runVersion(args []string) {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s version\n", os.Args[0])
	}
	_ = fs.Parse(args)

	fmt.Printf("Version: %s\n", versionValue(version))
	fmt.Printf("Commit: %s\n", versionValue(commit))
	fmt.Printf("Date: %s\n", versionValue(date))
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s doctor\n", os.Args[0])
	}
	_ = fs.Parse(args)

	deps := doctorDeps{lookPath: executil.LookPath, runner: execRunner{}, getwd: os.Getwd}
	result := runDoctorChecks(os.Stdout, deps)
	printDoctorResult(os.Stdout, result)
}

func runDoctorChecks(out io.Writer, deps doctorDeps) doctorResult {
	fmt.Fprintln(out, "BitRiver Live environment check")

	dockerPath, dockerErr := deps.lookPath("docker")
	dockerVersionOutput, dockerVersionErr := runCommandOutputWithRunner(deps.runner, dockerPath, dockerErr, "version")
	composeOutput, composeErr := runCommandOutputWithRunner(deps.runner, dockerPath, dockerErr, "compose", "version")

	cwd, cwdErr := deps.getwd()

	return doctorResult{
		dockerPath:        dockerPath,
		dockerErr:         dockerErr,
		dockerVersionOut:  dockerVersionOutput,
		dockerVersionErr:  dockerVersionErr,
		composeVersionOut: composeOutput,
		composeVersionErr: composeErr,
		workDir:           cwd,
		workDirErr:        cwdErr,
	}
}

func printDoctorResult(out io.Writer, result doctorResult) {
	if result.dockerErr != nil {
		fmt.Fprintf(out, "- docker in PATH: no (%v)\n", result.dockerErr)
	} else {
		fmt.Fprintf(out, "- docker in PATH: yes (%s)\n", result.dockerPath)
	}

	if result.dockerVersionErr != nil {
		fmt.Fprintf(out, "- docker version: failed (%v)\n", result.dockerVersionErr)
		if len(result.dockerVersionOut) > 0 {
			fmt.Fprintln(out, indentOutput(result.dockerVersionOut))
		}
	} else {
		fmt.Fprintln(out, "- docker version: ok")
	}

	if result.composeVersionErr != nil {
		fmt.Fprintf(out, "- docker compose version: failed (%v)\n", result.composeVersionErr)
		if len(result.composeVersionOut) > 0 {
			fmt.Fprintln(out, indentOutput(result.composeVersionOut))
		}
	} else {
		fmt.Fprintln(out, "- docker compose version: ok")
	}

	fmt.Fprintf(out, "- OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	if result.workDirErr != nil {
		fmt.Fprintf(out, "- Working directory: error (%v)\n", result.workDirErr)
	} else {
		fmt.Fprintf(out, "- Working directory: %s\n", result.workDir)
	}
}

func runCompose(args []string) {
	fs := flag.NewFlagSet("compose", flag.ExitOnError)
	defaultFile := filepath.Join("deploy", "docker-compose.yml")
	fileFlag := fs.String("file", defaultFile, "Path to docker-compose file")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s compose [--file path] <up|down>\n", os.Args[0])
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	action := fs.Arg(0)
	if err := composeAction(action, *fileFlag, execRunner{}); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func composeAction(action string, composeFile string, runner processRunner) error {
	composeArgs, err := buildComposeArgs(action, composeFile)
	if err != nil {
		return err
	}

	dockerPath, err := executil.LookPath("docker")
	if err != nil {
		return fmt.Errorf("docker not found in PATH: %w", err)
	}

	if err := runner.Run(dockerPath, composeArgs, executil.WithPrintCommand()); err != nil {
		return fmt.Errorf("docker compose %s failed: %w", action, err)
	}

	return nil
}

func runEnv(args []string) {
	fs := flag.NewFlagSet("env", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s env <command>\n", os.Args[0])
		fmt.Fprintln(fs.Output(), "Commands:")
		fmt.Fprintln(fs.Output(), "  init    Initialize .env file from template")
	}
	_ = fs.Parse(args)

	if fs.NArg() < 1 {
		fs.Usage()
		os.Exit(1)
	}

	switch fs.Arg(0) {
	case "init":
		runEnvInit(fs.Args()[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown env subcommand: %s\n", fs.Arg(0))
		fs.Usage()
		os.Exit(1)
	}
}

type quickstartConfig struct {
	envFile     string
	composeFile string
}

type quickstartDeps struct {
	doctor    func(io.Writer) doctorResult
	envInit   func(envPath string, templateRoot string, out io.Writer) error
	omeRender func(args []string) error
	composeUp func(composeFile string) error
	getwd     func() (string, error)
	stdout    io.Writer
}

func runQuickstart(args []string) {
	config, err := parseQuickstartFlags(args, os.Stderr)
	if err != nil {
		os.Exit(1)
	}

	deps := defaultQuickstartDeps(os.Stdout)

	if err := executeQuickstart(config, deps); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseQuickstartFlags(args []string, output io.Writer) (quickstartConfig, error) {
	fs := flag.NewFlagSet("quickstart", flag.ContinueOnError)
	fs.SetOutput(output)
	defaultCompose := filepath.Join("deploy", "docker-compose.yml")
	envFile := fs.String("env-file", ".env", "Path to the environment file")
	composeFile := fs.String("compose-file", defaultCompose, "Path to the Docker Compose file")
	fs.Usage = func() {
		fmt.Fprintf(output, "Usage: %s quickstart [options]\n", os.Args[0])
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return quickstartConfig{}, err
	}

	if fs.NArg() > 0 {
		fs.Usage()
		return quickstartConfig{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}

	return quickstartConfig{
		envFile:     *envFile,
		composeFile: *composeFile,
	}, nil
}

func defaultQuickstartDeps(stdout io.Writer) quickstartDeps {
	return quickstartDeps{
		doctor: func(out io.Writer) doctorResult {
			deps := doctorDeps{lookPath: executil.LookPath, runner: execRunner{}, getwd: os.Getwd}
			result := runDoctorChecks(out, deps)
			printDoctorResult(out, result)
			return result
		},
		envInit:   initEnvFile,
		omeRender: runOMERender,
		composeUp: func(composeFile string) error { return composeAction("up", composeFile, execRunner{}) },
		getwd:     os.Getwd,
		stdout:    stdout,
	}
}

func executeQuickstart(config quickstartConfig, deps quickstartDeps) error {
	stdout := deps.stdout
	if stdout == nil {
		stdout = io.Discard
	}

	workDir, err := deps.getwd()
	if err != nil {
		return fmt.Errorf("failed to determine working directory: %w", err)
	}

	fmt.Fprintln(stdout, "Running environment checks...")
	result := deps.doctor(stdout)
	if result.dockerErr != nil {
		return fmt.Errorf("docker is required for quickstart: %w", result.dockerErr)
	}
	if result.composeVersionErr != nil {
		return fmt.Errorf("docker compose v2 is required for quickstart: %w", result.composeVersionErr)
	}

	envPath := config.envFile
	if !filepath.IsAbs(envPath) {
		envPath = filepath.Join(workDir, envPath)
	}

	fmt.Fprintf(stdout, "\nPreparing environment file at %s...\n", envPath)
	if err := deps.envInit(envPath, workDir, stdout); err != nil {
		return fmt.Errorf("failed to initialize environment: %w", err)
	}

	fmt.Fprintln(stdout, "\nRendering OvenMediaEngine configuration...")
	if err := deps.omeRender([]string{"--env-file", envPath}); err != nil {
		return fmt.Errorf("python 3 is required to render OvenMediaEngine configuration: %w", err)
	}

	composePath := config.composeFile
	if !filepath.IsAbs(composePath) {
		composePath = filepath.Join(workDir, composePath)
	}

	fmt.Fprintf(stdout, "\nStarting Docker Compose using %s...\n", composePath)
	if err := deps.composeUp(composePath); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "\nQuickstart complete. Containers are starting in Docker.")
	return nil
}

func runEnvInit(args []string) {
	fs := flag.NewFlagSet("env init", flag.ExitOnError)
	envFile := fs.String("env-file", ".env", "Path to write the generated environment file")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s env init [--env-file path]\n", os.Args[0])
	}
	_ = fs.Parse(args)

	if fs.NArg() > 0 {
		fs.Usage()
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to determine working directory: %v\n", err)
		os.Exit(1)
	}

	envFilePath := *envFile
	if !filepath.IsAbs(envFilePath) {
		envFilePath = filepath.Join(cwd, envFilePath)
	}

	if err := initEnvFile(envFilePath, cwd, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func initEnvFile(envPath string, templateRoot string, out io.Writer) error {
	if _, err := os.Stat(envPath); err == nil {
		fmt.Fprintln(out, ".env already exists; leaving unchanged.")
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not check .env: %w", err)
	}

	templateCandidates := []string{
		filepath.Join(templateRoot, "deploy", ".env.example"),
		filepath.Join(templateRoot, ".env"), // Fallback for repositories that track a root .env as the template.
	}

	templatePath, err := envutil.FirstExistingPath(templateCandidates)
	if err != nil {
		return fmt.Errorf("no env template found; expected deploy/.env.example or repository .env")
	}

	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read env template: %w", err)
	}

	if err := os.WriteFile(envPath, templateData, 0o644); err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}

	if err := seedEnvSecrets(envPath); err != nil {
		return fmt.Errorf("failed to seed .env credentials: %w", err)
	}

	fmt.Fprintf(out, "Created .env from %s\n", filepath.Base(templatePath))
	return nil
}

func seedEnvSecrets(envPath string) error {
	redisPassword, err := generateSecret(24)
	if err != nil {
		return err
	}

	postgresPassword, err := generateSecret(24)
	if err != nil {
		return err
	}

	adminPassword, err := generateSecret(28)
	if err != nil {
		return err
	}

	srsToken, err := generateSecret(32)
	if err != nil {
		return err
	}

	omePassword, err := generateSecret(28)
	if err != nil {
		return err
	}

	omeToken, err := generateSecret(40)
	if err != nil {
		return err
	}

	transcoderToken, err := generateSecret(40)
	if err != nil {
		return err
	}

	updates := map[string]string{
		"BITRIVER_POSTGRES_PASSWORD":              postgresPassword,
		"BITRIVER_REDIS_PASSWORD":                 redisPassword,
		"BITRIVER_LIVE_CHAT_QUEUE_REDIS_PASSWORD": redisPassword,
		"BITRIVER_LIVE_ADMIN_PASSWORD":            adminPassword,
		"BITRIVER_SRS_TOKEN":                      srsToken,
		"BITRIVER_OME_PASSWORD":                   omePassword,
		"BITRIVER_OME_API_TOKEN":                  omeToken,
		"BITRIVER_OME_ACCESS_TOKEN":               omeToken,
		"BITRIVER_TRANSCODER_TOKEN":               transcoderToken,
	}

	return applyEnvUpdates(envPath, updates)
}

func applyEnvUpdates(envPath string, updates map[string]string) error {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return fmt.Errorf("read env for updates: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	handled := make(map[string]struct{}, len(updates))

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}

		key := line[:idx]
		if value, ok := updates[key]; ok {
			lines[i] = fmt.Sprintf("%s=%s", key, value)
			handled[key] = struct{}{}
		}
	}

	for key, value := range updates {
		if _, ok := handled[key]; ok {
			continue
		}

		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	content := strings.Join(lines, "\n")
	return os.WriteFile(envPath, []byte(content), 0o644)
}

func generateSecret(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}

	for i := range bytes {
		bytes[i] = alphabet[int(bytes[i])%len(alphabet)]
	}

	return string(bytes), nil
}

func runCommandOutput(binaryPath string, lookupErr error, args ...string) (string, error) {
	return runCommandOutputWithRunner(execRunner{}, binaryPath, lookupErr, args...)
}

func runCommandOutputWithRunner(runner processRunner, binaryPath string, lookupErr error, args ...string) (string, error) {
	if lookupErr != nil {
		return "", lookupErr
	}

	var buf bytes.Buffer
	if err := runner.Run(binaryPath, args, executil.WithStdout(&buf), executil.WithStderr(&buf)); err != nil {
		return buf.String(), err
	}

	return buf.String(), nil
}

func indentOutput(output string) string {
	if output == "" {
		return ""
	}

	var buf bytes.Buffer
	for _, line := range bytes.Split([]byte(output), []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		buf.WriteString("    ")
		buf.Write(line)
		buf.WriteByte('\n')
	}

	return buf.String()
}

func versionValue(value string) string {
	if value == "" {
		return "dev"
	}

	return value
}

func buildComposeArgs(action string, composeFile string) ([]string, error) {
	args := []string{"compose", "-f", composeFile}

	switch action {
	case "up":
		args = append(args, "up", "-d", "--remove-orphans")
	case "down":
		args = append(args, "down", "--remove-orphans")
	default:
		return nil, fmt.Errorf("unknown compose action: %s", action)
	}

	return args, nil
}
