package config

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestLoadIngestFromEnvDisabledWhenEmpty(t *testing.T) {
	cfg, err := LoadIngestFromEnv(Environment{values: map[string]string{}})
	if !errors.Is(err, ErrIngestConfigDisabled) {
		t.Fatalf("expected disabled error, got %v", err)
	}
	if cfg.Validate() == nil {
		t.Fatal("expected validation error when config is empty")
	}
}

func TestLoadIngestFromEnvMissingFields(t *testing.T) {
	cfg, err := LoadIngestFromEnv(Environment{values: map[string]string{"BITRIVER_SRS_API": "http://srs:1985"}})
	var missing MissingIngestConfigError
	if err == nil || !errors.As(err, &missing) {
		t.Fatalf("expected missing error, got %v", err)
	}
	want := []string{"BITRIVER_SRS_TOKEN", "BITRIVER_OME_API", "BITRIVER_OME_USERNAME", "BITRIVER_OME_PASSWORD", "BITRIVER_TRANSCODER_API", "BITRIVER_TRANSCODER_TOKEN"}
	if !slices.Equal(missing.Missing, want) {
		t.Fatalf("missing mismatch: got %v want %v", missing.Missing, want)
	}
	if cfg.SRSBaseURL != "" {
		t.Fatalf("expected empty config on validation failure, got %q", cfg.SRSBaseURL)
	}
}

func TestLoadIngestFromEnvDefaultsAndOverrides(t *testing.T) {
	env := Environment{values: map[string]string{
		"BITRIVER_SRS_API":                    "http://srs:1985",
		"BITRIVER_SRS_TOKEN":                  "secret",
		"BITRIVER_OME_API":                    "http://ome:8081",
		"BITRIVER_OME_USERNAME":               "admin",
		"BITRIVER_OME_PASSWORD":               "password",
		"BITRIVER_TRANSCODER_API":             "http://transcoder:9000",
		"BITRIVER_TRANSCODER_TOKEN":           "job-secret",
		"BITRIVER_INGEST_HTTP_MAX_ATTEMPTS":   "5",
		"BITRIVER_INGEST_HTTP_RETRY_INTERVAL": "1s",
	}}
	cfg, err := LoadIngestFromEnv(env)
	if err != nil {
		t.Fatalf("LoadIngestFromEnv: %v", err)
	}
	if cfg.HTTPMaxAttempts != 5 || cfg.HTTPRetryInterval != time.Second {
		t.Fatalf("unexpected retry overrides: %#v", cfg)
	}
	if cfg.HealthEndpoint != "/healthz" {
		t.Fatalf("expected default health endpoint, got %q", cfg.HealthEndpoint)
	}
	if len(cfg.LadderProfiles) != 3 {
		t.Fatalf("expected default ladder profiles, got %d", len(cfg.LadderProfiles))
	}
}

func TestLoadIngestFromEnvValidationErrors(t *testing.T) {
	_, err := LoadIngestFromEnv(Environment{values: map[string]string{
		"BITRIVER_SRS_API":                  "http://srs:1985",
		"BITRIVER_SRS_TOKEN":                "secret",
		"BITRIVER_OME_API":                  "http://ome:8081",
		"BITRIVER_OME_USERNAME":             "admin",
		"BITRIVER_OME_PASSWORD":             "password",
		"BITRIVER_TRANSCODER_API":           "http://transcoder:9000",
		"BITRIVER_TRANSCODER_TOKEN":         "job-secret",
		"BITRIVER_INGEST_MAX_BOOT_ATTEMPTS": "oops",
	}})
	if err == nil || err.Error() != "parse BITRIVER_INGEST_MAX_BOOT_ATTEMPTS: strconv.Atoi: parsing \"oops\": invalid syntax" {
		t.Fatalf("expected parse error, got %v", err)
	}
}
