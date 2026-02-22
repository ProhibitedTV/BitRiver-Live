package testsupport

import (
	"testing"
	"time"
)

const defaultPollInterval = 25 * time.Millisecond

// WaitUntil polls condition until it returns true or timeout elapses.
// description should describe the expected state for clearer timeout diagnostics.
func WaitUntil(t testing.TB, timeout time.Duration, description string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	attempts := 0
	for {
		attempts++
		if condition() {
			return
		}
		if time.Now().After(deadline) {
			if description == "" {
				description = "condition"
			}
			t.Fatalf("timed out after %s waiting for %s (poll_interval=%s, attempts=%d)", timeout, description, defaultPollInterval, attempts)
		}
		<-ticker.C
	}
}
