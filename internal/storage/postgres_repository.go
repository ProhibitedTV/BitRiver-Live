package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"github.com/jackc/pgx/v5"
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
	controller := r.ingestController
	var statuses []ingest.HealthStatus
	if controller == nil {
		statuses = []ingest.HealthStatus{{Component: "ingest", Status: "disabled"}}
	} else {
		statuses = controller.HealthChecks(ctx)
		if len(statuses) == 0 {
			statuses = []ingest.HealthStatus{{Component: "ingest", Status: "unknown"}}
		}
	}

	snapshot := append([]ingest.HealthStatus(nil), statuses...)
	r.ingestHealthMu.Lock()
	r.ingestHealth = snapshot
	r.ingestHealthUpdated = time.Now().UTC()
	r.ingestHealthMu.Unlock()

	return snapshot
}

func (r *postgresRepository) LastIngestHealth() ([]ingest.HealthStatus, time.Time) {
	r.ingestHealthMu.RLock()
	defer r.ingestHealthMu.RUnlock()
	clone := append([]ingest.HealthStatus(nil), r.ingestHealth...)
	return clone, r.ingestHealthUpdated
}

func (r *postgresRepository) CreateUser(params CreateUserParams) (models.User, error) {
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		return models.User{}, fmt.Errorf("email is required")
	}

	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		return models.User{}, fmt.Errorf("displayName is required")
	}

	roles := normalizeRoles(params.Roles)
	if params.SelfSignup {
		if params.Password == "" {
			return models.User{}, fmt.Errorf("password is required for self-service signup")
		}
		if len(roles) == 0 {
			roles = []string{"viewer"}
		}
	}

	id, err := r.generateID()
	if err != nil {
		return models.User{}, err
	}

	var passwordHash string
	if params.Password != "" {
		hashed, hashErr := hashPassword(params.Password)
		if hashErr != nil {
			return models.User{}, fmt.Errorf("hash password: %w", hashErr)
		}
		passwordHash = hashed
	}

	var createdAt time.Time
	createErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create user tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var existingID string
		err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", normalizedEmail).Scan(&existingID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check existing email: %w", err)
		}
		if err == nil {
			return fmt.Errorf("email %s already in use", params.Email)
		}

		err = tx.QueryRow(ctx, "INSERT INTO users (id, display_name, email, roles, password_hash, self_signup) VALUES ($1, $2, $3, $4, $5, $6) RETURNING created_at", id, displayName, normalizedEmail, roles, passwordHash, params.SelfSignup).Scan(&createdAt)
		if err != nil {
			return fmt.Errorf("insert user: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create user: %w", err)
		}
		return nil
	})
	if createErr != nil {
		return models.User{}, createErr
	}

	return models.User{
		ID:           id,
		DisplayName:  displayName,
		Email:        normalizedEmail,
		Roles:        roles,
		PasswordHash: passwordHash,
		SelfSignup:   params.SelfSignup,
		CreatedAt:    createdAt.UTC(),
	}, nil
}

func (r *postgresRepository) AuthenticateUser(email, password string) (models.User, error) {
	if password == "" {
		return models.User{}, fmt.Errorf("password is required")
	}
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}

	trimmedEmail := strings.TrimSpace(strings.ToLower(email))
	var user models.User
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users WHERE email = $1", trimmedEmail)
		scanned, scanErr := scanUser(row)
		if scanErr != nil {
			return scanErr
		}
		user = scanned
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrInvalidCredentials
	}
	if err != nil {
		return models.User{}, fmt.Errorf("authenticate user: %w", err)
	}
	if user.PasswordHash == "" {
		return models.User{}, ErrPasswordLoginUnsupported
	}
	if err := verifyPassword(user.PasswordHash, password); err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return models.User{}, ErrInvalidCredentials
		}
		return models.User{}, err
	}
	return user, nil
}

func (r *postgresRepository) ListUsers() []models.User {
	if r == nil || r.pool == nil {
		return nil
	}

	var users []models.User
	listErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users ORDER BY created_at ASC")
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			user, scanErr := scanUser(rows)
			if scanErr != nil {
				return scanErr
			}
			users = append(users, user)
		}
		return rows.Err()
	})
	if listErr != nil {
		return nil
	}
	return users
}

func (r *postgresRepository) GetUser(id string) (models.User, bool) {
	if r == nil || r.pool == nil {
		return models.User{}, false
	}

	var user models.User
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users WHERE id = $1", id)
		scanned, scanErr := scanUser(row)
		if scanErr != nil {
			return scanErr
		}
		user = scanned
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, false
	}
	if err != nil {
		return models.User{}, false
	}
	return user, true
}

func (r *postgresRepository) UpdateUser(id string, update UserUpdate) (models.User, error) {
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}

	var updated models.User
	updateErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin update user tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		row := tx.QueryRow(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users WHERE id = $1 FOR UPDATE", id)
		user, err := scanUser(row)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("user %s not found", id)
		}
		if err != nil {
			return fmt.Errorf("load user %s: %w", id, err)
		}

		if update.DisplayName != nil {
			name := strings.TrimSpace(*update.DisplayName)
			if name == "" {
				return fmt.Errorf("displayName cannot be empty")
			}
			user.DisplayName = name
		}

		if update.Email != nil {
			email := strings.TrimSpace(strings.ToLower(*update.Email))
			if email == "" {
				return fmt.Errorf("email cannot be empty")
			}
			var existingID string
			err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND id <> $2", email, id).Scan(&existingID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("check email uniqueness: %w", err)
			}
			if err == nil {
				return fmt.Errorf("email %s already in use", email)
			}
			user.Email = email
		}

		if update.Roles != nil {
			user.Roles = normalizeRoles(*update.Roles)
		}

		_, err = tx.Exec(ctx, "UPDATE users SET display_name = $1, email = $2, roles = $3 WHERE id = $4", user.DisplayName, user.Email, user.Roles, id)
		if err != nil {
			return fmt.Errorf("update user %s: %w", id, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit update user: %w", err)
		}

		updated = user
		return nil
	})
	if updateErr != nil {
		return models.User{}, updateErr
	}

	return updated, nil
}

func (r *postgresRepository) DeleteUser(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}

	deleteErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin delete user tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var userExists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", id).Scan(&userExists); err != nil {
			return fmt.Errorf("check user %s existence: %w", id, err)
		}
		if !userExists {
			return fmt.Errorf("user %s not found", id)
		}

		var ownedChannelID string
		err = tx.QueryRow(ctx, "SELECT id FROM channels WHERE owner_id = $1 LIMIT 1", id).Scan(&ownedChannelID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check owned channels: %w", err)
		}
		if err == nil {
			return fmt.Errorf("user %s owns channel %s; transfer or delete the channel first", id, ownedChannelID)
		}

		if _, err := tx.Exec(ctx, "UPDATE profiles SET top_friends = array_remove(top_friends, $1), updated_at = NOW() WHERE $1 = ANY(top_friends)", id); err != nil {
			return fmt.Errorf("remove user %s from top friends: %w", id, err)
		}

		if _, err := tx.Exec(ctx, "DELETE FROM users WHERE id = $1", id); err != nil {
			return fmt.Errorf("delete user %s: %w", id, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit delete user: %w", err)
		}

		return nil
	})
	if deleteErr != nil {
		return deleteErr
	}

	return nil
}

func (r *postgresRepository) acquireContext() (context.Context, context.CancelFunc) {
	if r == nil {
		return context.Background(), func() {}
	}
	if r.cfg.AcquireTimeout > 0 {
		return context.WithTimeout(context.Background(), r.cfg.AcquireTimeout)
	}
	return context.Background(), func() {}
}

func (r *postgresRepository) withConn(fn func(context.Context, *pgxpool.Conn) error) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx, cancel := r.acquireContext()
	defer cancel()
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire postgres connection: %w", err)
	}
	defer conn.Release()
	return fn(context.Background(), conn)
}

func (r *postgresRepository) generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func rollbackTx(ctx context.Context, tx pgx.Tx) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		// Ignore rollback errors when the transaction has already been closed.
	}
}

func scanUser(row pgx.Row) (models.User, error) {
	var (
		id, displayName, email, passwordHash string
		roles                                []string
		selfSignup                           bool
		createdAt                            time.Time
	)
	if err := row.Scan(&id, &displayName, &email, &roles, &passwordHash, &selfSignup, &createdAt); err != nil {
		return models.User{}, err
	}
	return models.User{
		ID:           id,
		DisplayName:  displayName,
		Email:        email,
		Roles:        rolesFromDB(roles),
		PasswordHash: passwordHash,
		SelfSignup:   selfSignup,
		CreatedAt:    createdAt.UTC(),
	}, nil
}

func rolesFromDB(roles []string) []string {
	if len(roles) == 0 {
		return nil
	}
	cloned := append([]string(nil), roles...)
	return cloned
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

func (r *postgresRepository) RotateChannelStreamKey(id string) (models.Channel, error) {
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
