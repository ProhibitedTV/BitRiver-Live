package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"bitriver-live/internal/envutil"
)

var requiredEnvKeys = []string{
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

var requiredImageTagKeys = []string{
	"BITRIVER_LIVE_IMAGE_TAG",
	"BITRIVER_VIEWER_IMAGE_TAG",
	"BITRIVER_SRS_CONTROLLER_IMAGE_TAG",
	"BITRIVER_TRANSCODER_IMAGE_TAG",
	"BITRIVER_SRS_IMAGE_TAG",
	"BITRIVER_OME_IMAGE_TAG",
}

func runEnvValidate(args []string) {
	fs := flag.NewFlagSet("env validate", flag.ExitOnError)
	envFile := fs.String("env-file", ".env", "Path to the environment file to validate")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s env validate [--env-file path]\n", os.Args[0])
	}
	_ = fs.Parse(args)

	if fs.NArg() > 0 {
		fs.Usage()
		os.Exit(1)
	}

	if err := validateEnvFile(*envFile, os.Environ(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func validateEnvFile(envPath string, environ []string, out io.Writer) error {
	if _, err := os.Stat(envPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(out, "Environment file not found at %s.\n", envPath)
			fmt.Fprintln(out, "Copy deploy/.env.example to .env and populate it before continuing.")
			return fmt.Errorf("environment file missing")
		}
		return fmt.Errorf("stat env file: %w", err)
	}

	merged, err := envutil.LoadFile(envPath, envutil.FromEnviron(environ))
	if err != nil {
		return err
	}

	var requiredKeys []string
	requiredKeys = append(requiredKeys, requiredEnvKeys...)
	requiredKeys = append(requiredKeys, requiredImageTagKeys...)

	missing := missingEnvKeys(requiredKeys, merged)
	if len(missing) > 0 {
		fmt.Fprintf(out, "The following required variables are unset or empty in %s:\n", envPath)
		for _, key := range missing {
			fmt.Fprintf(out, "  - %s\n", key)
		}
		return fmt.Errorf("missing required environment variables")
	}

	fmt.Fprintf(out, "Environment file %s looks ready.\n", envPath)
	return nil
}

func missingEnvKeys(required []string, values map[string]string) []string {
	var missing []string

	for _, key := range required {
		if strings.TrimSpace(values[key]) == "" {
			missing = append(missing, key)
		}
	}

	return missing
}
