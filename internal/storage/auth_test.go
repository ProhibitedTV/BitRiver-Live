package storage

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestAuthenticateUser(t *testing.T) {
	store := newTestStore(t)
	password := "hunter42!"
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
		Password:    password,
		SelfSignup:  true,
	})
	if err != nil {
		t.Fatalf("CreateUser self signup: %v", err)
	}
	if !user.SelfSignup {
		t.Fatalf("expected self signup flag to be set")
	}
	if user.PasswordHash == "" {
		t.Fatal("expected password hash to be stored")
	}
	if user.PasswordHash == password {
		t.Fatal("expected password hash to differ from password")
	}
	parts := strings.Split(user.PasswordHash, "$")
	if len(parts) != 5 {
		t.Fatalf("unexpected hash format: %s", user.PasswordHash)
	}
	if parts[0] != "pbkdf2" || parts[1] != "sha256" {
		t.Fatalf("unexpected hash identifiers: %v", parts[:2])
	}
	if parts[2] != strconv.Itoa(passwordHashIterations) {
		t.Fatalf("expected iteration count %d, got %s", passwordHashIterations, parts[2])
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		t.Fatalf("decode salt: %v", err)
	}
	if len(salt) != passwordHashSaltLength {
		t.Fatalf("expected salt length %d, got %d", passwordHashSaltLength, len(salt))
	}
	derived, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		t.Fatalf("decode derived key: %v", err)
	}
	if len(derived) != passwordHashKeyLength {
		t.Fatalf("expected key length %d, got %d", passwordHashKeyLength, len(derived))
	}
	if verifyErr := verifyPassword(user.PasswordHash, password); verifyErr != nil {
		t.Fatalf("verifyPassword failed: %v", verifyErr)
	}

	authenticated, err := store.AuthenticateUser("viewer@example.com", password)
	if err != nil {
		t.Fatalf("AuthenticateUser returned error: %v", err)
	}
	if authenticated.ID != user.ID {
		t.Fatalf("expected authenticated user %s, got %s", user.ID, authenticated.ID)
	}

	if _, err := store.AuthenticateUser("viewer@example.com", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid password error, got %v", err)
	}
	if _, err := store.AuthenticateUser("unknown@example.com", password); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected unknown email to return ErrInvalidCredentials, got %v", err)
	}

	reloaded, err := NewStorage(store.filePath)
	if err != nil {
		t.Fatalf("reload storage: %v", err)
	}
	persisted, ok := reloaded.FindUserByEmail("viewer@example.com")
	if !ok {
		t.Fatal("expected persisted user to be found after reload")
	}
	if persisted.PasswordHash != user.PasswordHash {
		t.Fatalf("expected password hash to persist across reloads")
	}
	if _, err := reloaded.AuthenticateUser("viewer@example.com", password); err != nil {
		t.Fatalf("AuthenticateUser on reloaded store returned error: %v", err)
	}
}

func TestSetUserPassword(t *testing.T) {
	store := newTestStore(t)
	email := "admin@example.com"
	originalPassword := "initialP@ss"
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Admin",
		Email:       email,
		Password:    originalPassword,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if _, err := store.AuthenticateUser(email, originalPassword); err != nil {
		t.Fatalf("AuthenticateUser with original password: %v", err)
	}

	newPassword := "Sup3rSecret!"
	updated, err := store.SetUserPassword(user.ID, newPassword)
	if err != nil {
		t.Fatalf("SetUserPassword: %v", err)
	}
	if updated.PasswordHash == "" {
		t.Fatalf("expected password hash to be set")
	}
	if verifyErr := verifyPassword(updated.PasswordHash, newPassword); verifyErr != nil {
		t.Fatalf("verifyPassword for new password: %v", verifyErr)
	}

	if _, err := store.AuthenticateUser(email, originalPassword); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials for old password, got %v", err)
	}
	if _, err := store.AuthenticateUser(email, newPassword); err != nil {
		t.Fatalf("AuthenticateUser with new password: %v", err)
	}

	persisted, ok := store.GetUser(user.ID)
	if !ok {
		t.Fatal("expected updated user to exist")
	}
	if persisted.PasswordHash != updated.PasswordHash {
		t.Fatalf("expected password hash to persist, got %q vs %q", persisted.PasswordHash, updated.PasswordHash)
	}
}

func TestSetUserPasswordValidatesLength(t *testing.T) {
	store := newTestStore(t)
	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	if _, err := store.SetUserPassword(user.ID, "short"); err == nil {
		t.Fatal("expected error for short password")
	}
}
