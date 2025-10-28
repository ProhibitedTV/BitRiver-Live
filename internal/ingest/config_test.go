package ingest

import (
	"testing"
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
	if err != nil {
		t.Fatalf("LoadConfigFromEnv returned error: %v", err)
	}
	if cfg.Enabled() {
		t.Fatal("expected ingest to remain disabled with empty configuration")
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

	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatal("expected validation error for partial ingest configuration")
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
}
