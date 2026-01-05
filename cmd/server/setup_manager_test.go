package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/envutil"
)

func TestSetupManagerAppliesConfigAndSignalsRestart(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	seed := "BITRIVER_LIVE_MODE=development\nBITRIVER_LIVE_ADMIN_EMAIL=old@example.com\n"
	if err := os.WriteFile(envPath, []byte(seed), 0o600); err != nil {
		t.Fatalf("seed env: %v", err)
	}

	restartCh := make(chan struct{}, 1)
	manager := newSetupManager(envPath, restartCh)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	cfg := api.SetupConfig{
		AdminEmail:       "admin@example.com",
		AdminPassword:    "super-secret",
		ViewerURL:        "https://viewer.example.com",
		PublicAPIURL:     "https://api.example.com",
		ViewerOrigin:     "https://viewer.internal",
		APIPort:          9090,
		TLSCertPath:      "/etc/ssl/certs/example.crt",
		TLSKeyPath:       "/etc/ssl/private/example.key",
		PostgresPassword: "postgres-pass",
		RedisPassword:    "redis-pass",
		MetricsToken:     "metrics-token",
		SRSToken:         "srs-token",
		OMEToken:         "ome-token",
		TranscoderToken:  "trans-token",
	}

	result, err := manager.ApplySetup(ctx, cfg)
	if err != nil {
		t.Fatalf("ApplySetup returned error: %v", err)
	}
	if !result.RestartScheduled {
		t.Fatalf("expected restart to be scheduled")
	}

	select {
	case <-restartCh:
	default:
		t.Fatalf("expected restart signal to be emitted")
	}

	values, err := envutil.LoadFile(envPath, nil)
	if err != nil {
		t.Fatalf("load env: %v", err)
	}
	if values["BITRIVER_LIVE_ADMIN_EMAIL"] != "admin@example.com" {
		t.Fatalf("admin email not written: %#v", values)
	}
	if values["BITRIVER_LIVE_PORT"] != "9090" || values["BITRIVER_LIVE_ADDR"] != ":9090" {
		t.Fatalf("api port not updated: %#v", values)
	}
	if values["BITRIVER_LIVE_MODE"] != "production" {
		t.Fatalf("expected mode to be forced to production, got %q", values["BITRIVER_LIVE_MODE"])
	}
	if values["BITRIVER_OME_ACCESS_TOKEN"] != "ome-token" {
		t.Fatalf("expected OME token to mirror access token")
	}
}

func TestSetupManagerRollsBackOnRestartFailure(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	original := "BITRIVER_LIVE_ADMIN_EMAIL=keeper@example.com\nBITRIVER_LIVE_MODE=production\n"
	if err := os.WriteFile(envPath, []byte(original), 0o600); err != nil {
		t.Fatalf("seed env: %v", err)
	}

	restartCh := make(chan struct{})
	manager := newSetupManager(envPath, restartCh)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	cfg := api.SetupConfig{
		AdminEmail:       "admin@example.com",
		ViewerURL:        "https://viewer.example.com",
		APIPort:          8080,
		PostgresPassword: "postgres-pass",
		RedisPassword:    "redis-pass",
		SRSToken:         "srs-token",
		OMEToken:         "ome-token",
		TranscoderToken:  "trans-token",
	}

	if _, err := manager.ApplySetup(ctx, cfg); err == nil {
		t.Fatalf("expected ApplySetup to fail due to restart timeout")
	}

	values, err := envutil.LoadFile(envPath, nil)
	if err != nil {
		t.Fatalf("load env: %v", err)
	}
	if values["BITRIVER_LIVE_ADMIN_EMAIL"] != "keeper@example.com" {
		t.Fatalf("expected env to be restored after failure, got %#v", values)
	}
}
