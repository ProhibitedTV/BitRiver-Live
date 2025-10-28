package storage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrPostgresUnavailable is returned when the Postgres repository has not yet
// been wired into the build.
var ErrPostgresUnavailable = fmt.Errorf("postgres repository unavailable")

type postgresRepository struct {
	pool                *pgxpool.Pool
	cfg                 PostgresConfig
	ingestController    ingest.Controller
	ingestMaxAttempts   int
	ingestRetryInterval time.Duration
	ingestHealthMu      sync.RWMutex
	ingestHealth        []ingest.HealthStatus
	ingestHealthUpdated time.Time
	recordingRetention  RecordingRetentionPolicy
	objectStorage       ObjectStorageConfig
	objectClient        objectStorageClient
}

func (r *postgresRepository) Close(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		r.pool.Close()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// NewPostgresRepository opens a Postgres-backed repository. The caller must
// ensure database migrations have been applied prior to invoking this
// constructor.
func NewPostgresRepository(dsn string, opts ...Option) (Repository, error) {
	cfg := newPostgresConfig(dsn, opts...)
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("postgres dsn required")
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	if cfg.MaxConnections > 0 {
		poolCfg.MaxConns = cfg.MaxConnections
	}
	if cfg.MinConnections >= 0 {
		poolCfg.MinConns = cfg.MinConnections
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckInterval > 0 {
		poolCfg.HealthCheckPeriod = cfg.HealthCheckInterval
	}
	if cfg.AcquireTimeout > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
		poolCfg.ConnConfig.ConnectTimeout = cfg.AcquireTimeout
	}
	if cfg.ApplicationName != "" {
		if poolCfg.ConnConfig.RuntimeParams == nil {
			poolCfg.ConnConfig.RuntimeParams = make(map[string]string)
		}
		poolCfg.ConnConfig.RuntimeParams["application_name"] = cfg.ApplicationName
	}

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}

	repo := &postgresRepository{
		pool:                pool,
		cfg:                 cfg,
		ingestController:    cfg.IngestController,
		ingestMaxAttempts:   cfg.IngestMaxAttempts,
		ingestRetryInterval: cfg.IngestRetryInterval,
		ingestHealth:        []ingest.HealthStatus{{Component: "ingest", Status: "disabled"}},
		ingestHealthUpdated: time.Now().UTC(),
		recordingRetention:  cfg.RecordingRetention,
		objectStorage:       cfg.ObjectStorage,
	}
	repo.objectClient = newObjectStorageClient(repo.objectStorage)
	return repo, nil
}

func (r *postgresRepository) IngestHealth(ctx context.Context) []ingest.HealthStatus {
	return []ingest.HealthStatus{{Component: "postgres", Status: "unknown"}}
}

func (r *postgresRepository) LastIngestHealth() ([]ingest.HealthStatus, time.Time) {
	r.ingestHealthMu.RLock()
	defer r.ingestHealthMu.RUnlock()
	clone := append([]ingest.HealthStatus(nil), r.ingestHealth...)
	return clone, r.ingestHealthUpdated
}

func (r *postgresRepository) CreateUser(params CreateUserParams) (models.User, error) {
	return models.User{}, ErrPostgresUnavailable
}

func (r *postgresRepository) AuthenticateUser(email, password string) (models.User, error) {
	return models.User{}, ErrPostgresUnavailable
}

func (r *postgresRepository) ListUsers() []models.User {
	return nil
}

func (r *postgresRepository) GetUser(id string) (models.User, bool) {
	return models.User{}, false
}

func (r *postgresRepository) UpdateUser(id string, update UserUpdate) (models.User, error) {
	return models.User{}, ErrPostgresUnavailable
}

func (r *postgresRepository) DeleteUser(id string) error {
	return ErrPostgresUnavailable
}

func (r *postgresRepository) UpsertProfile(userID string, update ProfileUpdate) (models.Profile, error) {
	return models.Profile{}, ErrPostgresUnavailable
}

func (r *postgresRepository) GetProfile(userID string) (models.Profile, bool) {
	return models.Profile{}, false
}

func (r *postgresRepository) ListProfiles() []models.Profile {
	return nil
}

func (r *postgresRepository) CreateChannel(ownerID, title, category string, tags []string) (models.Channel, error) {
	return models.Channel{}, ErrPostgresUnavailable
}

func (r *postgresRepository) UpdateChannel(id string, update ChannelUpdate) (models.Channel, error) {
	return models.Channel{}, ErrPostgresUnavailable
}

func (r *postgresRepository) DeleteChannel(id string) error {
	return ErrPostgresUnavailable
}

func (r *postgresRepository) GetChannel(id string) (models.Channel, bool) {
	return models.Channel{}, false
}

func (r *postgresRepository) ListChannels(ownerID string) []models.Channel {
	return nil
}

func (r *postgresRepository) FollowChannel(userID, channelID string) error {
	return ErrPostgresUnavailable
}

func (r *postgresRepository) UnfollowChannel(userID, channelID string) error {
	return ErrPostgresUnavailable
}

func (r *postgresRepository) IsFollowingChannel(userID, channelID string) bool {
	return false
}

func (r *postgresRepository) CountFollowers(channelID string) int {
	return 0
}

func (r *postgresRepository) ListFollowedChannelIDs(userID string) []string {
	return nil
}

func (r *postgresRepository) StartStream(channelID string, renditions []string) (models.StreamSession, error) {
	return models.StreamSession{}, ErrPostgresUnavailable
}

func (r *postgresRepository) StopStream(channelID string, peakConcurrent int) (models.StreamSession, error) {
	return models.StreamSession{}, ErrPostgresUnavailable
}

func (r *postgresRepository) CurrentStreamSession(channelID string) (models.StreamSession, bool) {
	return models.StreamSession{}, false
}

func (r *postgresRepository) ListStreamSessions(channelID string) ([]models.StreamSession, error) {
	return nil, ErrPostgresUnavailable
}

func (r *postgresRepository) ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error) {
	return nil, ErrPostgresUnavailable
}

func (r *postgresRepository) GetRecording(id string) (models.Recording, bool) {
	return models.Recording{}, false
}

func (r *postgresRepository) PublishRecording(id string) (models.Recording, error) {
	return models.Recording{}, ErrPostgresUnavailable
}

func (r *postgresRepository) DeleteRecording(id string) error {
	return ErrPostgresUnavailable
}

func (r *postgresRepository) CreateClipExport(recordingID string, params ClipExportParams) (models.ClipExport, error) {
	return models.ClipExport{}, ErrPostgresUnavailable
}

func (r *postgresRepository) ListClipExports(recordingID string) ([]models.ClipExport, error) {
	return nil, ErrPostgresUnavailable
}

func (r *postgresRepository) CreateChatMessage(channelID, userID, content string) (models.ChatMessage, error) {
	return models.ChatMessage{}, ErrPostgresUnavailable
}

func (r *postgresRepository) DeleteChatMessage(channelID, messageID string) error {
	return ErrPostgresUnavailable
}

func (r *postgresRepository) ListChatMessages(channelID string, limit int) ([]models.ChatMessage, error) {
	return nil, ErrPostgresUnavailable
}

func (r *postgresRepository) ChatRestrictions() chat.RestrictionsSnapshot {
	return chat.RestrictionsSnapshot{}
}

func (r *postgresRepository) IsChatBanned(channelID, userID string) bool {
	return false
}

func (r *postgresRepository) ChatTimeout(channelID, userID string) (time.Time, bool) {
	return time.Time{}, false
}

func (r *postgresRepository) ApplyChatEvent(evt chat.Event) error {
	return ErrPostgresUnavailable
}

func (r *postgresRepository) ListChatRestrictions(channelID string) []models.ChatRestriction {
	return nil
}

func (r *postgresRepository) CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (models.ChatReport, error) {
	return models.ChatReport{}, ErrPostgresUnavailable
}

func (r *postgresRepository) ListChatReports(channelID string, includeResolved bool) ([]models.ChatReport, error) {
	return nil, ErrPostgresUnavailable
}

func (r *postgresRepository) ResolveChatReport(reportID, resolverID, resolution string) (models.ChatReport, error) {
	return models.ChatReport{}, ErrPostgresUnavailable
}

func (r *postgresRepository) CreateTip(params CreateTipParams) (models.Tip, error) {
	return models.Tip{}, ErrPostgresUnavailable
}

func (r *postgresRepository) ListTips(channelID string, limit int) ([]models.Tip, error) {
	return nil, ErrPostgresUnavailable
}

func (r *postgresRepository) CreateSubscription(params CreateSubscriptionParams) (models.Subscription, error) {
	return models.Subscription{}, ErrPostgresUnavailable
}

func (r *postgresRepository) ListSubscriptions(channelID string, includeInactive bool) ([]models.Subscription, error) {
	return nil, ErrPostgresUnavailable
}

func (r *postgresRepository) GetSubscription(id string) (models.Subscription, bool) {
	return models.Subscription{}, false
}

func (r *postgresRepository) CancelSubscription(id, cancelledBy, reason string) (models.Subscription, error) {
	return models.Subscription{}, ErrPostgresUnavailable
}

var _ Repository = (*postgresRepository)(nil)
