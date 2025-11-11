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
