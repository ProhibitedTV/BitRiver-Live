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
	if ctx == nil {
		ctx = context.Background()
	}
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	_, execErr := conn.Exec(ctx, "SELECT 1")
	return execErr
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
	ctx, cancel := s.operationContext()
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
	ctx, cancel := s.operationContext()
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
	ctx, cancel := s.operationContext()
	defer cancel()
	_, err = s.pool.Exec(ctx, `DELETE FROM auth_mfa_challenges WHERE hashed_token = $1`, hashedToken)
	return err
}

// PurgeExpired deletes expired MFA challenges from the table.
func (s *PostgresMFAChallengeStore) PurgeExpired(now time.Time) error {
	if s.pool == nil {
		return fmt.Errorf("postgres mfa challenge pool not configured")
	}
	ctx, cancel := s.operationContext()
	defer cancel()
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_mfa_challenges WHERE expires_at <= $1`, now.UTC())
	return err
}

func (s *PostgresMFAChallengeStore) operationContext() (context.Context, context.CancelFunc) {
	if s.timeout > 0 {
		return context.WithTimeout(context.Background(), s.timeout)
	}
	return context.Background(), func() {}
}
