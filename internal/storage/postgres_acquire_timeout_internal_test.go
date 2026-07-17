//go:build postgres

package storage

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresRepositoryAcquireTimeoutCoversQueries(t *testing.T) {
	if strings.Contains(strings.ToLower(pgx.ErrNoRows.Error()), "stub") {
		t.Skip("pgx stubbed; acquire timeout requires the upstream driver")
	}

	dsn := strings.TrimSpace(os.Getenv("BITRIVER_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Fatal("BITRIVER_TEST_POSTGRES_DSN is required")
	}

	repository, err := NewPostgresRepository(dsn, WithPostgresAcquireTimeout(50*time.Millisecond))
	if err != nil {
		t.Fatalf("open postgres repository: %v", err)
	}
	repo, ok := repository.(*postgresRepository)
	if !ok {
		t.Fatalf("expected postgres repository implementation, got %T", repository)
	}
	t.Cleanup(func() {
		if err := repo.Close(context.Background()); err != nil {
			t.Fatalf("close postgres repository: %v", err)
		}
	})

	start := time.Now()
	err = repo.withConn(func(ctx context.Context, conn *pgxpool.Conn) error {
		_, execErr := conn.Exec(ctx, "SELECT pg_sleep(0.1)")
		return execErr
	})
	if err == nil {
		t.Fatal("expected query to fail due to context deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded; got %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("query exceeded expected timeout: %v", elapsed)
	}
}
