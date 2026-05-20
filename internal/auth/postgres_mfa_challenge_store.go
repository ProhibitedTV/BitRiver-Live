package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresMFAChallengeStore persists MFA challenges to a Postgres table.
type PostgresMFAChallengeStore struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

type postgresMFAChallengeStoreOptions struct {
	timeout time.Duration
}

// PostgresMFAChallengeStoreOption configures Postgres MFA challenge store behaviour.
type PostgresMFAChallengeStoreOption func(*postgresMFAChallengeStoreOptions)

// WithMFAChallengeTimeout limits how long the store waits for Postgres operations.
func WithMFAChallengeTimeout(timeout time.Duration) PostgresMFAChallengeStoreOption {
	return func(cfg *postgresMFAChallengeStoreOptions) {
		if timeout > 0 {
			cfg.timeout = timeout
		}
	}
}

// NewPostgresMFAChallengeStore opens a Postgres-backed MFA challenge store using the provided DSN.
func NewPostgresMFAChallengeStore(dsn string, opts ...PostgresMFAChallengeStoreOption) (*PostgresMFAChallengeStore, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres mfa challenge dsn required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres mfa challenge config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres mfa challenge pool: %w", err)
	}
	options := postgresMFAChallengeStoreOptions{timeout: defaultPostgresSessionTimeout}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return &PostgresMFAChallengeStore{pool: pool, timeout: options.timeout}, nil
}

// Close releases the Postgres connection pool resources.
func (s *PostgresMFAChallengeStore) Close(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		s.pool.Close()
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// Ping checks connectivity to the backing Postgres instance.
func (s *PostgresMFAChallengeStore) Ping(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("postgres mfa challenge pool not configured")
	}
	return pingPostgresPool(ctx, s.pool, s.timeout)
}

// Save stores or updates the MFA challenge token.
func (s *PostgresMFAChallengeStore) Save(token, userID string, expiresAt time.Time) error {
	if s.pool == nil {
		return fmt.Errorf("postgres mfa challenge pool not configured")
	}
	hashedToken, err := hashSessionToken(token)
	if err != nil {
		return err
	}
	ctx, cancel := postgresOperationContext(s.timeout)
	defer cancel()
	_, err = s.pool.Exec(ctx, `
INSERT INTO auth_mfa_challenges (hashed_token, user_id, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (hashed_token) DO UPDATE SET user_id = EXCLUDED.user_id, expires_at = EXCLUDED.expires_at
`, hashedToken, userID, expiresAt.UTC())
	return err
}

// Get fetches the MFA challenge details for the provided token.
func (s *PostgresMFAChallengeStore) Get(token string) (MFAChallengeRecord, bool, error) {
	if s.pool == nil {
		return MFAChallengeRecord{}, false, fmt.Errorf("postgres mfa challenge pool not configured")
	}
	hashedToken, err := hashSessionToken(token)
	if err != nil {
		return MFAChallengeRecord{}, false, err
	}
	ctx, cancel := postgresOperationContext(s.timeout)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
SELECT user_id, expires_at
FROM auth_mfa_challenges
WHERE hashed_token = $1
`, hashedToken)
	var record MFAChallengeRecord
	record.Token = token
	if err := row.Scan(&record.UserID, &record.ExpiresAt); err != nil {
		if isNoRows(err) {
			return MFAChallengeRecord{}, false, nil
		}
		return MFAChallengeRecord{}, false, err
	}
	return record, true, nil
}

// Delete removes the MFA challenge token.
func (s *PostgresMFAChallengeStore) Delete(token string) error {
	if s.pool == nil {
		return fmt.Errorf("postgres mfa challenge pool not configured")
	}
	hashedToken, err := hashSessionToken(token)
	if err != nil {
		return err
	}
	ctx, cancel := postgresOperationContext(s.timeout)
	defer cancel()
	_, err = s.pool.Exec(ctx, `DELETE FROM auth_mfa_challenges WHERE hashed_token = $1`, hashedToken)
	return err
}

// PurgeExpired deletes expired MFA challenges from the table.
func (s *PostgresMFAChallengeStore) PurgeExpired(now time.Time) error {
	if s.pool == nil {
		return fmt.Errorf("postgres mfa challenge pool not configured")
	}
	ctx, cancel := postgresOperationContext(s.timeout)
	defer cancel()
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_mfa_challenges WHERE expires_at <= $1`, now.UTC())
	return err
}
