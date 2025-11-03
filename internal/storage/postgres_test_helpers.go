//go:build postgres

package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"bitriver-live/internal/ingest"
	"github.com/jackc/pgx/v5/pgxpool"
)

var postgresTestTables = []string{
	"sessions",
	"chat_sessions",
	"chat_message_reports",
	"chat_messages",
	"chat_bans",
	"chat_timeouts",
	"chat_reports",
	"tips",
	"subscriptions",
	"clip_exports",
	"recording_thumbnails",
	"recording_renditions",
	"recordings",
	"stream_session_manifests",
	"stream_sessions",
	"follows",
	"channels",
	"profiles",
	"oauth_accounts",
	"users",
}

func postgresRepositoryFactory(t *testing.T, opts ...Option) (Repository, func(), error) {
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

	applyPostgresMigrationsForTest(t, ctx, pool)
	if err := truncatePostgresTablesForTest(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("truncate tables: %v", err)
	}

	defaults := []Option{WithIngestController(ingest.NoopController{}), WithIngestRetries(1, 0)}
	opts = append(defaults, opts...)
	repo, err := NewPostgresRepository(dsn, opts...)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}

	cleanup := func() {
		if err := truncatePostgresTablesForTest(context.Background(), pool); err != nil {
			t.Fatalf("truncate tables: %v", err)
		}

		switch closer := repo.(type) {
		case interface{ Close(context.Context) error }:
			if err := closer.Close(context.Background()); err != nil {
				t.Fatalf("close repository: %v", err)
			}
		case interface{ Close() error }:
			if err := closer.Close(); err != nil {
				t.Fatalf("close repository: %v", err)
			}
		}

		pool.Close()
	}

	return repo, cleanup, nil
}

func applyPostgresMigrationsForTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
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

		path := filepath.Join(migrationsDir, entry.Name())
		data, err := os.ReadFile(path)
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

func truncatePostgresTablesForTest(ctx context.Context, pool *pgxpool.Pool) error {
	if len(postgresTestTables) == 0 {
		return nil
	}

	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(postgresTestTables, ", "))
	_, err := pool.Exec(ctx, query)
	return err
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
