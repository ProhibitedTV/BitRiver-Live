package server

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"bitriver-live/internal/testsupport/redisstub"
)

func TestRedisStoreAllowCanceledContext(t *testing.T) {
	srv, err := redisstub.Start(redisstub.Options{})
	if err != nil {
		t.Fatalf("start redis stub: %v", err)
	}
	t.Cleanup(func() {
		_ = srv.Close()
	})

	store, err := newRedisStore(redisStoreConfig{Addr: srv.Addr(), Timeout: time.Second})
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close(context.Background())
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err = store.Allow(ctx, "login:test-cancel", 1, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestRedisStoreAllowTimeout(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
	})
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer func() {
					_ = c.Close()
				}()
				<-time.After(250 * time.Millisecond)
			}(conn)
		}
	}()

	store, err := newRedisStore(redisStoreConfig{Addr: ln.Addr().String(), Timeout: 25 * time.Millisecond})
	if err != nil {
		t.Fatalf("new redis store: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close(context.Background())
	})

	_, _, err = store.Allow(context.Background(), "login:test-timeout", 1, time.Second)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
