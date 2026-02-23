package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/ingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (r *postgresRepository) CreateChannel(ownerID, title, category string, tags []string) (domain.Channel, error) {
	if r == nil || r.pool == nil {
		return domain.Channel{}, ErrPostgresUnavailable
	}
	if strings.TrimSpace(ownerID) == "" {
		return domain.Channel{}, fmt.Errorf("owner %s not found", ownerID)
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		return domain.Channel{}, errors.New("title is required")
	}

	var (
		channel           domain.Channel
		insertedCreatedAt time.Time
		insertedUpdatedAt time.Time
		streamKey         string
		id                string
		normalizedTags    []string
		trimmedCategory   string
	)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create channel tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", ownerID).Scan(&exists); err != nil {
			return fmt.Errorf("check owner %s: %w", ownerID, err)
		}
		if !exists {
			return fmt.Errorf("owner %s not found", ownerID)
		}

		id, err = generateID()
		if err != nil {
			return err
		}
		streamKey, err = generateStreamKey()
		if err != nil {
			return err
		}
		normalizedTags = normalizeTags(tags)
		trimmedCategory = strings.TrimSpace(category)
		now := time.Now().UTC()

		err = tx.QueryRow(ctx, "INSERT INTO channels (id, owner_id, stream_key, title, category, tags, live_state, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, 'offline', $7, $8) RETURNING created_at, updated_at",
			id,
			ownerID,
			streamKey,
			trimmedTitle,
			trimmedCategory,
			normalizedTags,
			now,
			now,
		).Scan(&insertedCreatedAt, &insertedUpdatedAt)
		if err != nil {
			return fmt.Errorf("insert channel: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create channel: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Channel{}, err
	}

	channel = domain.Channel{
		ID:        id,
		OwnerID:   ownerID,
		StreamKey: streamKey,
		Title:     trimmedTitle,
		Category:  trimmedCategory,
		Tags:      normalizedTags,
		LiveState: "offline",
		CreatedAt: insertedCreatedAt.UTC(),
		UpdatedAt: insertedUpdatedAt.UTC(),
	}
	return channel, nil
}

// UpdateChannel executes UpdateChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) UpdateChannel(id string, update ChannelUpdate) (domain.Channel, error) {
	if r == nil || r.pool == nil {
		return domain.Channel{}, ErrPostgresUnavailable
	}
	var channel domain.Channel
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin update channel tx: %w", err)
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
				return fmt.Errorf("channel %s not found", id)
			}
			return fmt.Errorf("load channel %s: %w", id, err)
		}

		channel = domain.Channel{
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
				return errors.New("title cannot be empty")
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
				return fmt.Errorf("invalid liveState %s", state)
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
			return fmt.Errorf("update channel %s: %w", id, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit update channel: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Channel{}, err
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, nil
}

// RotateChannelStreamKey executes RotateChannelStreamKey.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) RotateChannelStreamKey(id string) (domain.Channel, error) {
	if r == nil || r.pool == nil {
		return domain.Channel{}, ErrPostgresUnavailable
	}
	var channel domain.Channel
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin rotate stream key tx: %w", err)
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
				return fmt.Errorf("channel %s not found", id)
			}
			return fmt.Errorf("load channel %s: %w", id, err)
		}

		newKey, err := generateStreamKey()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, "UPDATE channels SET stream_key = $1, updated_at = $2 WHERE id = $3", newKey, now, id); err != nil {
			return fmt.Errorf("update stream key: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit rotate stream key: %w", err)
		}

		channel = domain.Channel{
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
		return nil
	})
	if err != nil {
		return domain.Channel{}, err
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, nil
}

// DeleteChannel executes DeleteChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) DeleteChannel(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	return r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
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
	})
}

// GetChannel executes GetChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) GetChannel(id string) (domain.Channel, bool) {
	if r == nil || r.pool == nil {
		return domain.Channel{}, false
	}
	var channel domain.Channel
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var (
			channelID, ownerID, streamKey, title string
			category                             pgtype.Text
			tags                                 []string
			liveState                            string
			currentSession                       pgtype.Text
			createdAt, updatedAt                 time.Time
		)
		err := conn.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE id = $1", id).
			Scan(&channelID, &ownerID, &streamKey, &title, &category, &tags, &liveState, &currentSession, &createdAt, &updatedAt)
		if err != nil {
			return err
		}
		channel = domain.Channel{
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
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return domain.Channel{}, false
	}
	if channel.Tags == nil {
		channel.Tags = []string{}
	}
	return channel, true
}

// GetChannelByStreamKey executes GetChannelByStreamKey.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) GetChannelByStreamKey(streamKey string) (domain.Channel, bool) {
	if r == nil || r.pool == nil {
		return domain.Channel{}, false
	}
	key := strings.TrimSpace(streamKey)
	if key == "" {
		return domain.Channel{}, false
	}

	var channel domain.Channel
	found := false
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var (
			category       pgtype.Text
			tags           []string
			currentSession pgtype.Text
			createdAt      time.Time
			updatedAt      time.Time
		)
		row := conn.QueryRow(ctx, "SELECT id, owner_id, stream_key, title, category, tags, live_state, current_session_id, created_at, updated_at FROM channels WHERE stream_key = $1", key)
		if err := row.Scan(&channel.ID, &channel.OwnerID, &channel.StreamKey, &channel.Title, &category, &tags, &channel.LiveState, &currentSession, &createdAt, &updatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			return fmt.Errorf("load channel by stream key: %w", err)
		}
		channel.Tags = append([]string{}, tags...)
		if category.Valid {
			channel.Category = category.String
		}
		if currentSession.Valid {
			id := currentSession.String
			channel.CurrentSessionID = &id
		}
		channel.CreatedAt = createdAt.UTC()
		channel.UpdatedAt = updatedAt.UTC()
		found = true
		return nil
	})
	if err != nil || !found {
		return domain.Channel{}, false
	}
	return channel, true
}

// ListChannels executes ListChannels.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListChannels(ownerID, query string) []domain.Channel {
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

	channels := make([]domain.Channel, 0)
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
		channel := domain.Channel{
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

// FollowChannel executes FollowChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) FollowChannel(userID, channelID string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	return r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
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
	})
}

// UnfollowChannel executes UnfollowChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) UnfollowChannel(userID, channelID string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	return r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
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
	})
}

// IsFollowingChannel executes IsFollowingChannel.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) IsFollowingChannel(userID, channelID string) bool {
	if r == nil || r.pool == nil {
		return false
	}
	var exists bool
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM follows WHERE user_id = $1 AND channel_id = $2)", userID, channelID).Scan(&exists)
	})
	if err != nil {
		return false
	}
	return exists
}

// CountFollowers executes CountFollowers.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) CountFollowers(channelID string) int {
	if r == nil || r.pool == nil {
		return 0
	}
	var count int
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, "SELECT COUNT(*) FROM follows WHERE channel_id = $1", channelID).Scan(&count)
	})
	if err != nil {
		return 0
	}
	return count
}

// ListFollowedChannelIDs executes ListFollowedChannelIDs.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListFollowedChannelIDs(userID string) []string {
	if r == nil || r.pool == nil {
		return nil
	}
	ids := make([]string, 0)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		rows, err := conn.Query(ctx, "SELECT channel_id FROM follows WHERE user_id = $1 ORDER BY followed_at DESC", userID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var channelID string
			if err := rows.Scan(&channelID); err != nil {
				return err
			}
			ids = append(ids, channelID)
		}
		return rows.Err()
	})
	if err != nil {
		return nil
	}
	return ids
}

// StartStream executes StartStream.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) StartStream(channelID string, renditions []string) (domain.StreamSession, error) {
	if r == nil || r.pool == nil {
		return domain.StreamSession{}, ErrPostgresUnavailable
	}
	var (
		streamKey      string
		sessionID      string
		startedAt      time.Time
		currentSession pgtype.Text
	)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin start stream tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var (
			ownerID, title, category pgtype.Text
			tags                     []string
		)
		row := tx.QueryRow(ctx, "SELECT stream_key, current_session_id, owner_id, title, category, tags FROM channels WHERE id = $1 FOR UPDATE", channelID)
		if err := row.Scan(&streamKey, &currentSession, &ownerID, &title, &category, &tags); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("channel %s not found", channelID)
			}
			return fmt.Errorf("load channel %s: %w", channelID, err)
		}
		if currentSession.Valid {
			return errors.New("channel already live")
		}

		sessionID, err = generateID()
		if err != nil {
			return err
		}
		startedAt = time.Now().UTC()
		if _, err := tx.Exec(ctx, "UPDATE channels SET current_session_id = $1, live_state = 'starting', updated_at = $2 WHERE id = $3", sessionID, startedAt, channelID); err != nil {
			return fmt.Errorf("mark channel starting: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit mark channel starting: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.StreamSession{}, err
	}

	controller := r.ingestController
	if controller == nil {
		_ = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
			_, err := conn.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
			return err
		})
		return domain.StreamSession{}, ErrIngestControllerUnavailable
	}
	deadline := normalizeIngestTimeout(r.ingestTimeout)
	boot, bootErr := runIngestBootWithRetry(controller, ingest.BootParams{
		ChannelID:  channelID,
		SessionID:  sessionID,
		StreamKey:  streamKey,
		Renditions: append([]string{}, renditions...),
	}, deadline, r.ingestMaxAttempts, r.ingestRetryInterval)
	if bootErr != nil {
		_ = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
			_, err := conn.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
			return err
		})
		return domain.StreamSession{}, fmt.Errorf("boot ingest: %w", bootErr)
	}

	session := domain.StreamSession{
		ID:             sessionID,
		ChannelID:      channelID,
		StartedAt:      startedAt,
		Renditions:     append([]string{}, renditions...),
		PeakConcurrent: 0,
	}
	applyBootResultToSession(&session, boot, true)

	revertChannel := func() {
		_ = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
			_, err := conn.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = NOW() WHERE id = $1", channelID)
			return err
		})
	}
	shutdownIngest := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), deadline)
		_ = controller.ShutdownStream(shutdownCtx, channelID, sessionID, append([]string{}, session.IngestJobIDs...))
		cancel()
		revertChannel()
	}

	persistErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin persist stream session: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if _, err := tx.Exec(ctx, "INSERT INTO stream_sessions (id, channel_id, started_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids) VALUES ($1, $2, $3, $4, 0, $5, $6, $7, $8)",
			session.ID,
			session.ChannelID,
			session.StartedAt,
			session.Renditions,
			session.OriginURL,
			session.PlaybackURL,
			session.IngestEndpoints,
			session.IngestJobIDs,
		); err != nil {
			return fmt.Errorf("insert stream session: %w", err)
		}
		for _, manifest := range session.RenditionManifests {
			if _, err := tx.Exec(ctx, "INSERT INTO stream_session_manifests (session_id, name, manifest_url, bitrate) VALUES ($1, $2, $3, $4)", session.ID, manifest.Name, manifest.ManifestURL, manifest.Bitrate); err != nil {
				return fmt.Errorf("insert rendition manifest: %w", err)
			}
		}
		if _, err := tx.Exec(ctx, "UPDATE channels SET current_session_id = $1, live_state = 'live', updated_at = $2 WHERE id = $3", session.ID, session.StartedAt, channelID); err != nil {
			return fmt.Errorf("mark channel live: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit start stream: %w", err)
		}
		return nil
	})
	if persistErr != nil {
		shutdownIngest()
		return domain.StreamSession{}, persistErr
	}

	return session, nil
}

// StopStream executes StopStream.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) StopStream(channelID string, peakConcurrent int) (session domain.StreamSession, err error) {
	if r == nil || r.pool == nil {
		return domain.StreamSession{}, ErrPostgresUnavailable
	}

	var (
		channelTitle         string
		channelCategory      pgtype.Text
		channelTags          []string
		channelWasLive       bool
		cleanupAfterShutdown bool
		stopTimestamp        time.Time
	)
	defer func() {
		if err == nil || !channelWasLive || !cleanupAfterShutdown {
			return
		}
		timestamp := stopTimestamp
		if timestamp.IsZero() {
			timestamp = time.Now().UTC()
		}
		cleanupErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
			if _, execErr := conn.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = $1 WHERE id = $2", timestamp, channelID); execErr != nil {
				return fmt.Errorf("update channel %s: %w", channelID, execErr)
			}
			return nil
		})
		if cleanupErr != nil {
			err = fmt.Errorf("%w; cleanup stop stream: %v", err, cleanupErr)
		}
	}()

	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin stop stream tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var (
			streamKey       string
			currentSession  pgtype.Text
			renditions      []string
			ingestEndpoints []string
			ingestJobIDs    []string
			peak            int
			startedAt       time.Time
			endedAt         pgtype.Timestamptz
			originURL       string
			playbackURL     string
		)
		row := tx.QueryRow(ctx, "SELECT stream_key, current_session_id, title, category, tags FROM channels WHERE id = $1 FOR UPDATE", channelID)
		if err := row.Scan(&streamKey, &currentSession, &channelTitle, &channelCategory, &channelTags); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("channel %s not found", channelID)
			}
			return fmt.Errorf("load channel %s: %w", channelID, err)
		}
		if !currentSession.Valid {
			return errors.New("channel is not live")
		}
		channelWasLive = true
		sessionID := currentSession.String

		sessRow := tx.QueryRow(ctx, "SELECT started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids FROM stream_sessions WHERE id = $1 FOR UPDATE", sessionID)
		if err := sessRow.Scan(&startedAt, &endedAt, &renditions, &peak, &originURL, &playbackURL, &ingestEndpoints, &ingestJobIDs); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("session %s missing", sessionID)
			}
			return fmt.Errorf("load session %s: %w", sessionID, err)
		}
		manifestsRows, err := tx.Query(ctx, "SELECT name, manifest_url, bitrate FROM stream_session_manifests WHERE session_id = $1", sessionID)
		if err != nil {
			return fmt.Errorf("load session manifests: %w", err)
		}
		manifests := make([]domain.RenditionManifest, 0)
		for manifestsRows.Next() {
			var name, url string
			var bitrate pgtype.Int4
			if err := manifestsRows.Scan(&name, &url, &bitrate); err != nil {
				manifestsRows.Close()
				return fmt.Errorf("scan session manifest: %w", err)
			}
			entry := domain.RenditionManifest{Name: name, ManifestURL: url}
			if bitrate.Valid {
				entry.Bitrate = int(bitrate.Int32)
			}
			manifests = append(manifests, entry)
		}
		manifestsRows.Close()
		if err := manifestsRows.Err(); err != nil {
			return fmt.Errorf("read session manifests: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit load session: %w", err)
		}

		session = domain.StreamSession{
			ID:                 sessionID,
			ChannelID:          channelID,
			StartedAt:          startedAt.UTC(),
			Renditions:         append([]string{}, renditions...),
			PeakConcurrent:     peak,
			OriginURL:          originURL,
			PlaybackURL:        playbackURL,
			IngestEndpoints:    append([]string{}, ingestEndpoints...),
			IngestJobIDs:       append([]string{}, ingestJobIDs...),
			RenditionManifests: append([]domain.RenditionManifest{}, manifests...),
		}
		if endedAt.Valid {
			ts := endedAt.Time.UTC()
			session.EndedAt = &ts
		}
		return nil
	})
	if err != nil {
		return domain.StreamSession{}, err
	}

	deadline := normalizeIngestTimeout(r.ingestTimeout)
	controller := r.ingestController
	if controller == nil {
		return domain.StreamSession{}, ErrIngestControllerUnavailable
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	if err := controller.ShutdownStream(shutdownCtx, channelID, session.ID, append([]string{}, session.IngestJobIDs...)); err != nil {
		return domain.StreamSession{}, fmt.Errorf("shutdown ingest: %w", err)
	}
	cleanupAfterShutdown = true

	stopTimestamp = time.Now().UTC()
	session.EndedAt = &stopTimestamp
	if peakConcurrent > session.PeakConcurrent {
		session.PeakConcurrent = peakConcurrent
	}

	channel := domain.Channel{ID: channelID, Title: channelTitle}
	if channelCategory.Valid {
		channel.Category = channelCategory.String
	}
	if len(channelTags) > 0 {
		channel.Tags = append([]string{}, channelTags...)
	}

	recording, recErr := r.createRecording(session, channel, stopTimestamp)
	if recErr != nil {
		return domain.StreamSession{}, recErr
	}

	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin finalize stop stream tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if _, err := tx.Exec(ctx, "UPDATE stream_sessions SET ended_at = $1, peak_concurrent = $2 WHERE id = $3", session.EndedAt, session.PeakConcurrent, session.ID); err != nil {
			return fmt.Errorf("update stream session %s: %w", session.ID, err)
		}
		if _, err := tx.Exec(ctx, "UPDATE channels SET current_session_id = NULL, live_state = 'offline', updated_at = $1 WHERE id = $2", stopTimestamp, channelID); err != nil {
			return fmt.Errorf("update channel %s: %w", channelID, err)
		}
		if recording.ID != "" {
			if err := r.insertRecording(ctx, tx, recording); err != nil {
				return err
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit stop stream: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.StreamSession{}, err
	}

	return session, nil
}

// CurrentStreamSession executes CurrentStreamSession.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) CurrentStreamSession(channelID string) (domain.StreamSession, bool) {
	if r == nil || r.pool == nil {
		return domain.StreamSession{}, false
	}
	var current pgtype.Text
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		return conn.QueryRow(ctx, "SELECT current_session_id FROM channels WHERE id = $1", channelID).Scan(&current)
	}); err != nil {
		return domain.StreamSession{}, false
	}
	if !current.Valid {
		return domain.StreamSession{}, false
	}
	loadCtx, cancel := r.acquireContext()
	defer cancel()
	session, ok := r.loadStreamSession(loadCtx, current.String)
	if !ok {
		return domain.StreamSession{}, false
	}
	return session, true
}

// ListStreamSessions executes ListStreamSessions.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListStreamSessions(channelID string) ([]domain.StreamSession, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ids := make([]string, 0)
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}
		rows, err := conn.Query(ctx, "SELECT id FROM stream_sessions WHERE channel_id = $1 ORDER BY started_at DESC", channelID)
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan session id: %w", err)
			}
			ids = append(ids, id)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}

	sessions := make([]domain.StreamSession, 0, len(ids))
	for _, id := range ids {
		loadCtx, cancel := r.acquireContext()
		session, ok := r.loadStreamSession(loadCtx, id)
		cancel()
		if !ok {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// ListRecordings executes ListRecordings.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListRecordings(channelID string, includeUnpublished bool) ([]domain.Recording, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ids := make([]string, 0)
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}
		if err := r.purgeExpiredRecordings(ctx, r.retentionTime()); err != nil {
			slog.Default().Warn("purge expired recordings failed", "channel_id", channelID, "error", err)
		}
		query := "SELECT id FROM recordings WHERE channel_id = $1"
		if !includeUnpublished {
			query += " AND published_at IS NOT NULL"
		}
		query += " ORDER BY created_at DESC"
		rows, err := conn.Query(ctx, query, channelID)
		if err != nil {
			return fmt.Errorf("list recordings: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan recording id: %w", err)
			}
			ids = append(ids, id)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}

	recordings := make([]domain.Recording, 0, len(ids))
	for _, id := range ids {
		loadCtx, cancel := r.acquireContext()
		recording, ok, loadErr := r.loadRecording(loadCtx, id)
		cancel()
		if loadErr != nil {
			return nil, loadErr
		}
		if !ok {
			continue
		}
		recordings = append(recordings, recording)
	}
	return recordings, nil
}

// CreateUpload executes CreateUpload.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) CreateUpload(params CreateUploadParams) (domain.Upload, error) {
	if r == nil || r.pool == nil {
		return domain.Upload{}, ErrPostgresUnavailable
	}
	channelID := strings.TrimSpace(params.ChannelID)
	if channelID == "" {
		return domain.Upload{}, fmt.Errorf("channelId is required")
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
		return domain.Upload{}, fmt.Errorf("encode metadata: %w", err)
	}
	playbackURL := strings.TrimSpace(params.PlaybackURL)

	upload := domain.Upload{}
	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}

		id, err := generateID()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := conn.Exec(ctx, "INSERT INTO uploads (id, channel_id, title, filename, size_bytes, status, progress, playback_url, metadata, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'pending', 0, $6, $7, $8, $9)",
			id,
			channelID,
			title,
			filename,
			params.SizeBytes,
			playbackURL,
			metadataJSON,
			now,
			now,
		); err != nil {
			return fmt.Errorf("insert upload: %w", err)
		}
		upload = domain.Upload{
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
		return nil
	})
	if err != nil {
		return domain.Upload{}, err
	}
	return upload, nil
}

// ListUploads executes ListUploads.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListUploads(channelID string) ([]domain.Upload, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	ids := make([]string, 0)
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}
		rows, err := conn.Query(ctx, "SELECT id FROM uploads WHERE channel_id = $1 ORDER BY created_at DESC", channelID)
		if err != nil {
			return fmt.Errorf("list uploads: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("scan upload id: %w", err)
			}
			ids = append(ids, id)
		}
		return rows.Err()
	}); err != nil {
		return nil, err
	}

	uploads := make([]domain.Upload, 0, len(ids))
	for _, id := range ids {
		loadCtx, cancel := r.acquireContext()
		upload, ok, loadErr := r.loadUpload(loadCtx, id)
		cancel()
		if loadErr != nil {
			return nil, loadErr
		}
		if !ok {
			continue
		}
		uploads = append(uploads, upload)
	}
	return uploads, nil
}

// GetUpload executes GetUpload.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) GetUpload(id string) (domain.Upload, bool) {
	if r == nil || r.pool == nil {
		return domain.Upload{}, false
	}
	ctx, cancel := r.acquireContext()
	upload, ok, err := r.loadUpload(ctx, id)
	cancel()
	if err != nil || !ok {
		return domain.Upload{}, false
	}
	return upload, true
}

// EnsureUploadRecording creates or reuses a recording for a completed upload.
func (r *postgresRepository) EnsureUploadRecording(id, playbackURL string, completedAt time.Time) (domain.Recording, error) {
	if r == nil || r.pool == nil {
		return domain.Recording{}, ErrPostgresUnavailable
	}
	var result domain.Recording
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin ensure upload recording tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		upload, ok, err := r.loadUpload(ctx, id)
		if err != nil {
			return fmt.Errorf("load upload %s: %w", id, err)
		}
		if !ok {
			return fmt.Errorf("upload %s not found", id)
		}
		if upload.RecordingID != nil {
			recordingID := strings.TrimSpace(*upload.RecordingID)
			if recordingID != "" {
				recording, ok, loadErr := r.loadRecording(ctx, recordingID)
				if loadErr != nil {
					return fmt.Errorf("load recording %s: %w", recordingID, loadErr)
				}
				if ok {
					result = recording
					if err := tx.Commit(ctx); err != nil {
						return fmt.Errorf("commit ensure upload recording: %w", err)
					}
					return nil
				}
			}
		}
		var existingRecordingID pgtype.Text
		if err := tx.QueryRow(ctx, "SELECT id FROM recordings WHERE metadata ->> 'uploadId' = $1 ORDER BY created_at DESC LIMIT 1", upload.ID).Scan(&existingRecordingID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("query existing upload recording: %w", err)
		}
		if existingRecordingID.Valid {
			recording, ok, loadErr := r.loadRecording(ctx, strings.TrimSpace(existingRecordingID.String))
			if loadErr != nil {
				return fmt.Errorf("load existing upload recording %s: %w", existingRecordingID.String, loadErr)
			}
			if ok {
				result = recording
				if err := tx.Commit(ctx); err != nil {
					return fmt.Errorf("commit ensure upload recording: %w", err)
				}
				return nil
			}
		}

		now := completedAt.UTC()
		if now.IsZero() {
			now = time.Now().UTC()
		}
		title := strings.TrimSpace(upload.Title)
		if title == "" {
			title = strings.TrimSpace(upload.Filename)
		}
		if title == "" {
			title = fmt.Sprintf("Upload %s", upload.ID)
		}
		recordingID, err := generateID()
		if err != nil {
			return err
		}
		uploadSessionID := "upload-" + upload.ID
		if _, err := tx.Exec(ctx, "INSERT INTO stream_sessions (id, channel_id, started_at, ended_at, renditions, peak_concurrent, origin_url, playback_url, ingest_endpoints, ingest_job_ids) VALUES ($1, $2, $3, $4, ARRAY[]::TEXT[], 0, '', $5, ARRAY[]::TEXT[], ARRAY[]::TEXT[]) ON CONFLICT (id) DO NOTHING",
			uploadSessionID,
			upload.ChannelID,
			now,
			now,
			strings.TrimSpace(playbackURL),
		); err != nil {
			return fmt.Errorf("ensure upload session: %w", err)
		}
		metadata := map[string]string{
			"channelId": upload.ChannelID,
			"uploadId":  upload.ID,
			"source":    "upload",
		}
		metadataJSON, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("encode recording metadata: %w", err)
		}

		var retainUntil any
		if deadline := r.recordingDeadline(now, false); deadline != nil {
			retainUntil = deadline
		}
		if _, err := tx.Exec(ctx, "INSERT INTO recordings (id, channel_id, session_id, title, duration_seconds, playback_base_url, metadata, created_at, retain_until) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			recordingID,
			upload.ChannelID,
			uploadSessionID,
			title,
			0,
			strings.TrimSpace(playbackURL),
			metadataJSON,
			now,
			retainUntil,
		); err != nil {
			return fmt.Errorf("insert upload recording: %w", err)
		}
		result = domain.Recording{
			ID:              recordingID,
			ChannelID:       upload.ChannelID,
			SessionID:       uploadSessionID,
			Title:           title,
			DurationSeconds: 0,
			PlaybackBaseURL: strings.TrimSpace(playbackURL),
			Metadata:        metadata,
			CreatedAt:       now,
		}
		if deadline, ok := retainUntil.(*time.Time); ok && deadline != nil {
			result.RetainUntil = deadline
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit ensure upload recording: %w", err)
		}
		return nil
	})
	if err != nil {
		return domain.Recording{}, err
	}
	return result, nil
}

// UpdateUpload executes UpdateUpload.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) UpdateUpload(id string, update UploadUpdate) (domain.Upload, error) {
	if r == nil || r.pool == nil {
		return domain.Upload{}, ErrPostgresUnavailable
	}
	var result domain.Upload
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin update upload tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		upload, ok, err := r.loadUpload(ctx, id)
		if err != nil {
			return fmt.Errorf("load upload %s: %w", id, err)
		}
		if !ok {
			return fmt.Errorf("upload %s not found", id)
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
			return fmt.Errorf("encode metadata: %w", err)
		}
		var recordingID interface{}
		if upload.RecordingID != nil {
			recordingID = *upload.RecordingID
		}
		var completedAt interface{}
		if upload.CompletedAt != nil {
			completedAt = *upload.CompletedAt
		}
		if _, err := tx.Exec(ctx, "UPDATE uploads SET title = $1, status = $2, progress = $3, recording_id = $4, playback_url = $5, metadata = $6, error = $7, completed_at = $8, updated_at = $9 WHERE id = $10",
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
		); err != nil {
			return fmt.Errorf("update upload %s: %w", id, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit update upload: %w", err)
		}
		result = upload
		return nil
	})
	if err != nil {
		return domain.Upload{}, err
	}
	return result, nil
}

// DeleteUpload executes DeleteUpload.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) DeleteUpload(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx, cancel := r.acquireContext()
	command, err := r.pool.Exec(ctx, "DELETE FROM uploads WHERE id = $1", id)
	cancel()
	if err != nil {
		return fmt.Errorf("delete upload %s: %w", id, err)
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("upload %s not found", id)
	}
	return nil
}

// GetRecording executes GetRecording.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) GetRecording(id string) (domain.Recording, bool) {
	if r == nil || r.pool == nil {
		return domain.Recording{}, false
	}
	ctx, cancel := r.acquireContext()
	if err := r.purgeExpiredRecordings(ctx, r.retentionTime()); err != nil {
		slog.Default().Warn("purge expired recordings failed", "recording_id", id, "error", err)
	}
	recording, ok, err := r.loadRecording(ctx, id)
	cancel()
	if err != nil || !ok {
		return domain.Recording{}, false
	}
	return recording, true
}

// PublishRecording executes PublishRecording.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) PublishRecording(id string) (domain.Recording, error) {
	if r == nil || r.pool == nil {
		return domain.Recording{}, ErrPostgresUnavailable
	}

	var recording domain.Recording
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin publish recording tx: %w", err)
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
			return fmt.Errorf("recording %s not found", id)
		}
		if err != nil {
			return fmt.Errorf("load recording %s: %w", id, err)
		}
		if publishedAt.Valid {
			rec, _, loadErr := r.loadRecording(ctx, id)
			if loadErr != nil {
				return loadErr
			}
			recording = rec
			return nil
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx, "UPDATE recordings SET published_at = $1 WHERE id = $2", now, id); err != nil {
			return fmt.Errorf("publish recording %s: %w", id, err)
		}
		if deadline := r.recordingDeadline(now, true); deadline != nil {
			if _, err := tx.Exec(ctx, "UPDATE recordings SET retain_until = $1 WHERE id = $2", deadline, id); err != nil {
				return fmt.Errorf("update recording retention: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit publish recording: %w", err)
		}
		rec, _, loadErr := r.loadRecording(ctx, id)
		if loadErr != nil {
			return loadErr
		}
		if rec.ID == "" {
			return fmt.Errorf("recording %s not found", id)
		}
		recording = rec
		return nil
	})
	if err != nil {
		return domain.Recording{}, err
	}
	return recording, nil
}

// DeleteRecording executes DeleteRecording.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) DeleteRecording(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	ctx, cancel := r.acquireContext()
	recording, ok, err := r.loadRecording(ctx, id)
	if err != nil {
		cancel()
		return err
	}
	if !ok {
		cancel()
		return fmt.Errorf("recording %s not found", id)
	}
	if err := r.deleteRecordingArtifacts(recording); err != nil {
		cancel()
		return err
	}
	clipRows, err := r.pool.Query(ctx, "SELECT id, storage_object FROM clip_exports WHERE recording_id = $1", id)
	if err != nil {
		cancel()
		return fmt.Errorf("load clip exports: %w", err)
	}
	clips := make([]domain.ClipExport, 0)
	for clipRows.Next() {
		var clip domain.ClipExport
		var storageObject pgtype.Text
		if err := clipRows.Scan(&clip.ID, &storageObject); err != nil {
			clipRows.Close()
			return fmt.Errorf("scan clip export: %w", err)
		}
		if storageObject.Valid {
			clip.StorageObject = storageObject.String
		}
		clips = append(clips, clip)
	}
	clipRows.Close()
	for _, clip := range clips {
		if err := r.deleteClipArtifacts(clip); err != nil {
			cancel()
			return err
		}
	}
	_, err = r.pool.Exec(ctx, "DELETE FROM recordings WHERE id = $1", id)
	cancel()
	if err != nil {
		return fmt.Errorf("delete recording %s: %w", id, err)
	}
	return nil
}

// CreateClipExport executes CreateClipExport.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) CreateClipExport(recordingID string, params ClipExportParams) (domain.ClipExport, error) {
	if r == nil || r.pool == nil {
		return domain.ClipExport{}, ErrPostgresUnavailable
	}
	if strings.TrimSpace(recordingID) == "" {
		return domain.ClipExport{}, fmt.Errorf("recording id is required")
	}
	title := strings.TrimSpace(params.Title)
	if title == "" {
		return domain.ClipExport{}, fmt.Errorf("title is required")
	}
	clip := domain.ClipExport{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var (
			channelID string
			sessionID string
			duration  int
		)
		if err := conn.QueryRow(ctx, "SELECT channel_id, session_id, duration_seconds FROM recordings WHERE id = $1", recordingID).
			Scan(&channelID, &sessionID, &duration); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("recording %s not found", recordingID)
			}
			return fmt.Errorf("load recording %s: %w", recordingID, err)
		}
		if params.EndSeconds <= params.StartSeconds {
			return fmt.Errorf("endSeconds must be greater than startSeconds")
		}
		if params.StartSeconds < 0 {
			return fmt.Errorf("startSeconds must be non-negative")
		}
		if duration > 0 && params.EndSeconds > duration {
			return fmt.Errorf("clip exceeds recording duration")
		}
		id, err := generateID()
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		newClip := domain.ClipExport{
			ID:           id,
			RecordingID:  recordingID,
			ChannelID:    channelID,
			SessionID:    sessionID,
			Title:        title,
			StartSeconds: params.StartSeconds,
			EndSeconds:   params.EndSeconds,
			Status:       "pending",
			CreatedAt:    now,
		}
		if _, err := conn.Exec(ctx, "INSERT INTO clip_exports (id, recording_id, channel_id, session_id, title, start_seconds, end_seconds, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			newClip.ID,
			newClip.RecordingID,
			newClip.ChannelID,
			newClip.SessionID,
			newClip.Title,
			newClip.StartSeconds,
			newClip.EndSeconds,
			newClip.Status,
			newClip.CreatedAt,
		); err != nil {
			return fmt.Errorf("insert clip export: %w", err)
		}
		clip = newClip
		return nil
	})
	if err != nil {
		return domain.ClipExport{}, err
	}
	return clip, nil
}

// ListClipExports executes ListClipExports.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListClipExports(recordingID string) ([]domain.ClipExport, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	if strings.TrimSpace(recordingID) == "" {
		return nil, fmt.Errorf("recording id is required")
	}
	clips := make([]domain.ClipExport, 0)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM recordings WHERE id = $1)", recordingID).Scan(&exists); err != nil {
			return fmt.Errorf("check recording %s: %w", recordingID, err)
		}
		if !exists {
			return fmt.Errorf("recording %s not found", recordingID)
		}
		rows, err := conn.Query(ctx, "SELECT id, recording_id, channel_id, session_id, title, start_seconds, end_seconds, status, playback_url, created_at, completed_at, storage_object FROM clip_exports WHERE recording_id = $1 ORDER BY created_at DESC", recordingID)
		if err != nil {
			return fmt.Errorf("list clip exports: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var clip domain.ClipExport
			var completedAt pgtype.Timestamptz
			var playbackURL pgtype.Text
			var storageObject pgtype.Text
			if err := rows.Scan(&clip.ID, &clip.RecordingID, &clip.ChannelID, &clip.SessionID, &clip.Title, &clip.StartSeconds, &clip.EndSeconds, &clip.Status, &playbackURL, &clip.CreatedAt, &completedAt, &storageObject); err != nil {
				return fmt.Errorf("scan clip export: %w", err)
			}
			if completedAt.Valid {
				ts := completedAt.Time.UTC()
				clip.CompletedAt = &ts
			}
			if playbackURL.Valid {
				clip.PlaybackURL = playbackURL.String
			}
			if storageObject.Valid {
				clip.StorageObject = storageObject.String
			}
			clips = append(clips, clip)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return clips, nil
}
