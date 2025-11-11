package storage

import (
	"strings"
	"time"

	"bitriver-live/internal/ingest"
)

type Option interface {
	applyJSON(*Storage)
	applyPostgres(*PostgresConfig)
}

type optionAdapter struct {
	json func(*Storage)
	pg   func(*PostgresConfig)
}

func (o optionAdapter) applyJSON(store *Storage) {
	if o.json != nil && store != nil {
		o.json(store)
	}
}

func (o optionAdapter) applyPostgres(cfg *PostgresConfig) {
	if o.pg != nil && cfg != nil {
		o.pg(cfg)
	}
}

func composeOption(json func(*Storage), pg func(*PostgresConfig)) Option {
	return optionAdapter{json: json, pg: pg}
}

func postgresOnlyOption(pg func(*PostgresConfig)) Option {
	return optionAdapter{pg: pg}
}

func WithIngestController(controller ingest.Controller) Option {
	return composeOption(
		func(s *Storage) {
			s.ingestController = controller
		},
		func(cfg *PostgresConfig) {
			cfg.IngestController = controller
		},
	)
}

func WithIngestRetries(maxAttempts int, interval time.Duration) Option {
	return composeOption(
		func(s *Storage) {
			if maxAttempts > 0 {
				s.ingestMaxAttempts = maxAttempts
			}
			if interval >= 0 {
				s.ingestRetryInterval = interval
			}
		},
		func(cfg *PostgresConfig) {
			if maxAttempts > 0 {
				cfg.IngestMaxAttempts = maxAttempts
			}
			if interval >= 0 {
				cfg.IngestRetryInterval = interval
			}
		},
	)
}

func WithRecordingRetention(policy RecordingRetentionPolicy) Option {
	return composeOption(
		func(s *Storage) {
			if policy.Published >= 0 {
				s.recordingRetention.Published = policy.Published
			}
			if policy.Unpublished >= 0 {
				s.recordingRetention.Unpublished = policy.Unpublished
			}
		},
		func(cfg *PostgresConfig) {
			if policy.Published >= 0 {
				cfg.RecordingRetention.Published = policy.Published
			}
			if policy.Unpublished >= 0 {
				cfg.RecordingRetention.Unpublished = policy.Unpublished
			}
		},
	)
}

func WithObjectStorage(cfg ObjectStorageConfig) Option {
	stored := cfg
	return composeOption(
		func(s *Storage) {
			s.objectStorage = stored
		},
		func(cfg *PostgresConfig) {
			cfg.ObjectStorage = stored
		},
	)
}

func WithPostgresPoolLimits(maxConns, minConns int32) Option {
	return postgresOnlyOption(func(cfg *PostgresConfig) {
		if maxConns > 0 {
			cfg.MaxConnections = maxConns
		}
		if minConns >= 0 {
			cfg.MinConnections = minConns
		}
	})
}

// WithPostgresAcquireTimeout configures how long the repository waits to obtain a
// connection from the pool. The same deadline is reused for the initial
// statement executed with that connection, ensuring queries do not run longer
// than the acquisition timeout. Operators should size the timeout to cover
// both acquiring a connection and executing short setup queries.
func WithPostgresAcquireTimeout(timeout time.Duration) Option {
	return postgresOnlyOption(func(cfg *PostgresConfig) {
		if timeout > 0 {
			cfg.AcquireTimeout = timeout
		}
	})
}

func WithPostgresPoolDurations(maxLifetime, maxIdle, healthInterval time.Duration) Option {
	return postgresOnlyOption(func(cfg *PostgresConfig) {
		if maxLifetime > 0 {
			cfg.MaxConnLifetime = maxLifetime
		}
		if maxIdle > 0 {
			cfg.MaxConnIdleTime = maxIdle
		}
		if healthInterval > 0 {
			cfg.HealthCheckInterval = healthInterval
		}
	})
}

func WithPostgresApplicationName(name string) Option {
	return postgresOnlyOption(func(cfg *PostgresConfig) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			cfg.ApplicationName = trimmed
		}
	})
}
