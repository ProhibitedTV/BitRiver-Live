package chat

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"bitriver-live/internal/testsupport/redisstub"
)

func TestRedisQueueFanoutPlain(t *testing.T) {
	runRedisQueueIntegration(t, false)
}

func TestRedisQueueFanoutTLS(t *testing.T) {
	runRedisQueueIntegration(t, true)
}

func runRedisQueueIntegration(t *testing.T, useTLS bool) {
	t.Helper()
	srv, err := redisstub.Start(redisstub.Options{Password: "secret", EnableTLS: useTLS})
	if err != nil {
		t.Fatalf("failed to start redis stub: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})
	cfg := RedisQueueConfig{
		Addr:         srv.Addr(),
		Password:     "secret",
		Stream:       "test-stream",
		Group:        "test-group",
		BlockTimeout: 200 * time.Millisecond,
	}
	if useTLS {
		dir := t.TempDir()
		caPath := filepath.Join(dir, "ca.pem")
		if err := os.WriteFile(caPath, srv.CertPEM(), 0o600); err != nil {
			t.Fatalf("write ca file: %v", err)
		}
		cfg.TLS = RedisTLSConfig{CAFile: caPath}
	}
	queue, err := NewRedisQueue(cfg)
	if err != nil {
		t.Fatalf("create queue: %v", err)
	}
	sub := queue.Subscribe()
	t.Cleanup(func() {
		sub.Close()
	})
	event := Event{
		Type: EventTypeMessage,
		Message: &MessageEvent{
			ID:        "evt-1",
			ChannelID: "channel-1",
			UserID:    "user-1",
			Content:   "hello",
			CreatedAt: time.Now().UTC().Truncate(time.Millisecond),
		},
		OccurredAt: time.Now().UTC(),
	}
	if err := queue.Publish(context.Background(), event); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case got := <-sub.Events():
		if got.Type != event.Type || got.Message == nil || got.Message.Content != event.Message.Content {
			t.Fatalf("unexpected event: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for event")
	}
}
