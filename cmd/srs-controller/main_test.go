package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestProxyRequestForwardsToUpstream(t *testing.T) {
	var receivedAuth string
	var receivedBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/channels" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.RawQuery != "foo=bar" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		receivedBody = string(body)
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"channel-1"}`))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL + "/api/")
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}

	ctrl := &controller{
		token:   "secret",
		client:  upstream.Client(),
		baseURL: upstreamURL,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/channels?foo=bar", strings.NewReader(`{"name":"demo"}`))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	ctrl.proxyRequest(rr, req)

	res := rr.Result()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", res.StatusCode)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if string(body) != `{"id":"channel-1"}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if receivedAuth != "" {
		t.Fatalf("expected upstream auth header to be stripped, got %q", receivedAuth)
	}
	if receivedBody != `{"name":"demo"}` {
		t.Fatalf("unexpected forwarded body: %s", receivedBody)
	}
}

func TestProxyRequestRejectsUnauthorized(t *testing.T) {
	ctrl := &controller{token: "secret"}

	req := httptest.NewRequest(http.MethodGet, "/v1/channels", nil)
	rr := httptest.NewRecorder()

	ctrl.proxyRequest(rr, req)

	res := rr.Result()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.StatusCode)
	}
	if res.Header.Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("expected WWW-Authenticate header")
	}
}

func TestAuthorizedChecksBearerPrefix(t *testing.T) {
	ctrl := &controller{token: "secret"}
	cases := []struct {
		name   string
		header string
		ok     bool
	}{
		{name: "missing", header: "", ok: false},
		{name: "wrong prefix", header: "Token secret", ok: false},
		{name: "empty token", header: "Bearer   ", ok: false},
		{name: "wrong token", header: "Bearer nope", ok: false},
		{name: "match", header: "Bearer secret", ok: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/channels", nil)
			req.Header.Set("Authorization", tc.header)
			if got := ctrl.authorized(req); got != tc.ok {
				t.Fatalf("authorized(%q)=%v, want %v", tc.header, got, tc.ok)
			}
		})
	}
}

func TestResolveTargetPreservesQuery(t *testing.T) {
	upstream, err := url.Parse("http://example.com/api/")
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	ctrl := &controller{baseURL: upstream}
	req := httptest.NewRequest(http.MethodGet, "/v1/channels/123?expand=true", nil)
	target := ctrl.resolveTarget(req)
	if target.String() != "http://example.com/api/v1/channels/123?expand=true" {
		t.Fatalf("unexpected target: %s", target)
	}
}

func TestHealthzColdStartUpstreamUnreachable(t *testing.T) {
	upstreamURL, err := url.Parse("http://127.0.0.1:1/api/")
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	ctrl := &controller{client: &http.Client{}, baseURL: upstreamURL}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	ctrl.healthz(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
	payload := decodeHealthPayload(t, rr.Body.Bytes())
	if payload["status"] != "degraded" {
		t.Fatalf("expected degraded status, got %v", payload["status"])
	}
	if payload["upstreamError"] == nil {
		t.Fatalf("expected upstreamError in payload")
	}
}

func TestHealthzUpstreamReachable(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/versions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL + "/api/")
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}

	ctrl := &controller{client: upstream.Client(), baseURL: upstreamURL}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	ctrl.healthz(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	payload := decodeHealthPayload(t, rr.Body.Bytes())
	if payload["status"] != "ok" {
		t.Fatalf("expected ok status, got %v", payload["status"])
	}
}

func TestHealthzTransientFailureRecovery(t *testing.T) {
	var mode atomic.Int32
	mode.Store(1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if mode.Load() == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL + "/api/")
	if err != nil {
		t.Fatalf("parse upstream: %v", err)
	}
	ctrl := &controller{client: upstream.Client(), baseURL: upstreamURL}

	mode.Store(0)
	first := httptest.NewRecorder()
	ctrl.healthz(first, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if first.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected first probe 503, got %d", first.Code)
	}

	mode.Store(1)
	ctrl.mu.Lock()
	ctrl.lastProbeTime = time.Time{}
	ctrl.mu.Unlock()

	second := httptest.NewRecorder()
	ctrl.healthz(second, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if second.Code != http.StatusOK {
		t.Fatalf("expected recovered probe 200, got %d", second.Code)
	}
	payload := decodeHealthPayload(t, second.Body.Bytes())
	if payload["status"] != "ok" {
		t.Fatalf("expected ok status after recovery, got %v", payload["status"])
	}
}

func decodeHealthPayload(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return payload
}
