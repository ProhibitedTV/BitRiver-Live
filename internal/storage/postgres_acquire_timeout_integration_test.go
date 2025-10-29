//go:build postgres

package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestPostgresRepositoryAcquireTimeout(t *testing.T) {
	repo, cleanup, err := postgresRepositoryFactory(t,
		WithPostgresPoolLimits(1, 1),
		WithPostgresAcquireTimeout(50*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to open postgres repository: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	pgRepo, ok := repo.(*postgresRepository)
	if !ok {
		t.Fatalf("expected postgres repository instance")
	}

	conn, err := pgRepo.pool.Acquire(context.Background())
	if err != nil {
		t.Fatalf("failed to saturate pool: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		email := fmt.Sprintf("acquire-timeout-%d@example.com", time.Now().UnixNano())
		_, err := repo.CreateUser(CreateUserParams{
			Email:       email,
			DisplayName: "Acquire Timeout",
			Password:    "changeme",
			SelfSignup:  true,
		})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected acquire timeout error")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected context deadline exceeded; got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for acquire to fail")
	}

	conn.Release()
}
