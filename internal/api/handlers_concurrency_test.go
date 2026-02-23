package api

import (
	"sync"
	"testing"
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
