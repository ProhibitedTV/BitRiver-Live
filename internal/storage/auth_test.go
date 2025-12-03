package storage

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestCreateAndListUser(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected user ID to be set")
	}

	users := store.ListUsers()
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", users[0].Email)
	}
}

func TestAuthenticateOAuthCreatesUser(t *testing.T) {
	store := newTestStore(t)

	user, err := store.AuthenticateOAuth(OAuthLoginParams{
		Provider:    "example",
		Subject:     "subject-1",
		Email:       "viewer@example.com",
		DisplayName: "Viewer",
	})
	if err != nil {
		t.Fatalf("AuthenticateOAuth returned error: %v", err)
	}
	if user.ID == "" {
		t.Fatal("expected user id to be assigned")
	}
	if user.Email != "viewer@example.com" {
		t.Fatalf("expected normalized email, got %s", user.Email)
	}
	if !user.SelfSignup {
		t.Fatal("expected OAuth-created user to be marked as self signup")
	}
	if len(user.Roles) != 1 || user.Roles[0] != "viewer" {
		t.Fatalf("expected viewer role for OAuth user, got %v", user.Roles)
	}

	fetched, ok := store.FindUserByEmail("viewer@example.com")
	if !ok || fetched.ID != user.ID {
		t.Fatalf("expected user to be persisted, got %+v", fetched)
	}

	again, err := store.AuthenticateOAuth(OAuthLoginParams{Provider: "example", Subject: "subject-1"})
	if err != nil {
		t.Fatalf("AuthenticateOAuth second call returned error: %v", err)
	}
	if again.ID != user.ID {
		t.Fatalf("expected existing account to be reused, got %s", again.ID)
	}
}

func TestAuthenticateOAuthLinksExistingUser(t *testing.T) {
	store := newTestStore(t)

	existing, err := store.CreateUser(CreateUserParams{DisplayName: "Existing", Email: "linked@example.com", Roles: []string{"creator"}})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	linked, err := store.AuthenticateOAuth(OAuthLoginParams{Provider: "example", Subject: "subject-2", Email: "linked@example.com", DisplayName: "Viewer"})
	if err != nil {
		t.Fatalf("AuthenticateOAuth returned error: %v", err)
	}
	if linked.ID != existing.ID {
		t.Fatalf("expected OAuth login to link to existing user, got %s", linked.ID)
	}
}

func TestAuthenticateOAuthGeneratesFallbackEmail(t *testing.T) {
	store := newTestStore(t)

	user, err := store.AuthenticateOAuth(OAuthLoginParams{Provider: "acme", Subject: "unique"})
	if err != nil {
		t.Fatalf("AuthenticateOAuth returned error: %v", err)
	}
	if !strings.HasSuffix(user.Email, "@acme.oauth") {
		t.Fatalf("expected fallback email with domain, got %s", user.Email)
	}
	if user.DisplayName == "" {
		t.Fatal("expected fallback display name to be set")
	}
}

func TestUpdateAndDeleteUser(t *testing.T) {
	store := newTestStore(t)

	user, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	newDisplay := "Alice Cooper"
	newEmail := "alice.cooper@example.com"
	newRoles := []string{"Admin", "moderator", "admin"}
	updated, err := store.UpdateUser(user.ID, UserUpdate{DisplayName: &newDisplay, Email: &newEmail, Roles: &newRoles})
	if err != nil {
		t.Fatalf("UpdateUser returned error: %v", err)
	}
	if updated.DisplayName != newDisplay {
		t.Fatalf("expected display name %q, got %q", newDisplay, updated.DisplayName)
	}
	if updated.Email != "alice.cooper@example.com" {
		t.Fatalf("expected email normalized, got %s", updated.Email)
	}
	if len(updated.Roles) != 2 {
		t.Fatalf("expected deduplicated roles, got %v", updated.Roles)
	}

	if err := store.DeleteUser(user.ID); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	if _, ok := store.GetUser(user.ID); ok {
		t.Fatalf("expected user to be removed")
	}
}

func TestUpdateUserPersistFailureLeavesDataUntouched(t *testing.T) {
	store := newTestStore(t)

	original, err := store.CreateUser(CreateUserParams{
		DisplayName: "Alice",
		Email:       "alice@example.com",
		Roles:       []string{"creator"},
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	newEmail := "changed@example.com"
	store.persistOverride = func(dataset) error {
		return errors.New("persist failed")
	}

	if _, err := store.UpdateUser(original.ID, UserUpdate{Email: &newEmail}); err == nil {
		t.Fatalf("expected UpdateUser error when persist fails")
	}

	store.persistOverride = nil

	current, ok := store.GetUser(original.ID)
	if !ok {
		t.Fatalf("expected user %s to remain", original.ID)
	}
	if current.Email != original.Email {
		t.Fatalf("expected email %s, got %s", original.Email, current.Email)
	}
}

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

func TestRepositoryOAuthLinking(t *testing.T) {
	RunRepositoryOAuthLinking(t, jsonRepositoryFactory)
}
