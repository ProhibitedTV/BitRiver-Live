package api

import (
	"io"
	"log/slog"
	"sync"
	"testing"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/observability/tracing"
)

func TestHandlerAccessorsConcurrentInitialization(t *testing.T) {
	h := &Handler{}
	const goroutines = 64

	sessions := make(chan interface{}, goroutines)
	mfas := make(chan interface{}, goroutines)
	loggers := make(chan interface{}, goroutines)
	tracers := make(chan interface{}, goroutines)
	trackers := make(chan interface{}, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			sessions <- h.sessionManager()
			mfas <- h.mfaChallengeManager()
			loggers <- h.logger()
			tracers <- h.tracer()
			trackers <- h.srsTracker()
		}()
	}
	wg.Wait()
	close(sessions)
	close(mfas)
	close(loggers)
	close(tracers)
	close(trackers)

	assertAllSameNonNil := func(name string, ch <-chan interface{}) {
		t.Helper()
		var first interface{}
		for v := range ch {
			if v == nil {
				t.Fatalf("%s accessor returned nil", name)
			}
			if first == nil {
				first = v
				continue
			}
			if first != v {
				t.Fatalf("%s accessor returned multiple instances", name)
			}
		}
	}

	assertAllSameNonNil("sessionManager", sessions)
	assertAllSameNonNil("mfaChallengeManager", mfas)
	assertAllSameNonNil("logger", loggers)
	assertAllSameNonNil("tracer", tracers)
	assertAllSameNonNil("srsTracker", trackers)
}

func TestHandlerAccessorsConcurrentPreconfiguredDependencies(t *testing.T) {
	preconfiguredSessions := auth.NewSessionManager(0)
	preconfiguredMFAChallenges := auth.NewMFAChallengeManager(0)
	preconfiguredLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	preconfiguredTracer := &tracing.Tracer{}
	preconfiguredTracker := newSRSViewerTracker()

	h := &Handler{
		Sessions:      preconfiguredSessions,
		MFAChallenges: preconfiguredMFAChallenges,
		Logger:        preconfiguredLogger,
		Tracer:        preconfiguredTracer,
		srsViewers:    preconfiguredTracker,
	}

	const goroutines = 128

	sessions := make(chan *auth.SessionManager, goroutines)
	mfas := make(chan *auth.MFAChallengeManager, goroutines)
	loggers := make(chan *slog.Logger, goroutines)
	tracers := make(chan *tracing.Tracer, goroutines)
	trackers := make(chan *srsViewerTracker, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			sessions <- h.sessionManager()
			mfas <- h.mfaChallengeManager()
			loggers <- h.logger()
			tracers <- h.tracer()
			trackers <- h.srsTracker()
		}()
	}
	wg.Wait()
	close(sessions)
	close(mfas)
	close(loggers)
	close(tracers)
	close(trackers)

	assertAllExpectedSessionManagerPointers := func(ch <-chan *auth.SessionManager) {
		t.Helper()
		for v := range ch {
			if v != preconfiguredSessions {
				t.Fatalf("sessionManager accessor overwrote injected dependency: got %p, want %p", v, preconfiguredSessions)
			}
		}
	}

	assertAllExpectedMFAChallengeManagerPointers := func(ch <-chan *auth.MFAChallengeManager) {
		t.Helper()
		for v := range ch {
			if v != preconfiguredMFAChallenges {
				t.Fatalf("mfaChallengeManager accessor overwrote injected dependency: got %p, want %p", v, preconfiguredMFAChallenges)
			}
		}
	}

	assertAllExpectedLoggerPointers := func(ch <-chan *slog.Logger) {
		t.Helper()
		for v := range ch {
			if v != preconfiguredLogger {
				t.Fatalf("logger accessor overwrote injected dependency: got %p, want %p", v, preconfiguredLogger)
			}
		}
	}

	assertAllExpectedTracerPointers := func(ch <-chan *tracing.Tracer) {
		t.Helper()
		for v := range ch {
			if v != preconfiguredTracer {
				t.Fatalf("tracer accessor overwrote injected dependency: got %p, want %p", v, preconfiguredTracer)
			}
		}
	}

	assertAllExpectedTrackerPointers := func(ch <-chan *srsViewerTracker) {
		t.Helper()
		for v := range ch {
			if v != preconfiguredTracker {
				t.Fatalf("srsTracker accessor overwrote injected dependency: got %p, want %p", v, preconfiguredTracker)
			}
		}
	}

	assertAllExpectedSessionManagerPointers(sessions)
	assertAllExpectedMFAChallengeManagerPointers(mfas)
	assertAllExpectedLoggerPointers(loggers)
	assertAllExpectedTracerPointers(tracers)
	assertAllExpectedTrackerPointers(trackers)
}
