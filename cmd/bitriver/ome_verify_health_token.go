package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runOMEVerifyHealthToken(args []string) error {
	fs := flag.NewFlagSet("ome verify-health-token", flag.ContinueOnError)
	envPath := fs.String("env-file", defaultEnvFile(), "path to env file")
	configPath := fs.String("config", filepath.Join(repoRoot(), "deploy", "ome", "Server.generated.xml"), "path to generated OME config")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := os.Stat(*envPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("OME token verification failed: env file not found at %s", *envPath)
		}
		return fmt.Errorf("OME token verification failed: stat env file %s: %w", *envPath, err)
	}

	if _, err := os.Stat(*configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("OME token verification failed: generated config not found at %s", *configPath)
		}
		return fmt.Errorf("OME token verification failed: stat generated config %s: %w", *configPath, err)
	}

	values, err := loadEnvValues(*envPath, false)
	if err != nil {
		return fmt.Errorf("OME token verification failed: load env file %s: %w", *envPath, err)
	}

	renderedToken, err := parseOMERenderedAccessToken(*configPath)
	if err != nil {
		return fmt.Errorf("OME token verification failed: %w", err)
	}

	expectedToken := resolveOMECanonicalAccessToken(values)
	if expectedToken == "" {
		return fmt.Errorf("OME token verification failed: resolved runtime token from canonical precedence BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_ACCESS_TOKEN -> BITRIVER_OME_API_TOKEN is empty in %s", *envPath)
	}

	if renderedToken != expectedToken {
		return fmt.Errorf("OME token verification failed: rendered and runtime tokens differ.\n  rendered (<Managers><API><AccessToken>): %s\n  expected (BITRIVER_OME_HEALTHCHECK_TOKEN -> BITRIVER_OME_ACCESS_TOKEN -> BITRIVER_OME_API_TOKEN): %s\nFix by updating %s and re-rendering with:\n  go run ./cmd/bitriver ome render --force --env-file %s", renderedToken, expectedToken, *envPath, *envPath)
	}

	fmt.Fprintln(os.Stdout, "OME token verification passed: rendered AccessToken matches compose runtime health token source.")
	return nil
}

func parseOMERenderedAccessToken(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read generated config %s: %w", path, err)
	}

	var parsed struct {
		Managers struct {
			API struct {
				AccessToken string `xml:"AccessToken"`
			} `xml:"API"`
		} `xml:"Managers"`
	}
	if err := xml.Unmarshal(contents, &parsed); err != nil {
		return "", fmt.Errorf("parse generated config %s: %w", path, err)
	}

	renderedToken := strings.TrimSpace(parsed.Managers.API.AccessToken)
	if renderedToken == "" {
		return "", fmt.Errorf("<Managers><API><AccessToken> is empty in %s", path)
	}

	return renderedToken, nil
}
