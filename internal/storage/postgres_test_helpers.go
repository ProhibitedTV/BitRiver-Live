//go:build postgres

package storage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/ingest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func startEphemeralPostgres(t *testing.T) (string, func()) {
	t.Helper()

	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("BITRIVER_TEST_POSTGRES_DSN not set and docker unavailable")
	}

	user := os.Getenv("BITRIVER_TEST_POSTGRES_USER")
	if user == "" {
		user = "bitriver"
	}
	password := os.Getenv("BITRIVER_TEST_POSTGRES_PASSWORD")
	if password == "" {
		password = "bitriver"
	}
	db := os.Getenv("BITRIVER_TEST_POSTGRES_DB")
	if db == "" {
		db = "bitriver_test"
	}
	port := os.Getenv("BITRIVER_TEST_POSTGRES_PORT")
	if port == "" {
		port = "54329"
	}
	image := os.Getenv("BITRIVER_TEST_POSTGRES_IMAGE")
	if image == "" {
		image = "postgres:15-alpine"
	}

	containerName := fmt.Sprintf("bitr-postgres-test-%d", time.Now().UnixNano())
	args := []string{
		"run",
		"--rm",
		"--detach",
		"--name", containerName,
		"--publish", fmt.Sprintf("%s:5432", port),
		"--env", fmt.Sprintf("POSTGRES_USER=%s", user),
		"--env", fmt.Sprintf("POSTGRES_PASSWORD=%s", password),
		"--env", fmt.Sprintf("POSTGRES_DB=%s", db),
		"--health-cmd", fmt.Sprintf("pg_isready -U %s -d %s", user, db),
		"--health-interval", "5s",
		"--health-timeout", "5s",
		"--health-retries", "10",
		image,
	}

	if output, err := exec.Command("docker", args...).CombinedOutput(); err != nil {
		t.Skipf("start postgres container: %v: %s", err, string(output))
	}

	cleanup := func() {
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	}

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		output, err := exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", containerName).CombinedOutput()
		status := strings.TrimSpace(string(output))
		if err == nil && status == "healthy" {
			break
		}
		if status == "unhealthy" {
			logs, _ := exec.Command("docker", "logs", containerName).CombinedOutput()
			cleanup()
			t.Fatalf("postgres container unhealthy: %s", string(logs))
		}
		time.Sleep(time.Second)
	}

	output, err := exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", containerName).CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "healthy" {
		logs, _ := exec.Command("docker", "logs", containerName).CombinedOutput()
		cleanup()
		t.Fatalf("postgres container did not become healthy: %s", string(logs))
	}

	dsn := fmt.Sprintf("postgres://%s:%s@127.0.0.1:%s/%s?sslmode=disable", user, password, port, db)
	return dsn, cleanup
}

func postgresRepositoryFactory(t *testing.T, opts ...Option) (Repository, func(), error) {
	t.Helper()

	dsn := os.Getenv("BITRIVER_TEST_POSTGRES_DSN")
	var cleanupFns []func()
	if strings.TrimSpace(dsn) == "" {
		var dockerCleanup func()
		dsn, dockerCleanup = startEphemeralPostgres(t)
		if dockerCleanup != nil {
			cleanupFns = append(cleanupFns, dockerCleanup)
		}
		_ = os.Setenv("BITRIVER_TEST_POSTGRES_DSN", dsn)
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

		for i := len(cleanupFns) - 1; i >= 0; i-- {
			cleanupFns[i]()
		}
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
	tables, err := PostgresTablesForTest(ctx, pool)
	if err != nil {
		return err
	}

	if len(tables) == 0 {
		return nil
	}

	query := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", strings.Join(tables, ", "))
	_, err = pool.Exec(ctx, query)
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

// PostgresTablesForTest returns the list of public tables managed by the
// migrations so tests can truncate state between runs without duplicating the
// schema definition.
func PostgresTablesForTest(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `
                SELECT table_name
                FROM information_schema.tables
                WHERE table_schema = 'public'
                ORDER BY table_name
        `)
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}

		if name == "schema_migrations" {
			continue
		}

		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}

	return tables, nil
}
