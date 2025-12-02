package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitriver-live/internal/auth"
	"bitriver-live/internal/storage"
	"bitriver-live/internal/testsupport"
)

func TestAuthSessionLifecycle(t *testing.T) {
	store := newTestStorage(t)
	sessionStore := testsupport.NewSessionStoreStub()
	sessions := auth.NewSessionManager(30*time.Minute, auth.WithStore(sessionStore))
	handler := NewHandler(store, sessions)

	user, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Admin", Email: "admin@example.com", Password: "password123", Roles: []string{"admin"}})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	body := bytes.NewBufferString(`{"email":"admin@example.com","password":"password123"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	loginRec := httptest.NewRecorder()

	handler.Login(loginRec, loginReq)

	res := loginRec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected login status: %d", res.StatusCode)
	}

	cookie := findSessionCookie(t, res.Cookies())
	if cookie.Value == "" {
		t.Fatalf("expected session cookie value")
	}
	if cookie.MaxAge <= 0 {
		t.Fatalf("expected positive cookie max age, got %d", cookie.MaxAge)
	}

	var authResp authResponse
	if err := json.NewDecoder(res.Body).Decode(&authResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	if authResp.User.ID != user.ID {
		t.Fatalf("expected auth response user %s, got %s", user.ID, authResp.User.ID)
	}

	record, ok := sessionStore.Record(cookie.Value)
	if !ok {
		t.Fatalf("session token not persisted")
	}
	if time.Until(record.ExpiresAt) <= 0 {
		t.Fatalf("expected future session expiry, got %s", record.ExpiresAt)
	}

	refreshReq := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	refreshReq.AddCookie(cookie)
	refreshRec := httptest.NewRecorder()

	handler.Session(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusOK {
		t.Fatalf("unexpected session refresh status: %d", refreshRec.Code)
	}

	logoutReq := httptest.NewRequest(http.MethodDelete, "/api/session", nil)
	logoutReq.AddCookie(cookie)
	logoutRec := httptest.NewRecorder()

	handler.Session(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("unexpected logout status: %d", logoutRec.Code)
	}

	clearedCookie := findSessionCookie(t, logoutRec.Result().Cookies())
	if clearedCookie.MaxAge >= 0 {
		t.Fatalf("expected cleared cookie max age < 0, got %d", clearedCookie.MaxAge)
	}

	if _, ok := sessionStore.Record(cookie.Value); ok {
		t.Fatalf("expected session token to be revoked")
	}

	reusedReq := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	reusedReq.AddCookie(cookie)
	reusedRec := httptest.NewRecorder()
	handler.Session(reusedRec, reusedReq)
	if reusedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized after logout, got %d", reusedRec.Code)
	}
}

func TestAuthSessionIdleRefresh(t *testing.T) {
	store := newTestStorage(t)
	sessionStore := testsupport.NewSessionStoreStub()
	sessions := auth.NewSessionManager(10*time.Second, auth.WithStore(sessionStore), auth.WithIdleTimeout(2*time.Second))
	handler := NewHandler(store, sessions)

	_, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Admin", Email: "admin@example.com", Password: "password123", Roles: []string{"admin"}})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	body := bytes.NewBufferString(`{"email":"admin@example.com","password":"password123"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	loginRec := httptest.NewRecorder()
	handler.Login(loginRec, loginReq)

	loginRes := loginRec.Result()
	if loginRes.StatusCode != http.StatusOK {
		t.Fatalf("unexpected login status: %d", loginRes.StatusCode)
	}

	cookie := findSessionCookie(t, loginRes.Cookies())
	initialRecord, ok := sessionStore.Record(cookie.Value)
	if !ok {
		t.Fatalf("expected session to be stored")
	}

	time.Sleep(time.Second)
	refreshReq := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	refreshReq.AddCookie(cookie)
	refreshRec := httptest.NewRecorder()

	handler.Session(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusOK {
		t.Fatalf("unexpected refresh status: %d", refreshRec.Code)
	}

	refreshedCookie := findSessionCookie(t, refreshRec.Result().Cookies())
	if !refreshedCookie.Expires.After(cookie.Expires) {
		t.Fatalf("expected cookie expiry to move forward, got %v before %v", refreshedCookie.Expires, cookie.Expires)
	}

	refreshedRecord, ok := sessionStore.Record(cookie.Value)
	if !ok {
		t.Fatalf("expected session to remain stored")
	}
	if !refreshedRecord.ExpiresAt.After(initialRecord.ExpiresAt) {
		t.Fatalf("expected refreshed expiry after initial %v, got %v", initialRecord.ExpiresAt, refreshedRecord.ExpiresAt)
	}
	if refreshedRecord.AbsoluteExpiresAt != initialRecord.AbsoluteExpiresAt {
		t.Fatalf("expected absolute expiry to remain %v, got %v", initialRecord.AbsoluteExpiresAt, refreshedRecord.AbsoluteExpiresAt)
	}
}

func TestAuthInvalidCredentialsAndExpiredSession(t *testing.T) {
	store := newTestStorage(t)
	sessionStore := testsupport.NewSessionStoreStub()
	sessions := auth.NewSessionManager(5*time.Minute, auth.WithStore(sessionStore))
	handler := NewHandler(store, sessions)

	user, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	badLogin := bytes.NewBufferString(`{"email":"viewer@example.com","password":"wrong"}`)
	badReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", badLogin)
	badRec := httptest.NewRecorder()
	handler.Login(badRec, badReq)

	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for bad credentials, got %d", badRec.Code)
	}

	expiredToken := "expired-token"
	sessionStore.Seed(expiredToken, user.ID, time.Now().Add(-time.Hour))

	expiredReq := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	expiredReq.AddCookie(&http.Cookie{Name: "bitriver_session", Value: expiredToken})
	expiredRec := httptest.NewRecorder()
	handler.Session(expiredRec, expiredReq)

	if expiredRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for expired session, got %d", expiredRec.Code)
	}

	invalidReq := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	invalidReq.AddCookie(&http.Cookie{Name: "bitriver_session", Value: "bogus-token"})
	invalidRec := httptest.NewRecorder()
	handler.Session(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized for invalid session, got %d", invalidRec.Code)
	}
}

func TestProtectedEndpointPermissions(t *testing.T) {
	store := newTestStorage(t)
	sessionStore := testsupport.NewSessionStoreStub()
	sessions := auth.NewSessionManager(30*time.Minute, auth.WithStore(sessionStore))
	handler := NewHandler(store, sessions)

	admin, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Admin", Email: "admin@example.com", Password: "password123", Roles: []string{"admin"}})
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}
	viewer, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com", Password: "password123"})
	if err != nil {
		t.Fatalf("failed to create viewer: %v", err)
	}

	adminToken, _, err := handler.sessionManager().Create(admin.ID)
	if err != nil {
		t.Fatalf("failed to create admin session: %v", err)
	}
	viewerToken, _, err := handler.sessionManager().Create(viewer.ID)
	if err != nil {
		t.Fatalf("failed to create viewer session: %v", err)
	}

	viewerReq := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	viewerReq.AddCookie(&http.Cookie{Name: "bitriver_session", Value: viewerToken})
	viewerReq = authenticateRequest(t, handler, viewerReq)
	viewerRec := httptest.NewRecorder()
	handler.Users(viewerRec, viewerReq)

	if viewerRec.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden for viewer, got %d", viewerRec.Code)
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	adminReq.AddCookie(&http.Cookie{Name: "bitriver_session", Value: adminToken})
	adminReq = authenticateRequest(t, handler, adminReq)
	adminRec := httptest.NewRecorder()
	handler.Users(adminRec, adminReq)

	if adminRec.Code != http.StatusOK {
		t.Fatalf("expected OK for admin, got %d", adminRec.Code)
	}

	var users []userResponse
	if err := json.NewDecoder(adminRec.Body).Decode(&users); err != nil {
		t.Fatalf("failed to decode users response: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func authenticateRequest(t *testing.T, h *Handler, req *http.Request) *http.Request {
	t.Helper()
	user, _, err := h.AuthenticateRequest(req)
	if err != nil {
		t.Fatalf("failed to authenticate request: %v", err)
	}
	return req.WithContext(ContextWithUser(req.Context(), user))
}

func newTestStorage(t *testing.T) *storage.Storage {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.NewStorage(dir + "/store.json")
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}
	return store
}

func findSessionCookie(t *testing.T, cookies []*http.Cookie) *http.Cookie {
	t.Helper()
	for _, c := range cookies {
		if c.Name == "bitriver_session" {
			return c
		}
	}
	t.Fatalf("session cookie not found")
	return nil
}
