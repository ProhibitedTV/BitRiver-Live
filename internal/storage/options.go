package storage

import (
	"strings"
	"time"

	"bitriver-live/internal/ingest"
)

// Option configures how the storage repository behaves when constructing
// in-memory configuration or Postgres-backed storage. Each option can apply to
// JSON-derived configuration, the Postgres connection settings, or both.
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

// WithIngestController wires a custom ingest controller into the repository so
// callers can override how ingest operations are performed and monitored.
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

// WithIngestRetries adjusts the maximum number of ingest attempts and delay
// between retries used when ingesting recordings fails transiently.
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

// WithIngestTimeout changes the deadline allowed for a single ingest operation
// before it is aborted.
func WithIngestTimeout(timeout time.Duration) Option {
	return composeOption(
		func(s *Storage) {
			if timeout > 0 {
				s.ingestTimeout = timeout
			}
		},
		func(cfg *PostgresConfig) {
			if timeout > 0 {
				cfg.IngestTimeout = timeout
			}
		},
	)
}

// WithRecordingRetention customises how long published and unpublished
// recordings are retained before cleanup.
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

// WithRetentionClock overrides the clock used when evaluating recording
// retention windows. Primarily intended for tests that need deterministic
// retention behaviour.
func WithRetentionClock(clock func() time.Time) Option {
	return composeOption(
		func(s *Storage) {
			if clock != nil {
				s.retentionNow = clock
			}
		},
		func(cfg *PostgresConfig) {
			if clock != nil {
				cfg.RetentionClock = clock
			}
		},
	)
}

// WithObjectStorage overrides the object storage configuration used to archive
// or retrieve recording assets.
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

// WithPostgresPoolLimits caps the number of open connections in the Postgres
// pool and optionally sets a floor for idle connections kept ready.
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
// connection from the pool. The same deadline is reused across repository
// helpers so transactions, queries, and subsequent statements executed with the
// acquired connection respect the acquisition timeout. Operators should size the
// timeout to cover both acquiring a connection and executing the associated
// database work.
func WithPostgresAcquireTimeout(timeout time.Duration) Option {
	return postgresOnlyOption(func(cfg *PostgresConfig) {
		if timeout > 0 {
			cfg.AcquireTimeout = timeout
		}
	})
}

// WithPostgresPoolDurations adjusts how long connections live, how long they
// may remain idle, and how frequently health checks run against the pool.
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

// WithPostgresApplicationName sets the application name reported to Postgres
// for new connections, helping operators identify this service in monitoring
// tools.
func WithPostgresApplicationName(name string) Option {
	return postgresOnlyOption(func(cfg *PostgresConfig) {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			cfg.ApplicationName = trimmed
		}
	})
}
