package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeSessionManager struct {
	calls chan struct{}
	err   error
}

func newFakeSessionManager() *fakeSessionManager {
	return &fakeSessionManager{calls: make(chan struct{}, 1)}
}

func (f *fakeSessionManager) PurgeExpired() error {
	select {
	case f.calls <- struct{}{}:
	default:
	}
	return f.err
}

type blockingSessionManager struct {
	started chan struct{}
	release chan struct{}
}

func newBlockingSessionManager() *blockingSessionManager {
	return &blockingSessionManager{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
}

func (b *blockingSessionManager) PurgeExpired() error {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-b.release
	return nil
}

func (b *blockingSessionManager) Release() {
	select {
	case <-b.release:
		return
	default:
		close(b.release)
	}
}

type manualTicker struct {
	c       chan time.Time
	stopped chan struct{}
}

func newManualTicker() *manualTicker {
	return &manualTicker{
		c:       make(chan time.Time, 1),
		stopped: make(chan struct{}),
	}
}

func (m *manualTicker) C() <-chan time.Time {
	return m.c
}

func (m *manualTicker) Stop() {
	select {
	case <-m.stopped:
		return
	default:
		close(m.stopped)
	}
}

func (m *manualTicker) Tick() {
	select {
	case m.c <- time.Now():
	default:
	}
}

func TestStartSessionPurgeWorker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := newManualTicker()
	sessions := newFakeSessionManager()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	stop := startSessionPurgeWorkerWithTicker(ctx, logger, sessions, time.Minute, func(time.Duration) purgeTicker {
		return ticker
	})

	ticker.Tick()
	select {
	case <-sessions.calls:
	case <-time.After(time.Second):
		t.Fatal("expected purge to be invoked")
	}

	cancel()
	stop()

	select {
	case <-ticker.stopped:
	case <-time.After(time.Second):
		t.Fatal("expected ticker to stop after context cancellation")
	}
}

func TestSessionPurgeWorkerStopDoesNotBlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ticker := newManualTicker()
	sessions := newBlockingSessionManager()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	stop := startSessionPurgeWorkerWithTicker(ctx, logger, sessions, time.Minute, func(time.Duration) purgeTicker {
		return ticker
	})

	ticker.Tick()

	select {
	case <-sessions.started:
	case <-time.After(time.Second):
		t.Fatal("expected purge to begin")
	}

	cancel()

	stopped := make(chan struct{})
	go func() {
		stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected stop to return without waiting for purge completion")
	}

	sessions.Release()

	select {
	case <-ticker.stopped:
	case <-time.After(time.Second):
		t.Fatal("expected ticker to stop after releasing purge")
	}
}
