package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bitriver-live/internal/api"
	"bitriver-live/internal/auth"
	"bitriver-live/internal/chat"
	"bitriver-live/internal/observability/metrics"
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

func TestNewReturnsErrorWhenHandlerNil(t *testing.T) {
	t.Parallel()

	srv, err := New(nil, Config{})
	if err == nil {
		t.Fatalf("expected error when handler is nil, got server: %#v", srv)
	}
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

func TestAuthMiddlewareAllowsSRSHookWithoutSession(t *testing.T) {
	handler, _ := newTestHandler(t)
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/ingest/srs-hook", nil)
	rec := httptest.NewRecorder()

	authMiddleware(handler, next).ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatal("expected middleware to allow SRS hook without session")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", rec.Code)
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

func TestAuthMiddlewareAllowsUnauthenticatedProfileGet(t *testing.T) {
	handler, store := newTestHandler(t)
	user, err := store.CreateUser(storage.CreateUserParams{
		DisplayName: "Viewer",
		Email:       "viewer@example.com",
	})
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}

	profilePath := fmt.Sprintf("/api/profiles/%s", user.ID)

	getNextCalled := false
	getNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		getNextCalled = true
		handler.ProfileByID(w, r)
	})

	getReq := httptest.NewRequest(http.MethodGet, profilePath, nil)
	getRec := httptest.NewRecorder()

	authMiddleware(handler, getNext).ServeHTTP(getRec, getReq)

	if !getNextCalled {
		t.Fatal("expected middleware to call next handler for profile GET")
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", getRec.Code)
	}

	putNextCalled := false
	putNext := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putNextCalled = true
		handler.ProfileByID(w, r)
	})

	putReq := httptest.NewRequest(http.MethodPut, profilePath, strings.NewReader(`{}`))
	putRec := httptest.NewRecorder()

	authMiddleware(handler, putNext).ServeHTTP(putRec, putReq)

	if putNextCalled {
		t.Fatal("expected middleware not to call next handler for profile PUT without auth")
	}
	if putRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", putRec.Code)
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

type hijackableResponseRecorder struct {
	*httptest.ResponseRecorder
	conn      net.Conn
	rw        *bufio.ReadWriter
	handshake bytes.Buffer
	hijacked  bool
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func newHijackableResponseRecorder() (*hijackableResponseRecorder, net.Conn) {
	serverConn, clientConn := net.Pipe()
	recorder := &hijackableResponseRecorder{ResponseRecorder: httptest.NewRecorder(), conn: serverConn}
	writer := bufio.NewWriter(io.MultiWriter(&recorder.handshake, discardWriter{}))
	recorder.rw = bufio.NewReadWriter(bufio.NewReader(serverConn), writer)
	return recorder, clientConn
}

func (r *hijackableResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	r.hijacked = true
	return r.conn, r.rw, nil
}

func (r *hijackableResponseRecorder) Close() error {
	return r.conn.Close()
}

func TestChatWebsocketUpgradesThroughMiddleware(t *testing.T) {
	handler, store := newTestHandler(t)
	handler.ChatGateway = chat.NewGateway(chat.GatewayConfig{})

	user, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Viewer", Email: "viewer@example.com"})
	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	token, _, err := handler.Sessions.Create(user.ID)
	if err != nil {
		t.Fatalf("Create session: %v", err)
	}

	rl, err := newRateLimiter(RateLimitConfig{})
	if err != nil {
		t.Fatalf("newRateLimiter error: %v", err)
	}
	resolver, err := newClientIPResolver(RateLimitConfig{})
	if err != nil {
		t.Fatalf("newClientIPResolver error: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false}))
	auditLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{AddSource: false}))
	recorder := metrics.Default()

	handlerChain := http.Handler(http.HandlerFunc(handler.ChatWebsocket))
	handlerChain = authMiddleware(handler, handlerChain)
	handlerChain = rateLimitMiddleware(rl, resolver, logger, handlerChain)
	handlerChain = metricsMiddleware(recorder, handlerChain)
	handlerChain = auditMiddleware(auditLogger, resolver, handlerChain)
	handlerChain = loggingMiddleware(logger, resolver, handlerChain)

	req := httptest.NewRequest(http.MethodGet, "/api/chat/ws", nil)
	req.AddCookie(&http.Cookie{Name: "bitriver_session", Value: token})
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	rw, clientConn := newHijackableResponseRecorder()
	defer rw.Close()
	defer clientConn.Close()

	handlerChain.ServeHTTP(rw, req)

	if rw.Result().StatusCode == http.StatusBadRequest {
		t.Fatalf("expected websocket upgrade, got 400: %s", rw.Body.String())
	}
	if !rw.hijacked {
		t.Fatal("expected websocket handler to hijack the connection")
	}

	handshake := rw.handshake.String()
	if !strings.Contains(handshake, "101 Switching Protocols") {
		t.Fatalf("expected websocket upgrade, got %q", strings.TrimSpace(handshake))
	}
	lines := strings.Split(handshake, "\r\n")
	foundAccept := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(trimmed), strings.ToLower("Sec-WebSocket-Accept:")) {
			foundAccept = true
			break
		}
	}
	if !foundAccept {
		t.Fatalf("expected Sec-WebSocket-Accept header, got %q", handshake)
	}
}
