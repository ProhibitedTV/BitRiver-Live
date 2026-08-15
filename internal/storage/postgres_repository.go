package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"bitriver-live/internal/ctxutil"
	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrPostgresUnavailable is returned when the Postgres repository has not yet
// been wired into the build.
var ErrPostgresUnavailable = fmt.Errorf("postgres repository unavailable")

const defaultPostgresPingTimeout = 5 * time.Second

type postgresRepository struct {
	pool                 *pgxpool.Pool
	cfg                  PostgresConfig
	ingestController     ingest.Controller
	ingestMaxAttempts    int
	ingestRetryInterval  time.Duration
	ingestTimeout        time.Duration
	ingestHealthMu       sync.RWMutex
	ingestHealthInitDone chan struct{}
	ingestHealth         []ingest.HealthStatus
	ingestHealthUpdated  time.Time
	recordingRetention   RecordingRetentionPolicy
	chatRetention        ChatRetentionPolicy
	objectStorage        ObjectStorageConfig
	objectClient         objectStorageClient
	retentionNow         func() time.Time
}

// Close shuts down the backing Postgres pool and respects context cancellation.
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

// Ping verifies Postgres connectivity for this repository instance.
func (r *postgresRepository) Ping(ctx context.Context) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx = ctxutil.Normalize(ctx)
	return pingPostgresPool(ctx, r.pool)
}

// NewPostgresRepository opens a Postgres-backed repository. The caller must
// ensure database migrations have been applied prior to invoking this
// constructor.
func NewPostgresRepository(dsn string, opts ...Option) (Repository, error) {
	cfg := newPostgresConfig(dsn, opts...)
	if strings.TrimSpace(cfg.DSN) == "" {
		return nil, fmt.Errorf("postgres dsn required")
	}
	if strings.Contains(strings.ToLower(pgx.ErrNoRows.Error()), "stub") {
		return nil, wrapPostgresUnavailable(errors.New("pgx driver stubbed in this build"))
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

	pingTimeout := cfg.PingTimeout
	if pingTimeout <= 0 {
		pingTimeout = defaultPostgresPingTimeout
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err := pingPostgresPool(pingCtx, pool); err != nil {
		pool.Close()
		return nil, wrapPostgresUnavailable(err)
	}

	repo := &postgresRepository{
		pool:                pool,
		cfg:                 cfg,
		ingestController:    cfg.IngestController,
		ingestMaxAttempts:   cfg.IngestMaxAttempts,
		ingestRetryInterval: cfg.IngestRetryInterval,
		ingestTimeout:       normalizeIngestTimeout(cfg.IngestTimeout),
		recordingRetention:  cfg.RecordingRetention,
		chatRetention:       cfg.ChatRetention,
		objectStorage:       cfg.ObjectStorage,
		retentionNow:        cfg.RetentionClock,
	}
	repo.objectStorage = applyObjectStorageDefaults(repo.objectStorage)
	repo.objectClient = newObjectStorageClient(repo.objectStorage)
	return repo, nil
}

// pingPostgresPool acquires a connection and runs a lightweight `SELECT 1` health check.
func pingPostgresPool(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("postgres pool unavailable")
	}
	ctx = ctxutil.Normalize(ctx)
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire postgres connection: %w", err)
	}
	defer conn.Release()
	if _, execErr := conn.Exec(ctx, "SELECT 1"); execErr != nil {
		return fmt.Errorf("ping postgres: %w", execErr)
	}
	return nil
}

// wrapPostgresUnavailable annotates an underlying error with ErrPostgresUnavailable.
func wrapPostgresUnavailable(err error) error {
	if err == nil {
		return ErrPostgresUnavailable
	}
	return fmt.Errorf("%w: %w", ErrPostgresUnavailable, err)
}

// IngestHealth returns the latest ingest health snapshot and refreshes cached values.
func (r *postgresRepository) IngestHealth(ctx context.Context) []ingest.HealthStatus {
	if wait, probe := r.beginIngestHealthProbe(); !probe {
		<-wait
		snapshot, _ := r.LastIngestHealth()
		return snapshot
	}

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
	if done := r.ingestHealthInitDone; done != nil {
		close(done)
		r.ingestHealthInitDone = nil
	}
	r.ingestHealthMu.Unlock()

	return snapshot
}

// beginIngestHealthProbe coalesces callers while the empty cache is receiving
// its first probe. Once a snapshot exists, callers retain the normal refresh
// behavior.
func (r *postgresRepository) beginIngestHealthProbe() (<-chan struct{}, bool) {
	r.ingestHealthMu.Lock()
	defer r.ingestHealthMu.Unlock()
	if len(r.ingestHealth) > 0 && !r.ingestHealthUpdated.IsZero() {
		return nil, true
	}
	if r.ingestHealthInitDone != nil {
		return r.ingestHealthInitDone, false
	}
	r.ingestHealthInitDone = make(chan struct{})
	return nil, true
}

// LastIngestHealth returns the last cached ingest health snapshot and update timestamp.
func (r *postgresRepository) LastIngestHealth() ([]ingest.HealthStatus, time.Time) {
	r.ingestHealthMu.RLock()
	defer r.ingestHealthMu.RUnlock()
	clone := append([]ingest.HealthStatus(nil), r.ingestHealth...)
	return clone, r.ingestHealthUpdated
}

// acquireContext creates a background context with optional acquire timeout.
func (r *postgresRepository) acquireContext() (context.Context, context.CancelFunc) {
	if r == nil {
		return context.Background(), func() {}
	}
	if r.cfg.AcquireTimeout > 0 {
		return context.WithTimeout(context.Background(), r.cfg.AcquireTimeout)
	}
	return context.Background(), func() {}
}

// withConn acquires a pooled connection and executes fn with repository acquire timeout settings.
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

// encodeDonationAddresses marshals donation addresses to JSON, normalizing nil slices to empty arrays.
func encodeDonationAddresses(addresses []domain.CryptoAddress) ([]byte, error) {
	if addresses == nil {
		addresses = []domain.CryptoAddress{}
	}
	data, err := json.Marshal(addresses)
	if err != nil {
		return nil, fmt.Errorf("encode donation addresses: %w", err)
	}
	return data, nil
}

// decodeDonationAddresses unmarshals donation addresses from JSON, normalizing nil to an empty slice.
func decodeDonationAddresses(data []byte) ([]domain.CryptoAddress, error) {
	if len(data) == 0 {
		return []domain.CryptoAddress{}, nil
	}
	var addresses []domain.CryptoAddress
	if err := json.Unmarshal(data, &addresses); err != nil {
		return nil, fmt.Errorf("decode donation addresses: %w", err)
	}
	if addresses == nil {
		addresses = []domain.CryptoAddress{}
	}
	return addresses, nil
}

// encodeSocialLinks marshals social links to JSON, normalizing nil slices to empty arrays.
func encodeSocialLinks(links []domain.SocialLink) ([]byte, error) {
	if links == nil {
		links = []domain.SocialLink{}
	}
	data, err := json.Marshal(links)
	if err != nil {
		return nil, fmt.Errorf("encode social links: %w", err)
	}
	return data, nil
}

// decodeSocialLinks unmarshals social links from JSON, normalizing nil to an empty slice.
func decodeSocialLinks(data []byte) ([]domain.SocialLink, error) {
	if len(data) == 0 {
		return []domain.SocialLink{}, nil
	}
	var links []domain.SocialLink
	if err := json.Unmarshal(data, &links); err != nil {
		return nil, fmt.Errorf("decode social links: %w", err)
	}
	if links == nil {
		links = []domain.SocialLink{}
	}
	return links, nil
}

// rollbackTx best-effort rolls back tx and ignores already-closed transaction errors.
func rollbackTx(ctx context.Context, tx pgx.Tx) {
	if tx == nil {
		return
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Default().Debug("rollback transaction", "error", err)
	}
}

// ensureUserExists returns an error when the referenced user row does not exist.
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

// ensureChannelExists returns an error when the referenced channel row does not exist.
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

// UpsertProfile creates or updates a profile row for userID and returns the resulting profile state.
func (r *postgresRepository) UpsertProfile(userID string, update ProfileUpdate) (domain.Profile, error) {
	if r == nil || r.pool == nil {
		return domain.Profile{}, ErrPostgresUnavailable
	}
	profile := domain.Profile{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin upsert profile tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var userCreatedAt time.Time
		if err := tx.QueryRow(ctx, "SELECT created_at FROM users WHERE id = $1", userID).Scan(&userCreatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("user %s not found", userID)
			}
			return fmt.Errorf("load user %s: %w", userID, err)
		}

		profile = domain.Profile{
			UserID:            userID,
			Bio:               "",
			SocialLinks:       []domain.SocialLink{},
			TopFriends:        []string{},
			DonationAddresses: []domain.CryptoAddress{},
			CreatedAt:         userCreatedAt.UTC(),
			UpdatedAt:         userCreatedAt.UTC(),
		}
		var (
			avatar, banner           pgtype.Text
			featured                 pgtype.Text
			topFriends               []string
			socialLinksPayload       []byte
			donationAddressesPayload []byte
			createdAt, updatedAt     time.Time
		)
		row := tx.QueryRow(ctx, "SELECT bio, avatar_url, banner_url, featured_channel_id, top_friends, social_links, donation_addresses, created_at, updated_at FROM profiles WHERE user_id = $1", userID)
		switch err := row.Scan(&profile.Bio, &avatar, &banner, &featured, &topFriends, &socialLinksPayload, &donationAddressesPayload, &createdAt, &updatedAt); {
		case errors.Is(err, pgx.ErrNoRows):
			// Use defaults.
		case err != nil:
			return fmt.Errorf("load profile %s: %w", userID, err)
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
			if len(socialLinksPayload) > 0 {
				links, err := decodeSocialLinks(socialLinksPayload)
				if err != nil {
					return fmt.Errorf("decode social links: %w", err)
				}
				profile.SocialLinks = links
			}
			if len(topFriends) > 0 {
				profile.TopFriends = append([]string{}, topFriends...)
			}
			if len(donationAddressesPayload) > 0 {
				decoded, err := decodeDonationAddresses(donationAddressesPayload)
				if err != nil {
					return fmt.Errorf("decode donation addresses: %w", err)
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
		if update.SocialLinks != nil {
			normalized, err := NormalizeSocialLinks(*update.SocialLinks)
			if err != nil {
				return err
			}
			profile.SocialLinks = normalized
		}
		if update.FeaturedChannelID != nil {
			trimmed := strings.TrimSpace(*update.FeaturedChannelID)
			if trimmed == "" {
				profile.FeaturedChannelID = nil
			} else {
				var ownerID string
				err := tx.QueryRow(ctx, "SELECT owner_id FROM channels WHERE id = $1", trimmed).Scan(&ownerID)
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("featured channel %s not found", trimmed)
				}
				if err != nil {
					return fmt.Errorf("load featured channel %s: %w", trimmed, err)
				}
				if ownerID != userID {
					return errors.New("featured channel must belong to profile owner")
				}
				id := trimmed
				profile.FeaturedChannelID = &id
			}
		}
		if update.TopFriends != nil {
			if len(*update.TopFriends) > 8 {
				return errors.New("top friends cannot exceed eight entries")
			}
			seen := make(map[string]struct{}, len(*update.TopFriends))
			ordered := make([]string, 0, len(*update.TopFriends))
			for _, friendID := range *update.TopFriends {
				trimmed := strings.TrimSpace(friendID)
				if trimmed == "" {
					return errors.New("top friends must reference valid users")
				}
				if trimmed == userID {
					return errors.New("cannot add profile owner as a top friend")
				}
				if _, exists := seen[trimmed]; exists {
					return errors.New("duplicate user in top friends list")
				}
				seen[trimmed] = struct{}{}
				ordered = append(ordered, trimmed)
			}
			if len(ordered) > 0 {
				rows, err := tx.Query(ctx, "SELECT id FROM users WHERE id = ANY($1)", ordered)
				if err != nil {
					return fmt.Errorf("validate top friends: %w", err)
				}
				defer rows.Close()
				found := make(map[string]struct{}, len(ordered))
				for rows.Next() {
					var id string
					if err := rows.Scan(&id); err != nil {
						return fmt.Errorf("scan top friend id: %w", err)
					}
					found[id] = struct{}{}
				}
				if err := rows.Err(); err != nil {
					return fmt.Errorf("iterate top friends: %w", err)
				}
				for _, id := range ordered {
					if _, ok := found[id]; !ok {
						return fmt.Errorf("top friend %s not found", id)
					}
				}
			}
			profile.TopFriends = ordered
		}
		if update.DonationAddresses != nil {
			addresses := make([]domain.CryptoAddress, 0, len(*update.DonationAddresses))
			for _, addr := range *update.DonationAddresses {
				normalized, err := NormalizeDonationAddress(addr)
				if err != nil {
					return err
				}
				addresses = append(addresses, normalized)
			}
			profile.DonationAddresses = addresses
		}

		profile.UpdatedAt = now
		if profile.CreatedAt.IsZero() {
			profile.CreatedAt = now
		}

		socialLinksPayload, err = encodeSocialLinks(profile.SocialLinks)
		if err != nil {
			return err
		}
		donationPayload, err := encodeDonationAddresses(profile.DonationAddresses)
		if err != nil {
			return err
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
INSERT INTO profiles (user_id, bio, avatar_url, banner_url, featured_channel_id, top_friends, social_links, donation_addresses, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (user_id) DO UPDATE SET
        bio = EXCLUDED.bio,
        avatar_url = EXCLUDED.avatar_url,
        banner_url = EXCLUDED.banner_url,
        featured_channel_id = EXCLUDED.featured_channel_id,
        top_friends = EXCLUDED.top_friends,
        social_links = EXCLUDED.social_links,
        donation_addresses = EXCLUDED.donation_addresses,
        updated_at = EXCLUDED.updated_at
RETURNING created_at, updated_at`,
			userID,
			profile.Bio,
			profile.AvatarURL,
			profile.BannerURL,
			featuredValue,
			topFriendsValue,
			socialLinksPayload,
			donationPayload,
			profile.CreatedAt,
			profile.UpdatedAt,
		).Scan(&insertedCreatedAt, &insertedUpdatedAt)
		if err != nil {
			return fmt.Errorf("upsert profile %s: %w", userID, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit upsert profile: %w", err)
		}

		profile.CreatedAt = insertedCreatedAt.UTC()
		profile.UpdatedAt = insertedUpdatedAt.UTC()
		if profile.TopFriends == nil {
			profile.TopFriends = []string{}
		}
		if profile.DonationAddresses == nil {
			profile.DonationAddresses = []domain.CryptoAddress{}
		}
		return nil
	})
	if err != nil {
		return domain.Profile{}, err
	}
	return profile, nil
}

// GetProfile loads a profile for userID.
// It returns found=false when no profile row exists (including missing users).
func (r *postgresRepository) GetProfile(userID string) (domain.Profile, bool) {
	if r == nil || r.pool == nil {
		return domain.Profile{}, false
	}
	var (
		profile domain.Profile
		found   bool
		ok      bool
		loadErr error
	)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var (
			bio                      string
			avatar, banner, featured pgtype.Text
			topFriends               []string
			socialLinksPayload       []byte
			donationPayload          []byte
			createdAt, updatedAt     time.Time
		)
		err := conn.QueryRow(ctx, "SELECT bio, avatar_url, banner_url, featured_channel_id, top_friends, social_links, donation_addresses, created_at, updated_at FROM profiles WHERE user_id = $1", userID).
			Scan(&bio, &avatar, &banner, &featured, &topFriends, &socialLinksPayload, &donationPayload, &createdAt, &updatedAt)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			var userCreatedAt time.Time
			if err := conn.QueryRow(ctx, "SELECT created_at FROM users WHERE id = $1", userID).Scan(&userCreatedAt); err != nil {
				loadErr = err
				return nil
			}
			profile = domain.Profile{
				UserID:            userID,
				Bio:               "",
				SocialLinks:       []domain.SocialLink{},
				AvatarURL:         "",
				BannerURL:         "",
				TopFriends:        []string{},
				DonationAddresses: []domain.CryptoAddress{},
				CreatedAt:         userCreatedAt.UTC(),
				UpdatedAt:         userCreatedAt.UTC(),
			}
			found = false
			ok = true
			return nil
		case err != nil:
			loadErr = err
			return nil
		default:
			profile = domain.Profile{
				UserID:      userID,
				Bio:         bio,
				CreatedAt:   createdAt.UTC(),
				UpdatedAt:   updatedAt.UTC(),
				TopFriends:  []string{},
				SocialLinks: []domain.SocialLink{},
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
			if len(socialLinksPayload) > 0 {
				links, err := decodeSocialLinks(socialLinksPayload)
				if err != nil {
					loadErr = err
					return nil
				}
				profile.SocialLinks = links
			}
			if len(topFriends) > 0 {
				profile.TopFriends = append([]string{}, topFriends...)
			}
			if len(donationPayload) > 0 {
				addresses, err := decodeDonationAddresses(donationPayload)
				if err != nil {
					loadErr = err
					return nil
				}
				profile.DonationAddresses = addresses
			} else {
				profile.DonationAddresses = []domain.CryptoAddress{}
			}
			if profile.TopFriends == nil {
				profile.TopFriends = []string{}
			}
			found = true
			ok = true
			return nil
		}
	})
	if err != nil {
		return domain.Profile{}, false
	}
	if loadErr != nil || !ok {
		return domain.Profile{}, false
	}
	if profile.SocialLinks == nil {
		profile.SocialLinks = []domain.SocialLink{}
	}
	if profile.TopFriends == nil {
		profile.TopFriends = []string{}
	}
	if profile.DonationAddresses == nil {
		profile.DonationAddresses = []domain.CryptoAddress{}
	}
	return profile, found
}

// ListProfiles returns all profiles ordered by creation time.
// On query failures it returns nil.
func (r *postgresRepository) ListProfiles() []domain.Profile {
	if r == nil || r.pool == nil {
		return nil
	}
	profiles := make([]domain.Profile, 0)
	var queryErr error
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, "SELECT user_id, bio, avatar_url, banner_url, featured_channel_id, top_friends, social_links, donation_addresses, created_at, updated_at FROM profiles ORDER BY created_at ASC")
		if err != nil {
			queryErr = err
			return nil
		}
		defer rows.Close()

		for rows.Next() {
			var (
				userID                   string
				bio                      string
				avatar, banner, featured pgtype.Text
				topFriends               []string
				socialLinksPayload       []byte
				donationPayload          []byte
				createdAt, updatedAt     time.Time
			)
			if err := rows.Scan(&userID, &bio, &avatar, &banner, &featured, &topFriends, &socialLinksPayload, &donationPayload, &createdAt, &updatedAt); err != nil {
				queryErr = err
				return nil
			}
			profile := domain.Profile{
				UserID:      userID,
				Bio:         bio,
				CreatedAt:   createdAt.UTC(),
				UpdatedAt:   updatedAt.UTC(),
				TopFriends:  []string{},
				SocialLinks: []domain.SocialLink{},
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
			if len(socialLinksPayload) > 0 {
				links, err := decodeSocialLinks(socialLinksPayload)
				if err != nil {
					queryErr = err
					return nil
				}
				profile.SocialLinks = links
			}
			if len(topFriends) > 0 {
				profile.TopFriends = append([]string{}, topFriends...)
			}
			if len(donationPayload) > 0 {
				addresses, err := decodeDonationAddresses(donationPayload)
				if err != nil {
					queryErr = err
					return nil
				}
				profile.DonationAddresses = addresses
			} else {
				profile.DonationAddresses = []domain.CryptoAddress{}
			}
			if profile.SocialLinks == nil {
				profile.SocialLinks = []domain.SocialLink{}
			}
			if profile.TopFriends == nil {
				profile.TopFriends = []string{}
			}
			profiles = append(profiles, profile)
		}
		if err := rows.Err(); err != nil {
			queryErr = err
			return nil
		}
		return nil
	})
	if err != nil || queryErr != nil {
		return nil
	}
	return profiles
}

var _ Repository = (*postgresRepository)(nil)
