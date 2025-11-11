package main

import (
	"log/slog"
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
