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
		default:
			return fmt.Errorf("unsupported chat event %q", evt.Type)
		}
	})
}

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
