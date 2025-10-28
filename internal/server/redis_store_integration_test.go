package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bitriver-live/internal/testsupport/redisstub"
)

func TestRedisStoreAllowPlain(t *testing.T) {
	runRedisStoreIntegration(t, false)
}

func TestRedisStoreAllowTLS(t *testing.T) {
	runRedisStoreIntegration(t, true)
}

func runRedisStoreIntegration(t *testing.T, useTLS bool) {
	t.Helper()
	srv, err := redisstub.Start(redisstub.Options{Password: "secret", EnableTLS: useTLS})
	if err != nil {
		t.Fatalf("start redis stub: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})
	cfg := redisStoreConfig{
		Addr:     srv.Addr(),
		Password: "secret",
		Timeout:  time.Second,
	}
	if useTLS {
		dir := t.TempDir()
		caPath := filepath.Join(dir, "ca.pem")
		if err := os.WriteFile(caPath, srv.CertPEM(), 0o600); err != nil {
			t.Fatalf("write ca: %v", err)
		}
		cfg.TLS = RedisTLSConfig{CAFile: caPath}
	}
	store, err := newRedisStore(cfg)
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close(context.Background())
	})
	allowed, retry, err := store.Allow("login:test", 2, time.Second)
	if err != nil || !allowed || retry != 0 {
		t.Fatalf("first allow unexpected: allowed=%v retry=%v err=%v", allowed, retry, err)
	}
	allowed, retry, err = store.Allow("login:test", 2, time.Second)
	if err != nil || !allowed {
		t.Fatalf("second allow unexpected: allowed=%v retry=%v err=%v", allowed, retry, err)
	}
	allowed, retry, err = store.Allow("login:test", 2, time.Second)
	if err != nil {
		t.Fatalf("third allow err: %v", err)
	}
	if allowed {
		t.Fatalf("expected throttle on third attempt")
	}
	if retry < 0 {
		t.Fatalf("expected non-negative retry, got %v", retry)
	}
}
