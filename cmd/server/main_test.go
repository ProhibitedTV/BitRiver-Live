package main

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/server"
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

func TestResolveSessionCookieSecureMode(t *testing.T) {
	t.Parallel()

	if mode := resolveSessionCookieSecureMode("production"); mode != api.SessionCookieSecureAlways {
		t.Fatalf("expected production mode to force secure cookies, got %v", mode)
	}

	if mode := resolveSessionCookieSecureMode("development"); mode != api.SessionCookieSecureAuto {
		t.Fatalf("expected development mode to keep auto secure cookies, got %v", mode)
	}

	if mode := resolveSessionCookieSecureMode(" "); mode != api.SessionCookieSecureAuto {
		t.Fatalf("expected empty mode to keep auto secure cookies, got %v", mode)
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
		name            string
		flagDriver      string
		envDriver       string
		storageDriver   string
		storageDSN      string
		flagDSN         string
		envDSN          string
		requirePostgres bool
		want            sessionStoreConfig
		wantErr         bool
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
		{
			name:            "ProductionUsesPostgresWithSharedDSN",
			storageDriver:   "postgres",
			storageDSN:      "postgres://main",
			requirePostgres: true,
			want:            sessionStoreConfig{Driver: "postgres", DSN: "postgres://main"},
		},
		{
			name:            "ProductionRejectsExplicitMemory",
			flagDriver:      "memory",
			storageDriver:   "postgres",
			storageDSN:      "postgres://main",
			requirePostgres: true,
			wantErr:         true,
		},
		{
			name:            "ProductionRejectsImplicitMemory",
			storageDriver:   "json",
			requirePostgres: true,
			wantErr:         true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := resolveSessionStoreConfig(tc.flagDriver, tc.envDriver, tc.storageDriver, tc.storageDSN, tc.flagDSN, tc.envDSN, tc.requirePostgres)
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

func TestStartupSummaryPostgresRedis(t *testing.T) {
	summary := newStartupSummary(startupSummaryInput{
		StorageDriver: "postgres",
		StorageDSN:    "postgres://user:secret@localhost/db?sslmode=disable",
		SessionConfig: sessionStoreConfig{Driver: "postgres", DSN: "postgres://session:secret@localhost/sessions"},
		RateLimit: server.RateLimitConfig{
			RedisAddr:       "127.0.0.1:6379",
			RedisMasterName: "mymaster",
		},
		ChatDriver: "redis",
		ChatConfig: chat.RedisQueueConfig{
			Addr:   "redis://chat:6379",
			Stream: "chat-stream",
			Group:  "chat-group",
		},
		IngestConfig: ingest.Config{
			SRSBaseURL:        "http://srs",
			OMEBaseURL:        "http://ome",
			JobBaseURL:        "http://job",
			HealthEndpoint:    "/healthz",
			MaxBootAttempts:   5,
			RetryInterval:     750 * time.Millisecond,
			HTTPMaxAttempts:   4,
			HTTPRetryInterval: 2 * time.Second,
		},
		IngestControllerActive: true,
	})
	args := summary.LogArgs()
	mapped := summaryArgsToMap(t, args)
	datastore := mappedValueAsMap(t, mapped, "datastore")
	if got := datastore["driver"]; got != "postgres" {
		t.Fatalf("expected datastore driver postgres, got %v", got)
	}
	if raw, ok := datastore["dsn"].(string); !ok || (strings.Contains(raw, "secret")) || (!strings.Contains(raw, "*****") && !strings.Contains(raw, "%2A")) {
		t.Fatalf("expected datastore DSN to be redacted, got %q", datastore["dsn"])
	}
	session := mappedValueAsMap(t, mapped, "session_store")
	if got := session["driver"]; got != "postgres" {
		t.Fatalf("expected session driver postgres, got %v", got)
	}
	if raw, ok := session["dsn"].(string); !ok || (strings.Contains(raw, "secret")) || (!strings.Contains(raw, "*****") && !strings.Contains(raw, "%2A")) {
		t.Fatalf("expected session DSN to be redacted, got %q", session["dsn"])
	}
	login := mappedValueAsMap(t, mapped, "login_throttle")
	if got := login["driver"]; got != "redis" {
		t.Fatalf("expected login throttle driver redis, got %v", got)
	}
	if _, ok := login["addr"]; !ok {
		t.Fatalf("expected login throttle addr to be present")
	}
	if _, ok := login["master_name"]; !ok {
		t.Fatalf("expected login throttle master_name to be present")
	}
	chatSummary := mappedValueAsMap(t, mapped, "chat_queue")
	if got := chatSummary["driver"]; got != "redis" {
		t.Fatalf("expected chat queue driver redis, got %v", got)
	}
	if chatSummary["stream"] != "chat-stream" {
		t.Fatalf("expected chat stream to be recorded, got %v", chatSummary["stream"])
	}
	ingestSummary := mappedValueAsMap(t, mapped, "ingest")
	if got := ingestSummary["enabled"]; got != true {
		t.Fatalf("expected ingest to be enabled, got %v", got)
	}
	for _, key := range []string{"srs_api", "ome_api", "transcoder_api", "health_endpoint", "max_boot_attempts", "retry_interval", "http_max_attempts", "http_retry_interval"} {
		if _, ok := ingestSummary[key]; !ok {
			t.Fatalf("expected ingest summary to include %s", key)
		}
	}
}

func TestStartupSummaryMemoryDefaults(t *testing.T) {
	summary := newStartupSummary(startupSummaryInput{
		StorageDriver: "json",
		StoragePath:   "/tmp/data.json",
		SessionConfig: sessionStoreConfig{Driver: "memory"},
		ChatDriver:    "memory",
		RateLimit:     server.RateLimitConfig{},
	})
	args := summary.LogArgs()
	mapped := summaryArgsToMap(t, args)
	datastore := mappedValueAsMap(t, mapped, "datastore")
	if datastore["driver"] != "json" {
		t.Fatalf("expected datastore driver json, got %v", datastore["driver"])
	}
	if datastore["path"] != "/tmp/data.json" {
		t.Fatalf("expected datastore path to be recorded, got %v", datastore["path"])
	}
	session := mappedValueAsMap(t, mapped, "session_store")
	if session["driver"] != "memory" {
		t.Fatalf("expected session driver memory, got %v", session["driver"])
	}
	if _, ok := session["dsn"]; ok {
		t.Fatalf("did not expect session DSN for memory driver")
	}
	login := mappedValueAsMap(t, mapped, "login_throttle")
	if login["driver"] != "memory" {
		t.Fatalf("expected login throttle driver memory, got %v", login["driver"])
	}
	chatSummary := mappedValueAsMap(t, mapped, "chat_queue")
	if chatSummary["driver"] != "memory" {
		t.Fatalf("expected chat queue driver memory, got %v", chatSummary["driver"])
	}
	ingestSummary := mappedValueAsMap(t, mapped, "ingest")
	if ingestSummary["enabled"] != false {
		t.Fatalf("expected ingest to be disabled, got %v", ingestSummary["enabled"])
	}
}

func summaryArgsToMap(t *testing.T, args []any) map[string]any {
	t.Helper()
	if len(args)%2 != 0 {
		t.Fatalf("summary args must be key/value pairs, got %d values", len(args))
	}
	mapped := make(map[string]any, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			t.Fatalf("summary key at position %d was not a string", i)
		}
		mapped[key] = args[i+1]
	}
	return mapped
}

func mappedValueAsMap(t *testing.T, mapped map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := mapped[key]
	if !ok {
		t.Fatalf("missing key %q", key)
	}
	inner, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value for %q was not a map, got %T", key, value)
	}
	return inner
}
