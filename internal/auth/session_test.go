package auth

import (
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

	userID, expires, ok := manager.Validate(token)
	if !ok {
		t.Fatal("expected token to validate")
	}
	if userID != "user-123" {
		t.Fatalf("expected user id user-123, got %s", userID)
	}
	if !expires.Equal(expiresAt) {
		t.Fatalf("expected expiry %v, got %v", expiresAt, expires)
	}

	manager.Revoke(token)
	if _, _, ok := manager.Validate(token); ok {
		t.Fatal("expected revoked token to be invalid")
	}
}

func TestSessionExpiration(t *testing.T) {
	manager := NewSessionManager(10 * time.Millisecond)
	token, _, err := manager.Create("user-123")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	manager.PurgeExpired()
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if len(manager.sessions) != 0 {
		t.Fatalf("expected expired session to be purged")
	}
	if _, _, ok := manager.Validate(token); ok {
		t.Fatal("expected expired token to be invalid")
	}
}

func TestCreateRequiresUserID(t *testing.T) {
	manager := NewSessionManager(time.Minute)
	if _, _, err := manager.Create(""); err == nil {
		t.Fatal("expected error for empty user id")
	}
}
