package storage

import (
	"time"

	"bitriver-live/internal/ingest"
)

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
	RecordingRetention  RecordingRetentionPolicy
	ObjectStorage       ObjectStorageConfig
}

func newPostgresConfig(dsn string, opts ...Option) PostgresConfig {
	cfg := PostgresConfig{
		DSN:               dsn,
		IngestController:  ingest.NoopController{},
		IngestMaxAttempts: 1,
		RecordingRetention: RecordingRetentionPolicy{
			Published:   90 * 24 * time.Hour,
			Unpublished: 14 * 24 * time.Hour,
		},
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
	return cfg
}
