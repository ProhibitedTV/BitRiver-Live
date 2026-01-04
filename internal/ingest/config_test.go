package ingest

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestConfigDisabledWhenEmpty(t *testing.T) {
	t.Setenv("BITRIVER_SRS_API", "")
	t.Setenv("BITRIVER_SRS_TOKEN", "")
	t.Setenv("BITRIVER_OME_API", "")
	t.Setenv("BITRIVER_OME_USERNAME", "")
	t.Setenv("BITRIVER_OME_PASSWORD", "")
	t.Setenv("BITRIVER_TRANSCODER_API", "")
	t.Setenv("BITRIVER_TRANSCODER_TOKEN", "")

	cfg, err := LoadConfigFromEnv()
	if !errors.Is(err, ErrConfigDisabled) {
		t.Fatalf("expected disabled error, got %v", err)
	}
	if cfg.Enabled() {
		t.Fatal("expected ingest to remain disabled with empty configuration")
	}
	if !errors.Is(cfg.Validate(), ErrConfigDisabled) {
		t.Fatalf("expected validate to report disabled config")
	}
}

func TestConfigPartialFailsValidation(t *testing.T) {
	t.Setenv("BITRIVER_SRS_API", "http://srs:1985")
	t.Setenv("BITRIVER_SRS_TOKEN", "")
	t.Setenv("BITRIVER_OME_API", "")
	t.Setenv("BITRIVER_OME_USERNAME", "")
	t.Setenv("BITRIVER_OME_PASSWORD", "")
	t.Setenv("BITRIVER_TRANSCODER_API", "")
	t.Setenv("BITRIVER_TRANSCODER_TOKEN", "")

	cfg, err := LoadConfigFromEnv()
	var missing MissingConfigError
	if err == nil || !errors.As(err, &missing) {
		t.Fatalf("expected missing config error, got %v", err)
	}
	if got, want := missing.Missing, []string{"BITRIVER_SRS_TOKEN", "BITRIVER_OME_API", "BITRIVER_OME_USERNAME", "BITRIVER_OME_PASSWORD", "BITRIVER_TRANSCODER_API", "BITRIVER_TRANSCODER_TOKEN"}; !slices.Equal(got, want) {
		t.Fatalf("unexpected missing fields: %v", got)
	}
	if cfg.Enabled() {
		t.Fatal("expected partial configuration to leave ingest disabled")
	}
}

func TestConfigEnabledWithCompleteSettings(t *testing.T) {
	t.Setenv("BITRIVER_SRS_API", "http://srs:1985")
	t.Setenv("BITRIVER_SRS_TOKEN", "secret")
	t.Setenv("BITRIVER_OME_API", "http://ome:8081")
	t.Setenv("BITRIVER_OME_USERNAME", "admin")
	t.Setenv("BITRIVER_OME_PASSWORD", "password")
	t.Setenv("BITRIVER_TRANSCODER_API", "http://transcoder:9000")
	t.Setenv("BITRIVER_TRANSCODER_TOKEN", "job-secret")
	t.Setenv("BITRIVER_TRANSCODE_LADDER", "1080p:6000,720p:4000")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv: %v", err)
	}
	if !cfg.Enabled() {
		t.Fatal("expected ingest to be enabled with complete configuration")
	}
	if len(cfg.LadderProfiles) != 2 {
		t.Fatalf("expected ladder profiles to parse, got %d", len(cfg.LadderProfiles))
	}
	if cfg.HTTPMaxAttempts != 30 {
		t.Fatalf("expected default HTTP retries, got %d", cfg.HTTPMaxAttempts)
	}
	if cfg.HTTPRetryInterval != 2*time.Second {
		t.Fatalf("expected default HTTP retry interval, got %s", cfg.HTTPRetryInterval)
	}
}

func TestConfigHTTPRetryOverrides(t *testing.T) {
	t.Setenv("BITRIVER_SRS_API", "http://srs:1985")
	t.Setenv("BITRIVER_SRS_TOKEN", "secret")
	t.Setenv("BITRIVER_OME_API", "http://ome:8081")
	t.Setenv("BITRIVER_OME_USERNAME", "admin")
	t.Setenv("BITRIVER_OME_PASSWORD", "password")
	t.Setenv("BITRIVER_TRANSCODER_API", "http://transcoder:9000")
	t.Setenv("BITRIVER_TRANSCODER_TOKEN", "job-secret")
	t.Setenv("BITRIVER_INGEST_HTTP_MAX_ATTEMPTS", "5")
	t.Setenv("BITRIVER_INGEST_HTTP_RETRY_INTERVAL", "1s")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv: %v", err)
	}
	if cfg.HTTPMaxAttempts != 5 {
		t.Fatalf("expected HTTP max attempts override, got %d", cfg.HTTPMaxAttempts)
	}
	if cfg.HTTPRetryInterval != time.Second {
		t.Fatalf("expected HTTP retry interval override, got %s", cfg.HTTPRetryInterval)
	}
}
