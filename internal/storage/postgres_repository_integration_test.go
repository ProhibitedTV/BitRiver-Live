//go:build postgres

package storage_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"bitriver-live/internal/ingest"
	"bitriver-live/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

// postgresRepositoryFactory opens a Postgres-backed repository for integration
// scenarios, applying migrations and ensuring tables are truncated between
// tests. The factory requires BITRIVER_TEST_POSTGRES_DSN to point at a clean
// database dedicated to automated runs.
func postgresRepositoryFactory(t *testing.T, opts ...storage.Option) (storage.Repository, func(), error) {
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

	applyPostgresMigrations(t, ctx, pool)
	if err := truncatePostgresTables(ctx, pool); err != nil {
		pool.Close()
		t.Fatalf("truncate tables: %v", err)
	}

	defaults := []storage.Option{storage.WithIngestController(ingest.NoopController{}), storage.WithIngestRetries(1, 0)}
	opts = append(defaults, opts...)
	repo, err := storage.NewPostgresRepository(dsn, opts...)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}

	t.Cleanup(func() {
		if err := truncatePostgresTables(context.Background(), pool); err != nil {
			t.Fatalf("truncate tables: %v", err)
		}
	})
	t.Cleanup(func() {
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
	})
	t.Cleanup(func() { pool.Close() })

	return repo, nil, nil
}

func TestPostgresRepositoryConnection(t *testing.T) {
	repo, _, err := postgresRepositoryFactory(t)
	if errors.Is(err, storage.ErrPostgresUnavailable) {
		t.Skip("postgres repository unavailable in this build")
	}
	if err != nil {
		t.Fatalf("failed to open postgres repository: %v", err)
	}
	if repo == nil {
		t.Fatalf("expected postgres repository instance")
	}
}

func TestPostgresUserLifecycle(t *testing.T) {
	storage.RunRepositoryUserLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresChatRestrictionsLifecycle(t *testing.T) {
	storage.RunRepositoryChatRestrictionsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresChatReportsLifecycle(t *testing.T) {
	storage.RunRepositoryChatReportsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresTipsLifecycle(t *testing.T) {
	storage.RunRepositoryTipsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresSubscriptionsLifecycle(t *testing.T) {
	storage.RunRepositorySubscriptionsLifecycle(t, postgresRepositoryFactory)
}

func TestPostgresIngestHealthSnapshots(t *testing.T) {
	storage.RunRepositoryIngestHealthSnapshots(t, postgresRepositoryFactory)
}

func TestPostgresRecordingRetention(t *testing.T) {
	storage.RunRepositoryRecordingRetention(t, postgresRepositoryFactory)
}

var postgresTables = []string{
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
	"users",
}

func applyPostgresMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	entries, err := os.ReadDir("deploy/migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		path := filepath.Join("deploy/migrations", entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read migration %s: %v", entry.Name(), err)
		}
		for _, stmt := range splitSQLStatements(string(data)) {
			if stmt == "" {
				continue
			}
			if _, err := pool.Exec(ctx, stmt); err != nil {
				t.Fatalf("apply migration %s: %v", entry.Name(), err)
			}
		}
	}
}

func truncatePostgresTables(ctx context.Context, pool *pgxpool.Pool) error {
	if len(postgresTables) == 0 {
		return nil
	}
	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(postgresTables, ", "))
	_, err := pool.Exec(ctx, query)
	return err
}

func splitSQLStatements(script string) []string {
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
