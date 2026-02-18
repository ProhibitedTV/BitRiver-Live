package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	models "bitriver-live/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetMFASettings returns mfasettings from the configured backing services.
func (r *postgresRepository) GetMFASettings(userID string) (models.MFASettings, bool, error) {
	if r == nil || r.pool == nil {
		return models.MFASettings{}, false, ErrPostgresUnavailable
	}

	userID = strings.TrimSpace(userID)
	if userID == "" {
		return models.MFASettings{}, false, fmt.Errorf("userID is required")
	}

	var settings models.MFASettings
	err := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		row := conn.QueryRow(ctx, `
SELECT user_id, secret, recovery_codes, enabled, created_at, updated_at, enabled_at, last_used_at
FROM auth_mfa
WHERE user_id = $1
`, userID)
		return row.Scan(
			&settings.UserID,
			&settings.Secret,
			&settings.RecoveryCodes,
			&settings.Enabled,
			&settings.CreatedAt,
			&settings.UpdatedAt,
			&settings.EnabledAt,
			&settings.LastUsedAt,
		)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return models.MFASettings{}, false, nil
	}
	if err != nil {
		return models.MFASettings{}, false, fmt.Errorf("get mfa settings: %w", err)
	}
	settings.CreatedAt = settings.CreatedAt.UTC()
	settings.UpdatedAt = settings.UpdatedAt.UTC()
	if settings.EnabledAt != nil {
		enabledAt := settings.EnabledAt.UTC()
		settings.EnabledAt = &enabledAt
	}
	if settings.LastUsedAt != nil {
		lastUsed := settings.LastUsedAt.UTC()
		settings.LastUsedAt = &lastUsed
	}
	return settings, true, nil
}

// UpsertMFASettings performs upsert mfasettings and returns an error when dependent systems reject the operation.
func (r *postgresRepository) UpsertMFASettings(settings models.MFASettings) (models.MFASettings, error) {
	if r == nil || r.pool == nil {
		return models.MFASettings{}, ErrPostgresUnavailable
	}

	userID := strings.TrimSpace(settings.UserID)
	if userID == "" {
		return models.MFASettings{}, fmt.Errorf("userID is required")
	}
	if settings.RecoveryCodes == nil {
		settings.RecoveryCodes = []string{}
	}

	now := time.Now().UTC()
	if settings.CreatedAt.IsZero() {
		settings.CreatedAt = now
	}
	settings.UpdatedAt = now
	settings.UserID = userID

	upsertErr := r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, `
INSERT INTO auth_mfa (user_id, secret, recovery_codes, enabled, created_at, updated_at, enabled_at, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (user_id) DO UPDATE
SET secret = EXCLUDED.secret,
    recovery_codes = EXCLUDED.recovery_codes,
    enabled = EXCLUDED.enabled,
    updated_at = EXCLUDED.updated_at,
    enabled_at = EXCLUDED.enabled_at,
    last_used_at = EXCLUDED.last_used_at
`, settings.UserID, settings.Secret, settings.RecoveryCodes, settings.Enabled, settings.CreatedAt, settings.UpdatedAt, settings.EnabledAt, settings.LastUsedAt)
		return err
	})
	if upsertErr != nil {
		return models.MFASettings{}, fmt.Errorf("upsert mfa settings: %w", upsertErr)
	}
	return settings, nil
}

// DeleteMFASettings deletes mfasettings and returns an error when persistence or validation fails.
func (r *postgresRepository) DeleteMFASettings(userID string) error {
	if r == nil || r.pool == nil {
		return ErrPostgresUnavailable
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("userID is required")
	}
	return r.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, err := conn.Exec(ctx, `DELETE FROM auth_mfa WHERE user_id = $1`, userID)
		return err
	})
}
