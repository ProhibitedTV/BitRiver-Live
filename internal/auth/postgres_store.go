package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSessionStore persists sessions to a Postgres table, allowing multiple
// API replicas to share authentication state.
type PostgresSessionStore struct {
	pool    *pgxpool.Pool
	timeout time.Duration
}

type postgresSessionStoreOptions struct {
	timeout time.Duration
}

// PostgresSessionStoreOption configures Postgres session store behaviour.
type PostgresSessionStoreOption func(*postgresSessionStoreOptions)

const defaultPostgresSessionTimeout = 5 * time.Second

// WithTimeout limits how long the store waits for Postgres operations to complete.
func WithTimeout(timeout time.Duration) PostgresSessionStoreOption {
	return func(cfg *postgresSessionStoreOptions) {
		if timeout > 0 {
			cfg.timeout = timeout
		}
	}
}

// NewPostgresSessionStore opens a Postgres-backed session store using the provided DSN.
func NewPostgresSessionStore(dsn string, opts ...PostgresSessionStoreOption) (*PostgresSessionStore, error) {
	if dsn == "" {
		return nil, fmt.Errorf("postgres session dsn required")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres session config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres session pool: %w", err)
	}
	options := postgresSessionStoreOptions{timeout: defaultPostgresSessionTimeout}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	return &PostgresSessionStore{pool: pool, timeout: options.timeout}, nil
}

// Close releases the Postgres connection pool resources.
func (s *PostgresSessionStore) Close(ctx context.Context) error {
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
func (s *PostgresSessionStore) Ping(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("postgres session pool not configured")
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

// Save stores or updates the session token.
func (s *PostgresSessionStore) Save(token, userID string, expiresAt, absoluteExpiresAt time.Time) error {
	if s.pool == nil {
		return fmt.Errorf("postgres session pool not configured")
	}
	hashedToken, err := hashSessionToken(token)
	if err != nil {
		return err
	}
	ctx, cancel := s.operationContext()
	defer cancel()
	_, err = s.pool.Exec(ctx, `
INSERT INTO auth_sessions (token, hashed_token, user_id, expires_at, absolute_expires_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (hashed_token) DO UPDATE SET user_id = EXCLUDED.user_id, expires_at = EXCLUDED.expires_at, absolute_expires_at = EXCLUDED.absolute_expires_at
`, hashedToken, hashedToken, userID, expiresAt.UTC(), absoluteExpiresAt.UTC())
	return err
}

// Get fetches the session details for the provided token.
func (s *PostgresSessionStore) Get(token string) (SessionRecord, bool, error) {
	if s.pool == nil {
		return SessionRecord{}, false, fmt.Errorf("postgres session pool not configured")
	}
	hashedToken, err := hashSessionToken(token)
	if err != nil {
		return SessionRecord{}, false, err
	}
	ctx, cancel := s.operationContext()
	defer cancel()
	row := s.pool.QueryRow(ctx, `
SELECT user_id, expires_at, absolute_expires_at
FROM auth_sessions
WHERE hashed_token = $1
`, hashedToken)
	var record SessionRecord
	record.Token = token
	if err := row.Scan(&record.UserID, &record.ExpiresAt, &record.AbsoluteExpiresAt); err != nil {
		if isNoRows(err) {
			return SessionRecord{}, false, nil
		}
		return SessionRecord{}, false, err
	}
	return record, true, nil
}

// Delete removes the session token.
func (s *PostgresSessionStore) Delete(token string) error {
	if s.pool == nil {
		return fmt.Errorf("postgres session pool not configured")
	}
	hashedToken, err := hashSessionToken(token)
	if err != nil {
		return err
	}
	ctx, cancel := s.operationContext()
	defer cancel()
	_, err = s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE hashed_token = $1`, hashedToken)
	return err
}

// PurgeExpired deletes expired sessions from the table.
func (s *PostgresSessionStore) PurgeExpired(now time.Time) error {
	if s.pool == nil {
		return fmt.Errorf("postgres session pool not configured")
	}
	ctx, cancel := s.operationContext()
	defer cancel()
	_, err := s.pool.Exec(ctx, `DELETE FROM auth_sessions WHERE expires_at <= $1 OR absolute_expires_at <= $1`, now.UTC())
	return err
}

func (s *PostgresSessionStore) operationContext() (context.Context, context.CancelFunc) {
	if s.timeout > 0 {
		return context.WithTimeout(context.Background(), s.timeout)
	}
	return context.Background(), func() {}
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, pgx.ErrNoRows)
}
