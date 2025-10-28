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
