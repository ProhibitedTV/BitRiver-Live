package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	version = "dev"
	commit  = "dev"
	date    = "dev"
)

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

	fmt.Println("BitRiver Live environment check")

	dockerPath, dockerErr := exec.LookPath("docker")
	if dockerErr != nil {
		fmt.Printf("- docker in PATH: no (%v)\n", dockerErr)
	} else {
		fmt.Printf("- docker in PATH: yes (%s)\n", dockerPath)
	}

	dockerVersionOutput, dockerVersionErr := runCommandOutput(dockerPath, dockerErr, "version")
	if dockerVersionErr != nil {
		fmt.Printf("- docker version: failed (%v)\n", dockerVersionErr)
		if len(dockerVersionOutput) > 0 {
			fmt.Println(indentOutput(dockerVersionOutput))
		}
	} else {
		fmt.Println("- docker version: ok")
	}

	composeOutput, composeErr := runCommandOutput(dockerPath, dockerErr, "compose", "version")
	if composeErr != nil {
		fmt.Printf("- docker compose version: failed (%v)\n", composeErr)
		if len(composeOutput) > 0 {
			fmt.Println(indentOutput(composeOutput))
		}
	} else {
		fmt.Println("- docker compose version: ok")
	}

	fmt.Printf("- OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Printf("- Working directory: error (%v)\n", err)
	} else {
		fmt.Printf("- Working directory: %s\n", cwd)
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
	composeArgs, err := buildComposeArgs(action, *fileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		fmt.Fprintf(os.Stderr, "docker not found in PATH: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command(dockerPath, composeArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("Executing: %s\n", strings.Join(cmd.Args, " "))
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "docker compose %s failed: %v\n", action, err)
		os.Exit(1)
	}
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

	var templatePath string
	for _, candidate := range templateCandidates {
		if _, err := os.Stat(candidate); err == nil {
			templatePath = candidate
			break
		}
	}

	if templatePath == "" {
		return fmt.Errorf("no env template found; expected deploy/.env.example or repository .env")
	}

	templateData, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read env template: %w", err)
	}

	if err := os.WriteFile(envPath, templateData, 0o644); err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}

	fmt.Fprintf(out, "Created .env from %s\n", filepath.Base(templatePath))
	return nil
}

func runCommandOutput(binaryPath string, lookupErr error, args ...string) (string, error) {
	if lookupErr != nil {
		return "", lookupErr
	}

	cmd := exec.Command(binaryPath, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
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
