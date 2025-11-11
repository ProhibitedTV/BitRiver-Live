package main

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type sessionPurger interface {
	PurgeExpired() error
}

type purgeTicker interface {
	C() <-chan time.Time
	Stop()
}

type timeTicker struct {
	ticker *time.Ticker
}

func (t timeTicker) C() <-chan time.Time {
	return t.ticker.C
}

func (t timeTicker) Stop() {
	t.ticker.Stop()
}

type tickerFactory func(time.Duration) purgeTicker

func startSessionPurgeWorker(ctx context.Context, logger *slog.Logger, sessions sessionPurger, interval time.Duration) func() {
	return startSessionPurgeWorkerWithTicker(ctx, logger, sessions, interval, func(d time.Duration) purgeTicker {
		return timeTicker{ticker: time.NewTicker(d)}
	})
}

func startSessionPurgeWorkerWithTicker(
	ctx context.Context,
	logger *slog.Logger,
	sessions sessionPurger,
	interval time.Duration,
	newTicker tickerFactory,
) func() {
	if sessions == nil || interval <= 0 {
		return func() {}
	}
	workerCtx, cancel := context.WithCancel(ctx)
	ticker := newTicker(interval)
	done := make(chan struct{})
	go func() {
		defer func() {
			ticker.Stop()
			close(done)
		}()
		for {
			select {
			case <-workerCtx.Done():
				return
			case <-ticker.C():
				if err := sessions.PurgeExpired(); err != nil && logger != nil {
					logger.Error("failed to purge expired sessions", "error", err)
				}
			}
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}
