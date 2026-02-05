package storage

import "time"

const defaultIngestOperationTimeout = 12 * time.Second

// normalizeIngestTimeout performs normalize ingest timeout and propagates validation or dependency failures to the caller.
func normalizeIngestTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultIngestOperationTimeout
	}
	return timeout
}
