package config

import "testing"

func TestLoadTranscoderFromEnvDefaults(t *testing.T) {
	env := Environment{values: map[string]string{
		"JOB_CONTROLLER_TOKEN":                "token",
		"BITRIVER_TRANSCODER_PUBLIC_BASE_URL": "https://cdn.example.com",
	}}
	cfg, err := LoadTranscoderFromEnv(env)
	if err != nil {
		t.Fatalf("LoadTranscoderFromEnv: %v", err)
	}
	if cfg.Bind != ":9000" {
		t.Fatalf("expected default bind, got %q", cfg.Bind)
	}
	if cfg.OutputRoot != "./work" {
		t.Fatalf("expected default output root, got %q", cfg.OutputRoot)
	}
}

func TestLoadTranscoderFromEnvValidation(t *testing.T) {
	_, err := LoadTranscoderFromEnv(Environment{values: map[string]string{"BITRIVER_TRANSCODER_PUBLIC_BASE_URL": "https://cdn.example.com"}})
	if err == nil || err.Error() != "JOB_CONTROLLER_TOKEN must be configured before starting the transcoder" {
		t.Fatalf("expected token validation error, got %v", err)
	}
}

func TestLoadSRSControllerFromEnvValidation(t *testing.T) {
	_, err := LoadSRSControllerFromEnv(Environment{values: map[string]string{"BITRIVER_SRS_TOKEN": "tok", "SRS_CONTROLLER_UPSTREAM": "localhost"}})
	if err == nil || err.Error() != "SRS_CONTROLLER_UPSTREAM must include scheme and host" {
		t.Fatalf("expected upstream validation error, got %v", err)
	}
}
