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
	pool *pgxpool.Pool
}

// NewPostgresSessionStore opens a Postgres-backed session store using the provided DSN.
func NewPostgresSessionStore(dsn string) (*PostgresSessionStore, error) {
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
	return &PostgresSessionStore{pool: pool}, nil
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

// Save stores or updates the session token.
func (s *PostgresSessionStore) Save(token, userID string, expiresAt time.Time) error {
	if s.pool == nil {
		return fmt.Errorf("postgres session pool not configured")
	}
	_, err := s.pool.Exec(context.Background(), `
INSERT INTO auth_sessions (token, user_id, expires_at)
VALUES ($1, $2, $3)
ON CONFLICT (token) DO UPDATE SET user_id = EXCLUDED.user_id, expires_at = EXCLUDED.expires_at
`, token, userID, expiresAt.UTC())
	return err
}

// Get fetches the session details for the provided token.
func (s *PostgresSessionStore) Get(token string) (SessionRecord, bool, error) {
	if s.pool == nil {
		return SessionRecord{}, false, fmt.Errorf("postgres session pool not configured")
	}
	row := s.pool.QueryRow(context.Background(), `
SELECT user_id, expires_at
FROM auth_sessions
WHERE token = $1
`, token)
	var record SessionRecord
	record.Token = token
	if err := row.Scan(&record.UserID, &record.ExpiresAt); err != nil {
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
	_, err := s.pool.Exec(context.Background(), `DELETE FROM auth_sessions WHERE token = $1`, token)
	return err
}

// PurgeExpired deletes expired sessions from the table.
func (s *PostgresSessionStore) PurgeExpired(now time.Time) error {
	if s.pool == nil {
		return fmt.Errorf("postgres session pool not configured")
	}
	_, err := s.pool.Exec(context.Background(), `DELETE FROM auth_sessions WHERE expires_at <= $1`, now.UTC())
	return err
}

func isNoRows(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, pgx.ErrNoRows)
}
