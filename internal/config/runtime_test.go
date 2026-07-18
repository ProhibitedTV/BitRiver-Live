package config

import (
	"strings"
	"testing"
)

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

func TestLoadSRSControllerFromEnvResolvesPublicAndInternalRTMPBases(t *testing.T) {
	cfg, err := LoadSRSControllerFromEnv(Environment{values: map[string]string{
		"BITRIVER_SRS_TOKEN":                    "tok",
		"SRS_CONTROLLER_PUBLIC_RTMP_BASE_URL":   "rtmp://ingest.example.com:1935/live/",
		"SRS_CONTROLLER_INTERNAL_RTMP_BASE_URL": "rtmp://srs:1935/live/",
	}})
	if err != nil {
		t.Fatalf("LoadSRSControllerFromEnv: %v", err)
	}
	if got := cfg.PublicRTMPBaseURL.String(); got != "rtmp://ingest.example.com:1935/live" {
		t.Fatalf("unexpected public RTMP base: %q", got)
	}
	if got := cfg.InternalRTMPBaseURL.String(); got != "rtmp://srs:1935/live" {
		t.Fatalf("unexpected internal RTMP base: %q", got)
	}
}

func TestLoadSRSControllerFromEnvRejectsNonRTMPBase(t *testing.T) {
	_, err := LoadSRSControllerFromEnv(Environment{values: map[string]string{
		"BITRIVER_SRS_TOKEN":                  "tok",
		"SRS_CONTROLLER_PUBLIC_RTMP_BASE_URL": "https://ingest.example.com/live",
	}})
	if err == nil || !strings.Contains(err.Error(), "SRS_CONTROLLER_PUBLIC_RTMP_BASE_URL") {
		t.Fatalf("expected public RTMP validation error, got %v", err)
	}
}

func TestLoadServerDatastoreFromEnv(t *testing.T) {
	env := Environment{values: map[string]string{
		"BITRIVER_LIVE_POSTGRES_DSN":         "postgres://primary",
		"BITRIVER_LIVE_SESSION_POSTGRES_DSN": "postgres://session",
		"DATABASE_URL":                       "postgres://database-url",
	}}
	cfg := LoadServerDatastoreFromEnv(env)
	if cfg.PostgresDSN != "postgres://primary" {
		t.Fatalf("expected BITRIVER_LIVE_POSTGRES_DSN precedence, got %q", cfg.PostgresDSN)
	}
	if cfg.SessionPostgresDSN != "postgres://session" {
		t.Fatalf("expected session dsn override, got %q", cfg.SessionPostgresDSN)
	}
}

func TestLoadServerDatastoreFromEnvFallbacks(t *testing.T) {
	env := Environment{values: map[string]string{
		"DATABASE_URL": "postgres://database-url",
	}}
	cfg := LoadServerDatastoreFromEnv(env)
	if cfg.PostgresDSN != "postgres://database-url" {
		t.Fatalf("expected DATABASE_URL fallback, got %q", cfg.PostgresDSN)
	}
	if cfg.SessionPostgresDSN != cfg.PostgresDSN {
		t.Fatalf("expected session DSN to fallback to primary DSN, got %q", cfg.SessionPostgresDSN)
	}
}

func TestResolveServerDatastoreConfigFlagPrecedence(t *testing.T) {
	env := Environment{values: map[string]string{
		"BITRIVER_LIVE_POSTGRES_DSN":         "postgres://env-primary",
		"BITRIVER_LIVE_SESSION_POSTGRES_DSN": "postgres://env-session",
	}}
	cfg := ResolveServerDatastoreConfig("postgres://flag-primary", "", env)
	if cfg.PostgresDSN != "postgres://flag-primary" {
		t.Fatalf("expected postgres flag precedence, got %q", cfg.PostgresDSN)
	}
	if cfg.SessionPostgresDSN != "postgres://env-session" {
		t.Fatalf("expected session env value when session flag unset, got %q", cfg.SessionPostgresDSN)
	}

	cfg = ResolveServerDatastoreConfig("postgres://flag-primary", "postgres://flag-session", env)
	if cfg.SessionPostgresDSN != "postgres://flag-session" {
		t.Fatalf("expected session flag precedence, got %q", cfg.SessionPostgresDSN)
	}
}
