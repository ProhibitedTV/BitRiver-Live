package main

import (
	"bytes"
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

func TestLogIngestConfigResultDisabled(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	if fatal := logIngestConfigResult(logger, ingest.ErrConfigDisabled); fatal {
		t.Fatal("expected disabled ingest to be non-fatal")
	}

	if got := buf.String(); !strings.Contains(got, "disabled") || !strings.Contains(got, "BITRIVER_SRS_API") {
		t.Fatalf("unexpected log output: %s", got)
	}
}

func TestLogIngestConfigResultMissingFields(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	err := ingest.MissingConfigError{Missing: []string{"BITRIVER_SRS_API", "BITRIVER_SRS_TOKEN"}}

	if fatal := logIngestConfigResult(logger, err); !fatal {
		t.Fatal("expected missing ingest config to be fatal")
	}

	if got := buf.String(); !strings.Contains(got, "missing_env") || !strings.Contains(got, "BITRIVER_SRS_TOKEN") {
		t.Fatalf("unexpected log output: %s", got)
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

func TestResolveMode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		flag    string
		env     string
		want    string
		wantErr bool
	}{
		{name: "FlagWins", flag: "development", env: "production", want: "development"},
		{name: "EnvOnly", env: "production", want: "production"},
		{name: "Missing", wantErr: true},
		{name: "Invalid", flag: "staging", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mode, err := resolveMode(tc.flag, tc.env)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if mode != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, mode)
			}
		})
	}
}

func TestValidateMetricsProtection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mode    string
		cfg     server.MetricsAccessConfig
		wantErr bool
	}{
		{name: "ProductionRequiresProtection", mode: "production", wantErr: true},
		{name: "ProductionWithToken", mode: "production", cfg: server.MetricsAccessConfig{Token: "secret"}},
		{name: "DevelopmentAllowsEmpty", mode: "development"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := validateMetricsProtection(tc.mode, tc.cfg)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if !strings.Contains(err.Error(), "BITRIVER_LIVE_METRICS_TOKEN") {
					t.Fatalf("expected metrics env hint, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveStorageDriverRejectsNonPostgres(t *testing.T) {
	if _, explicit, err := resolveStorageDriver("json", "", "postgres://example"); err == nil {
		t.Fatal("expected error for non-postgres storage driver")
	} else if !explicit {
		t.Fatal("expected explicit=true when flag provided")
	}
}

func TestResolveStorageDriverMissingConfigFails(t *testing.T) {
	if _, _, err := resolveStorageDriver("", "", ""); err == nil {
		t.Fatal("resolveStorageDriver expected error when no configuration provided")
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

func TestValidateProductionDatastoreRejectsInsecureSSLMode(t *testing.T) {
	err := validateProductionDatastore("postgres", "postgres://resolved@db.example/db?sslmode=disable", "postgres://env")
	if err == nil {
		t.Fatal("expected error when sslmode=disable is configured in production")
	}
	if !strings.Contains(err.Error(), "sslmode") {
		t.Fatalf("expected sslmode guidance, got %v", err)
	}
}

func TestValidateProductionDatastoreAllowsComposeSSLModeDisable(t *testing.T) {
	err := validateProductionDatastore("postgres", "postgres://user:pass@postgres:5432/db?sslmode=disable", "postgres://env")
	if err != nil {
		t.Fatalf("expected sslmode=disable to be allowed for local compose, got %v", err)
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
			storageDriver: "postgres",
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
			name:          "ErrorsWhenPostgresSelectedWithoutDSN",
			flagDriver:    "postgres",
			storageDriver: "postgres",
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
			storageDriver:   "postgres",
			requirePostgres: true,
			wantErr:         true,
		},
		{
			name:            "ProductionRejectsInsecureSessionDSN",
			storageDriver:   "postgres",
			storageDSN:      "postgres://main?sslmode=require",
			envDSN:          "postgres://sessions@db.example/sessions?sslmode=disable",
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
		StorageDSN:    "postgres://user:secret@localhost/db?sslmode=require",
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

func TestResolveLoginLimit(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		flag    int
		env     string
		want    int
		wantErr bool
	}{
		{
			name:    "ProductionRequiresExplicitLimit",
			mode:    "production",
			flag:    0,
			wantErr: true,
		},
		{
			name: "ProductionUsesFlag",
			mode: "production",
			flag: 7,
			want: 7,
		},
		{
			name: "ProductionUsesEnv",
			mode: "production",
			env:  "12",
			want: 12,
		},
		{
			name: "DevelopmentFallsBackToDefault",
			mode: "development",
			want: defaultLoginLimitNonProduction,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("BITRIVER_LIVE_RATE_LOGIN_LIMIT", tc.env)

			got, err := resolveLoginLimit(tc.mode, tc.flag, "BITRIVER_LIVE_RATE_LOGIN_LIMIT")
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
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
