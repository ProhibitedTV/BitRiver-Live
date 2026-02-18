package storage

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"bitriver-live/internal/chat"
	"bitriver-live/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateChatMessage executes CreateChatMessage.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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

	id, err := generateID()
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

// DeleteChatMessage executes DeleteChatMessage.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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

// ListChatMessages executes ListChatMessages.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
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

	if err := r.purgeExpiredChatMessages(ctx, r.retentionTime()); err != nil {
		return nil, fmt.Errorf("purge chat messages: %w", err)
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

// purgeExpiredChatMessages executes purgeExpiredChatMessages.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) purgeExpiredChatMessages(ctx context.Context, now time.Time) error {
	retention := r.chatRetention.Messages
	if retention <= 0 || r == nil || r.pool == nil {
		return nil
	}
	cutoff := now.Add(-retention)
	if _, err := r.pool.Exec(ctx, "DELETE FROM chat_messages WHERE created_at <= $1", cutoff); err != nil {
		return err
	}
	return nil
}

// ChatRestrictions executes ChatRestrictions.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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

// IsChatBanned executes IsChatBanned.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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

// ChatTimeout executes ChatTimeout.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this signature does not return `error`; not-found/absence is represented by the
// boolean return value.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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

// ApplyChatEvent executes ApplyChatEvent.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) ApplyChatEvent(evt chat.Event) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}

	return r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		switch evt.Type {
		case chat.EventTypeMessage:
			if evt.Message == nil {
				return fmt.Errorf("message payload missing")
			}
			msg := evt.Message
			if msg.ID == "" || msg.ChannelID == "" || msg.UserID == "" {
				return fmt.Errorf("invalid message event")
			}
			if _, err := conn.Exec(ctx, "INSERT INTO chat_messages (id, channel_id, user_id, content, created_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (id) DO UPDATE SET channel_id = EXCLUDED.channel_id, user_id = EXCLUDED.user_id, content = EXCLUDED.content, created_at = EXCLUDED.created_at", msg.ID, msg.ChannelID, msg.UserID, msg.Content, msg.CreatedAt.UTC()); err != nil {
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
				if _, err := conn.Exec(ctx, "INSERT INTO chat_bans (channel_id, user_id, actor_id, reason, issued_at) VALUES ($1, $2, $3, $4, $5) ON CONFLICT (channel_id, user_id) DO UPDATE SET actor_id = EXCLUDED.actor_id, reason = EXCLUDED.reason, issued_at = EXCLUDED.issued_at", mod.ChannelID, mod.TargetID, actorParam, reason, issued); err != nil {
					return fmt.Errorf("apply ban event: %w", err)
				}
				return nil
			case chat.ModerationActionUnban:
				if _, err := conn.Exec(ctx, "DELETE FROM chat_bans WHERE channel_id = $1 AND user_id = $2", mod.ChannelID, mod.TargetID); err != nil {
					return fmt.Errorf("apply unban event: %w", err)
				}
				return nil
			case chat.ModerationActionTimeout:
				if mod.ExpiresAt == nil {
					return nil
				}
				expires := mod.ExpiresAt.UTC()
				if _, err := conn.Exec(ctx, "INSERT INTO chat_timeouts (channel_id, user_id, actor_id, reason, issued_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT (channel_id, user_id) DO UPDATE SET actor_id = EXCLUDED.actor_id, reason = EXCLUDED.reason, issued_at = EXCLUDED.issued_at, expires_at = EXCLUDED.expires_at", mod.ChannelID, mod.TargetID, actorParam, reason, issued, expires); err != nil {
					return fmt.Errorf("apply timeout event: %w", err)
				}
				return nil
			case chat.ModerationActionRemoveTimeout:
				if _, err := conn.Exec(ctx, "DELETE FROM chat_timeouts WHERE channel_id = $1 AND user_id = $2", mod.ChannelID, mod.TargetID); err != nil {
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
			if _, err := conn.Exec(ctx, "INSERT INTO chat_reports (id, channel_id, reporter_id, target_id, reason, message_id, evidence_url, status, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO UPDATE SET channel_id = EXCLUDED.channel_id, reporter_id = EXCLUDED.reporter_id, target_id = EXCLUDED.target_id, reason = EXCLUDED.reason, message_id = EXCLUDED.message_id, evidence_url = EXCLUDED.evidence_url, status = EXCLUDED.status, created_at = EXCLUDED.created_at", rep.ID, rep.ChannelID, rep.ReporterID, rep.TargetID, rep.Reason, messageParam, evidenceParam, status, rep.CreatedAt.UTC()); err != nil {
				return fmt.Errorf("apply report event: %w", err)
			}
			return nil
		case chat.EventTypeAutoMod:
			if evt.AutoMod == nil {
				return fmt.Errorf("automod payload missing")
			}
			action := evt.AutoMod
			if strings.TrimSpace(action.ID) == "" {
				return fmt.Errorf("automod action id missing")
			}
			var filterID any
			if strings.TrimSpace(action.FilterID) != "" {
				filterID = strings.TrimSpace(action.FilterID)
			}
			actionName := strings.TrimSpace(action.Action)
			if actionName == "" {
				actionName = "blocked"
			}
			created := action.CreatedAt.UTC()
			if created.IsZero() {
				created = time.Now().UTC()
			}
			if _, err := conn.Exec(ctx, "INSERT INTO chat_automod_actions (id, channel_id, user_id, filter_id, filter_kind, filter_pattern, message, action, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ON CONFLICT (id) DO UPDATE SET channel_id = EXCLUDED.channel_id, user_id = EXCLUDED.user_id, filter_id = EXCLUDED.filter_id, filter_kind = EXCLUDED.filter_kind, filter_pattern = EXCLUDED.filter_pattern, message = EXCLUDED.message, action = EXCLUDED.action, created_at = EXCLUDED.created_at", strings.TrimSpace(action.ID), strings.TrimSpace(action.ChannelID), strings.TrimSpace(action.UserID), filterID, strings.TrimSpace(action.FilterKind), strings.TrimSpace(action.FilterPattern), action.Message, actionName, created); err != nil {
				return fmt.Errorf("apply automod event: %w", err)
			}
			return nil
		default:
			return fmt.Errorf("unsupported chat event %q", evt.Type)
		}
	})
}

// ListChatRestrictions executes ListChatRestrictions.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: this helper does not return `error`; failures are handled by callers.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListChatRestrictions(channelID string) []models.ChatRestriction {
	if r == nil || r.pool == nil {
		return nil
	}
	restrictions := make([]models.ChatRestriction, 0)
	aborted := false
	now := time.Now().UTC()
	if err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		banRows, err := conn.Query(ctx, "SELECT user_id, actor_id, reason, issued_at FROM chat_bans WHERE channel_id = $1", channelID)
		if err == nil {
			defer banRows.Close()
			for banRows.Next() {
				var (
					userID string
					actor  pgtype.Text
					reason string
					issued time.Time
				)
				if err := banRows.Scan(&userID, &actor, &reason, &issued); err != nil {
					aborted = true
					return nil
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
				aborted = true
				return nil
			}
		}

		if _, err := conn.Exec(ctx, "DELETE FROM chat_timeouts WHERE channel_id = $1 AND expires_at <= $2", channelID, now); err != nil {
			return nil
		}

		timeoutRows, err := conn.Query(ctx, "SELECT user_id, actor_id, reason, issued_at, expires_at FROM chat_timeouts WHERE channel_id = $1 AND expires_at > $2", channelID, now)
		if err != nil {
			return nil
		}
		defer timeoutRows.Close()
		for timeoutRows.Next() {
			var (
				userID  string
				actor   pgtype.Text
				reason  string
				issued  time.Time
				expires time.Time
			)
			if err := timeoutRows.Scan(&userID, &actor, &reason, &issued, &expires); err != nil {
				aborted = true
				return nil
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
		if err := timeoutRows.Err(); err != nil {
			aborted = true
			return nil
		}
		return nil
	}); err != nil {
		return nil
	}
	if aborted {
		return restrictions
	}
	sort.Slice(restrictions, func(i, j int) bool {
		if restrictions[i].IssuedAt.Equal(restrictions[j].IssuedAt) {
			return restrictions[i].ID < restrictions[j].ID
		}
		return restrictions[i].IssuedAt.After(restrictions[j].IssuedAt)
	})
	return restrictions
}

// CreateChatReport executes CreateChatReport.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) CreateChatReport(channelID, reporterID, targetID, reason, messageID, evidenceURL string) (models.ChatReport, error) {
	if r == nil || r.pool == nil {
		return models.ChatReport{}, ErrPostgresUnavailable
	}

	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return models.ChatReport{}, fmt.Errorf("reason is required")
	}

	id, err := generateID()
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
			var messageExists bool
			if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM chat_messages WHERE id = $1 AND channel_id = $2)", trimmedMessageID, channelID).Scan(&messageExists); err != nil {
				return fmt.Errorf("check chat message %s: %w", trimmedMessageID, err)
			}
			if messageExists {
				messageParam = trimmedMessageID
			}
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
			EvidenceURL: trimmedEvidence,
			Status:      status,
			CreatedAt:   now,
		}
		if messageParam != nil {
			report.MessageID = trimmedMessageID
		}
		return nil
	})
	if createErr != nil {
		return models.ChatReport{}, createErr
	}
	return report, nil
}

// ListChatReports executes ListChatReports.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListChatReports(channelID string, includeResolved bool) ([]models.ChatReport, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	reports := make([]models.ChatReport, 0)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}

		if err := r.purgeExpiredChatReports(ctx, r.retentionTime()); err != nil {
			return fmt.Errorf("purge chat reports: %w", err)
		}

		query := "SELECT id, channel_id, reporter_id, target_id, reason, message_id, evidence_url, status, resolution, resolver_id, created_at, resolved_at FROM chat_reports WHERE channel_id = $1"
		args := []any{channelID}
		if !includeResolved {
			query += " AND LOWER(status) <> 'resolved'"
		}
		query += " ORDER BY created_at DESC, id ASC"

		rows, err := conn.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list chat reports: %w", err)
		}
		defer rows.Close()

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
				return fmt.Errorf("scan chat report: %w", err)
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
			return fmt.Errorf("iterate chat reports: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reports, nil
}

// purgeExpiredChatReports executes purgeExpiredChatReports.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) purgeExpiredChatReports(ctx context.Context, now time.Time) error {
	retention := r.chatRetention.ModerationLogs
	if retention <= 0 || r == nil || r.pool == nil {
		return nil
	}
	cutoff := now.Add(-retention)
	if _, err := r.pool.Exec(ctx, "DELETE FROM chat_reports WHERE COALESCE(resolved_at, created_at) <= $1", cutoff); err != nil {
		return err
	}
	return nil
}

// ResolveChatReport executes ResolveChatReport.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
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

// CreateAppeal executes CreateAppeal.
func (r *postgresRepository) CreateAppeal(reportID, reporterID, reason string) (models.Appeal, error) {
	if r == nil || r.pool == nil {
		return models.Appeal{}, ErrPostgresUnavailable
	}
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return models.Appeal{}, fmt.Errorf("reason is required")
	}
	appealID, err := generateID()
	if err != nil {
		return models.Appeal{}, err
	}
	eventID, err := generateID()
	if err != nil {
		return models.Appeal{}, err
	}
	appeal := models.Appeal{}
	now := time.Now().UTC()
	err = r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create appeal tx: %w", err)
		}
		defer rollbackTx(ctx, tx)
		var channelID, reportReporter string
		if err := tx.QueryRow(ctx, "SELECT channel_id, reporter_id FROM chat_reports WHERE id = $1", reportID).Scan(&channelID, &reportReporter); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("report %s not found", reportID)
			}
			return fmt.Errorf("load report %s: %w", reportID, err)
		}
		if reportReporter != reporterID {
			return fmt.Errorf("forbidden")
		}
		if err := ensureUserExists(ctx, tx, reporterID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "INSERT INTO appeals (id, report_id, channel_id, reporter_id, reason, status, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)", appealID, reportID, channelID, reporterID, trimmedReason, AppealStatusOpen, now); err != nil {
			return fmt.Errorf("insert appeal: %w", err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO appeal_events (id, appeal_id, actor_id, action, note, created_at) VALUES ($1,$2,$3,$4,$5,$6)", eventID, appealID, reporterID, "submitted", trimmedReason, now); err != nil {
			return fmt.Errorf("insert appeal event: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create appeal: %w", err)
		}
		appeal = models.Appeal{ID: appealID, ReportID: reportID, ChannelID: channelID, ReporterID: reporterID, Reason: trimmedReason, Status: AppealStatusOpen, CreatedAt: now,
			Events: []models.AppealEvent{{ID: eventID, AppealID: appealID, ActorID: reporterID, Action: "submitted", Note: trimmedReason, CreatedAt: now}}}
		return nil
	})
	if err != nil {
		return models.Appeal{}, err
	}
	return appeal, nil
}

// ListAppeals executes ListAppeals.
func (r *postgresRepository) ListAppeals(channelID, requesterID string, includeClosed bool) ([]models.Appeal, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}
	appeals := make([]models.Appeal, 0)
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		var exists bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM channels WHERE id = $1)", channelID).Scan(&exists); err != nil {
			return fmt.Errorf("check channel %s: %w", channelID, err)
		}
		if !exists {
			return fmt.Errorf("channel %s not found", channelID)
		}
		query := "SELECT id, report_id, channel_id, reporter_id, reason, status, resolution, resolver_id, created_at, resolved_at FROM appeals WHERE channel_id = $1"
		args := []any{channelID}
		if strings.TrimSpace(requesterID) != "" {
			query += " AND reporter_id = $2"
			args = append(args, requesterID)
		}
		if !includeClosed {
			query += " AND LOWER(status) <> 'resolved'"
		}
		query += " ORDER BY created_at DESC, id ASC"
		rows, err := conn.Query(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("list appeals: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var a models.Appeal
			var resolution, resolver pgtype.Text
			var resolved pgtype.Timestamptz
			if err := rows.Scan(&a.ID, &a.ReportID, &a.ChannelID, &a.ReporterID, &a.Reason, &a.Status, &resolution, &resolver, &a.CreatedAt, &resolved); err != nil {
				return fmt.Errorf("scan appeal: %w", err)
			}
			if resolution.Valid {
				a.Resolution = resolution.String
			}
			if resolver.Valid {
				a.ResolverID = resolver.String
			}
			if resolved.Valid {
				ts := resolved.Time.UTC()
				a.ResolvedAt = &ts
			}
			a.CreatedAt = a.CreatedAt.UTC()
			a.Status = strings.ToLower(a.Status)
			events, err := r.listAppealEventsTx(ctx, conn, a.ID)
			if err != nil {
				return err
			}
			a.Events = events
			appeals = append(appeals, a)
		}
		return rows.Err()
	})
	return appeals, err
}

func (r *postgresRepository) listAppealEventsTx(ctx context.Context, conn *pgxpool.Conn, appealID string) ([]models.AppealEvent, error) {
	rows, err := conn.Query(ctx, "SELECT id, appeal_id, actor_id, action, note, created_at FROM appeal_events WHERE appeal_id = $1 ORDER BY created_at ASC, id ASC", appealID)
	if err != nil {
		return nil, fmt.Errorf("list appeal events: %w", err)
	}
	defer rows.Close()
	events := make([]models.AppealEvent, 0)
	for rows.Next() {
		var e models.AppealEvent
		var note pgtype.Text
		if err := rows.Scan(&e.ID, &e.AppealID, &e.ActorID, &e.Action, &note, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan appeal event: %w", err)
		}
		if note.Valid {
			e.Note = note.String
		}
		e.CreatedAt = e.CreatedAt.UTC()
		events = append(events, e)
	}
	return events, rows.Err()
}

// ResolveAppeal executes ResolveAppeal.
func (r *postgresRepository) ResolveAppeal(appealID, resolverID, resolution string) (models.Appeal, error) {
	if r == nil || r.pool == nil {
		return models.Appeal{}, ErrPostgresUnavailable
	}
	trimmed := strings.TrimSpace(resolution)
	if trimmed == "" {
		trimmed = AppealStatusResolved
	}
	return r.updateAppealStatus(appealID, resolverID, AppealStatusResolved, trimmed, "resolved")
}

// ReopenAppeal executes ReopenAppeal.
func (r *postgresRepository) ReopenAppeal(appealID, actorID, note string) (models.Appeal, error) {
	return r.updateAppealStatus(appealID, actorID, AppealStatusOpen, strings.TrimSpace(note), "reopened")
}

func (r *postgresRepository) updateAppealStatus(appealID, actorID, status, note, action string) (models.Appeal, error) {
	if r == nil || r.pool == nil {
		return models.Appeal{}, ErrPostgresUnavailable
	}
	appeal := models.Appeal{}
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin update appeal tx: %w", err)
		}
		defer rollbackTx(ctx, tx)
		if err := ensureUserExists(ctx, tx, actorID); err != nil {
			return err
		}
		now := time.Now().UTC()
		eventID, err := generateID()
		if err != nil {
			return err
		}
		query := "UPDATE appeals SET status=$1, resolution=$2, resolver_id=$3, resolved_at=$4 WHERE id=$5 RETURNING id, report_id, channel_id, reporter_id, reason, status, resolution, resolver_id, created_at, resolved_at"
		var resolutionT, resolver pgtype.Text
		var resolved pgtype.Timestamptz
		var resolvedAt any = now
		var resolverAny any = actorID
		if status == AppealStatusOpen {
			resolvedAt = nil
			resolverAny = nil
		}
		if err := tx.QueryRow(ctx, query, status, note, resolverAny, resolvedAt, appealID).Scan(&appeal.ID, &appeal.ReportID, &appeal.ChannelID, &appeal.ReporterID, &appeal.Reason, &appeal.Status, &resolutionT, &resolver, &appeal.CreatedAt, &resolved); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("appeal %s not found", appealID)
			}
			return fmt.Errorf("update appeal %s: %w", appealID, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO appeal_events (id, appeal_id, actor_id, action, note, created_at) VALUES ($1,$2,$3,$4,$5,$6)", eventID, appealID, actorID, action, note, now); err != nil {
			return fmt.Errorf("insert appeal event: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit update appeal: %w", err)
		}
		if resolutionT.Valid {
			appeal.Resolution = resolutionT.String
		}
		if resolver.Valid {
			appeal.ResolverID = resolver.String
		}
		if resolved.Valid {
			ts := resolved.Time.UTC()
			appeal.ResolvedAt = &ts
		}
		events, err := r.listAppealEventsTx(ctx, conn, appeal.ID)
		if err != nil {
			return err
		}
		appeal.Events = events
		appeal.CreatedAt = appeal.CreatedAt.UTC()
		appeal.Status = strings.ToLower(appeal.Status)
		return nil
	})
	if err != nil {
		return models.Appeal{}, err
	}
	return appeal, nil
}

// ListChatFilters executes ListChatFilters.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListChatFilters(channelID string) ([]models.ChatFilter, error) {
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

	rows, err := r.pool.Query(ctx, "SELECT id, channel_id, kind, pattern, enabled, created_at, updated_at FROM chat_filters WHERE channel_id = $1 ORDER BY created_at DESC, id ASC", channelID)
	if err != nil {
		return nil, fmt.Errorf("list chat filters: %w", err)
	}
	defer rows.Close()

	filters := make([]models.ChatFilter, 0)
	for rows.Next() {
		var filter models.ChatFilter
		var createdAt time.Time
		var updatedAt time.Time
		if err := rows.Scan(&filter.ID, &filter.ChannelID, &filter.Kind, &filter.Pattern, &filter.Enabled, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan chat filter: %w", err)
		}
		filter.CreatedAt = createdAt.UTC()
		filter.UpdatedAt = updatedAt.UTC()
		filters = append(filters, filter)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat filters: %w", err)
	}
	return filters, nil
}

// CreateChatFilter executes CreateChatFilter.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) CreateChatFilter(channelID string, params ChatFilterParams) (models.ChatFilter, error) {
	if r == nil || r.pool == nil {
		return models.ChatFilter{}, ErrPostgresUnavailable
	}
	kind, pattern, err := normalizeChatFilter(params.Kind, params.Pattern)
	if err != nil {
		return models.ChatFilter{}, err
	}
	id, err := generateID()
	if err != nil {
		return models.ChatFilter{}, err
	}
	now := time.Now().UTC()
	filter := models.ChatFilter{}
	saveErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin create chat filter tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, "INSERT INTO chat_filters (id, channel_id, kind, pattern, enabled, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, $6, $7)", id, channelID, kind, pattern, params.Enabled, now, now); err != nil {
			return fmt.Errorf("insert chat filter: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit chat filter: %w", err)
		}

		filter = models.ChatFilter{
			ID:        id,
			ChannelID: channelID,
			Kind:      kind,
			Pattern:   pattern,
			Enabled:   params.Enabled,
			CreatedAt: now,
			UpdatedAt: now,
		}
		return nil
	})
	if saveErr != nil {
		return models.ChatFilter{}, saveErr
	}
	return filter, nil
}

// UpdateChatFilter executes UpdateChatFilter.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) UpdateChatFilter(id string, update ChatFilterUpdate) (models.ChatFilter, error) {
	if r == nil || r.pool == nil {
		return models.ChatFilter{}, ErrPostgresUnavailable
	}
	updated := models.ChatFilter{}
	updateErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin update chat filter tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var existing models.ChatFilter
		var createdAt time.Time
		var updatedAt time.Time
		row := tx.QueryRow(ctx, "SELECT id, channel_id, kind, pattern, enabled, created_at, updated_at FROM chat_filters WHERE id = $1", id)
		if err := row.Scan(&existing.ID, &existing.ChannelID, &existing.Kind, &existing.Pattern, &existing.Enabled, &createdAt, &updatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return fmt.Errorf("filter %s not found", id)
			}
			return fmt.Errorf("lookup chat filter: %w", err)
		}
		existing.CreatedAt = createdAt.UTC()
		existing.UpdatedAt = updatedAt.UTC()

		kind := existing.Kind
		pattern := existing.Pattern
		if update.Kind != nil {
			kind = *update.Kind
		}
		if update.Pattern != nil {
			pattern = *update.Pattern
		}
		if update.Enabled != nil {
			existing.Enabled = *update.Enabled
		}

		normalizedKind, normalizedPattern, err := normalizeChatFilter(kind, pattern)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		updateRow := tx.QueryRow(ctx, "UPDATE chat_filters SET kind = $1, pattern = $2, enabled = $3, updated_at = $4 WHERE id = $5 RETURNING id, channel_id, kind, pattern, enabled, created_at, updated_at", normalizedKind, normalizedPattern, existing.Enabled, now, id)
		if err := updateRow.Scan(&updated.ID, &updated.ChannelID, &updated.Kind, &updated.Pattern, &updated.Enabled, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("update chat filter %s: %w", id, err)
		}
		updated.CreatedAt = createdAt.UTC()
		updated.UpdatedAt = updatedAt.UTC()

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit chat filter update: %w", err)
		}
		return nil
	})
	if updateErr != nil {
		return models.ChatFilter{}, updateErr
	}
	return updated, nil
}

// DeleteChatFilter executes DeleteChatFilter.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) DeleteChatFilter(id string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	deleteErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin delete chat filter tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		var exists bool
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM chat_filters WHERE id = $1)", id).Scan(&exists); err != nil {
			return fmt.Errorf("lookup chat filter %s: %w", id, err)
		}
		if !exists {
			return fmt.Errorf("filter %s not found", id)
		}

		if _, err := tx.Exec(ctx, "DELETE FROM chat_filters WHERE id = $1", id); err != nil {
			return fmt.Errorf("delete chat filter %s: %w", id, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit delete chat filter: %w", err)
		}
		return nil
	})
	return deleteErr
}

// ListChatAutoModActions executes ListChatAutoModActions.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: returns a full result set (no cursor/offset pagination contract).
// Ordering is whatever this implementation explicitly enforces; otherwise it is unspecified.
func (r *postgresRepository) ListChatAutoModActions(channelID string, limit int) ([]models.ChatAutoModAction, error) {
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

	if err := r.purgeExpiredChatAutoModActions(ctx, r.retentionTime()); err != nil {
		return nil, fmt.Errorf("purge chat automod actions: %w", err)
	}

	query := "SELECT id, channel_id, user_id, filter_id, filter_kind, filter_pattern, message, action, created_at FROM chat_automod_actions WHERE channel_id = $1 ORDER BY created_at DESC, id ASC"
	args := []any{channelID}
	if limit > 0 {
		query += " LIMIT $2"
		args = append(args, limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list chat automod actions: %w", err)
	}
	defer rows.Close()

	actions := make([]models.ChatAutoModAction, 0)
	for rows.Next() {
		var action models.ChatAutoModAction
		var filterID pgtype.Text
		var createdAt time.Time
		if err := rows.Scan(&action.ID, &action.ChannelID, &action.UserID, &filterID, &action.FilterKind, &action.FilterPattern, &action.Message, &action.Action, &createdAt); err != nil {
			return nil, fmt.Errorf("scan chat automod action: %w", err)
		}
		if filterID.Valid {
			action.FilterID = filterID.String
		}
		action.CreatedAt = createdAt.UTC()
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat automod actions: %w", err)
	}
	return actions, nil
}

// purgeExpiredChatAutoModActions executes purgeExpiredChatAutoModActions.
// Inputs: callers must prevalidate required IDs, ownership, and user-provided payload shape;
// this function still normalizes/trims where needed and rejects empty required fields.
// Errors: returns validation errors for malformed inputs and wrapped infrastructure errors for
// storage/object backend failures; not-found is returned as an error when applicable.
// Transactions/connections: uses repository-managed PostgreSQL connections/transactions and
// does not mutate caller arguments or perform automatic retries unless explicitly coded below.
// Ordering/pagination: not a list method; no ordering or pagination guarantees apply.
func (r *postgresRepository) purgeExpiredChatAutoModActions(ctx context.Context, now time.Time) error {
	retention := r.chatRetention.ModerationLogs
	if retention <= 0 || r == nil || r.pool == nil {
		return nil
	}
	cutoff := now.Add(-retention)
	if _, err := r.pool.Exec(ctx, "DELETE FROM chat_automod_actions WHERE created_at <= $1", cutoff); err != nil {
		return err
	}
	return nil
}
