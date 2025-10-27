package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/auth"
	"bitriver-live/internal/storage"
)

func newTestHandler(t *testing.T) (*api.Handler, *storage.Storage) {
	t.Helper()
	dir := t.TempDir()
	storePath := filepath.Join(dir, "store.json")
	store, err := storage.NewStorage(storePath)
	if err != nil {
		t.Fatalf("NewStorage error: %v", err)
	}
	sessions := auth.NewSessionManager(time.Hour)
	return api.NewHandler(store, sessions), store
}

func TestAuthMiddlewareAcceptsCookie(t *testing.T) {
	handler, store := newTestHandler(t)
	user, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Tester",
		Email:       "tester@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	token, _, err := handler.Sessions.Create(user.ID)
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		ctxUser, ok := api.UserFromContext(r.Context())
		if !ok {
			t.Fatal("expected user in context")
		}
		if ctxUser.ID != user.ID {
			t.Fatalf("expected user %s, got %s", user.ID, ctxUser.ID)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "bitriver_session", Value: token})
	rec := httptest.NewRecorder()

	authMiddleware(handler, next).ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected middleware to call next handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestAuthMiddlewareRejectsMissingSession(t *testing.T) {
	handler, _ := newTestHandler(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected call to next handler")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rec := httptest.NewRecorder()

	authMiddleware(handler, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] == "" {
		t.Fatal("expected error message in response")
	}
}
