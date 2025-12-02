package auth

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	manager := NewSessionManager(50 * time.Millisecond)
	token, expiresAt, err := manager.Create("user-123")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
	if expiresAt.Before(time.Now()) {
		t.Fatal("expected expiry in the future")
	}

	userID, expires, ok, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected token to validate")
	}
	if userID != "user-123" {
		t.Fatalf("expected user id user-123, got %s", userID)
	}
	if !expires.Equal(expiresAt) {
		t.Fatalf("expected expiry %v, got %v", expiresAt, expires)
	}

	if err := manager.Revoke(token); err != nil {
		t.Fatalf("Revoke returned error: %v", err)
	}
	if _, _, ok, err := manager.Validate(token); err != nil || ok {
		if err != nil {
			t.Fatalf("Validate returned error for revoked token: %v", err)
		}
		t.Fatal("expected revoked token to be invalid")
	}
}

func TestSessionExpiration(t *testing.T) {
	store := NewMemorySessionStore()
	manager := NewSessionManager(10*time.Millisecond, WithStore(store))
	token, _, err := manager.Create("user-123")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if err := manager.PurgeExpired(); err != nil {
		t.Fatalf("PurgeExpired returned error: %v", err)
	}
	if _, ok, err := store.Get(token); err != nil {
		t.Fatalf("Get returned error: %v", err)
	} else if ok {
		t.Fatalf("expected expired session to be purged")
	}
	if _, _, ok, err := manager.Validate(token); err != nil || ok {
		if err != nil {
			t.Fatalf("Validate returned error for expired token: %v", err)
		}
		t.Fatal("expected expired token to be invalid")
	}
}

func TestCreateRequiresUserID(t *testing.T) {
	manager := NewSessionManager(time.Minute)
	if _, _, err := manager.Create(""); err == nil {
		t.Fatal("expected error for empty user id")
	}
}

func TestSessionPersistsAcrossManagers(t *testing.T) {
	store := NewMemorySessionStore()
	first := NewSessionManager(time.Minute, WithStore(store))
	token, _, err := first.Create("persistent-user")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	second := NewSessionManager(time.Minute, WithStore(store))
	userID, _, ok, err := second.Validate(token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected token to validate after manager restart")
	}
	if userID != "persistent-user" {
		t.Fatalf("expected user persistent-user, got %s", userID)
	}
}

func TestConcurrentValidationAcrossManagers(t *testing.T) {
	store := NewMemorySessionStore()
	primary := NewSessionManager(time.Minute, WithStore(store))
	token, _, err := primary.Create("user-xyz")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	const workers = 8
	wg := sync.WaitGroup{}
	wg.Add(workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			replica := NewSessionManager(time.Minute, WithStore(store))
			userID, _, ok, err := replica.Validate(token)
			if err != nil {
				errs <- err
				return
			}
			if !ok {
				errs <- fmt.Errorf("token rejected by replica")
				return
			}
			if userID != "user-xyz" {
				errs <- fmt.Errorf("unexpected user id %s", userID)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("replica validation error: %v", err)
	}
}

func TestValidateRefreshesIdleTimeout(t *testing.T) {
	store := NewMemorySessionStore()
	manager := NewSessionManager(time.Hour, WithStore(store), WithIdleTimeout(50*time.Millisecond))

	token, initialExpiry, err := manager.Create("user-refresh")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	_, refreshed, ok, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected token to validate")
	}
	if !refreshed.After(initialExpiry) {
		t.Fatalf("expected refreshed expiry after initial %v, got %v", initialExpiry, refreshed)
	}
	if record, _, _ := store.Get(token); !record.ExpiresAt.Equal(refreshed) {
		t.Fatalf("expected store expiry to refresh to %v, got %v", refreshed, record.ExpiresAt)
	}
}

func TestValidateHonorsAbsoluteTTL(t *testing.T) {
	store := NewMemorySessionStore()
	manager := NewSessionManager(100*time.Millisecond, WithStore(store), WithIdleTimeout(80*time.Millisecond))

	token, _, err := manager.Create("user-absolute")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	record, ok, err := store.Get(token)
	if err != nil || !ok {
		t.Fatalf("expected session record, got ok=%v err=%v", ok, err)
	}
	absoluteExpiry := record.AbsoluteExpiresAt

	time.Sleep(70 * time.Millisecond)
	_, refreshed, ok, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected token to validate before absolute expiry")
	}
	if refreshed.After(absoluteExpiry) {
		t.Fatalf("expected refresh capped at %v, got %v", absoluteExpiry, refreshed)
	}
	if !refreshed.Equal(absoluteExpiry) {
		t.Fatalf("expected refresh to use absolute expiry %v, got %v", absoluteExpiry, refreshed)
	}
}
