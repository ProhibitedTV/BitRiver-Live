package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"bitriver-live/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateTip creates tip and returns an error when persistence or validation fails.
func (r *postgresRepository) CreateTip(params CreateTipParams) (models.Tip, error) {
	if r == nil || r.pool == nil {
		return models.Tip{}, ErrPostgresUnavailable
	}

	amount := params.Amount
	if amount.MinorUnits() <= 0 {
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

	id, err := generateID()
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
		if err := tx.QueryRow(ctx, "INSERT INTO tips (id, channel_id, from_user_id, amount, currency, provider, reference, wallet_address, message, created_at) VALUES ($1, $2, $3, $4::numeric / 100000000::numeric, $5, $6, $7, $8, $9, $10) RETURNING created_at", id, params.ChannelID, params.FromUserID, amount.MinorUnits(), currency, provider, reference, wallet, message, now).Scan(&createdAt); err != nil {
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

// ListTips returns tips from the configured backing services.
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

		query := "SELECT id, channel_id, from_user_id, (amount * 100000000)::bigint AS amount_minor, currency, provider, reference, wallet_address, message, created_at FROM tips WHERE channel_id = $1 ORDER BY created_at DESC, id ASC"
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
			var amountMinor int64
			if err := rows.Scan(&tip.ID, &tip.ChannelID, &tip.FromUserID, &amountMinor, &tip.Currency, &tip.Provider, &tip.Reference, &walletAddress, &message, &createdAt); err != nil {
				return fmt.Errorf("scan tip: %w", err)
			}
			tip.Amount = models.NewMoneyFromMinorUnits(amountMinor)
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

// CreateSubscription creates subscription and returns an error when persistence or validation fails.
func (r *postgresRepository) CreateSubscription(params CreateSubscriptionParams) (models.Subscription, error) {
	if r == nil || r.pool == nil {
		return models.Subscription{}, ErrPostgresUnavailable
	}

	if params.Duration <= 0 {
		return models.Subscription{}, fmt.Errorf("duration must be positive")
	}

	amount := params.Amount
	if amount.MinorUnits() < 0 {
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

	id, err := generateID()
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

		_, err = tx.Exec(ctx, "INSERT INTO subscriptions (id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, external_reference) VALUES ($1, $2, $3, $4, $5, $6, $7::numeric / 100000000::numeric, $8, $9, $10, $11, $12, $13)", id, params.ChannelID, params.UserID, tier, provider, reference, amount.MinorUnits(), currency, started, expires, params.AutoRenew, "active", externalRef)
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

// ListSubscriptions returns subscriptions from the configured backing services.
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

		query := "SELECT id, channel_id, user_id, tier, provider, reference, (amount * 100000000)::bigint AS amount_minor, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference FROM subscriptions WHERE channel_id = $1"
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

// GetSubscription returns subscription from the configured backing services.
func (r *postgresRepository) GetSubscription(id string) (models.Subscription, bool) {
	if r == nil || r.pool == nil {
		return models.Subscription{}, false
	}

	ctx, cancel := r.acquireContext()
	row := r.pool.QueryRow(ctx, "SELECT id, channel_id, user_id, tier, provider, reference, (amount * 100000000)::bigint AS amount_minor, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference FROM subscriptions WHERE id = $1", id)
	cancel()

	sub, err := scanSubscriptionRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Subscription{}, false
		}
		return models.Subscription{}, false
	}

	return sub, true
}

// CancelSubscription performs cancel subscription and returns an error when dependent systems reject the operation.
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

		row := tx.QueryRow(ctx, "SELECT id, channel_id, user_id, tier, provider, reference, (amount * 100000000)::bigint AS amount_minor, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference FROM subscriptions WHERE id = $1 FOR UPDATE", id)
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

// AuthenticateOAuth performs authenticate oauth and returns an error when dependent systems reject the operation.
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
			if scanErr := tx.QueryRow(ctx, "SELECT id FROM users WHERE email = $1", normalizedEmail).Scan(&userID); scanErr != nil && !errors.Is(scanErr, pgx.ErrNoRows) {
				return fmt.Errorf("lookup user by email: %w", scanErr)
			}
		}

		now := time.Now().UTC()
		if userID == "" {
			userID, err = generateID()
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

// scanSubscriptionRow scans subscription row from database rows and returns an error when type conversion fails.
func scanSubscriptionRow(row pgx.Row) (models.Subscription, error) {
	var (
		sub               models.Subscription
		cancelledBy       pgtype.Text
		cancelledReason   pgtype.Text
		cancelledAt       pgtype.Timestamptz
		externalReference pgtype.Text
	)
	var amountMinor int64
	if err := row.Scan(&sub.ID, &sub.ChannelID, &sub.UserID, &sub.Tier, &sub.Provider, &sub.Reference, &amountMinor, &sub.Currency, &sub.StartedAt, &sub.ExpiresAt, &sub.AutoRenew, &sub.Status, &cancelledBy, &cancelledReason, &cancelledAt, &externalReference); err != nil {
		return models.Subscription{}, err
	}
	sub.Amount = models.NewMoneyFromMinorUnits(amountMinor)
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
