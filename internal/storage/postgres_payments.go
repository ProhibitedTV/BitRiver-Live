package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"bitriver-live/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateTip creates tip and returns an error when persistence or validation fails.
func (r *postgresRepository) CreateTip(params CreateTipParams) (domain.Tip, error) {
	if r == nil || r.pool == nil {
		return domain.Tip{}, ErrPostgresUnavailable
	}

	amount := params.Amount
	if amount.MinorUnits() <= 0 {
		return domain.Tip{}, fmt.Errorf("amount must be positive")
	}

	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return domain.Tip{}, fmt.Errorf("currency is required")
	}

	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return domain.Tip{}, fmt.Errorf("provider is required")
	}

	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("tip-%d", time.Now().UnixNano())
	}
	if utf8.RuneCountInString(reference) > MaxTipReferenceLength {
		return domain.Tip{}, fmt.Errorf("reference exceeds %d characters", MaxTipReferenceLength)
	}

	wallet := strings.TrimSpace(params.WalletAddress)
	if utf8.RuneCountInString(wallet) > MaxTipWalletAddressLength {
		return domain.Tip{}, fmt.Errorf("wallet address exceeds %d characters", MaxTipWalletAddressLength)
	}

	message := strings.TrimSpace(params.Message)
	if utf8.RuneCountInString(message) > MaxTipMessageLength {
		return domain.Tip{}, fmt.Errorf("message exceeds %d characters", MaxTipMessageLength)
	}

	id, err := generateID()
	if err != nil {
		return domain.Tip{}, err
	}

	now := time.Now().UTC()
	var tip domain.Tip
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
		idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
		if err := tx.QueryRow(ctx, "INSERT INTO tips (id, channel_id, from_user_id, amount, currency, provider, reference, wallet_address, message, status, idempotency_key, created_at) VALUES ($1, $2, $3, $4::numeric / 100000000::numeric, $5, $6, $7, $8, $9, $10, $11, $12) RETURNING created_at", id, params.ChannelID, params.FromUserID, amount.MinorUnits(), currency, provider, reference, wallet, message, domain.PaymentStatePending, idempotencyKey, now).Scan(&createdAt); err != nil {
			return fmt.Errorf("insert tip: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create tip: %w", err)
		}

		tip = domain.Tip{
			ID:             id,
			ChannelID:      params.ChannelID,
			FromUserID:     params.FromUserID,
			Amount:         amount,
			Currency:       currency,
			Provider:       provider,
			Reference:      reference,
			WalletAddress:  wallet,
			Message:        message,
			Status:         domain.PaymentStatePending,
			IdempotencyKey: idempotencyKey,
			CreatedAt:      createdAt.UTC(),
		}

		return nil
	})
	if saveErr != nil {
		return domain.Tip{}, saveErr
	}

	return tip, nil
}

// ListTips returns tips from the configured backing services.
func (r *postgresRepository) ListTips(channelID string, limit int) ([]domain.Tip, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}

	tips := make([]domain.Tip, 0)
	listErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			return fmt.Errorf("begin list tips tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}

		query := "SELECT id, channel_id, from_user_id, (amount * 100000000)::bigint AS amount_minor, currency, provider, reference, wallet_address, message, status, idempotency_key, created_at FROM tips WHERE channel_id = $1 ORDER BY created_at DESC, id ASC"
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
			var tip domain.Tip
			var walletAddress, message, idemKey pgtype.Text
			var createdAt time.Time
			var amountMinor int64
			if err := rows.Scan(&tip.ID, &tip.ChannelID, &tip.FromUserID, &amountMinor, &tip.Currency, &tip.Provider, &tip.Reference, &walletAddress, &message, &tip.Status, &idemKey, &createdAt); err != nil {
				return fmt.Errorf("scan tip: %w", err)
			}
			tip.Amount = domain.NewMoneyFromMinorUnits(amountMinor)
			if walletAddress.Valid {
				tip.WalletAddress = walletAddress.String
			}
			if message.Valid {
				tip.Message = message.String
			}
			if idemKey.Valid {
				tip.IdempotencyKey = idemKey.String
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
func (r *postgresRepository) CreateSubscription(params CreateSubscriptionParams) (domain.Subscription, error) {
	if r == nil || r.pool == nil {
		return domain.Subscription{}, ErrPostgresUnavailable
	}

	if params.Duration <= 0 {
		return domain.Subscription{}, fmt.Errorf("duration must be positive")
	}

	amount := params.Amount
	if amount.MinorUnits() < 0 {
		return domain.Subscription{}, fmt.Errorf("amount cannot be negative")
	}

	currency := strings.ToUpper(strings.TrimSpace(params.Currency))
	if currency == "" {
		return domain.Subscription{}, fmt.Errorf("currency is required")
	}

	tier := strings.TrimSpace(params.Tier)
	if tier == "" {
		tier = "supporter"
	}

	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	if provider == "" {
		return domain.Subscription{}, fmt.Errorf("provider is required")
	}

	reference := strings.TrimSpace(params.Reference)
	if reference == "" {
		reference = fmt.Sprintf("sub-%d", time.Now().UnixNano())
	}

	externalRef := strings.TrimSpace(params.ExternalReference)

	id, err := generateID()
	if err != nil {
		return domain.Subscription{}, err
	}

	started := time.Now().UTC()
	expires := started.Add(params.Duration)

	var subscription domain.Subscription
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

		idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
		_, err = tx.Exec(ctx, "INSERT INTO subscriptions (id, channel_id, user_id, tier, provider, reference, amount, currency, started_at, expires_at, auto_renew, status, external_reference, idempotency_key) VALUES ($1, $2, $3, $4, $5, $6, $7::numeric / 100000000::numeric, $8, $9, $10, $11, $12, $13, $14)", id, params.ChannelID, params.UserID, tier, provider, reference, amount.MinorUnits(), currency, started, expires, params.AutoRenew, domain.PaymentStatePending, externalRef, idempotencyKey)
		if err != nil {
			return fmt.Errorf("insert subscription: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit create subscription: %w", err)
		}

		subscription = domain.Subscription{
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
			Status:            domain.PaymentStatePending,
			ExternalReference: externalRef,
			IdempotencyKey:    idempotencyKey,
		}

		return nil
	})
	if saveErr != nil {
		return domain.Subscription{}, saveErr
	}

	return subscription, nil
}

// ListSubscriptions returns subscriptions from the configured backing services.
func (r *postgresRepository) ListSubscriptions(channelID string, includeInactive bool) ([]domain.Subscription, error) {
	if r == nil || r.pool == nil {
		return nil, ErrPostgresUnavailable
	}

	subscriptions := make([]domain.Subscription, 0)
	listErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
		if err != nil {
			return fmt.Errorf("begin list subscriptions tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		if err := ensureChannelExists(ctx, tx, channelID); err != nil {
			return err
		}

		query := "SELECT id, channel_id, user_id, tier, provider, reference, (amount * 100000000)::bigint AS amount_minor, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference, idempotency_key FROM subscriptions WHERE channel_id = $1"
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
func (r *postgresRepository) GetSubscription(id string) (domain.Subscription, bool) {
	if r == nil || r.pool == nil {
		return domain.Subscription{}, false
	}

	ctx, cancel := r.acquireContext()
	row := r.pool.QueryRow(ctx, "SELECT id, channel_id, user_id, tier, provider, reference, (amount * 100000000)::bigint AS amount_minor, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference, idempotency_key FROM subscriptions WHERE id = $1", id)
	cancel()

	sub, err := scanSubscriptionRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, false
		}
		return domain.Subscription{}, false
	}

	return sub, true
}

// CancelSubscription performs cancel subscription and returns an error when dependent systems reject the operation.
func (r *postgresRepository) CancelSubscription(id, cancelledBy, reason string) (domain.Subscription, error) {
	if r == nil || r.pool == nil {
		return domain.Subscription{}, ErrPostgresUnavailable
	}

	trimmedReason := strings.TrimSpace(reason)

	var updated domain.Subscription
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin cancel subscription tx: %w", err)
		}
		defer rollbackTx(ctx, tx)

		row := tx.QueryRow(ctx, "SELECT id, channel_id, user_id, tier, provider, reference, (amount * 100000000)::bigint AS amount_minor, currency, started_at, expires_at, auto_renew, status, cancelled_by, cancelled_reason, cancelled_at, external_reference, idempotency_key FROM subscriptions WHERE id = $1 FOR UPDATE", id)
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
		return domain.Subscription{}, err
	}

	return updated, nil
}

// AuthenticateOAuth performs authenticate oauth and returns an error when dependent systems reject the operation.
func (r *postgresRepository) AuthenticateOAuth(params OAuthLoginParams) (domain.User, error) {
	if r == nil || r.pool == nil {
		return domain.User{}, ErrPostgresUnavailable
	}

	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	subject := strings.TrimSpace(params.Subject)
	if provider == "" {
		return domain.User{}, fmt.Errorf("provider is required")
	}
	if subject == "" {
		return domain.User{}, fmt.Errorf("subject is required")
	}

	normalizedEmail := strings.TrimSpace(strings.ToLower(params.Email))
	if normalizedEmail == "" {
		normalizedEmail = fallbackOAuthEmail(provider, subject)
	}
	displayName := strings.TrimSpace(params.DisplayName)
	if displayName == "" {
		displayName = defaultOAuthDisplayName(provider, normalizedEmail, subject)
	}

	var user domain.User
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
			user = domain.User{
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
		return domain.User{}, err
	}
	return user, nil
}

var _ Repository = (*postgresRepository)(nil)

// scanSubscriptionRow scans subscription row from database rows and returns an error when type conversion fails.
func scanSubscriptionRow(row pgx.Row) (domain.Subscription, error) {
	var (
		sub               domain.Subscription
		cancelledBy       pgtype.Text
		cancelledReason   pgtype.Text
		cancelledAt       pgtype.Timestamptz
		externalReference pgtype.Text
	)
	var amountMinor int64
	if err := row.Scan(&sub.ID, &sub.ChannelID, &sub.UserID, &sub.Tier, &sub.Provider, &sub.Reference, &amountMinor, &sub.Currency, &sub.StartedAt, &sub.ExpiresAt, &sub.AutoRenew, &sub.Status, &cancelledBy, &cancelledReason, &cancelledAt, &externalReference); err != nil {
		return domain.Subscription{}, err
	}
	sub.Amount = domain.NewMoneyFromMinorUnits(amountMinor)
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

// ProcessPaymentWebhook persists a verified provider webhook and applies state transitions atomically.
func (r *postgresRepository) ProcessPaymentWebhook(params ProcessPaymentWebhookParams) (domain.PaymentTransaction, error) {
	if r == nil || r.pool == nil {
		return domain.PaymentTransaction{}, ErrPostgresUnavailable
	}
	provider := strings.ToLower(strings.TrimSpace(params.Provider))
	eventID := strings.TrimSpace(params.EventID)
	entityType := strings.ToLower(strings.TrimSpace(params.EntityType))
	reference := strings.TrimSpace(params.Reference)
	status := strings.ToLower(strings.TrimSpace(params.Status))
	idempotencyKey := strings.TrimSpace(params.IdempotencyKey)
	if provider == "" || eventID == "" || entityType == "" || reference == "" || status == "" {
		return domain.PaymentTransaction{}, fmt.Errorf("provider, eventID, entityType, reference and status are required")
	}
	var txOut domain.PaymentTransaction
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("begin payment webhook tx: %w", err)
		}
		defer rollbackTx(ctx, tx)
		row := tx.QueryRow(ctx, "SELECT id, provider, event_id, entity_type, entity_id, reference, status, idempotency_key, created_at FROM payment_transactions WHERE provider=$1 AND event_id=$2", provider, eventID)
		var existing domain.PaymentTransaction
		var idem pgtype.Text
		if err := row.Scan(&existing.ID, &existing.Provider, &existing.EventID, &existing.EntityType, &existing.EntityID, &existing.Reference, &existing.Status, &idem, &existing.CreatedAt); err == nil {
			if idem.Valid {
				existing.IdempotencyKey = idem.String
			}
			txOut = existing
			_ = tx.Commit(ctx)
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		var entityID string
		if entityType == "tip" {
			if err := tx.QueryRow(ctx, "UPDATE tips SET status=$1, idempotency_key=COALESCE(NULLIF($2,''), idempotency_key) WHERE provider=$3 AND reference=$4 RETURNING id", status, idempotencyKey, provider, reference).Scan(&entityID); err != nil {
				return err
			}
		} else if entityType == "subscription" {
			if err := tx.QueryRow(ctx, "UPDATE subscriptions SET status=$1, idempotency_key=COALESCE(NULLIF($2,''), idempotency_key) WHERE provider=$3 AND reference=$4 RETURNING id", status, idempotencyKey, provider, reference).Scan(&entityID); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("unsupported entity type %s", entityType)
		}
		id, err := generateID()
		if err != nil {
			return err
		}
		created := time.Now().UTC()
		_, err = tx.Exec(ctx, "INSERT INTO payment_transactions (id, provider, event_id, entity_type, entity_id, reference, status, idempotency_key, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)", id, provider, eventID, entityType, entityID, reference, status, idempotencyKey, created)
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		txOut = domain.PaymentTransaction{ID: id, Provider: provider, EventID: eventID, EntityType: entityType, EntityID: entityID, Reference: reference, Status: status, IdempotencyKey: idempotencyKey, CreatedAt: created}
		return nil
	})
	if err != nil {
		return domain.PaymentTransaction{}, err
	}
	return txOut, nil
}
