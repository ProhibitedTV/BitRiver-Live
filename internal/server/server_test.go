package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/auth"
	"bitriver-live/internal/storage"
	"bitriver-live/web"
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

func TestAuthMiddlewareAllowsExpiredSessionOnOptionalRoutes(t *testing.T) {
	handler, store := newTestHandler(t)
	owner, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Owner",
		Email:       "owner@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Lobby", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel error: %v", err)
	}
	token, _, err := handler.Sessions.Create(owner.ID)
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}
	if err := handler.Sessions.Revoke(token); err != nil {
		t.Fatalf("Revoke session: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/channels/%s", channel.ID), nil)
	req.AddCookie(&http.Cookie{Name: "bitriver_session", Value: token})
	rec := httptest.NewRecorder()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		handler.ChannelByID(w, r)
	})

	authMiddleware(handler, next).ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected middleware to call next handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == "bitriver_session" {
			if c.MaxAge == -1 {
				cleared = true
			} else {
				t.Fatalf("expected session cookie to be cleared, got MaxAge=%d", c.MaxAge)
			}
		}
	}
	if !cleared {
		t.Fatal("expected response to clear session cookie")
	}
}

func TestAuthMiddlewareRejectsInvalidSession(t *testing.T) {
	handler, _ := newTestHandler(t)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected call to next handler")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.AddCookie(&http.Cookie{Name: "bitriver_session", Value: "expired-token"})
	rec := httptest.NewRecorder()

	authMiddleware(handler, next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", rec.Code)
	}
}

func TestClientIPResolverIgnoresForwardedByDefault(t *testing.T) {
	resolver, err := newClientIPResolver(RateLimitConfig{})
	if err != nil {
		t.Fatalf("newClientIPResolver error: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")
	ip, source := resolver.ClientIPFromRequest(req)
	if ip != "198.51.100.10" {
		t.Fatalf("expected remote addr, got %q", ip)
	}
	if source != ipSourceRemoteAddr {
		t.Fatalf("expected source %q, got %q", ipSourceRemoteAddr, source)
	}
}

func TestClientIPResolverTrustsForwardedWhenEnabled(t *testing.T) {
	resolver, err := newClientIPResolver(RateLimitConfig{TrustForwardedHeaders: true})
	if err != nil {
		t.Fatalf("newClientIPResolver error: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:1111"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	ip, source := resolver.ClientIPFromRequest(req)
	if ip != "203.0.113.5" {
		t.Fatalf("expected first forwarded ip, got %q", ip)
	}
	if source != ipSourceXForwardedFor {
		t.Fatalf("expected source %q, got %q", ipSourceXForwardedFor, source)
	}
}

func TestClientIPResolverTrustedProxyCIDR(t *testing.T) {
	resolver, err := newClientIPResolver(RateLimitConfig{TrustedProxies: []string{"10.0.0.0/8"}})
	if err != nil {
		t.Fatalf("newClientIPResolver error: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.1.2.3:5555"
	req.Header.Set("X-Real-IP", "203.0.113.10")
	ip, source := resolver.ClientIPFromRequest(req)
	if ip != "203.0.113.10" {
		t.Fatalf("expected real ip header, got %q", ip)
	}
	if source != ipSourceXRealIP {
		t.Fatalf("expected source %q, got %q", ipSourceXRealIP, source)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.RemoteAddr = "198.51.100.20:4444"
	req2.Header.Set("X-Forwarded-For", "203.0.113.11")
	ip2, source2 := resolver.ClientIPFromRequest(req2)
	if ip2 != "198.51.100.20" {
		t.Fatalf("expected remote addr for untrusted proxy, got %q", ip2)
	}
	if source2 != ipSourceRemoteAddr {
		t.Fatalf("expected source %q, got %q", ipSourceRemoteAddr, source2)
	}
}

func TestSPAHandlerServesSignup(t *testing.T) {
	staticFS, err := web.Static()
	if err != nil {
		t.Fatalf("Static error: %v", err)
	}
	index, err := fs.ReadFile(staticFS, "index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}

	handler := spaHandler(staticFS, index, http.FileServer(http.FS(staticFS)))

	req := httptest.NewRequest(http.MethodGet, "/signup", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `<form id="signup-form"`) {
		t.Fatalf("expected signup form markup in response, got %q", body)
	}
}

func TestRateLimitMiddlewareSpoofedHeadersIgnoredByDefault(t *testing.T) {
	rl, err := newRateLimiter(RateLimitConfig{LoginLimit: 1, LoginWindow: time.Minute})
	if err != nil {
		t.Fatalf("newRateLimiter error: %v", err)
	}
	resolver, err := newClientIPResolver(RateLimitConfig{})
	if err != nil {
		t.Fatalf("newClientIPResolver error: %v", err)
	}
	handler := rateLimitMiddleware(rl, resolver, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req1.RemoteAddr = "198.51.100.1:1234"
	req1.Header.Set("X-Forwarded-For", "203.0.113.1")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusNoContent {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req2.RemoteAddr = "198.51.100.1:5678"
	req2.Header.Set("X-Forwarded-For", "203.0.113.2")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be throttled, got %d", rec2.Code)
	}
}

func TestRateLimitMiddlewareHonorsTrustedForwardedHeaders(t *testing.T) {
	rl, err := newRateLimiter(RateLimitConfig{LoginLimit: 1, LoginWindow: time.Minute})
	if err != nil {
		t.Fatalf("newRateLimiter error: %v", err)
	}
	resolver, err := newClientIPResolver(RateLimitConfig{TrustedProxies: []string{"10.0.0.0/8"}})
	if err != nil {
		t.Fatalf("newClientIPResolver error: %v", err)
	}
	handler := rateLimitMiddleware(rl, resolver, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req1.RemoteAddr = "10.1.2.3:9999"
	req1.Header.Set("X-Forwarded-For", "203.0.113.50")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusNoContent {
		t.Fatalf("expected first request to succeed, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req2.RemoteAddr = "10.1.2.3:10000"
	req2.Header.Set("X-Forwarded-For", "203.0.113.50")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request to be throttled, got %d", rec2.Code)
	}
}
