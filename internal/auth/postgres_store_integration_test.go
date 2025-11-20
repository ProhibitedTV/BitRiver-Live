//go:build postgres

package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresSessionStoreTimeout(t *testing.T) {
	store, cleanup := openPostgresSessionStoreForTest(t, WithTimeout(50*time.Millisecond))
	if cleanup != nil {
		defer cleanup()
	}

	ctx := context.Background()
	conn, err := store.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to acquire setup connection: %v", err)
	}

	if _, err := conn.Exec(ctx, `CREATE OR REPLACE FUNCTION slow_auth_sessions_trigger() RETURNS trigger AS $$ BEGIN PERFORM pg_sleep(0.2); RETURN NEW; END; $$ LANGUAGE plpgsql;`); err != nil {
		conn.Release()
		t.Fatalf("failed to create slow trigger function: %v", err)
	}
	if _, err := conn.Exec(ctx, `DROP TRIGGER IF EXISTS slow_auth_sessions_trigger ON auth_sessions`); err != nil {
		conn.Release()
		t.Fatalf("failed to drop existing trigger: %v", err)
	}
	if _, err := conn.Exec(ctx, `CREATE TRIGGER slow_auth_sessions_trigger BEFORE INSERT ON auth_sessions FOR EACH ROW EXECUTE FUNCTION slow_auth_sessions_trigger()`); err != nil {
		conn.Release()
		t.Fatalf("failed to create slow trigger: %v", err)
	}
	conn.Release()

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cleanupConn, err := store.pool.Acquire(cleanupCtx)
		if err != nil {
			return
		}
		defer cleanupConn.Release()
		_, _ = cleanupConn.Exec(cleanupCtx, `DROP TRIGGER IF EXISTS slow_auth_sessions_trigger ON auth_sessions`)
		_, _ = cleanupConn.Exec(cleanupCtx, `DROP FUNCTION IF EXISTS slow_auth_sessions_trigger()`)
	}()

	err = store.Save("timeout-token", "timeout-user", time.Now().Add(time.Hour))
	if err == nil {
		t.Fatal("expected timeout error from slow trigger")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded; got %v", err)
	}
}

func TestPostgresSessionStoreSavesHashedTokens(t *testing.T) {
	store, cleanup := openPostgresSessionStoreForTest(t)
	if cleanup != nil {
		defer cleanup()
	}

	token := "raw-session-token"
	expiresAt := time.Now().Add(time.Hour)

	if err := store.Save(token, "user-id", expiresAt); err != nil {
		t.Fatalf("save session: %v", err)
	}

	hashedToken, err := hashSessionToken(token)
	if err != nil {
		t.Fatalf("hash token: %v", err)
	}

	ctx := context.Background()
	conn, err := store.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	var storedToken, storedHash, storedUser string
	var storedExpires time.Time
	if err := conn.QueryRow(ctx, `SELECT token, hashed_token, user_id, expires_at FROM auth_sessions WHERE hashed_token = $1`, hashedToken).
		Scan(&storedToken, &storedHash, &storedUser, &storedExpires); err != nil {
		t.Fatalf("fetch stored session: %v", err)
	}

	if storedUser != "user-id" {
		t.Fatalf("expected user-id, got %s", storedUser)
	}
	if storedToken != storedHash {
		t.Fatalf("expected stored token and hash to match")
	}
	if storedToken == token {
		t.Fatalf("expected persisted token to be hashed")
	}

	record, ok, err := store.Get(token)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if !ok {
		t.Fatalf("expected session to be found")
	}
	if record.Token != token {
		t.Fatalf("expected record token to match input")
	}
	if !record.ExpiresAt.Equal(storedExpires) {
		t.Fatalf("expected expiresAt %v, got %v", storedExpires, record.ExpiresAt)
	}
}

func TestPostgresSessionStoreDeleteUsesHashes(t *testing.T) {
	store, cleanup := openPostgresSessionStoreForTest(t)
	if cleanup != nil {
		defer cleanup()
	}

	token := "token-to-delete"
	if err := store.Save(token, "user-id", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("save session: %v", err)
	}

	if err := store.Delete(token); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	ctx := context.Background()
	conn, err := store.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire connection: %v", err)
	}
	defer conn.Release()

	hashedToken, err := hashSessionToken(token)
	if err != nil {
		t.Fatalf("hash token: %v", err)
	}

	var count int
	if err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM auth_sessions WHERE hashed_token = $1`, hashedToken).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected session to be deleted, got %d rows", count)
	}
}

func openPostgresSessionStoreForTest(t *testing.T, opts ...PostgresSessionStoreOption) (*PostgresSessionStore, func()) {
	t.Helper()

	dsn := os.Getenv("BITRIVER_TEST_POSTGRES_DSN")
	if strings.TrimSpace(dsn) == "" {
		t.Skip("BITRIVER_TEST_POSTGRES_DSN not set")
	}

	ctx := context.Background()
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse postgres config: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatalf("open postgres pool: %v", err)
	}

	applyAuthMigrationsForTest(t, ctx, pool)
	if _, err := pool.Exec(ctx, `TRUNCATE TABLE auth_sessions`); err != nil {
		pool.Close()
		t.Fatalf("truncate auth_sessions: %v", err)
	}

	pool.Close()

	store, err := NewPostgresSessionStore(dsn, opts...)
	if err != nil {
		t.Fatalf("open postgres session store: %v", err)
	}

	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if store.pool != nil {
			conn, err := store.pool.Acquire(cleanupCtx)
			if err == nil {
				_, _ = conn.Exec(cleanupCtx, `TRUNCATE TABLE auth_sessions`)
				conn.Release()
			}
		}
		_ = store.Close(context.Background())
	}

	return store, cleanup
}

func applyAuthMigrationsForTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("determine repository root: runtime.Caller failed")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	migrationsDir := filepath.Join(repoRoot, "deploy", "migrations")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(migrationsDir, entry.Name()))
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}

		for _, stmt := range splitSQLStatementsForTest(string(data)) {
			if stmt == "" {
				continue
			}
			if _, err := pool.Exec(ctx, stmt); err != nil {
				t.Fatalf("apply migration %s: %v", entry.Name(), err)
			}
		}
	}
}

func splitSQLStatementsForTest(script string) []string {
	parts := strings.Split(script, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		statements = append(statements, trimmed)
	}
	return statements
}
