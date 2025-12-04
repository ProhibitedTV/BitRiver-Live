package storage

import (
	"time"

	"bitriver-live/internal/ingest"
)

// PostgresConfig describes how the repository initialises its Postgres
// connection pool, orchestrates ingest behaviour, and integrates with object
// storage backends when persisting recording metadata.
type PostgresConfig struct {
	DSN                 string
	MaxConnections      int32
	MinConnections      int32
	MaxConnLifetime     time.Duration
	MaxConnIdleTime     time.Duration
	HealthCheckInterval time.Duration
	AcquireTimeout      time.Duration
	ApplicationName     string
	IngestController    ingest.Controller
	IngestMaxAttempts   int
	IngestRetryInterval time.Duration
	IngestTimeout       time.Duration
	RecordingRetention  RecordingRetentionPolicy
	ObjectStorage       ObjectStorageConfig
	RetentionClock      func() time.Time
}

func newPostgresConfig(dsn string, opts ...Option) PostgresConfig {
	cfg := PostgresConfig{
		DSN:               dsn,
		IngestController:  ingest.NoopController{},
		IngestMaxAttempts: 1,
		IngestTimeout:     defaultIngestOperationTimeout,
		RecordingRetention: RecordingRetentionPolicy{
			Published:   90 * 24 * time.Hour,
			Unpublished: 14 * 24 * time.Hour,
		},
		RetentionClock: func() time.Time { return time.Now().UTC() },
	}
	for _, opt := range opts {
		if opt != nil {
			opt.applyPostgres(&cfg)
		}
	}
	if cfg.IngestController == nil {
		cfg.IngestController = ingest.NoopController{}
	}
	if cfg.IngestMaxAttempts <= 0 {
		cfg.IngestMaxAttempts = 1
	}
	cfg.IngestTimeout = normalizeIngestTimeout(cfg.IngestTimeout)
	return cfg
}
