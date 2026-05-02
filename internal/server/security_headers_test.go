package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestSecurityHeadersMiddlewareUsesDefaults(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)

	middleware := securityHeadersMiddleware(SecurityConfig{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	middleware.ServeHTTP(rec, req)

	res := rec.Result()
	assertDefaultSecurityHeaders(t, res)
}

func TestSecurityHeadersMiddlewareSkipsCSPOnViewerRoutes(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/viewer", nil)

	middleware := securityHeadersMiddleware(SecurityConfig{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	middleware.ServeHTTP(rec, req)

	res := rec.Result()
	assertHeaderMissing(t, res, "Content-Security-Policy")
	assertHeaderEquals(t, res, "X-Frame-Options", defaultFrameOptions)
	assertHeaderEquals(t, res, "Referrer-Policy", defaultReferrerPolicy)
	assertHeaderEquals(t, res, "Permissions-Policy", defaultPermissionsPolicy)
	assertHeaderEquals(t, res, "X-Content-Type-Options", defaultContentTypeOptions)
}

func TestSecurityHeadersCanBeOverridden(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)

	cfg := SecurityConfig{
		ContentSecurityPolicy: "default-src 'self' https://cdn.example.com",
		FrameOptions:          "SAMEORIGIN",
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy:     "geolocation=(self)",
		ContentTypeOptions:    "nosniff",
	}
	middleware := securityHeadersMiddleware(cfg, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	middleware.ServeHTTP(rec, req)

	res := rec.Result()
	assertHeaderEquals(t, res, "Content-Security-Policy", cfg.ContentSecurityPolicy)
	assertHeaderEquals(t, res, "X-Frame-Options", cfg.FrameOptions)
	assertHeaderEquals(t, res, "Referrer-Policy", cfg.ReferrerPolicy)
	assertHeaderEquals(t, res, "Permissions-Policy", cfg.PermissionsPolicy)
	assertHeaderEquals(t, res, "X-Content-Type-Options", cfg.ContentTypeOptions)
}

func TestServerAppliesSecurityHeadersToAdminAndRootRoutes(t *testing.T) {
	handler, _ := newTestHandler(t)

	srv, err := New(handler, Config{
		Addr:      "127.0.0.1:0",
		TLS:       TLSConfig{},
		RateLimit: RateLimitConfig{},
		CORS:      CORSConfig{},
		Security:  SecurityConfig{},
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "admin", path: "/healthz"},
		{name: "root", path: "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			srv.httpServer.Handler.ServeHTTP(rec, req)

			res := rec.Result()
			assertDefaultSecurityHeaders(t, res)
			if res.StatusCode == 0 {
				t.Fatalf("expected status code on %s", tc.path)
			}
		})
	}
}

func TestServerAppliesViewerSecurityHeadersToProxiedViewerRoutes(t *testing.T) {
	t.Parallel()

	handler, _ := newTestHandler(t)
	viewerOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><html><body>viewer</body></html>"))
	}))
	defer viewerOrigin.Close()

	originURL, err := url.Parse(viewerOrigin.URL)
	if err != nil {
		t.Fatalf("parse viewer origin: %v", err)
	}

	srv, err := New(handler, Config{
		Addr:         "127.0.0.1:0",
		TLS:          TLSConfig{},
		RateLimit:    RateLimitConfig{},
		CORS:         CORSConfig{},
		Security:     SecurityConfig{},
		ViewerOrigin: originURL,
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/viewer", nil)

	srv.httpServer.Handler.ServeHTTP(rec, req)

	res := rec.Result()
	assertViewerSecurityHeaders(t, res)
}

func TestServerAppliesConfiguredSecurityHeaders(t *testing.T) {
	handler, _ := newTestHandler(t)

	customHeaders := SecurityConfig{
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'self'", // non-default to prove config is used
		FrameOptions:          "SAMEORIGIN",
		ReferrerPolicy:        "same-origin",
		PermissionsPolicy:     "geolocation=(self)",
		ContentTypeOptions:    "nosniff",
	}

	srv, err := New(handler, Config{
		Addr:      "127.0.0.1:0",
		TLS:       TLSConfig{},
		RateLimit: RateLimitConfig{},
		CORS:      CORSConfig{},
		Security:  customHeaders,
	})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	srv.httpServer.Handler.ServeHTTP(rec, req)

	res := rec.Result()
	assertHeaderEquals(t, res, "Content-Security-Policy", customHeaders.ContentSecurityPolicy)
	assertHeaderEquals(t, res, "X-Frame-Options", customHeaders.FrameOptions)
	assertHeaderEquals(t, res, "Referrer-Policy", customHeaders.ReferrerPolicy)
	assertHeaderEquals(t, res, "Permissions-Policy", customHeaders.PermissionsPolicy)
	assertHeaderEquals(t, res, "X-Content-Type-Options", customHeaders.ContentTypeOptions)
}

func assertDefaultSecurityHeaders(t *testing.T, res *http.Response) {
	t.Helper()
	assertHeaderEquals(t, res, "Content-Security-Policy", defaultContentSecurityPolicy(defaultFrameAncestors))
	assertHeaderEquals(t, res, "X-Frame-Options", defaultFrameOptions)
	assertHeaderEquals(t, res, "Referrer-Policy", defaultReferrerPolicy)
	assertHeaderEquals(t, res, "Permissions-Policy", defaultPermissionsPolicy)
	assertHeaderEquals(t, res, "X-Content-Type-Options", defaultContentTypeOptions)
}

func assertViewerSecurityHeaders(t *testing.T, res *http.Response) {
	t.Helper()
	assertHeaderEquals(t, res, "Content-Security-Policy", defaultViewerContentSecurityPolicy(defaultFrameAncestors))
	assertHeaderEquals(t, res, "X-Frame-Options", defaultFrameOptions)
	assertHeaderEquals(t, res, "Referrer-Policy", defaultReferrerPolicy)
	assertHeaderEquals(t, res, "Permissions-Policy", defaultPermissionsPolicy)
	assertHeaderEquals(t, res, "X-Content-Type-Options", defaultContentTypeOptions)
}

func assertHeaderEquals(t *testing.T, res *http.Response, key, expected string) {
	t.Helper()
	if got := res.Header.Get(key); got != expected {
		t.Fatalf("expected %s=%q, got %q", key, expected, got)
	}
}

func assertHeaderMissing(t *testing.T, res *http.Response, key string) {
	t.Helper()
	if got := res.Header.Get(key); got != "" {
		t.Fatalf("expected %s to be unset, got %q", key, got)
	}
}
