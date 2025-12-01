package server

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSMiddlewareAllowsConfiguredOrigins(t *testing.T) {
	policy, err := newCORSPolicy(CORSConfig{AdminOrigins: []string{"https://admin.example.com"}})
	if err != nil {
		t.Fatalf("newCORSPolicy error: %v", err)
	}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Origin", "https://admin.example.com")
	req.Host = "api.example.com"
	rec := httptest.NewRecorder()

	corsMiddleware(policy, nil, next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://admin.example.com" {
		t.Fatalf("unexpected allow origin header: %q", got)
	}
}

func TestCORSMiddlewareAllowsPreflightForViewerOrigin(t *testing.T) {
	policy, err := newCORSPolicy(CORSConfig{ViewerOrigins: []string{"https://viewer.example.com"}})
	if err != nil {
		t.Fatalf("newCORSPolicy error: %v", err)
	}

	req := httptest.NewRequest(http.MethodOptions, "/api/directory", nil)
	req.Header.Set("Origin", "https://viewer.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")
	req.Host = "api.example.com"
	rec := httptest.NewRecorder()

	corsMiddleware(policy, nil, http.NotFoundHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for preflight, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("expected allow methods to be set")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Fatalf("unexpected allow headers: %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://viewer.example.com" {
		t.Fatalf("unexpected allow origin: %q", got)
	}
}

func TestCORSMiddlewareBlocksUnknownOrigin(t *testing.T) {
	policy, err := newCORSPolicy(CORSConfig{})
	if err != nil {
		t.Fatalf("newCORSPolicy error: %v", err)
	}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	req.Host = "api.example.com"
	rec := httptest.NewRecorder()

	corsMiddleware(policy, nil, next).ServeHTTP(rec, req)

	if called {
		t.Fatal("expected request to be blocked before reaching next handler")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed origin, got %d", rec.Code)
	}
}

func TestCORSMiddlewareAllowsSameOriginByDefault(t *testing.T) {
	policy, err := newCORSPolicy(CORSConfig{})
	if err != nil {
		t.Fatalf("newCORSPolicy error: %v", err)
	}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Host = "example.com"
	rec := httptest.NewRecorder()

	corsMiddleware(policy, nil, next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected same-origin request to reach next handler")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://example.com" {
		t.Fatalf("expected allow origin header for same-origin request, got %q", got)
	}
}

func TestCORSMiddlewareBlocksMismatchedSchemeForSameHost(t *testing.T) {
	policy, err := newCORSPolicy(CORSConfig{})
	if err != nil {
		t.Fatalf("newCORSPolicy error: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Host = "example.com"
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()

	corsMiddleware(policy, nil, http.NotFoundHandler()).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when origin scheme differs from request, got %d", rec.Code)
	}
}
