package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/ingest"
	"bitriver-live/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
	repo.objectStorage = applyObjectStorageDefaults(repo.objectStorage)
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

func (r *postgresRepository) SetUserPassword(id, password string) (models.User, error) {
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}
	if len(password) < 8 {
		return models.User{}, fmt.Errorf("password must be at least 8 characters")
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	var user models.User
	var roles []string
	updateErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, "UPDATE users SET password_hash = $1 WHERE id = $2 RETURNING id, display_name, email, roles, password_hash, self_signup, created_at", hashed, id)
		if err := row.Scan(&user.ID, &user.DisplayName, &user.Email, &roles, &user.PasswordHash, &user.SelfSignup, &user.CreatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("user %s not found", id)
			}
			return fmt.Errorf("update user password: %w", err)
		}
		return nil
	})
	if updateErr != nil {
		return models.User{}, updateErr
	}

	user.Roles = roles
	return user, nil
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
	return fn(ctx, conn)
}

func (r *postgresRepository) generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate id: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (r *postgresRepository) generateStreamKey() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate stream key: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(bytes)), nil
}

func encodeDonationAddresses(addresses []models.CryptoAddress) ([]byte, error) {
	if addresses == nil {
		addresses = []models.CryptoAddress{}
	}
	data, err := json.Marshal(addresses)
	if err != nil {
		return nil, fmt.Errorf("encode donation addresses: %w", err)
	}
	return data, nil
}

func decodeDonationAddresses(data []byte) ([]models.CryptoAddress, error) {
	if len(data) == 0 {
		return []models.CryptoAddress{}, nil
	}
	var addresses []models.CryptoAddress
	if err := json.Unmarshal(data, &addresses); err != nil {
		return nil, fmt.Errorf("decode donation addresses: %w", err)
	}
	if addresses == nil {
		addresses = []models.CryptoAddress{}
	}
	return addresses, nil
}

func (r *postgresRepository) loadStreamSession(ctx context.Context, id string) (models.StreamSession, bool) {
	if strings.TrimSpace(id) == "" {
		return models.StreamSession{}, false
	}
	var (
		channelID       string
		startedAt       time.Time
		endedAt         pgtype.Timestamptz
		renditions      []string
		peak            int
		originURL       string
		playbackURL     string
		ingestEndpoints []string
		ingestJobIDs    []string
	)
	err := r.pool.QueryRow(ctx, "SELECT channel_id, started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids FROM stream_sessions WHERE id = $1", id).
		Scan(&channelID, &startedAt, &endedAt, &renditions, &peak, &originURL, &playbackURL, &ingestEndpoints, &ingestJobIDs)
	if err != nil {
		return models.StreamSession{}, false
	}
	manifestsRows, err := r.pool.Query(ctx, "SELECT name, manifest_url, bitrate FROM stream_session_manifests WHERE session_id = $1", id)
	if err != nil {
		return models.StreamSession{}, false
	}
	defer manifestsRows.Close()
	manifests := make([]models.RenditionManifest, 0)
	for manifestsRows.Next() {
		var name, url string
		var bitrate pgtype.Int4
		if err := manifestsRows.Scan(&name, &url, &bitrate); err != nil {
			return models.StreamSession{}, false
		}
		entry := models.RenditionManifest{Name: name, ManifestURL: url}
		if bitrate.Valid {
			entry.Bitrate = int(bitrate.Int32)
		}
		manifests = append(manifests, entry)
	}
	if err := manifestsRows.Err(); err != nil {
		return models.StreamSession{}, false
	}
	session := models.StreamSession{
		ID:                 id,
		ChannelID:          channelID,
		StartedAt:          startedAt.UTC(),
		Renditions:         append([]string{}, renditions...),
		PeakConcurrent:     peak,
		OriginURL:          originURL,
		PlaybackURL:        playbackURL,
		IngestEndpoints:    append([]string{}, ingestEndpoints...),
		IngestJobIDs:       append([]string{}, ingestJobIDs...),
		RenditionManifests: manifests,
	}
	if endedAt.Valid {
		ts := endedAt.Time.UTC()
		session.EndedAt = &ts
	}
	if session.Renditions == nil {
		session.Renditions = []string{}
	}
	if session.RenditionManifests == nil {
		session.RenditionManifests = []models.RenditionManifest{}
	}
	if session.IngestEndpoints == nil {
		session.IngestEndpoints = []string{}
	}
	if session.IngestJobIDs == nil {
		session.IngestJobIDs = []string{}
	}
	return session, true
}

func (r *postgresRepository) recordingDeadline(now time.Time, published bool) *time.Time {
	var window time.Duration
	if published {
		window = r.recordingRetention.Published
	} else {
		window = r.recordingRetention.Unpublished
	}
	if window <= 0 {
		return nil
	}
	deadline := now.Add(window)
	return &deadline
}

func (r *postgresRepository) createRecording(session models.StreamSession, channel models.Channel, ended time.Time) (models.Recording, error) {
	recordingID, err := r.generateID()
	if err != nil {
		return models.Recording{}, err
	}
	duration := int(ended.Sub(session.StartedAt).Round(time.Second).Seconds())
	if duration < 0 {
		duration = 0
	}
	title := strings.TrimSpace(channel.Title)
	if title == "" {
		title = fmt.Sprintf("Recording %s", session.ID)
	}
	metadata := map[string]string{
		"channelId":  channel.ID,
		"sessionId":  session.ID,
		"startedAt":  session.StartedAt.UTC().Format(time.RFC3339Nano),
		"endedAt":    ended.UTC().Format(time.RFC3339Nano),
		"renditions": strconv.Itoa(len(session.RenditionManifests)),
	}
	if session.PeakConcurrent > 0 {
		metadata["peakConcurrent"] = strconv.Itoa(session.PeakConcurrent)
	}
	recording := models.Recording{
		ID:              recordingID,
		ChannelID:       channel.ID,
		SessionID:       session.ID,
		Title:           title,
		DurationSeconds: duration,
		PlaybackBaseURL: session.PlaybackURL,
		Metadata:        metadata,
		CreatedAt:       ended,
	}
	if deadline := r.recordingDeadline(ended, false); deadline != nil {
		recording.RetainUntil = deadline
	}
	if len(session.RenditionManifests) > 0 {
		renditions := make([]models.RecordingRendition, 0, len(session.RenditionManifests))
		for _, manifest := range session.RenditionManifests {
			renditions = append(renditions, models.RecordingRendition{
				Name:        manifest.Name,
				ManifestURL: manifest.ManifestURL,
				Bitrate:     manifest.Bitrate,
			})
		}
		recording.Renditions = renditions
	}
	if err := r.populateRecordingArtifacts(&recording, session); err != nil {
		return models.Recording{}, err
	}
	return recording, nil
}

func (r *postgresRepository) populateRecordingArtifacts(recording *models.Recording, session models.StreamSession) error {
	client := r.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if recording.Metadata == nil {
		recording.Metadata = make(map[string]string)
	}

	createdAt := recording.CreatedAt.UTC().Format(time.RFC3339Nano)
	if len(session.RenditionManifests) > 0 {
		for idx, manifest := range session.RenditionManifests {
			key := buildObjectKey("recordings", recording.ID, "manifests", normalizeObjectComponent(manifest.Name)+".json")
			payload := map[string]any{
				"recordingId": recording.ID,
				"sessionId":   recording.SessionID,
				"name":        manifest.Name,
				"source":      manifest.ManifestURL,
				"createdAt":   createdAt,
			}
			if manifest.Bitrate > 0 {
				payload["bitrate"] = manifest.Bitrate
			}
			data, err := json.Marshal(payload)
			if err != nil {
				return fmt.Errorf("encode manifest payload: %w", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), r.objectStorage.requestTimeout())
			ref, err := client.Upload(ctx, key, "application/json", data)
			cancel()
			if err != nil {
				return fmt.Errorf("upload manifest %s: %w", manifest.Name, err)
			}
			if ref.Key != "" {
				recording.Metadata[manifestMetadataKey(manifest.Name)] = ref.Key
			}
			if ref.URL != "" && idx < len(recording.Renditions) {
				recording.Renditions[idx].ManifestURL = ref.URL
			}
		}
	}

	thumbID, err := r.generateID()
	if err != nil {
		return fmt.Errorf("generate thumbnail id: %w", err)
	}
	thumbKey := buildObjectKey("recordings", recording.ID, "thumbnails", thumbID+".json")
	thumbPayload := map[string]any{
		"recordingId": recording.ID,
		"sessionId":   recording.SessionID,
		"createdAt":   createdAt,
	}
	thumbData, err := json.Marshal(thumbPayload)
	if err != nil {
		return fmt.Errorf("encode thumbnail payload: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.objectStorage.requestTimeout())
	ref, err := client.Upload(ctx, thumbKey, "application/json", thumbData)
	cancel()
	if err != nil {
		return fmt.Errorf("upload thumbnail: %w", err)
	}
	if ref.Key != "" {
		recording.Metadata[thumbnailMetadataKey(thumbID)] = ref.Key
	}
	thumbnail := models.RecordingThumbnail{
		ID:          thumbID,
		RecordingID: recording.ID,
		URL:         ref.URL,
		CreatedAt:   recording.CreatedAt,
	}
	recording.Thumbnails = append(recording.Thumbnails, thumbnail)
	return nil
}

func (r *postgresRepository) insertRecording(ctx context.Context, tx pgx.Tx, recording models.Recording) error {
	metadata := recording.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("encode recording metadata: %w", err)
	}
	var publishedAt any
	if recording.PublishedAt != nil {
		publishedAt = recording.PublishedAt
	}
	var retainUntil any
	if recording.RetainUntil != nil {
		retainUntil = recording.RetainUntil
	}
	_, err = tx.Exec(ctx, "INSERT INTO recordings (id, channel_id, session_id, title, duration_seconds, playback_base_url, metadata, published_at, created_at, retain_until) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
		recording.ID,
		recording.ChannelID,
		recording.SessionID,
		recording.Title,
		recording.DurationSeconds,
		recording.PlaybackBaseURL,
		metadataJSON,
		publishedAt,
		recording.CreatedAt,
		retainUntil,
	)
	if err != nil {
		return fmt.Errorf("insert recording %s: %w", recording.ID, err)
	}
	for _, rendition := range recording.Renditions {
		if _, err := tx.Exec(ctx, "INSERT INTO recording_renditions (recording_id, name, manifest_url, bitrate) VALUES ($1, $2, $3, $4)", recording.ID, rendition.Name, rendition.ManifestURL, rendition.Bitrate); err != nil {
			return fmt.Errorf("insert recording rendition %s: %w", rendition.Name, err)
		}
	}
	for _, thumb := range recording.Thumbnails {
		if _, err := tx.Exec(ctx, "INSERT INTO recording_thumbnails (id, recording_id, url, width, height, created_at) VALUES ($1, $2, $3, $4, $5, $6)", thumb.ID, recording.ID, thumb.URL, thumb.Width, thumb.Height, thumb.CreatedAt); err != nil {
			return fmt.Errorf("insert recording thumbnail %s: %w", thumb.ID, err)
		}
	}
	return nil
}

func (r *postgresRepository) deleteRecordingArtifacts(recording models.Recording) error {
	client := r.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	if len(recording.Metadata) == 0 {
		return nil
	}
	deleted := make(map[string]struct{})
	for key, objectKey := range recording.Metadata {
		if !strings.HasPrefix(key, metadataManifestPrefix) && !strings.HasPrefix(key, metadataThumbnailPrefix) {
			continue
		}
		trimmed := strings.TrimSpace(objectKey)
		if trimmed == "" {
			continue
		}
		if _, exists := deleted[trimmed]; exists {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), r.objectStorage.requestTimeout())
		err := client.Delete(ctx, trimmed)
		cancel()
		if err != nil {
			return fmt.Errorf("delete object %s: %w", trimmed, err)
		}
		deleted[trimmed] = struct{}{}
	}
	return nil
}

func (r *postgresRepository) deleteClipArtifacts(clip models.ClipExport) error {
	client := r.objectClient
	if client == nil || !client.Enabled() {
		return nil
	}
	trimmed := strings.TrimSpace(clip.StorageObject)
	if trimmed == "" {
		return nil
	}
	if err := client.Delete(context.Background(), trimmed); err != nil {
		return fmt.Errorf("delete clip object %s: %w", trimmed, err)
	}
	return nil
}

func (r *postgresRepository) purgeExpiredRecordings(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	now := time.Now().UTC()
	rows, err := r.pool.Query(ctx, "SELECT id, metadata FROM recordings WHERE retain_until IS NOT NULL AND retain_until <= $1", now)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := make([]string, 0)
	recordings := make(map[string]models.Recording)
	for rows.Next() {
		var id string
		var metadataBytes []byte
		if err := rows.Scan(&id, &metadataBytes); err != nil {
			return err
		}
		meta := make(map[string]string)
		if len(metadataBytes) > 0 {
			if err := json.Unmarshal(metadataBytes, &meta); err != nil {
				return fmt.Errorf("decode recording metadata: %w", err)
			}
		}
		recordings[id] = models.Recording{ID: id, Metadata: meta}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		recording := recordings[id]
		if err := r.deleteRecordingArtifacts(recording); err != nil {
			return err
		}
		clipRows, err := r.pool.Query(ctx, "SELECT id, storage_object FROM clip_exports WHERE recording_id = $1", id)
		if err != nil {
			return fmt.Errorf("load clip exports for recording %s: %w", id, err)
		}
		clips := make([]models.ClipExport, 0)
		for clipRows.Next() {
			var clip models.ClipExport
			if err := clipRows.Scan(&clip.ID, &clip.StorageObject); err != nil {
				clipRows.Close()
				return fmt.Errorf("scan clip export: %w", err)
			}
			clips = append(clips, clip)
		}
		clipRows.Close()
		for _, clip := range clips {
			if err := r.deleteClipArtifacts(clip); err != nil {
				return err
			}
		}
		if _, err := r.pool.Exec(ctx, "DELETE FROM recordings WHERE id = $1", id); err != nil {
			return fmt.Errorf("delete recording %s: %w", id, err)
		}
	}
	return nil
}

func (r *postgresRepository) loadRecording(ctx context.Context, id string) (models.Recording, bool, error) {
	var (
		channelID       string
		sessionID       string
		title           string
		duration        int
		playbackBaseURL string
		metadataBytes   []byte
		publishedAt     pgtype.Timestamptz
		createdAt       time.Time
		retainUntil     pgtype.Timestamptz
	)
	err := r.pool.QueryRow(ctx, "SELECT channel_id, session_id, title, duration_seconds, playback_base_url, metadata, published_at, created_at, retain_until FROM recordings WHERE id = $1", id).
		Scan(&channelID, &sessionID, &title, &duration, &playbackBaseURL, &metadataBytes, &publishedAt, &createdAt, &retainUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Recording{}, false, nil
	}
	if err != nil {
		return models.Recording{}, false, err
	}
	metadata := make(map[string]string)
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return models.Recording{}, false, fmt.Errorf("decode recording metadata: %w", err)
		}
	}
	recording := models.Recording{
		ID:              id,
		ChannelID:       channelID,
		SessionID:       sessionID,
		Title:           title,
		DurationSeconds: duration,
		PlaybackBaseURL: playbackBaseURL,
		Metadata:        metadata,
		CreatedAt:       createdAt.UTC(),
	}
	if publishedAt.Valid {
		ts := publishedAt.Time.UTC()
		recording.PublishedAt = &ts
	}
	if retainUntil.Valid {
		ts := retainUntil.Time.UTC()
		recording.RetainUntil = &ts
	}
	renditionsRows, err := r.pool.Query(ctx, "SELECT name, manifest_url, bitrate FROM recording_renditions WHERE recording_id = $1", id)
	if err != nil {
		return models.Recording{}, false, fmt.Errorf("load recording renditions: %w", err)
	}
	renditions := make([]models.RecordingRendition, 0)
	for renditionsRows.Next() {
		var name, url string
		var bitrate pgtype.Int4
		if err := renditionsRows.Scan(&name, &url, &bitrate); err != nil {
			renditionsRows.Close()
			return models.Recording{}, false, fmt.Errorf("scan recording rendition: %w", err)
		}
		entry := models.RecordingRendition{Name: name, ManifestURL: url}
		if bitrate.Valid {
			entry.Bitrate = int(bitrate.Int32)
		}
		renditions = append(renditions, entry)
	}
	renditionsRows.Close()
	if err := renditionsRows.Err(); err != nil {
		return models.Recording{}, false, fmt.Errorf("read recording renditions: %w", err)
	}
	recording.Renditions = renditions

	thumbRows, err := r.pool.Query(ctx, "SELECT id, url, width, height, created_at FROM recording_thumbnails WHERE recording_id = $1", id)
	if err != nil {
		return models.Recording{}, false, fmt.Errorf("load recording thumbnails: %w", err)
	}
	thumbnails := make([]models.RecordingThumbnail, 0)
	for thumbRows.Next() {
		var thumb models.RecordingThumbnail
		thumb.RecordingID = id
		if err := thumbRows.Scan(&thumb.ID, &thumb.URL, &thumb.Width, &thumb.Height, &thumb.CreatedAt); err != nil {
			thumbRows.Close()
			return models.Recording{}, false, fmt.Errorf("scan recording thumbnail: %w", err)
		}
		thumbnails = append(thumbnails, thumb)
	}
	thumbRows.Close()
	if err := thumbRows.Err(); err != nil {
		return models.Recording{}, false, fmt.Errorf("read recording thumbnails: %w", err)
	}
	recording.Thumbnails = thumbnails

	clipRows, err := r.pool.Query(ctx, "SELECT id, title, start_seconds, end_seconds, status FROM clip_exports WHERE recording_id = $1", id)
	if err != nil {
		return models.Recording{}, false, fmt.Errorf("load clip exports: %w", err)
	}
	clips := make([]models.ClipExportSummary, 0)
	for clipRows.Next() {
		var clip models.ClipExportSummary
		if err := clipRows.Scan(&clip.ID, &clip.Title, &clip.StartSeconds, &clip.EndSeconds, &clip.Status); err != nil {
			clipRows.Close()
			return models.Recording{}, false, fmt.Errorf("scan clip export: %w", err)
		}
		clips = append(clips, clip)
	}
	clipRows.Close()
	if err := clipRows.Err(); err != nil {
		return models.Recording{}, false, fmt.Errorf("read clip exports: %w", err)
	}
	if len(clips) > 0 {
		sort.Slice(clips, func(i, j int) bool {
			if clips[i].StartSeconds == clips[j].StartSeconds {
				return clips[i].ID < clips[j].ID
			}
			return clips[i].StartSeconds < clips[j].StartSeconds
		})
		recording.Clips = clips
	}
	return recording, true, nil
}

func (r *postgresRepository) loadUpload(ctx context.Context, id string) (models.Upload, bool, error) {
	var (
		channelID     string
		title         string
		filename      string
		sizeBytes     int64
		status        string
		progress      int
		recordingID   pgtype.Text
		playbackURL   pgtype.Text
		metadataBytes []byte
		errorText     pgtype.Text
		createdAt     time.Time
		updatedAt     time.Time
		completedAt   pgtype.Timestamptz
	)
	err := r.pool.QueryRow(ctx, "SELECT channel_id, title, filename, size_bytes, status, progress, recording_id, playback_url, metadata, error, created_at, updated_at, completed_at FROM uploads WHERE id = $1", id).
		Scan(&channelID, &title, &filename, &sizeBytes, &status, &progress, &recordingID, &playbackURL, &metadataBytes, &errorText, &createdAt, &updatedAt, &completedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Upload{}, false, nil
	}
	if err != nil {
		return models.Upload{}, false, err
	}
	metadata := make(map[string]string)
	if len(metadataBytes) > 0 {
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
			return models.Upload{}, false, fmt.Errorf("decode upload metadata: %w", err)
		}
	}
	upload := models.Upload{
		ID:        id,
		ChannelID: channelID,
		Title:     title,
		Filename:  filename,
		SizeBytes: sizeBytes,
		Status:    status,
		Progress:  progress,
		Metadata:  metadata,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: updatedAt.UTC(),
	}
	if recordingID.Valid {
		value := strings.TrimSpace(recordingID.String)
		if value != "" {
			upload.RecordingID = &value
		}
	}
	if playbackURL.Valid {
		upload.PlaybackURL = playbackURL.String
	}
	if errorText.Valid {
		upload.Error = errorText.String
	}
	if completedAt.Valid {
		ts := completedAt.Time.UTC()
		upload.CompletedAt = &ts
	}
	return upload, true, nil
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

func scanSubscriptionRow(row pgx.Row) (models.Subscription, error) {
	var (
		sub               models.Subscription
		cancelledBy       pgtype.Text
		cancelledReason   pgtype.Text
		cancelledAt       pgtype.Timestamptz
		externalReference pgtype.Text
	)
	if err := row.Scan(&sub.ID, &sub.ChannelID, &sub.UserID, &sub.Tier, &sub.Provider, &sub.Reference, &sub.Amount, &sub.Currency, &sub.StartedAt, &sub.ExpiresAt, &sub.AutoRenew, &sub.Status, &cancelledBy, &cancelledReason, &cancelledAt, &externalReference); err != nil {
		return models.Subscription{}, err
	}
	sub.StartedAt = sub.StartedAt.UTC()
	sub.ExpiresAt = sub.ExpiresAt.UTC()
	if cancelledBy.Valid {
		sub.CancelledBy = cancelledBy.String
	}
	if cancelledReason.Valid {
		sub.CancelledReason = cancelledReason.String
	}
	if cancelledAt.Valid {
		ts := cancelledAt.Time.UTC()
		sub.CancelledAt = &ts
	} else {
		sub.CancelledAt = nil
	}
	if externalReference.Valid {
		sub.ExternalReference = externalReference.String
	}
	return sub, nil
}

func ensureUserExists(ctx context.Context, tx pgx.Tx, userID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists); err != nil {
		return fmt.Errorf("check user %s: %w", userID, err)
	}
	if !exists {
		return fmt.Errorf("user %s not found", userID)
	}
	return nil
}

func ensureChannelExists(ctx context.Context, tx pgx.Tx, channelID string) error {
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
		return fmt.Errorf("check channel %s: %w", channelID, err)
	}
	if !exists {
		return fmt.Errorf("channel %s not found", channelID)
	}
	return nil
}

func (r *postgresRepository) UpsertProfile(userID string, update ProfileUpdate) (models.Profile, error) {
	if r == nil || r.pool == nil {
		return models.Profile{}, ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.Profile{}, fmt.Errorf("begin upsert profile tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var userCreatedAt time.Time
	if err := tx.QueryRow(ctx, "SELECT created_at FROM users WHERE id = $1", userID).Scan(&userCreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Profile{}, fmt.Errorf("user %s not found", userID)
		}
		return models.Profile{}, fmt.Errorf("load user %s: %w", userID, err)
	}

	profile := models.Profile{
		UserID:            userID,
		Bio:               "",
		TopFriends:        []string{},
		DonationAddresses: []models.CryptoAddress{},
		CreatedAt:         userCreatedAt.UTC(),
		UpdatedAt:         userCreatedAt.UTC(),
	}
	var (
		avatar, banner           pgtype.Text
		featured                 pgtype.Text
		topFriends               []string
		donationAddressesPayload []byte
		createdAt, updatedAt     time.Time
	)
	row := tx.QueryRow(ctx, "SELECT bio, avatar_url, banner_url, featured_channel_id, top_friends, donation_addresses, created_at, updated_at FROM profiles WHERE user_id = $1", userID)
	switch err := row.Scan(&profile.Bio, &avatar, &banner, &featured, &topFriends, &donationAddressesPayload, &createdAt, &updatedAt); {
	case errors.Is(err, pgx.ErrNoRows):
		// Use defaults.
	case err != nil:
		return models.Profile{}, fmt.Errorf("load profile %s: %w", userID, err)
	default:
		if avatar.Valid {
			profile.AvatarURL = avatar.String
		}
		if banner.Valid {
			profile.BannerURL = banner.String
		}
		if featured.Valid {
			id := featured.String
			profile.FeaturedChannelID = &id
		}
		if len(topFriends) > 0 {
			profile.TopFriends = append([]string{}, topFriends...)
		}
		if len(donationAddressesPayload) > 0 {
			decoded, err := decodeDonationAddresses(donationAddressesPayload)
			if err != nil {
				return models.Profile{}, fmt.Errorf("decode donation addresses: %w", err)
			}
			profile.DonationAddresses = decoded
		}
		profile.CreatedAt = createdAt.UTC()
		profile.UpdatedAt = updatedAt.UTC()
	}

	now := time.Now().UTC()

	if update.Bio != nil {
		profile.Bio = strings.TrimSpace(*update.Bio)
	}
	if update.AvatarURL != nil {
		profile.AvatarURL = strings.TrimSpace(*update.AvatarURL)
	}
	if update.BannerURL != nil {
		profile.BannerURL = strings.TrimSpace(*update.BannerURL)
	}
	if update.FeaturedChannelID != nil {
		trimmed := strings.TrimSpace(*update.FeaturedChannelID)
		if trimmed == "" {
			profile.FeaturedChannelID = nil
		} else {
			var ownerID string
			err := tx.QueryRow(ctx, "SELECT owner_id FROM channels WHERE id = $1", trimmed).Scan(&ownerID)
			if errors.Is(err, pgx.ErrNoRows) {
				return models.Profile{}, fmt.Errorf("featured channel %s not found", trimmed)
			}
			if err != nil {
				return models.Profile{}, fmt.Errorf("load featured channel %s: %w", trimmed, err)
			}
			if ownerID != userID {
				return models.Profile{}, errors.New("featured channel must belong to profile owner")
			}
			id := trimmed
			profile.FeaturedChannelID = &id
		}
	}
	if update.TopFriends != nil {
		if len(*update.TopFriends) > 8 {
			return models.Profile{}, errors.New("top friends cannot exceed eight entries")
		}
		seen := make(map[string]struct{}, len(*update.TopFriends))
		ordered := make([]string, 0, len(*update.TopFriends))
		for _, friendID := range *update.TopFriends {
			trimmed := strings.TrimSpace(friendID)
			if trimmed == "" {
				return models.Profile{}, errors.New("top friends must reference valid users")
			}
			if trimmed == userID {
				return models.Profile{}, errors.New("cannot add profile owner as a top friend")
			}
			if _, exists := seen[trimmed]; exists {
				return models.Profile{}, errors.New("duplicate user in top friends list")
			}
			seen[trimmed] = struct{}{}
			ordered = append(ordered, trimmed)
		}
		if len(ordered) > 0 {
			rows, err := tx.Query(ctx, "SELECT id FROM users WHERE id = ANY($1)", ordered)
			if err != nil {
				return models.Profile{}, fmt.Errorf("validate top friends: %w", err)
			}
			found := make(map[string]struct{}, len(ordered))
			for rows.Next() {
				var id string
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return models.Profile{}, fmt.Errorf("scan top friend id: %w", err)
				}
				found[id] = struct{}{}
			}
			rows.Close()
			for _, id := range ordered {
				if _, ok := found[id]; !ok {
					return models.Profile{}, fmt.Errorf("top friend %s not found", id)
				}
			}
		}
		profile.TopFriends = ordered
	}
	if update.DonationAddresses != nil {
		addresses := make([]models.CryptoAddress, 0, len(*update.DonationAddresses))
		for _, addr := range *update.DonationAddresses {
			currency := strings.ToUpper(strings.TrimSpace(addr.Currency))
			if currency == "" {
				return models.Profile{}, errors.New("donation currency is required")
			}
			address := strings.TrimSpace(addr.Address)
			if address == "" {
				return models.Profile{}, errors.New("donation address is required")
			}
			note := strings.TrimSpace(addr.Note)
			addresses = append(addresses, models.CryptoAddress{Currency: currency, Address: address, Note: note})
		}
		profile.DonationAddresses = addresses
	}

	profile.UpdatedAt = now
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = now
	}

	donationPayload, err := encodeDonationAddresses(profile.DonationAddresses)
	if err != nil {
		return models.Profile{}, err
	}
	var featuredValue any
	if profile.FeaturedChannelID != nil {
		featuredValue = *profile.FeaturedChannelID
	}
	topFriendsValue := profile.TopFriends
	if topFriendsValue == nil {
		topFriendsValue = []string{}
	}

	var insertedCreatedAt, insertedUpdatedAt time.Time
	err = tx.QueryRow(ctx, `
INSERT INTO profiles (user_id, bio, avatar_url, banner_url, featured_channel_id, top_friends, donation_addresses, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id) DO UPDATE SET
	bio = EXCLUDED.bio,
	avatar_url = EXCLUDED.avatar_url,
	banner_url = EXCLUDED.banner_url,
	featured_channel_id = EXCLUDED.featured_channel_id,
	top_friends = EXCLUDED.top_friends,
	donation_addresses = EXCLUDED.donation_addresses,
	updated_at = EXCLUDED.updated_at
RETURNING created_at, updated_at`,
		userID,
		profile.Bio,
		profile.AvatarURL,
		profile.BannerURL,
		featuredValue,
		topFriendsValue,
		donationPayload,
		profile.CreatedAt,
		profile.UpdatedAt,
	).Scan(&insertedCreatedAt, &insertedUpdatedAt)
	if err != nil {
		return models.Profile{}, fmt.Errorf("upsert profile %s: %w", userID, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.Profile{}, fmt.Errorf("commit upsert profile: %w", err)
	}

	profile.CreatedAt = insertedCreatedAt.UTC()
	profile.UpdatedAt = insertedUpdatedAt.UTC()
	if profile.TopFriends == nil {
		profile.TopFriends = []string{}
	}
	if profile.DonationAddresses == nil {
		profile.DonationAddresses = []models.CryptoAddress{}
	}
	return profile, nil
}

func (r *postgresRepository) GetProfile(userID string) (models.Profile, bool) {
	if r == nil || r.pool == nil {
		return models.Profile{}, false
	}
	ctx := context.Background()
	var (
		bio                      string
		avatar, banner, featured pgtype.Text
		topFriends               []string
		donationPayload          []byte
		createdAt, updatedAt     time.Time
	)
	err := r.pool.QueryRow(ctx, "SELECT bio, avatar_url, banner_url, featured_channel_id, top_friends, donation_addresses, created_at, updated_at FROM profiles WHERE user_id = $1", userID).
		Scan(&bio, &avatar, &banner, &featured, &topFriends, &donationPayload, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var userCreatedAt time.Time
		if err := r.pool.QueryRow(ctx, "SELECT created_at FROM users WHERE id = $1", userID).Scan(&userCreatedAt); err != nil {
			return models.Profile{}, false
		}
		profile := models.Profile{
			UserID:            userID,
			Bio:               "",
			AvatarURL:         "",
			BannerURL:         "",
			TopFriends:        []string{},
			DonationAddresses: []models.CryptoAddress{},
			CreatedAt:         userCreatedAt.UTC(),
			UpdatedAt:         userCreatedAt.UTC(),
		}
		return profile, false
	}
	if err != nil {
		return models.Profile{}, false
	}

	profile := models.Profile{
		UserID:     userID,
		Bio:        bio,
		CreatedAt:  createdAt.UTC(),
		UpdatedAt:  updatedAt.UTC(),
		TopFriends: []string{},
	}
	if avatar.Valid {
		profile.AvatarURL = avatar.String
	}
	if banner.Valid {
		profile.BannerURL = banner.String
	}
	if featured.Valid {
		id := featured.String
		profile.FeaturedChannelID = &id
	}
	if len(topFriends) > 0 {
		profile.TopFriends = append([]string{}, topFriends...)
	}
	if len(donationPayload) > 0 {
		addresses, err := decodeDonationAddresses(donationPayload)
		if err != nil {
			return models.Profile{}, false
		}
		profile.DonationAddresses = addresses
	} else {
		profile.DonationAddresses = []models.CryptoAddress{}
	}
	if profile.TopFriends == nil {
		profile.TopFriends = []string{}
	}
	return profile, true
}

func (r *postgresRepository) ListProfiles() []models.Profile {
	if r == nil || r.pool == nil {
		return nil
	}
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, "SELECT user_id, bio, avatar_url, banner_url, featured_channel_id, top_friends, donation_addresses, created_at, updated_at FROM profiles ORDER BY created_at ASC")
	if err != nil {
		return nil
	}
	defer rows.Close()

	profiles := make([]models.Profile, 0)
	for rows.Next() {
		var (
			userID                   string
			bio                      string
			avatar, banner, featured pgtype.Text
			topFriends               []string
			donationPayload          []byte
			createdAt, updatedAt     time.Time
		)
		if err := rows.Scan(&userID, &bio, &avatar, &banner, &featured, &topFriends, &donationPayload, &createdAt, &updatedAt); err != nil {
			return nil
		}
		profile := models.Profile{
			UserID:     userID,
			Bio:        bio,
			CreatedAt:  createdAt.UTC(),
			UpdatedAt:  updatedAt.UTC(),
			TopFriends: []string{},
		}
		if avatar.Valid {
			profile.AvatarURL = avatar.String
		}
		if banner.Valid {
			profile.BannerURL = banner.String
		}
		if featured.Valid {
			id := featured.String
			profile.FeaturedChannelID = &id
		}
		if len(topFriends) > 0 {
			profile.TopFriends = append([]string{}, topFriends...)
		}
		if len(donationPayload) > 0 {
			addresses, err := decodeDonationAddresses(donationPayload)
			if err != nil {
				return nil
			}
			profile.DonationAddresses = addresses
		} else {
			profile.DonationAddresses = []models.CryptoAddress{}
		}
		if profile.TopFriends == nil {
			profile.TopFriends = []string{}
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return profiles
}

func (r *postgresRepository) CreateChannel(ownerID, title, category string, tags []string) (models.Channel, error) {
	if r == nil || r.pool == nil {
		return models.Channel{}, ErrPostgresUnavailable
	}
	if strings.TrimSpace(ownerID) == "" {
		return models.Channel{}, fmt.Errorf("owner %s not found", ownerID)
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return models.Channel{}, errors.New("title is required")
	}

	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.Channel{}, fmt.Errorf("begin create channel tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", ownerID).Scan(&exists); err != nil {
		return models.Channel{}, fmt.Errorf("check owner %s: %w", ownerID, err)
	}
	if !exists {
		return models.Channel{}, fmt.Errorf("owner %s not found", ownerID)
	}

	id, err := r.generateID()
	if err != nil {
		return models.Channel{}, err
	}
	streamKey, err := r.generateStreamKey()
	if err != nil {
		return models.Channel{}, err
	}
	normalizedTags := normalizeTags(tags)
	now := time.Now().UTC()

	var insertedCreatedAt, insertedUpdatedAt time.Time
	err = tx.QueryRow(ctx, "INSERT INTO channels (id, owner_id, stream_key, title, category, tags, live_state, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 'offline', $7, $8) RETURNING created_at, updated_at",
		id,
		ownerID,
		streamKey,
		trimmedTitle,
		strings.TrimSpace(category),
		normalizedTags,
		now,
		now,
	).Scan(&insertedCreatedAt, &insertedUpdatedAt)
	if err != nil {
		return models.Channel{}, fmt.Errorf("insert channel: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Channel{}, fmt.Errorf("commit create channel: %w", err)
	}

	channel := models.Channel{
		ID:        id,
		OwnerID:   ownerID,
		StreamKey: streamKey,
		Title:     trimmedTitle,
		Category:  strings.TrimSpace(category),
		Tags:      normalizedTags,
		LiveState: "offline",
		CreatedAt: insertedCreatedAt.UTC(),
		UpdatedAt: insertedUpdatedAt.UTC(),
	}
	return channel, nil
}

func (r *postgresRepository) UpdateChannel(id string, update ChannelUpdate) (models.Channel, error) {
	if r == nil || r.pool == nil {
		return models.Channel{}, ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.Channel{}, fmt.Errorf("begin update channel tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var (
		channelID, ownerID, streamKey, title string
		category                             pgtype.Text
		tags                                 []string
		liveState                            string
		currentSession                       pgtype.Text
		createdAt, updatedAt                 time.Time
	)
	row := tx.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE id = $1 FOR UPDATE", id)
	if err := row.Scan(&channelID, &ownerID, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Channel{}, fmt.Errorf("channel %s not found", id)
		}
		return models.Channel{}, fmt.Errorf("load channel %s: %w", id, err)
	}

	channel := models.Channel{
		ID:        channelID,
		OwnerID:   ownerID,
		StreamKey: streamKey,
		Title:     title,
		Tags:      append([]string{}, tags...),
		LiveState: liveState,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: updatedAt.UTC(),
	}
	if category.Valid {
		channel.Category = category.String
	}
	if currentSession.Valid {
		id := currentSession.String
		channel.CurrentSessionID = &id
	}

	if update.Title != nil {
		trimmed := strings.TrimSpace(*update.Title)
		if trimmed == "" {
			return models.Channel{}, errors.New("title cannot be empty")
		}
		channel.Title = trimmed
	}
	if update.Category != nil {
		channel.Category = strings.TrimSpace(*update.Category)
	}
	if update.Tags != nil {
		channel.Tags = normalizeTags(*update.Tags)
	}
	if update.LiveState != nil {
		state := strings.ToLower(strings.TrimSpace(*update.LiveState))
		switch state {
		case "offline", "live", "starting", "ended":
			channel.LiveState = state
		default:
			return models.Channel{}, fmt.Errorf("invalid liveState %s", state)
		}
	}

	channel.UpdatedAt = time.Now().UTC()
	_, err = tx.Exec(ctx, "UPDATE channels SET title = $1, category = $2, tags = $3, live_state = $4, updated_at = $5 WHERE id = $6",
		channel.Title,
		channel.Category,
		channel.Tags,
		channel.LiveState,
		channel.UpdatedAt,
		channel.ID,
	)
	if err != nil {
		return models.Channel{}, fmt.Errorf("update channel %s: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Channel{}, fmt.Errorf("commit update channel: %w", err)
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, nil
}

func (r *postgresRepository) RotateChannelStreamKey(id string) (models.Channel, error) {
	if r == nil || r.pool == nil {
		return models.Channel{}, ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.Channel{}, fmt.Errorf("begin rotate stream key tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var (
		channelID, ownerID, streamKey, title string
		category                             pgtype.Text
		tags                                 []string
		liveState                            string
		currentSession                       pgtype.Text
		createdAt, updatedAt                 time.Time
	)
	row := tx.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE id = $1 FOR UPDATE", id)
	if err := row.Scan(&channelID, &ownerID, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Channel{}, fmt.Errorf("channel %s not found", id)
		}
		return models.Channel{}, fmt.Errorf("load channel %s: %w", id, err)
	}

	newKey, err := r.generateStreamKey()
	if err != nil {
		return models.Channel{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, "UPDATE channels SET stream_key = $1, updated_at = $2 WHERE id = $3", newKey, now, id); err != nil {
		return models.Channel{}, fmt.Errorf("update stream key: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Channel{}, fmt.Errorf("commit rotate stream key: %w", err)
	}

	channel := models.Channel{
		ID:        channelID,
		OwnerID:   ownerID,
		StreamKey: newKey,
		Title:     title,
		Tags:      append([]string{}, tags...),
		LiveState: liveState,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: now,
	}
	if category.Valid {
		channel.Category = category.String
	}
	if currentSession.Valid {
		current := currentSession.String
		channel.CurrentSessionID = &current
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, nil
}

func (r *postgresRepository) DeleteChannel(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin delete channel tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var currentSession pgtype.Text
	if err := tx.QueryRow(ctx, "SELECT current_session_id FROM channels WHERE id = $1 FOR UPDATE", id).Scan(&currentSession); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("channel %s not found", id)
		}
		return fmt.Errorf("load channel %s: %w", id, err)
	}
	if currentSession.Valid {
		return errors.New("cannot delete a channel with an active stream")
	}

	if _, err := tx.Exec(ctx, "UPDATE profiles SET featured_channel_id = NULL WHERE featured_channel_id = $1", id); err != nil {
		return fmt.Errorf("clear featured channel references: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM channels WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete channel %s: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete channel: %w", err)
	}
	return nil
}

func (r *postgresRepository) GetChannel(id string) (models.Channel, bool) {
	if r == nil || r.pool == nil {
		return models.Channel{}, false
	}
	ctx := context.Background()
	var (
		channelID, ownerID, streamKey, title string
		category                             pgtype.Text
		tags                                 []string
		liveState                            string
		currentSession                       pgtype.Text
		createdAt, updatedAt                 time.Time
	)
	err := r.pool.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE id = $1", id).
		Scan(&channelID, &ownerID, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return models.Channel{}, false
	}
	channel := models.Channel{
		ID:        channelID,
		OwnerID:   ownerID,
		StreamKey: streamKey,
		Title:     title,
		Tags:      append([]string{}, tags...),
		LiveState: liveState,
		CreatedAt: createdAt.UTC(),
		UpdatedAt: updatedAt.UTC(),
	}
	if category.Valid {
		channel.Category = category.String
	}
	if currentSession.Valid {
		current := currentSession.String
		channel.CurrentSessionID = &current
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, true
}

func (r *postgresRepository) ListChannels(ownerID, query string) []models.Channel {
	if r == nil || r.pool == nil {
		return nil
	}
	ctx, cancel := r.acquireContext()
	defer cancel()
	baseQuery := "SELECT c.id, c.owner_id, c.stream_key, c.title, c.category, c.tags, c.live_state, c.current_session_id, c.created_at, c.updated_at FROM channels c JOIN users u ON u.id = c.owner_id"
	trimmedOwner := strings.TrimSpace(ownerID)
	trimmedQuery := strings.TrimSpace(query)
	var (
		args    []interface{}
		clauses []string
	)
	if trimmedOwner != "" {
		args = append(args, trimmedOwner)
		clauses = append(clauses, fmt.Sprintf("c.owner_id = $%d", len(args)))
	}
	if trimmedQuery != "" {
		args = append(args, "%"+trimmedQuery+"%")
		argPos := len(args)
		clauses = append(clauses, fmt.Sprintf("(c.title ILIKE $%[1]d OR u.display_name ILIKE $%[1]d OR EXISTS (SELECT 1 FROM unnest(c.tags) AS tag WHERE tag ILIKE $%[1]d))", argPos))
	}
	if len(clauses) > 0 {
		baseQuery += " WHERE " + strings.Join(clauses, " AND ")
	}
	baseQuery += " ORDER BY CASE WHEN c.live_state = 'live' THEN 0 ELSE 1 END, c.created_at ASC"
	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	channels := make([]models.Channel, 0)
	for rows.Next() {
		var (
			channelID, ownerIDVal, streamKey, title string
			category                                pgtype.Text
			tags                                    []string
			liveState                               string
			currentSession                          pgtype.Text
			createdAt, updatedAt                    time.Time
		)
		if err := rows.Scan(&channelID, &ownerIDVal, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt); err != nil {
			return nil
		}
		channel := models.Channel{
			ID:        channelID,
			OwnerID:   ownerIDVal,
			StreamKey: streamKey,
			Title:     title,
			Tags:      append([]string{}, tags...),
			LiveState: liveState,
			CreatedAt: createdAt.UTC(),
			UpdatedAt: updatedAt.UTC(),
		}
		if category.Valid {
			channel.Category = category.String
		}
		if currentSession.Valid {
			current := currentSession.String
			channel.CurrentSessionID = &current
		}
		if channel.Tags == nil {
			channel.Tags = []string{}
		}
		channels = append(channels, channel)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return channels
}

func (r *postgresRepository) FollowChannel(userID, channelID string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin follow channel tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	if err := ensureUserExists(ctx, tx, userID); err != nil {
		return err
	}
	if err := ensureChannelExists(ctx, tx, channelID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, "INSERT INTO follows (user_id, channel_id, followed_at) VALUES ($1, $2, NOW()) ON CONFLICT DO NOTHING", userID, channelID); err != nil {
		return fmt.Errorf("follow channel %s: %w", channelID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit follow channel: %w", err)
	}
	return nil
}

func (r *postgresRepository) UnfollowChannel(userID, channelID string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin unfollow channel tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	if err := ensureUserExists(ctx, tx, userID); err != nil {
		return err
	}
	if err := ensureChannelExists(ctx, tx, channelID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, "DELETE FROM follows WHERE user_id = $1 AND channel_id = $2", userID, channelID); err != nil {
		return fmt.Errorf("unfollow channel %s: %w", channelID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit unfollow channel: %w", err)
	}
	return nil
}

func (r *postgresRepository) IsFollowingChannel(userID, channelID string) bool {
	if r == nil || r.pool == nil {
		return false
	}
	ctx := context.Background()
	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM follows WHERE user_id = $1 AND channel_id = $2)", userID, channelID).Scan(&exists); err != nil {
		return false
	}
	return exists
}

func (r *postgresRepository) CountFollowers(channelID string) int {
	if r == nil || r.pool == nil {
		return 0
	}
	ctx := context.Background()
	var count int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM follows WHERE channel_id = $1", channelID).Scan(&count); err != nil {
		return 0
	}
	return count
}

func (r *postgresRepository) ListFollowedChannelIDs(userID string) []string {
	if r == nil || r.pool == nil {
		return nil
	}
	ctx := context.Background()
	rows, err := r.pool.Query(ctx, "SELECT channel_id FROM follows WHERE user_id = $1 ORDER BY followed_at DESC", userID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var channelID string
		if err := rows.Scan(&channelID); err != nil {
			return nil
		}
		ids = append(ids, channelID)
	}
	if err := rows.Err(); err != nil {
		return nil
	}
	return ids
}

func (r *postgresRepository) StartStream(channelID string, renditions []string) (models.StreamSession, error) {
	if r == nil || r.pool == nil {
		return models.StreamSession{}, ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.StreamSession{}, fmt.Errorf("begin start stream tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var (
		streamKey                string
		currentSession           pgtype.Text
		ownerID, title, category pgtype.Text
		tags                     []string
	)
	row := tx.QueryRow(ctx, "SELECT stream_key, current_session_id, owner_id, title, category, tags FROM channels WHERE id = $1 FOR UPDATE", channelID)
	if err := row.Scan(&streamKey, &currentSession, &ownerID, &title, &category, &tags); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
		}
		return models.StreamSession{}, fmt.Errorf("load channel %s: %w", channelID, err)
	}
	if currentSession.Valid {
		return models.StreamSession{}, errors.New("channel already live")
	}

	sessionID, err := r.generateID()
	if err != nil {
		return models.StreamSession{}, err
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, "UPDATE channels SET current_session_id = $1, live_state = 'starting', updated_at = $2 WHERE id = $3", sessionID, now, channelID); err != nil {
		return models.StreamSession{}, fmt.Errorf("mark channel starting: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.StreamSession{}, fmt.Errorf("commit mark channel starting: %w", err)
	}

	attempts := r.ingestMaxAttempts
	if attempts <= 0 {
		attempts = 1
	}
	controller := r.ingestController
	var boot ingest.BootResult
	var bootErr error
	for attempt := 0; attempt < attempts; attempt++ {
		boot, bootErr = controller.BootStream(ctx, ingest.BootParams{
			ChannelID:  channelID,
			SessionID:  sessionID,
			StreamKey:  streamKey,
			Renditions: append([]string{}, renditions...),
		})
		if bootErr == nil {
			break
		}
		if attempt < attempts-1 && r.ingestRetryInterval > 0 {
			time.Sleep(r.ingestRetryInterval)
		}
	}
	if bootErr != nil {
		_, _ = r.pool.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
		return models.StreamSession{}, fmt.Errorf("boot ingest: %w", bootErr)
	}

	session := models.StreamSession{
		ID:             sessionID,
		ChannelID:      channelID,
		StartedAt:      now,
		Renditions:     append([]string{}, renditions...),
		PeakConcurrent: 0,
		OriginURL:      boot.OriginURL,
		PlaybackURL:    boot.PlaybackURL,
		IngestJobIDs:   append([]string{}, boot.JobIDs...),
	}
	ingestEndpoints := make([]string, 0, 2)
	if boot.PrimaryIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.PrimaryIngest)
	}
	if boot.BackupIngest != "" {
		ingestEndpoints = append(ingestEndpoints, boot.BackupIngest)
	}
	if len(ingestEndpoints) > 0 {
		session.IngestEndpoints = ingestEndpoints
	}
	if len(boot.Renditions) > 0 {
		manifests := make([]models.RenditionManifest, 0, len(boot.Renditions))
		for _, rendition := range boot.Renditions {
			manifests = append(manifests, models.RenditionManifest{
				Name:        rendition.Name,
				ManifestURL: rendition.ManifestURL,
				Bitrate:     rendition.Bitrate,
			})
		}
		session.RenditionManifests = manifests
	}

	tx, err = r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		_ = r.ingestController.ShutdownStream(context.Background(), channelID, sessionID, append([]string{}, session.IngestJobIDs...))
		_, _ = r.pool.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
		return models.StreamSession{}, fmt.Errorf("begin persist stream session: %w", err)
	}
	defer rollbackTx(ctx, tx)

	_, err = tx.Exec(ctx, "INSERT INTO stream_sessions (id, channel_id, started_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids) VALUES ($1, $2, $3, $4, 0, $5, $6, $7, $8)",
		session.ID,
		session.ChannelID,
		session.StartedAt,
		session.Renditions,
		session.OriginURL,
		session.PlaybackURL,
		session.IngestEndpoints,
		session.IngestJobIDs,
	)
	if err != nil {
		_ = r.ingestController.ShutdownStream(context.Background(), channelID, sessionID, append([]string{}, session.IngestJobIDs...))
		_, _ = r.pool.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
		return models.StreamSession{}, fmt.Errorf("insert stream session: %w", err)
	}
	for _, manifest := range session.RenditionManifests {
		if _, err := tx.Exec(ctx, "INSERT INTO stream_session_manifests (session_id, name, manifest_url, bitrate) VALUES ($1, $2, $3, $4)", session.ID, manifest.Name, manifest.ManifestURL, manifest.Bitrate); err != nil {
			_ = r.ingestController.ShutdownStream(context.Background(), channelID, sessionID, append([]string{}, session.IngestJobIDs...))
			_, _ = r.pool.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
			return models.StreamSession{}, fmt.Errorf("insert rendition manifest: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, "UPDATE channels SET current_session_id = $1, live_state = 'live', updated_at = $2 WHERE id = $3", session.ID, session.StartedAt, channelID); err != nil {
		_ = r.ingestController.ShutdownStream(context.Background(), channelID, sessionID, append([]string{}, session.IngestJobIDs...))
		_, _ = r.pool.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
		return models.StreamSession{}, fmt.Errorf("mark channel live: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		_ = r.ingestController.ShutdownStream(context.Background(), channelID, sessionID, append([]string{}, session.IngestJobIDs...))
		_, _ = r.pool.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
		return models.StreamSession{}, fmt.Errorf("commit start stream: %w", err)
	}

	return session, nil
}

func (r *postgresRepository) StopStream(channelID string, peakConcurrent int) (models.StreamSession, error) {
	if r == nil || r.pool == nil {
		return models.StreamSession{}, ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.StreamSession{}, fmt.Errorf("begin stop stream tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var (
		streamKey       string
		currentSession  pgtype.Text
		channelTitle    string
		channelCategory pgtype.Text
		channelTags     []string
	)
	row := tx.QueryRow(ctx, "SELECT stream_key, current_session_id, title, category, tags FROM channels WHERE id = $1 FOR UPDATE", channelID)
	if err := row.Scan(&streamKey, &currentSession, &channelTitle, &channelCategory, &channelTags); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.StreamSession{}, fmt.Errorf("channel %s not found", channelID)
		}
		return models.StreamSession{}, fmt.Errorf("load channel %s: %w", channelID, err)
	}
	if !currentSession.Valid {
		return models.StreamSession{}, errors.New("channel is not live")
	}
	sessionID := currentSession.String

	var session models.StreamSession
	var renditions []string
	var ingestEndpoints []string
	var ingestJobIDs []string
	var peak int
	var startedAt time.Time
	var endedAt pgtype.Timestamptz
	var originURL, playbackURL string
	sessRow := tx.QueryRow(ctx, "SELECT started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids FROM stream_sessions WHERE id = $1 FOR UPDATE", sessionID)
	if err := sessRow.Scan(&startedAt, &endedAt, &renditions, &peak, &originURL, &playbackURL, &ingestEndpoints, &ingestJobIDs); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.StreamSession{}, fmt.Errorf("session %s missing", sessionID)
		}
		return models.StreamSession{}, fmt.Errorf("load session %s: %w", sessionID, err)
	}
	manifestsRows, err := tx.Query(ctx, "SELECT name, manifest_url, bitrate FROM stream_session_manifests WHERE session_id = $1", sessionID)
	if err != nil {
		return models.StreamSession{}, fmt.Errorf("load session manifests: %w", err)
	}
	manifests := make([]models.RenditionManifest, 0)
	for manifestsRows.Next() {
		var name, url string
		var bitrate pgtype.Int4
		if err := manifestsRows.Scan(&name, &url, &bitrate); err != nil {
			manifestsRows.Close()
			return models.StreamSession{}, fmt.Errorf("scan session manifest: %w", err)
		}
		entry := models.RenditionManifest{Name: name, ManifestURL: url}
		if bitrate.Valid {
			entry.Bitrate = int(bitrate.Int32)
		}
		manifests = append(manifests, entry)
	}
	manifestsRows.Close()
	if err := manifestsRows.Err(); err != nil {
		return models.StreamSession{}, fmt.Errorf("read session manifests: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.StreamSession{}, fmt.Errorf("commit load session: %w", err)
	}

	session = models.StreamSession{
		ID:                 sessionID,
		ChannelID:          channelID,
		StartedAt:          startedAt.UTC(),
		Renditions:         append([]string{}, renditions...),
		PeakConcurrent:     peak,
		OriginURL:          originURL,
		PlaybackURL:        playbackURL,
		IngestEndpoints:    append([]string{}, ingestEndpoints...),
		IngestJobIDs:       append([]string{}, ingestJobIDs...),
		RenditionManifests: append([]models.RenditionManifest{}, manifests...),
	}
	if endedAt.Valid {
		ts := endedAt.Time.UTC()
		session.EndedAt = &ts
	}

	if err := r.ingestController.ShutdownStream(context.Background(), channelID, sessionID, append([]string{}, ingestJobIDs...)); err != nil {
		return models.StreamSession{}, fmt.Errorf("shutdown ingest: %w", err)
	}

	now := time.Now().UTC()
	session.EndedAt = &now
	if peakConcurrent > session.PeakConcurrent {
		session.PeakConcurrent = peakConcurrent
	}

	channel := models.Channel{ID: channelID, Title: channelTitle}
	if channelCategory.Valid {
		channel.Category = channelCategory.String
	}
	if len(channelTags) > 0 {
		channel.Tags = append([]string{}, channelTags...)
	}

	recording, recErr := r.createRecording(session, channel, now)
	if recErr != nil {
		return models.StreamSession{}, recErr
	}

	tx, err = r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.StreamSession{}, fmt.Errorf("begin finalize stop stream tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	_, err = tx.Exec(ctx, "UPDATE stream_sessions SET ended_at = $1, peak_concurrent = $2 WHERE id = $3", session.EndedAt, session.PeakConcurrent, session.ID)
	if err != nil {
		return models.StreamSession{}, fmt.Errorf("update stream session %s: %w", session.ID, err)
	}
	_, err = tx.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = $1 WHERE id = $2", now, channelID)
	if err != nil {
		return models.StreamSession{}, fmt.Errorf("update channel %s: %w", channelID, err)
	}
	if recording.ID != "" {
		if err := r.insertRecording(ctx, tx, recording); err != nil {
			return models.StreamSession{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return models.StreamSession{}, fmt.Errorf("commit stop stream: %w", err)
	}

	return session, nil
}

func (r *postgresRepository) CurrentStreamSession(channelID string) (models.StreamSession, bool) {
	if r == nil || r.pool == nil {
		return models.StreamSession{}, false
	}
	ctx := context.Background()
	var current pgtype.Text
	if err := r.pool.QueryRow(ctx, "SELECT current_session_id FROM channels WHERE id = $1", channelID).Scan(&current); err != nil {
		return models.StreamSession{}, false
	}
	if !current.Valid {
		return models.StreamSession{}, false
	}
	session, ok := r.loadStreamSession(ctx, current.String)
	if !ok {
		return models.StreamSession{}, false
	}
	return session, true
}

func (r *postgresRepository) ListStreamSessions(channelID string) ([]models.StreamSession, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ctx := context.Background()
	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check channel %s: %w", channelID, err)
	}
	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	rows, err := r.pool.Query(ctx, "SELECT id FROM stream_sessions WHERE channel_id = $1 ORDER BY started_at DESC", channelID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]models.StreamSession, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan session id: %w", err)
		}
		session, ok := r.loadStreamSession(ctx, id)
		if !ok {
			continue
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *postgresRepository) ListRecordings(channelID string, includeUnpublished bool) ([]models.Recording, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ctx := context.Background()
	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check channel %s: %w", channelID, err)
	}
	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	if err := r.purgeExpiredRecordings(ctx); err != nil {
		return nil, err
	}
	query := "SELECT id FROM recordings WHERE channel_id = $1"
	if !includeUnpublished {
		query += " AND published_at IS NOT NULL"
	}
	query += " ORDER BY created_at DESC"
	rows, err := r.pool.Query(ctx, query, channelID)
	if err != nil {
		return nil, fmt.Errorf("list recordings: %w", err)
	}
	defer rows.Close()

	recordings := make([]models.Recording, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan recording id: %w", err)
		}
		recording, ok, loadErr := r.loadRecording(ctx, id)
		if loadErr != nil {
			return nil, loadErr
		}
		if !ok {
			continue
		}
		recordings = append(recordings, recording)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return recordings, nil
}

func (r *postgresRepository) CreateUpload(params CreateUploadParams) (models.Upload, error) {
	if r == nil || r.pool == nil {
		return models.Upload{}, ErrPostgresUnavailable
	}
	channelID := strings.TrimSpace(params.ChannelID)
	if channelID == "" {
		return models.Upload{}, fmt.Errorf("channelId is required")
	}
	ctx := context.Background()
	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
		return models.Upload{}, fmt.Errorf("check channel %s: %w", channelID, err)
	}
	if !exists {
		return models.Upload{}, fmt.Errorf("channel %s not found", channelID)
	}

	id, err := r.generateID()
	if err != nil {
		return models.Upload{}, err
	}

	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "Uploaded video"
	}
	filename := strings.TrimSpace(params.Filename)
	if filename == "" {
		filename = "upload.mp4"
	}
	metadata := make(map[string]string, len(params.Metadata))
	for k, v := range params.Metadata {
		if strings.TrimSpace(k) == "" {
			continue
		}
		metadata[k] = v
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return models.Upload{}, fmt.Errorf("encode metadata: %w", err)
	}
	playbackURL := strings.TrimSpace(params.PlaybackURL)
	now := time.Now().UTC()
	_, err = r.pool.Exec(ctx, "INSERT INTO uploads (id, channel_id, title, filename, size_bytes, status, progress, playback_url, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6, $7, $8, $9)",
		id,
		channelID,
		title,
		filename,
		params.SizeBytes,
		playbackURL,
		metadataJSON,
		now,
		now,
	)
	if err != nil {
		return models.Upload{}, fmt.Errorf("insert upload: %w", err)
	}
	upload := models.Upload{
		ID:          id,
		ChannelID:   channelID,
		Title:       title,
		Filename:    filename,
		SizeBytes:   params.SizeBytes,
		Status:      "pending",
		Progress:    0,
		Metadata:    metadata,
		PlaybackURL: playbackURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return upload, nil
}

func (r *postgresRepository) ListUploads(channelID string) ([]models.Upload, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ctx := context.Background()
	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check channel %s: %w", channelID, err)
	}
	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}
	rows, err := r.pool.Query(ctx, "SELECT id FROM uploads WHERE channel_id = $1 ORDER BY created_at DESC", channelID)
	if err != nil {
		return nil, fmt.Errorf("list uploads: %w", err)
	}
	defer rows.Close()

	uploads := make([]models.Upload, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan upload id: %w", err)
		}
		upload, ok, loadErr := r.loadUpload(ctx, id)
		if loadErr != nil {
			return nil, loadErr
		}
		if !ok {
			continue
		}
		uploads = append(uploads, upload)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return uploads, nil
}

func (r *postgresRepository) GetUpload(id string) (models.Upload, bool) {
	if r == nil || r.pool == nil {
		return models.Upload{}, false
	}
	ctx := context.Background()
	upload, ok, err := r.loadUpload(ctx, id)
	if err != nil || !ok {
		return models.Upload{}, false
	}
	return upload, true
}

func (r *postgresRepository) UpdateUpload(id string, update UploadUpdate) (models.Upload, error) {
	if r == nil || r.pool == nil {
		return models.Upload{}, ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.Upload{}, fmt.Errorf("begin update upload tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	upload, ok, err := r.loadUpload(ctx, id)
	if err != nil {
		return models.Upload{}, fmt.Errorf("load upload %s: %w", id, err)
	}
	if !ok {
		return models.Upload{}, fmt.Errorf("upload %s not found", id)
	}

	if update.Title != nil {
		if trimmed := strings.TrimSpace(*update.Title); trimmed != "" {
			upload.Title = trimmed
		}
	}
	if update.Status != nil {
		upload.Status = strings.TrimSpace(*update.Status)
	}
	if update.Progress != nil {
		progress := *update.Progress
		if progress < 0 {
			progress = 0
		}
		if progress > 100 {
			progress = 100
		}
		upload.Progress = progress
	}
	if update.RecordingID != nil {
		trimmed := strings.TrimSpace(*update.RecordingID)
		if trimmed == "" {
			upload.RecordingID = nil
		} else {
			upload.RecordingID = &trimmed
		}
	}
	if update.PlaybackURL != nil {
		upload.PlaybackURL = strings.TrimSpace(*update.PlaybackURL)
	}
	if update.Metadata != nil {
		if upload.Metadata == nil {
			upload.Metadata = make(map[string]string, len(update.Metadata))
		}
		for k, v := range update.Metadata {
			if strings.TrimSpace(k) == "" {
				continue
			}
			if v == "" {
				delete(upload.Metadata, k)
				continue
			}
			upload.Metadata[k] = v
		}
	}
	if update.Error != nil {
		upload.Error = strings.TrimSpace(*update.Error)
	}
	if update.CompletedAt != nil {
		if update.CompletedAt.IsZero() {
			upload.CompletedAt = nil
		} else {
			ts := update.CompletedAt.UTC()
			upload.CompletedAt = &ts
		}
	}

	upload.UpdatedAt = time.Now().UTC()

	metadataJSON, err := json.Marshal(upload.Metadata)
	if err != nil {
		return models.Upload{}, fmt.Errorf("encode metadata: %w", err)
	}
	var recordingID interface{}
	if upload.RecordingID != nil {
		recordingID = *upload.RecordingID
	}
	var completedAt interface{}
	if upload.CompletedAt != nil {
		completedAt = *upload.CompletedAt
	}
	_, err = tx.Exec(ctx, "UPDATE uploads SET title = $1, status = $2, progress = $3, recording_id = $4, playback_url = $5, metadata = $6, error = $7, completed_at = $8, updated_at = $9 WHERE id = $10",
		upload.Title,
		upload.Status,
		upload.Progress,
		recordingID,
		upload.PlaybackURL,
		metadataJSON,
		upload.Error,
		completedAt,
		upload.UpdatedAt,
		id,
	)
	if err != nil {
		return models.Upload{}, fmt.Errorf("update upload %s: %w", id, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Upload{}, fmt.Errorf("commit update upload: %w", err)
	}
	return upload, nil
}

func (r *postgresRepository) DeleteUpload(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx := context.Background()
	command, err := r.pool.Exec(ctx, "DELETE FROM uploads WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete upload %s: %w", id, err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("upload %s not found", id)
	}
	return nil
}

func (r *postgresRepository) GetRecording(id string) (models.Recording, bool) {
	if r == nil || r.pool == nil {
		return models.Recording{}, false
	}
	ctx := context.Background()
	if err := r.purgeExpiredRecordings(ctx); err != nil {
		return models.Recording{}, false
	}
	recording, ok, err := r.loadRecording(ctx, id)
	if err != nil || !ok {
		return models.Recording{}, false
	}
	return recording, true
}

func (r *postgresRepository) PublishRecording(id string) (models.Recording, error) {
	if r == nil || r.pool == nil {
		return models.Recording{}, ErrPostgresUnavailable
	}
	ctx := context.Background()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return models.Recording{}, fmt.Errorf("begin publish recording tx: %w", err)
	}
	defer rollbackTx(ctx, tx)

	var (
		channelID       string
		sessionID       string
		title           string
		duration        int
		playbackBaseURL string
		metadataBytes   []byte
		createdAt       time.Time
		retainUntil     pgtype.Timestamptz
		publishedAt     pgtype.Timestamptz
	)
	err = tx.QueryRow(ctx, "SELECT channel_id, session_id, title, duration_seconds, playback_base_url, metadata, created_at, retain_until, published_at FROM recordings WHERE id = $1 FOR UPDATE", id).
		Scan(&channelID, &sessionID, &title, &duration, &playbackBaseURL, &metadataBytes, &createdAt, &retainUntil, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Recording{}, fmt.Errorf("recording %s not found", id)
	}
	if err != nil {
		return models.Recording{}, fmt.Errorf("load recording %s: %w", id, err)
	}
	if publishedAt.Valid {
		recording, _, loadErr := r.loadRecording(ctx, id)
		if loadErr != nil {
			return models.Recording{}, loadErr
		}
		return recording, nil
	}
	now := time.Now().UTC()
	_, err = tx.Exec(ctx, "UPDATE recordings SET published_at = $1 WHERE id = $2", now, id)
	if err != nil {
		return models.Recording{}, fmt.Errorf("publish recording %s: %w", id, err)
	}
	if deadline := r.recordingDeadline(now, true); deadline != nil {
		if _, err := tx.Exec(ctx, "UPDATE recordings SET retain_until = $1 WHERE id = $2", deadline, id); err != nil {
			return models.Recording{}, fmt.Errorf("update recording retention: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Recording{}, fmt.Errorf("commit publish recording: %w", err)
	}
	recording, _, err := r.loadRecording(ctx, id)
	if err != nil {
		return models.Recording{}, err
	}
	if recording.ID == "" {
		return models.Recording{}, fmt.Errorf("recording %s not found", id)
	}
	return recording, nil
}

func (r *postgresRepository) DeleteRecording(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx := context.Background()
	recording, ok, err := r.loadRecording(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("recording %s not found", id)
	}
	if err := r.deleteRecordingArtifacts(recording); err != nil {
		return err
	}
	clipRows, err := r.pool.Query(ctx, "SELECT id, storage_object FROM clip_exports WHERE recording_id = $1", id)
	if err != nil {
		return fmt.Errorf("load clip exports: %w", err)
	}
	clips := make([]models.ClipExport, 0)
	for clipRows.Next() {
		var clip models.ClipExport
		if err := clipRows.Scan(&clip.ID, &clip.StorageObject); err != nil {
			clipRows.Close()
			return fmt.Errorf("scan clip export: %w", err)
		}
		clips = append(clips, clip)
	}
	clipRows.Close()
	for _, clip := range clips {
		if err := r.deleteClipArtifacts(clip); err != nil {
			return err
		}
	}
	if _, err := r.pool.Exec(ctx, "DELETE FROM recordings WHERE id = $1", id); err != nil {
		return fmt.Errorf("delete recording %s: %w", id, err)
	}
	return nil
}

func (r *postgresRepository) CreateClipExport(recordingID string, params ClipExportParams) (models.ClipExport, error) {
	if r == nil || r.pool == nil {
		return models.ClipExport{}, ErrPostgresUnavailable
	}
	if strings.TrimSpace(recordingID) == "" {
		return models.ClipExport{}, fmt.Errorf("recording id is required")
	}
	ctx := context.Background()
	var (
		channelID string
		sessionID string
		duration  int
	)
	err := r.pool.QueryRow(ctx, "SELECT channel_id, session_id, duration_seconds FROM recordings WHERE id = $1", recordingID).
		Scan(&channelID, &sessionID, &duration)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.ClipExport{}, fmt.Errorf("recording %s not found", recordingID)
	}
	if err != nil {
		return models.ClipExport{}, fmt.Errorf("load recording %s: %w", recordingID, err)
	}
	if params.EndSeconds <= params.StartSeconds {
		return models.ClipExport{}, fmt.Errorf("endSeconds must be greater than startSeconds")
	}
	if params.StartSeconds < 0 {
		return models.ClipExport{}, fmt.Errorf("startSeconds must be non-negative")
	}
	if duration > 0 && params.EndSeconds > duration {
		return models.ClipExport{}, fmt.Errorf("clip exceeds recording duration")
	}
	id, err := r.generateID()
	if err != nil {
		return models.ClipExport{}, err
	}
	now := time.Now().UTC()
	clip := models.ClipExport{
		ID:           id,
		RecordingID:  recordingID,
		ChannelID:    channelID,
		SessionID:    sessionID,
		Title:        strings.TrimSpace(params.Title),
		StartSeconds: params.StartSeconds,
		EndSeconds:   params.EndSeconds,
		Status:       "pending",
		CreatedAt:    now,
	}
	_, err = r.pool.Exec(ctx, "INSERT INTO clip_exports (id, recording_id, channel_id, session_id, title, start_seconds, end_seconds, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		clip.ID,
		clip.RecordingID,
		clip.ChannelID,
		clip.SessionID,
		clip.Title,
		clip.StartSeconds,
		clip.EndSeconds,
		clip.Status,
		clip.CreatedAt,
	)
	if err != nil {
		return models.ClipExport{}, fmt.Errorf("insert clip export: %w", err)
	}
	return clip, nil
}

func (r *postgresRepository) ListClipExports(recordingID string) ([]models.ClipExport, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	if strings.TrimSpace(recordingID) == "" {
		return nil, fmt.Errorf("recording id is required")
	}
	ctx := context.Background()
	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM recordings WHERE id = $1)", recordingID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check recording %s: %w", recordingID, err)
	}
	if !exists {
		return nil, fmt.Errorf("recording %s not found", recordingID)
	}
	rows, err := r.pool.Query(ctx, "SELECT id, recording_id, channel_id, session_id, title, start_seconds, end_seconds, status, playback_url, created_at, completed_at, storage_object FROM clip_exports WHERE recording_id = $1 ORDER BY created_at DESC", recordingID)
	if err != nil {
		return nil, fmt.Errorf("list clip exports: %w", err)
	}
	defer rows.Close()
	clips := make([]models.ClipExport, 0)
	for rows.Next() {
		var clip models.ClipExport
		var completedAt pgtype.Timestamptz
		if err := rows.Scan(&clip.ID, &clip.RecordingID, &clip.ChannelID, &clip.SessionID, &clip.Title, &clip.StartSeconds, &clip.EndSeconds, &clip.Status, &clip.PlaybackURL, &clip.CreatedAt, &completedAt, &clip.StorageObject); err != nil {
			return nil, fmt.Errorf("scan clip export: %w", err)
		}
		if completedAt.Valid {
			ts := completedAt.Time.UTC()
			clip.CompletedAt = &ts
		}
		clips = append(clips, clip)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return clips, nil
}

func (r *postgresRepository) CreateChatMessage(channelID, userID, content string) (models.ChatMessage, error) {
	if r == nil || r.pool == nil {
		return models.ChatMessage{}, ErrPostgresUnavailable
	}

	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return models.ChatMessage{}, errors.New("message content cannot be empty")
	}
	if len([]rune(trimmed)) > 500 {
		return models.ChatMessage{}, errors.New("message content exceeds 500 characters")
	}

	id, err := r.generateID()
	if err != nil {
		return models.ChatMessage{}, err
	}

	createdAt := time.Now().UTC()
	message := models.ChatMessage{}
	saveErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create chat message tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}
		if err := ensureUserExists(ctx, tx, userID); err != nil {
			return err
		}

		var banned bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM chat_bans WHERE channel_id = $1 AND user_id = $2)", channelID, userID).Scan(&banned); err != nil {
			return fmt.Errorf("check chat ban: %w", err)
		}
		if banned {
			return fmt.Errorf("user is banned")
		}

		var timeoutExpiry pgtype.Timestamptz
		err = tx.QueryRow(ctx, "SELECT expires_at FROM chat_timeouts WHERE channel_id = $1 AND user_id = $2", channelID, userID).Scan(&timeoutExpiry)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("lookup chat timeout: %w", err)
		}
		if err == nil {
			expiry := timeoutExpiry.Time.UTC()
			if time.Now().UTC().Before(expiry) {
				return fmt.Errorf("user is timed out")
			}
			if _, err := tx.Exec(ctx, "DELETE FROM chat_timeouts WHERE channel_id = $1 AND user_id = $2", channelID, userID); err != nil {
				return fmt.Errorf("clear expired timeout: %w", err)
			}
		}

		if _, err := tx.Exec(ctx, "INSERT INTO chat_messages (id, channel_id, user_id, content, created_at) VALUES ($1, $2, $3, $4, $5)", id, channelID, userID, trimmed, createdAt); err != nil {
			return fmt.Errorf("insert chat message: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit chat message: %w", err)
		}

		message = models.ChatMessage{
			ID:        id,
			ChannelID: channelID,
			UserID:    userID,
			Content:   trimmed,
			CreatedAt: createdAt,
		}

		return nil
	})
	if saveErr != nil {
		return models.ChatMessage{}, saveErr
	}

	return message, nil
}

func (r *postgresRepository) DeleteChatMessage(channelID, messageID string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}

	deleteErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin delete chat message tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}

		var existingChannel string
		if err := tx.QueryRow(ctx, "SELECT channel_id FROM chat_messages WHERE id = $1", messageID).Scan(&existingChannel); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("message %s not found for channel %s", messageID, channelID)
			}
			return fmt.Errorf("lookup chat message %s: %w", messageID, err)
		}
		if existingChannel != channelID {
			return fmt.Errorf("message %s not found for channel %s", messageID, channelID)
		}

		if _, err := tx.Exec(ctx, "DELETE FROM chat_messages WHERE id = $1", messageID); err != nil {
			return fmt.Errorf("delete chat message %s: %w", messageID, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit delete chat message: %w", err)
		}
		return nil
	})

	return deleteErr
}

func (r *postgresRepository) ListChatMessages(channelID string, limit int) ([]models.ChatMessage, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ctx, cancel := r.acquireContext()
	defer cancel()

	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check channel %s: %w", channelID, err)
	}
	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	query := "SELECT id, channel_id, user_id, content, created_at FROM chat_messages WHERE channel_id = $1 ORDER BY created_at DESC, id ASC"
	args := []any{channelID}
	if limit > 0 {
		query += " LIMIT $2"
		args = append(args, limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()

	messages := make([]models.ChatMessage, 0)
	for rows.Next() {
		var msg models.ChatMessage
		var createdAt time.Time
		if err := rows.Scan(&msg.ID, &msg.ChannelID, &msg.UserID, &msg.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		msg.CreatedAt = createdAt.UTC()
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat messages: %w", err)
	}

	return messages, nil
}

func (r *postgresRepository) ChatRestrictions() chat.RestrictionsSnapshot {
	snapshot := chat.RestrictionsSnapshot{
		Bans:            map[string]map[string]struct{}{},
		Timeouts:        map[string]map[string]time.Time{},
		BanActors:       map[string]map[string]string{},
		BanReasons:      map[string]map[string]string{},
		TimeoutActors:   map[string]map[string]string{},
		TimeoutReasons:  map[string]map[string]string{},
		TimeoutIssuedAt: map[string]map[string]time.Time{},
	}
	if r == nil || r.pool == nil {
		return snapshot
	}

	ctx, cancel := r.acquireContext()
	defer cancel()

	banRows, err := r.pool.Query(ctx, "SELECT channel_id, user_id, actor_id, reason, issued_at FROM chat_bans")
	if err == nil {
		defer banRows.Close()
		for banRows.Next() {
			var channelID, userID string
			var actor pgtype.Text
			var reason string
			var issued time.Time
			if err := banRows.Scan(&channelID, &userID, &actor, &reason, &issued); err != nil {
				return snapshot
			}
			if snapshot.Bans[channelID] == nil {
				snapshot.Bans[channelID] = make(map[string]struct{})
			}
			snapshot.Bans[channelID][userID] = struct{}{}
			if snapshot.BanActors[channelID] == nil {
				snapshot.BanActors[channelID] = make(map[string]string)
			}
			if actor.Valid {
				snapshot.BanActors[channelID][userID] = actor.String
			} else {
				snapshot.BanActors[channelID][userID] = ""
			}
			if snapshot.BanReasons[channelID] == nil {
				snapshot.BanReasons[channelID] = make(map[string]string)
			}
			snapshot.BanReasons[channelID][userID] = reason
		}
		if err := banRows.Err(); err != nil {
			return snapshot
		}
	}

	now := time.Now().UTC()
	timeoutRows, err := r.pool.Query(ctx, "SELECT channel_id, user_id, actor_id, reason, issued_at, expires_at FROM chat_timeouts WHERE expires_at > $1", now)
	if err != nil {
		return snapshot
	}
	defer timeoutRows.Close()
	for timeoutRows.Next() {
		var channelID, userID string
		var actor pgtype.Text
		var reason string
		var issued, expires time.Time
		if err := timeoutRows.Scan(&channelID, &userID, &actor, &reason, &issued, &expires); err != nil {
			return snapshot
		}
		if snapshot.Timeouts[channelID] == nil {
			snapshot.Timeouts[channelID] = make(map[string]time.Time)
		}
		snapshot.Timeouts[channelID][userID] = expires.UTC()
		if snapshot.TimeoutActors[channelID] == nil {
			snapshot.TimeoutActors[channelID] = make(map[string]string)
		}
		if actor.Valid {
			snapshot.TimeoutActors[channelID][userID] = actor.String
		} else {
			snapshot.TimeoutActors[channelID][userID] = ""
		}
		if snapshot.TimeoutReasons[channelID] == nil {
			snapshot.TimeoutReasons[channelID] = make(map[string]string)
		}
		snapshot.TimeoutReasons[channelID][userID] = reason
		if snapshot.TimeoutIssuedAt[channelID] == nil {
			snapshot.TimeoutIssuedAt[channelID] = make(map[string]time.Time)
		}
		snapshot.TimeoutIssuedAt[channelID][userID] = issued.UTC()
	}
	if err := timeoutRows.Err(); err != nil {
		return snapshot
	}
	return snapshot
}

func (r *postgresRepository) IsChatBanned(channelID, userID string) bool {
	if r == nil || r.pool == nil {
		return false
	}
	ctx, cancel := r.acquireContext()
	defer cancel()
	var banned bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM chat_bans WHERE channel_id = $1 AND user_id = $2)", channelID, userID).Scan(&banned); err != nil {
		return false
	}
	return banned
}

func (r *postgresRepository) ChatTimeout(channelID, userID string) (time.Time, bool) {
	if r == nil || r.pool == nil {
		return time.Time{}, false
	}
	ctx, cancel := r.acquireContext()
	defer cancel()
	var expires time.Time
	if err := r.pool.QueryRow(ctx, "SELECT expires_at FROM chat_timeouts WHERE channel_id = $1 AND user_id = $2", channelID, userID).Scan(&expires); err != nil {
		return time.Time{}, false
	}
	return expires.UTC(), true
}

func (r *postgresRepository) ApplyChatEvent(evt chat.Event) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}

	ctx := context.Background()
	switch evt.Type {
	case chat.EventTypeMessage:
		if evt.Message == nil {
			return fmt.Errorf("message payload missing")
		}
		msg := evt.Message
		if msg.ID == "" || msg.ChannelID == "" || msg.UserID == "" {
			return fmt.Errorf("invalid message event")
		}
		_, err := r.pool.Exec(ctx, "INSERT INTO chat_messages (id, channel_id, user_id, content, created_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO UPDATE SET channel_id = EXCLUDED.channel_id, user_id = EXCLUDED.user_id, content = EXCLUDED.content, created_at = EXCLUDED.created_at", msg.ID, msg.ChannelID, msg.UserID, msg.Content, msg.CreatedAt.UTC())
		if err != nil {
			return fmt.Errorf("persist chat message event: %w", err)
		}
		return nil
	case chat.EventTypeModeration:
		if evt.Moderation == nil {
			return fmt.Errorf("moderation payload missing")
		}
		mod := evt.Moderation
		issued := evt.OccurredAt.UTC()
		if issued.IsZero() {
			issued = time.Now().UTC()
		}
		actor := strings.TrimSpace(mod.ActorID)
		var actorParam any
		if actor != "" {
			actorParam = actor
		}
		reason := strings.TrimSpace(mod.Reason)
		switch mod.Action {
		case chat.ModerationActionBan:
			_, err := r.pool.Exec(ctx, "INSERT INTO chat_bans (channel_id, user_id, actor_id, reason, issued_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (channel_id, user_id) DO UPDATE SET actor_id = EXCLUDED.actor_id, reason = EXCLUDED.reason, issued_at = EXCLUDED.issued_at", mod.ChannelID, mod.TargetID, actorParam, reason, issued)
			if err != nil {
				return fmt.Errorf("apply ban event: %w", err)
			}
			return nil
		case chat.ModerationActionUnban:
			_, err := r.pool.Exec(ctx, "DELETE FROM chat_bans WHERE channel_id = $1 AND user_id = $2", mod.ChannelID, mod.TargetID)
			if err != nil {
				return fmt.Errorf("apply unban event: %w", err)
			}
			return nil
		case chat.ModerationActionTimeout:
			if mod.ExpiresAt == nil {
				return nil
			}
			expires := mod.ExpiresAt.UTC()
			_, err := r.pool.Exec(ctx, "INSERT INTO chat_timeouts (channel_id, user_id, actor_id, reason, issued_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (channel_id, user_id) DO UPDATE SET actor_id = EXCLUDED.actor_id, reason = EXCLUDED.reason, issued_at = EXCLUDED.issued_at, expires_at = EXCLUDED.expires_at", mod.ChannelID, mod.TargetID, actorParam, reason, issued, expires)
			if err != nil {
				return fmt.Errorf("apply timeout event: %w", err)
			}
			return nil
		case chat.ModerationActionRemoveTimeout:
			_, err := r.pool.Exec(ctx, "DELETE FROM chat_timeouts WHERE channel_id = $1 AND user_id = $2", mod.ChannelID, mod.TargetID)
			if err != nil {
				return fmt.Errorf("apply remove timeout event: %w", err)
			}
			return nil
		default:
			return fmt.Errorf("unsupported moderation action %q", mod.Action)
		}
	case chat.EventTypeReport:
		if evt.Report == nil {
			return fmt.Errorf("report payload missing")
		}
		rep := evt.Report
		if strings.TrimSpace(rep.ID) == "" {
			return fmt.Errorf("report id missing")
		}
		status := strings.ToLower(strings.TrimSpace(rep.Status))
		if status == "" {
			status = "open"
		}
		var messageParam any
		if strings.TrimSpace(rep.MessageID) != "" {
			messageParam = strings.TrimSpace(rep.MessageID)
		}
		var evidenceParam any
		if strings.TrimSpace(rep.EvidenceURL) != "" {
			evidenceParam = strings.TrimSpace(rep.EvidenceURL)
		}
		_, err := r.pool.Exec(ctx, "INSERT INTO chat_reports (id, channel_id, reporter_id, target_id, reason, message_id, evidence_url, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO UPDATE SET channel_id = EXCLUDED.channel_id, reporter_id = EXCLUDED.reporter_id, target_id = EXCLUDED.target_id, reason = EXCLUDED.reason, message_id = EXCLUDED.message_id, evidence_url = EXCLUDED.evidence_url, status = EXCLUDED.status, created_at = EXCLUDED.created_at", rep.ID, rep.ChannelID, rep.ReporterID, rep.TargetID, rep.Reason, messageParam, evidenceParam, status, rep.CreatedAt.UTC())
		if err != nil {
			return fmt.Errorf("apply report event: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported chat event %q", evt.Type)
	}
}

func (r *postgresRepository) ListChatRestrictions(channelID string) []models.ChatRestriction {
	if r == nil || r.pool == nil {
		return nil
	}
	ctx := context.Background()

	restrictions := make([]models.ChatRestriction, 0)

	banRows, err := r.pool.Query(ctx, "SELECT user_id, actor_id, reason, issued_at FROM chat_bans WHERE channel_id = $1", channelID)
	if err == nil {
		defer banRows.Close()
		for banRows.Next() {
			var userID string
			var actor pgtype.Text
			var reason string
			var issued time.Time
			if err := banRows.Scan(&userID, &actor, &reason, &issued); err != nil {
				return restrictions
			}
			restriction := models.ChatRestriction{
				ID:        fmt.Sprintf("ban:%s:%s", channelID, userID),
				Type:      "ban",
				ChannelID: channelID,
				TargetID:  userID,
				Reason:    reason,
				IssuedAt:  issued.UTC(),
			}
			if actor.Valid {
				restriction.ActorID = actor.String
			}
			restrictions = append(restrictions, restriction)
		}
		if err := banRows.Err(); err != nil {
			return restrictions
		}
	}

	timeoutRows, err := r.pool.Query(ctx, "SELECT user_id, actor_id, reason, issued_at, expires_at FROM chat_timeouts WHERE channel_id = $1", channelID)
	if err != nil {
		return restrictions
	}
	defer timeoutRows.Close()
	for timeoutRows.Next() {
		var userID string
		var actor pgtype.Text
		var reason string
		var issued, expires time.Time
		if err := timeoutRows.Scan(&userID, &actor, &reason, &issued, &expires); err != nil {
			return restrictions
		}
		expiry := expires.UTC()
		restriction := models.ChatRestriction{
			ID:        fmt.Sprintf("timeout:%s:%s", channelID, userID),
			Type:      "timeout",
			ChannelID: channelID,
			TargetID:  userID,
			Reason:    reason,
			IssuedAt:  issued.UTC(),
			ExpiresAt: &expiry,
		}
		if actor.Valid {
			restriction.ActorID = actor.String
		}
		restrictions = append(restrictions, restriction)
	}
	sort.Slice(restrictions, func(i, j int) bool {
		if restrictions[i].IssuedAt.Equal(restrictions[j].IssuedAt) {
			return restrictions[i].ID < restrictions[j].ID
		}
		return restrictions[i].IssuedAt.After(restrictions[j].IssuedAt)
	})
	return restrictions
}

func (r *postgresRepository) CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (models.ChatReport, error) {
	if r == nil || r.pool == nil {
		return models.ChatReport{}, ErrPostgresUnavailable
	}

	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return models.ChatReport{}, fmt.Errorf("reason is required")
	}

	id, err := r.generateID()
	if err != nil {
		return models.ChatReport{}, err
	}

	trimmedMessageID := strings.TrimSpace(messageID)
	trimmedEvidence := strings.TrimSpace(evidenceURL)
	now := time.Now().UTC()
	report := models.ChatReport{}

	createErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create chat report tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}
		if err := ensureUserExists(ctx, tx, reporterID); err != nil {
			return err
		}
		if err := ensureUserExists(ctx, tx, targetID); err != nil {
			return err
		}

		var messageParam any
		if trimmedMessageID != "" {
			messageParam = trimmedMessageID
		}
		var evidenceParam any
		if trimmedEvidence != "" {
			evidenceParam = trimmedEvidence
		}

		status := "open"
		if _, err := tx.Exec(ctx, "INSERT INTO chat_reports (id, channel_id, reporter_id, target_id, reason, message_id, evidence_url, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)", id, channelID, reporterID, targetID, trimmedReason, messageParam, evidenceParam, status, now); err != nil {
			return fmt.Errorf("insert chat report: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit chat report: %w", err)
		}

		report = models.ChatReport{
			ID:          id,
			ChannelID:   channelID,
			ReporterID:  reporterID,
			TargetID:    targetID,
			Reason:      trimmedReason,
			MessageID:   trimmedMessageID,
			EvidenceURL: trimmedEvidence,
			Status:      status,
			CreatedAt:   now,
		}
		return nil
	})
	if createErr != nil {
		return models.ChatReport{}, createErr
	}
	return report, nil
}

func (r *postgresRepository) ListChatReports(channelID string, includeResolved bool) ([]models.ChatReport, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ctx := context.Background()

	var exists bool
	if err := r.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("check channel %s: %w", channelID, err)
	}
	if !exists {
		return nil, fmt.Errorf("channel %s not found", channelID)
	}

	query := "SELECT id, channel_id, reporter_id, target_id, reason, message_id, evidence_url, status, resolution, resolver_id, created_at, resolved_at FROM chat_reports WHERE channel_id = $1"
	args := []any{channelID}
	if !includeResolved {
		query += " AND LOWER(status) <> 'resolved'"
	}
	query += " ORDER BY created_at DESC, id ASC"

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list chat reports: %w", err)
	}
	defer rows.Close()

	reports := make([]models.ChatReport, 0)
	for rows.Next() {
		var (
			report      models.ChatReport
			messageID   pgtype.Text
			evidenceURL pgtype.Text
			status      string
			resolution  pgtype.Text
			resolverID  pgtype.Text
			createdAt   time.Time
			resolvedAt  pgtype.Timestamptz
		)
		if err := rows.Scan(&report.ID, &report.ChannelID, &report.ReporterID, &report.TargetID, &report.Reason, &messageID, &evidenceURL, &status, &resolution, &resolverID, &createdAt, &resolvedAt); err != nil {
			return nil, fmt.Errorf("scan chat report: %w", err)
		}
		if messageID.Valid {
			report.MessageID = messageID.String
		}
		if evidenceURL.Valid {
			report.EvidenceURL = evidenceURL.String
		}
		report.Status = strings.ToLower(status)
		if resolution.Valid {
			report.Resolution = resolution.String
		}
		if resolverID.Valid {
			report.ResolverID = resolverID.String
		}
		report.CreatedAt = createdAt.UTC()
		if resolvedAt.Valid {
			ts := resolvedAt.Time.UTC()
			report.ResolvedAt = &ts
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat reports: %w", err)
	}
	return reports, nil
}

func (r *postgresRepository) ResolveChatReport(reportID, resolverID, resolution string) (models.ChatReport, error) {
	if r == nil || r.pool == nil {
		return models.ChatReport{}, ErrPostgresUnavailable
	}

	resolved := models.ChatReport{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin resolve chat report tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var (
			messageID      pgtype.Text
			evidenceURL    pgtype.Text
			status         string
			resolutionText pgtype.Text
			resolver       pgtype.Text
			createdAt      time.Time
			resolvedAt     pgtype.Timestamptz
		)
		row := tx.QueryRow(ctx, "SELECT id, channel_id, reporter_id, target_id, reason, message_id, evidence_url, status, resolution, resolver_id, created_at, resolved_at FROM chat_reports WHERE id = $1", reportID)
		if err := row.Scan(&resolved.ID, &resolved.ChannelID, &resolved.ReporterID, &resolved.TargetID, &resolved.Reason, &messageID, &evidenceURL, &status, &resolutionText, &resolver, &createdAt, &resolvedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("report %s not found", reportID)
			}
			return fmt.Errorf("load chat report %s: %w", reportID, err)
		}
		if messageID.Valid {
			resolved.MessageID = messageID.String
		}
		if evidenceURL.Valid {
			resolved.EvidenceURL = evidenceURL.String
		}
		if resolutionText.Valid {
			resolved.Resolution = resolutionText.String
		}
		if resolver.Valid {
			resolved.ResolverID = resolver.String
		}
		resolved.Status = strings.ToLower(status)
		resolved.CreatedAt = createdAt.UTC()
		if resolvedAt.Valid {
			ts := resolvedAt.Time.UTC()
			resolved.ResolvedAt = &ts
		}

		if strings.EqualFold(resolved.Status, "resolved") {
			return nil
		}

		if err := ensureUserExists(ctx, tx, resolverID); err != nil {
			return err
		}

		trimmed := strings.TrimSpace(resolution)
		if trimmed == "" {
			trimmed = "resolved"
		}
		now := time.Now().UTC()

		updateRow := tx.QueryRow(ctx, "UPDATE chat_reports SET status = 'resolved', resolution = $1, resolver_id = $2, resolved_at = $3 WHERE id = $4 RETURNING id, channel_id, reporter_id, target_id, reason, message_id, evidence_url, status, resolution, resolver_id, created_at, resolved_at", trimmed, resolverID, now, reportID)
		if err := updateRow.Scan(&resolved.ID, &resolved.ChannelID, &resolved.ReporterID, &resolved.TargetID, &resolved.Reason, &messageID, &evidenceURL, &status, &resolutionText, &resolver, &createdAt, &resolvedAt); err != nil {
			return fmt.Errorf("update chat report %s: %w", reportID, err)
		}
		if messageID.Valid {
			resolved.MessageID = messageID.String
		} else {
			resolved.MessageID = ""
		}
		if evidenceURL.Valid {
			resolved.EvidenceURL = evidenceURL.String
		} else {
			resolved.EvidenceURL = ""
		}
		resolved.Status = strings.ToLower(status)
		if resolutionText.Valid {
			resolved.Resolution = resolutionText.String
		} else {
			resolved.Resolution = ""
		}
		if resolver.Valid {
			resolved.ResolverID = resolver.String
		} else {
			resolved.ResolverID = ""
		}
		resolved.CreatedAt = createdAt.UTC()
		if resolvedAt.Valid {
			ts := resolvedAt.Time.UTC()
			resolved.ResolvedAt = &ts
		} else {
			resolved.ResolvedAt = nil
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit resolve chat report: %w", err)
		}
		return nil
	})
	if err != nil {
		return models.ChatReport{}, err
	}
	return resolved, nil
}

func (r *postgresRepository) CreateTip(params CreateTipParams) (models.Tip, error) {
	if r == nil || r.pool == nil {
		return models.Tip{}, ErrPostgresUnavailable
	}

	amount := params.Amount
	if amount <= 0 {
		return models.Tip{}, fmt.Errorf("amount must be positive")
	}

	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return models.Tip{}, fmt.Errorf("currency is required")
	}

	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return models.Tip{}, fmt.Errorf("provider is required")
	}

	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("tip-%d", time.Now().UnixNano())
	}
	if utf8.RuneCountInString(reference) > MaxTipReferenceLength {
		return models.Tip{}, fmt.Errorf("reference exceeds %d characters", MaxTipReferenceLength)
	}

	wallet := strings.TrimSpace(params.WalletAddress)
	if utf8.RuneCountInString(wallet) > MaxTipWalletAddressLength {
		return models.Tip{}, fmt.Errorf("wallet address exceeds %d characters", MaxTipWalletAddressLength)
	}

	message := strings.TrimSpace(params.Message)
	if utf8.RuneCountInString(message) > MaxTipMessageLength {
		return models.Tip{}, fmt.Errorf("message exceeds %d characters", MaxTipMessageLength)
	}

	id, err := r.generateID()
	if err != nil {
		return models.Tip{}, err
	}

	now := time.Now().UTC()
	var tip models.Tip
	saveErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create tip tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, params.ChannelID); err != nil {
			return err
		}
		if err := ensureUserExists(ctx, tx, params.FromUserID); err != nil {
			return err
		}

		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM tips WHERE provider = $1 AND reference = $2)", provider, reference).Scan(&exists); err != nil {
			return fmt.Errorf("check tip reference: %w", err)
		}
		if exists {
			return fmt.Errorf("tip reference %s/%s already exists", provider, reference)
		}

		var createdAt time.Time
		if err := tx.QueryRow(ctx, "INSERT INTO tips (id, channel_id, from_user_id, amount, currency, provider, reference, wallet_address, message, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING created_at", id, params.ChannelID, params.FromUserID, amount, currency, provider, reference, wallet, message, now).Scan(&createdAt); err != nil {
			return fmt.Errorf("insert tip: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create tip: %w", err)
		}

		tip = models.Tip{
			ID:            id,
			ChannelID:     params.ChannelID,
			FromUserID:    params.FromUserID,
			Amount:        amount,
			Currency:      currency,
			Provider:      provider,
			Reference:     reference,
			WalletAddress: wallet,
			Message:       message,
			CreatedAt:     createdAt.UTC(),
		}

		return nil
	})
	if saveErr != nil {
		return models.Tip{}, saveErr
	}

	return tip, nil
}

func (r *postgresRepository) ListTips(channelID string, limit int) ([]models.Tip, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}

	tips := make([]models.Tip, 0)
	listErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			return fmt.Errorf("begin list tips tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}

		query := "SELECT id, channel_id, from_user_id, amount, currency, provider, reference, wallet_address, message, created_at FROM tips WHERE channel_id = $1 ORDER BY created_at DESC, id ASC"
		args := []any{channelID}
		if limit > 0 {
			query += " LIMIT $2"
			args = append(args, limit)
		}

		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list tips: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var tip models.Tip
			var walletAddress, message pgtype.Text
			var createdAt time.Time
			if err := rows.Scan(&tip.ID, &tip.ChannelID, &tip.FromUserID, &tip.Amount, &tip.Currency, &tip.Provider, &tip.Reference, &walletAddress, &message, &createdAt); err != nil {
				return fmt.Errorf("scan tip: %w", err)
			}
			if walletAddress.Valid {
				tip.WalletAddress = walletAddress.String
			}
			if message.Valid {
				tip.Message = message.String
			}
			tip.CreatedAt = createdAt.UTC()
			tips = append(tips, tip)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit list tips: %w", err)
		}

		return nil
	})
	if listErr != nil {
		return nil, listErr
	}

	return tips, nil
}

func (r *postgresRepository) CreateSubscription(params CreateSubscriptionParams) (models.Subscription, error) {
	if r == nil || r.pool == nil {
		return models.Subscription{}, ErrPostgresUnavailable
	}

	if params.Duration <= 0 {
		return models.Subscription{}, fmt.Errorf("duration must be positive")
	}

	amount := params.Amount
	if amount < 0 {
		return models.Subscription{}, fmt.Errorf("amount cannot be negative")
	}

	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return models.Subscription{}, fmt.Errorf("currency is required")
	}

	tier := strings.TrimSpace(params.Tier)
	if tier == "" {
		tier = "supporter"
	}

	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return models.Subscription{}, fmt.Errorf("provider is required")
	}

	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("sub-%d", time.Now().UnixNano())
	}

	externalRef := strings.TrimSpace(params.ExternalReference)

	id, err := r.generateID()
	if err != nil {
		return models.Subscription{}, err
	}

	started := time.Now().UTC()
	expires := started.Add(params.Duration)

	var subscription models.Subscription
	saveErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create subscription tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, params.ChannelID); err != nil {
			return err
		}
		if err := ensureUserExists(ctx, tx, params.UserID); err != nil {
			return err
		}

		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM subscriptions WHERE provider = $1 AND reference = $2)", provider, reference).Scan(&exists); err != nil {
			return fmt.Errorf("check subscription reference: %w", err)
		}
		if exists {
			return fmt.Errorf("subscription reference %s/%s already exists", provider, reference)
		}

		_, err = tx.Exec(ctx, "INSERT INTO subscriptions (id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, external_reference) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)", id, params.ChannelID, params.UserID, tier, provider, reference, amount, currency, started, expires, params.AutoRenew, "active", externalRef)
		if err != nil {
			return fmt.Errorf("insert subscription: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create subscription: %w", err)
		}

		subscription = models.Subscription{
			ID:                id,
			ChannelID:         params.ChannelID,
			UserID:            params.UserID,
			Tier:              tier,
			Provider:          provider,
			Reference:         reference,
			Amount:            amount,
			Currency:          currency,
			StartedAt:         started,
			ExpiresAt:         expires,
			AutoRenew:         params.AutoRenew,
			Status:            "active",
			ExternalReference: externalRef,
		}

		return nil
	})
	if saveErr != nil {
		return models.Subscription{}, saveErr
	}

	return subscription, nil
}

func (r *postgresRepository) ListSubscriptions(channelID string, includeInactive bool) ([]models.Subscription, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}

	subscriptions := make([]models.Subscription, 0)
	listErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			return fmt.Errorf("begin list subscriptions tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}

		query := "SELECT id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference FROM subscriptions WHERE channel_id = $1"
		args := []any{channelID}
		if !includeInactive {
			query += " AND status = 'active'"
		}
		query += " ORDER BY started_at DESC, id ASC"

		rows, err := tx.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list subscriptions: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			sub, err := scanSubscriptionRow(rows)
			if err != nil {
				return fmt.Errorf("scan subscription: %w", err)
			}
			subscriptions = append(subscriptions, sub)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit list subscriptions: %w", err)
		}

		return nil
	})
	if listErr != nil {
		return nil, listErr
	}

	return subscriptions, nil
}

func (r *postgresRepository) GetSubscription(id string) (models.Subscription, bool) {
	if r == nil || r.pool == nil {
		return models.Subscription{}, false
	}

	ctx := context.Background()
	row := r.pool.QueryRow(ctx, "SELECT id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference FROM subscriptions WHERE id = $1", id)

	sub, err := scanSubscriptionRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Subscription{}, false
		}
		return models.Subscription{}, false
	}

	return sub, true
}

func (r *postgresRepository) CancelSubscription(id, cancelledBy, reason string) (models.Subscription, error) {
	if r == nil || r.pool == nil {
		return models.Subscription{}, ErrPostgresUnavailable
	}

	trimmedReason := strings.TrimSpace(reason)

	var updated models.Subscription
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin cancel subscription tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		row := tx.QueryRow(ctx, "SELECT id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference FROM subscriptions WHERE id = $1 FOR UPDATE", id)
		sub, err := scanSubscriptionRow(row)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("subscription %s not found", id)
			}
			return fmt.Errorf("load subscription: %w", err)
		}

		if strings.EqualFold(sub.Status, "cancelled") {
			updated = sub
			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("commit cancel subscription no-op: %w", err)
			}
			return nil
		}

		if err := ensureUserExists(ctx, tx, cancelledBy); err != nil {
			return err
		}

		now := time.Now().UTC()
		finalReason := trimmedReason
		if finalReason == "" {
			if cancelledBy == sub.UserID {
				finalReason = "user_cancelled"
			} else {
				finalReason = "cancelled_by_admin"
			}
		}

		_, err = tx.Exec(ctx, "UPDATE subscriptions SET status = $1, auto_renew = FALSE, cancelled_by = $2, cancelled_reason = $3, cancelled_at = $4 WHERE id = $5", "cancelled", cancelledBy, finalReason, now, id)
		if err != nil {
			return fmt.Errorf("update subscription cancellation: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit cancel subscription: %w", err)
		}

		sub.Status = "cancelled"
		sub.AutoRenew = false
		sub.CancelledBy = cancelledBy
		sub.CancelledReason = finalReason
		sub.CancelledAt = &now

		updated = sub
		return nil
	})
	if err != nil {
		return models.Subscription{}, err
	}

	return updated, nil
}

func (r *postgresRepository) AuthenticateOAuth(params OAuthLoginParams) (models.User, error) {
	if r == nil || r.pool == nil {
		return models.User{}, ErrPostgresUnavailable
	}

	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	subject := strings.TrimSpace(params.Subject)
	if provider == "" {
		return models.User{}, fmt.Errorf("provider is required")
	}
	if subject == "" {
		return models.User{}, fmt.Errorf("subject is required")
	}

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		normalizedEmail = fallbackOAuthEmail(provider, subject)
	}
	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		displayName = defaultOAuthDisplayName(provider, normalizedEmail, subject)
	}

	var user models.User
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin oauth tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var userID string
		lookupErr := tx.QueryRow(ctx, "SELECT user_id FROM oauth_accounts WHERE provider = $1 AND subject = $2", provider, subject).Scan(&userID)
		if lookupErr != nil && !errors.Is(lookupErr, pgx.ErrNoRows) {
			return fmt.Errorf("lookup oauth account: %w", lookupErr)
		}
		if lookupErr == nil {
			row := tx.QueryRow(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users WHERE id = $1", userID)
			loaded, err := scanUser(row)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					if _, execErr := tx.Exec(ctx, "DELETE FROM oauth_accounts WHERE provider = $1 AND subject = $2", provider, subject); execErr != nil {
						return fmt.Errorf("delete stale oauth account: %w", execErr)
					}
				} else {
					return fmt.Errorf("load oauth user: %w", err)
				}
			} else {
				user = loaded
				if err := tx.Commit(ctx); err != nil {
					return fmt.Errorf("commit oauth tx: %w", err)
				}
				return nil
			}
		}

		if userID == "" && normalizedEmail != "" {
			err = tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", normalizedEmail).Scan(&userID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("lookup user by email: %w", err)
			}
			if errors.Is(err, pgx.ErrNoRows) {
				err = nil
			}
		}

		now := time.Now().UTC()
		if userID == "" {
			userID, err = r.generateID()
			if err != nil {
				return err
			}
			roles := []string{"viewer"}
			createdAt := now
			err = tx.QueryRow(ctx, "INSERT INTO users (id, display_name, email, roles, self_signup) VALUES ($1, $2, $3, $4, $5) RETURNING created_at", userID, displayName, normalizedEmail, roles, true).Scan(&createdAt)
			if err != nil {
				return fmt.Errorf("create oauth user: %w", err)
			}
			user = models.User{
				ID:          userID,
				DisplayName: displayName,
				Email:       normalizedEmail,
				Roles:       roles,
				SelfSignup:  true,
				CreatedAt:   createdAt.UTC(),
			}
		} else {
			row := tx.QueryRow(ctx, "SELECT id, display_name, email, roles, password_hash, self_signup, created_at FROM users WHERE id = $1 FOR UPDATE", userID)
			loaded, err := scanUser(row)
			if err != nil {
				return fmt.Errorf("load existing user: %w", err)
			}
			if strings.TrimSpace(loaded.DisplayName) == "" {
				loaded.DisplayName = displayName
				if _, err := tx.Exec(ctx, "UPDATE users SET display_name = $1 WHERE id = $2", loaded.DisplayName, loaded.ID); err != nil {
					return fmt.Errorf("update user display name: %w", err)
				}
			}
			user = loaded
		}

		_, err = tx.Exec(ctx, `INSERT INTO oauth_accounts (provider, subject, user_id, email, display_name, linked_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (provider, subject) DO UPDATE
SET user_id = EXCLUDED.user_id, email = EXCLUDED.email, display_name = EXCLUDED.display_name, linked_at = NOW()`, provider, subject, user.ID, normalizedEmail, displayName)
		if err != nil {
			return fmt.Errorf("upsert oauth account: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit oauth tx: %w", err)
		}
		return nil
	})
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

var _ Repository = (*postgresRepository)(nil)
