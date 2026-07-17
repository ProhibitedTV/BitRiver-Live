package main

import (
	"errors"
	"flag"
	"fmt"
	"runtime"
	"strings"
)

type migrationCommandConfig struct {
	ComposeFile string
	EnvFile     string
	Mode        string
	Repair      string
	Filename    string
	Checksum    string
}

func runMigrationCommand(args []string) error {
	fs := flag.NewFlagSet("migrations", flag.ContinueOnError)
	composeFile := fs.String("compose-file", defaultComposeFile(), "compose file containing the postgres-migrations service")
	envFile := fs.String("env-file", defaultEnvFile(), "environment file used by Docker Compose")
	mode := fs.String("mode", "plan", "migration operation: plan, status, apply, or repair")
	repair := fs.String("repair-action", "", "repair action: retry or mark-applied")
	filename := fs.String("file", "", "migration filename for repair")
	checksum := fs.String("checksum", "", "exact recorded SHA-256 checksum for repair confirmation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %s", strings.Join(fs.Args(), " "))
	}

	cfg := migrationCommandConfig{
		ComposeFile: strings.TrimSpace(*composeFile),
		EnvFile:     strings.TrimSpace(*envFile),
		Mode:        strings.ToLower(strings.TrimSpace(*mode)),
		Repair:      strings.ToLower(strings.TrimSpace(*repair)),
		Filename:    strings.TrimSpace(*filename),
		Checksum:    strings.ToLower(strings.TrimSpace(*checksum)),
	}
	if err := validateMigrationCommand(cfg); err != nil {
		return err
	}
	return runMigrationCompose(cfg)
}

func validateMigrationCommand(cfg migrationCommandConfig) error {
	if cfg.ComposeFile == "" {
		return errors.New("compose file must not be empty")
	}
	if cfg.EnvFile == "" {
		return errors.New("env file must not be empty")
	}

	switch cfg.Mode {
	case "plan", "status", "apply":
		if cfg.Repair != "" || cfg.Filename != "" || cfg.Checksum != "" {
			return fmt.Errorf("repair flags require --mode repair")
		}
	case "repair":
		if cfg.Repair != "retry" && cfg.Repair != "mark-applied" {
			return errors.New("--mode repair requires --repair-action retry or mark-applied")
		}
		if cfg.Filename == "" {
			return errors.New("--mode repair requires --file")
		}
		if len(cfg.Checksum) != 64 {
			return errors.New("--mode repair requires a 64-character --checksum")
		}
		for _, char := range cfg.Checksum {
			if !strings.ContainsRune("0123456789abcdef", char) {
				return errors.New("--checksum must contain only lowercase hexadecimal characters")
			}
		}
	default:
		return fmt.Errorf("unsupported migration mode %q", cfg.Mode)
	}
	return nil
}

func runMigrationCompose(cfg migrationCommandConfig) error {
	if _, err := lookPathRunner("docker"); err != nil {
		return err
	}

	args := append(composeArgsWithEnv(cfg.ComposeFile, cfg.EnvFile), "run", "--rm")
	if runtime.GOOS == "windows" || !stdinIsTerminal() {
		args = append(args, "-T")
	}
	args = append(args, "postgres-migrations", cfg.Mode)
	if cfg.Mode == "repair" {
		args = append(args, cfg.Repair, cfg.Filename, cfg.Checksum)
	}
	return commandRunner("docker", args...)
}
