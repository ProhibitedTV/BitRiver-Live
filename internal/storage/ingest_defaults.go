package storage

import "time"

const defaultIngestOperationTimeout = 12 * time.Second

func normalizeIngestTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return defaultIngestOperationTimeout
	}
	return timeout
}
