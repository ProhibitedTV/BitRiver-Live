package main

import (
	"log/slog"
	"strings"
	"testing"

	"bitriver-live/internal/chat"
)

func TestConfigureChatQueueMemory(t *testing.T) {
	queue, err := configureChatQueue("", chat.RedisQueueConfig{}, slog.Default())
	if err != nil {
		t.Fatalf("configureChatQueue returned error: %v", err)
	}
	if queue == nil {
		t.Fatalf("configureChatQueue returned nil queue")
	}
}

func TestConfigureChatQueueRedisMissingAddress(t *testing.T) {
	_, err := configureChatQueue("redis", chat.RedisQueueConfig{}, slog.Default())
	if err == nil {
		t.Fatal("configureChatQueue redis expected error when addr missing")
	}
}

func TestResolveStorageDriverDefaultsToPostgres(t *testing.T) {
	dsn := "postgres://example"
	driver, explicit, err := resolveStorageDriver("", "", dsn)
	if err != nil {
		t.Fatalf("resolveStorageDriver returned error: %v", err)
	}
	if explicit {
		t.Fatalf("expected postgres default to be implicit, got explicit")
	}
	if driver != "postgres" {
		t.Fatalf("expected postgres driver, got %q", driver)
	}
}

func TestResolveStorageDriverMissingConfigFails(t *testing.T) {
	if _, _, err := resolveStorageDriver("", "", ""); err == nil {
		t.Fatal("resolveStorageDriver expected error when no configuration provided")
	}
}

func TestValidateProductionDatastoreRejectsNonPostgres(t *testing.T) {
	if err := validateProductionDatastore("json", "postgres://example", "postgres://env"); err == nil {
		t.Fatal("expected error when production mode uses non-postgres driver")
	}
}

func TestValidateProductionDatastoreRequiresEnvDSN(t *testing.T) {
	err := validateProductionDatastore("postgres", "postgres://resolved", "")
	if err == nil {
		t.Fatal("expected error when BITRIVER_LIVE_POSTGRES_DSN is missing")
	}
	if !strings.Contains(err.Error(), "BITRIVER_LIVE_POSTGRES_DSN") {
		t.Fatalf("expected error to mention BITRIVER_LIVE_POSTGRES_DSN, got %q", err)
	}
}

func TestValidateProductionDatastoreRequiresResolvedDSN(t *testing.T) {
	if err := validateProductionDatastore("postgres", "", "postgres://env"); err == nil {
		t.Fatal("expected error when resolved Postgres DSN is empty")
	}
}

func TestResolvePostgresDSNPriority(t *testing.T) {
	t.Setenv("BITRIVER_LIVE_POSTGRES_DSN", "postgres://env")
	t.Setenv("DATABASE_URL", "postgres://database")
	got := resolvePostgresDSN("postgres://flag")
	if got != "postgres://flag" {
		t.Fatalf("expected flag DSN to win, got %q", got)
	}
	got = resolvePostgresDSN("")
	if got != "postgres://env" {
		t.Fatalf("expected BITRIVER_LIVE_POSTGRES_DSN to win, got %q", got)
	}
	t.Setenv("BITRIVER_LIVE_POSTGRES_DSN", "")
	got = resolvePostgresDSN("")
	if got != "postgres://database" {
		t.Fatalf("expected DATABASE_URL fallback, got %q", got)
	}
}

func TestResolveSessionStoreConfig(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		flagDriver    string
		envDriver     string
		storageDriver string
		storageDSN    string
		flagDSN       string
		envDSN        string
		want          sessionStoreConfig
		wantErr       bool
	}{
		{
			name:          "DefaultsToPostgresWhenStorageIsPostgres",
			storageDriver: "postgres",
			storageDSN:    "postgres://main",
			want:          sessionStoreConfig{Driver: "postgres", DSN: "postgres://main"},
		},
		{
			name:          "DefaultsToPostgresWhenSessionDSNProvided",
			storageDriver: "json",
			envDSN:        "postgres://sessions",
			want:          sessionStoreConfig{Driver: "postgres", DSN: "postgres://sessions"},
		},
		{
			name:          "ExplicitMemoryWins",
			flagDriver:    "memory",
			storageDriver: "postgres",
			storageDSN:    "postgres://main",
			want:          sessionStoreConfig{Driver: "memory"},
		},
		{
			name:          "DefaultsToMemoryWithoutHints",
			storageDriver: "json",
			want:          sessionStoreConfig{Driver: "memory"},
		},
		{
			name:          "ErrorsWhenPostgresSelectedWithoutDSN",
			flagDriver:    "postgres",
			storageDriver: "json",
			wantErr:       true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := resolveSessionStoreConfig(tc.flagDriver, tc.envDriver, tc.storageDriver, tc.storageDSN, tc.flagDSN, tc.envDSN)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Driver != tc.want.Driver {
				t.Fatalf("expected driver %q, got %q", tc.want.Driver, cfg.Driver)
			}
			if cfg.DSN != tc.want.DSN {
				t.Fatalf("expected DSN %q, got %q", tc.want.DSN, cfg.DSN)
			}
		})
	}
}
