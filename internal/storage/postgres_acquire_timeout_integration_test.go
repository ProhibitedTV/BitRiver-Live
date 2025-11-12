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

func TestPostgresRepositoryAcquireTimeoutUpsertProfile(t *testing.T) {
	repo, cleanup, err := postgresRepositoryFactory(t,
		WithPostgresPoolLimits(2, 1),
		WithPostgresAcquireTimeout(100*time.Millisecond),
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

	email := fmt.Sprintf("profile-timeout-%d@example.com", time.Now().UnixNano())
	user, err := repo.CreateUser(CreateUserParams{
		Email:       email,
		DisplayName: "Acquire Timeout Profile",
		Password:    "changeme",
		SelfSignup:  true,
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	ctx := context.Background()
	conn, err := pgRepo.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("failed to acquire setup connection: %v", err)
	}

	if _, err := conn.Exec(ctx, `CREATE OR REPLACE FUNCTION slow_profile_trigger() RETURNS trigger AS $$ BEGIN PERFORM pg_sleep(0.2); RETURN NEW; END; $$ LANGUAGE plpgsql;`); err != nil {
		conn.Release()
		t.Fatalf("failed to create slow trigger function: %v", err)
	}
	if _, err := conn.Exec(ctx, `DROP TRIGGER IF EXISTS slow_profiles_trigger ON profiles`); err != nil {
		conn.Release()
		t.Fatalf("failed to drop existing trigger: %v", err)
	}
	if _, err := conn.Exec(ctx, `CREATE TRIGGER slow_profiles_trigger BEFORE INSERT OR UPDATE ON profiles FOR EACH ROW EXECUTE FUNCTION slow_profile_trigger()`); err != nil {
		conn.Release()
		t.Fatalf("failed to create slow trigger: %v", err)
	}
	conn.Release()

	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		cleanupConn, err := pgRepo.pool.Acquire(cleanupCtx)
		if err != nil {
			return
		}
		defer cleanupConn.Release()
		_, _ = cleanupConn.Exec(cleanupCtx, `DROP TRIGGER IF EXISTS slow_profiles_trigger ON profiles`)
		_, _ = cleanupConn.Exec(cleanupCtx, `DROP FUNCTION IF EXISTS slow_profile_trigger()`)
	}()

	bio := "slow update"
	_, err = repo.UpsertProfile(user.ID, ProfileUpdate{Bio: &bio})
	if err == nil {
		t.Fatal("expected acquire timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded; got %v", err)
	}
}
