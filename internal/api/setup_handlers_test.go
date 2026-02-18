package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bitriver-live/internal/domain"
)

type setupStub struct {
	cfg    SetupConfig
	result SetupResult
	err    error
	called bool
}

func (s *setupStub) ApplySetup(_ context.Context, cfg SetupConfig) (SetupResult, error) {
	s.called = true
	s.cfg = cfg
	return s.result, s.err
}

func TestSetupWizardRequiresAdmin(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.Setup = &setupStub{result: SetupResult{RestartScheduled: true}}

	req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewBufferString("{}"))
	rr := httptest.NewRecorder()

	handler.SetupWizard(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", rr.Code)
	}
}

func TestSetupWizardBlocksNonAdmins(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.Setup = &setupStub{result: SetupResult{RestartScheduled: true}}

	user := domain.User{ID: "viewer-1", DisplayName: "Viewer", Roles: []string{"viewer"}}
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewBufferString("{}")), user)
	rr := httptest.NewRecorder()

	handler.SetupWizard(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden, got %d", rr.Code)
	}
}

func TestSetupWizardValidatesPayload(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.Setup = &setupStub{result: SetupResult{RestartScheduled: true}}

	user := domain.User{ID: "admin-1", DisplayName: "Admin", Roles: []string{"admin"}}
	body := map[string]any{
		"adminEmail":       "not-an-email",
		"viewerUrl":        "https://viewer.example.com",
		"apiPort":          8080,
		"postgresPassword": "postgres-pass",
		"redisPassword":    "redis-pass",
		"srsToken":         "srs",
		"omeToken":         "ome",
		"transcoderToken":  "trans",
	}
	raw, _ := json.Marshal(body)

	req := withUser(httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewBuffer(raw)), user)
	rr := httptest.NewRecorder()

	handler.SetupWizard(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request, got %d", rr.Code)
	}
}

func TestSetupWizardSuccess(t *testing.T) {
	handler, _ := newTestHandler(t)
	stub := &setupStub{result: SetupResult{RestartScheduled: true}}
	handler.Setup = stub

	user := domain.User{ID: "admin-1", DisplayName: "Admin", Roles: []string{"admin"}}
	body := map[string]any{
		"adminEmail":       "admin@example.com",
		"viewerUrl":        "https://viewer.example.com",
		"publicApiUrl":     "https://api.example.com",
		"viewerOrigin":     "https://viewer.internal",
		"apiPort":          8081,
		"tlsCertPath":      "/etc/ssl/certs/example.crt",
		"tlsKeyPath":       "/etc/ssl/private/example.key",
		"postgresPassword": "postgres-pass",
		"redisPassword":    "redis-pass",
		"metricsToken":     "metrics-secret",
		"srsToken":         "srs-token",
		"omeToken":         "ome-token",
		"transcoderToken":  "transcoder-token",
	}
	raw, _ := json.Marshal(body)

	req := withUser(httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewBuffer(raw)), user)
	rr := httptest.NewRecorder()

	handler.SetupWizard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d", rr.Code)
	}
	if !stub.called {
		t.Fatalf("expected setup manager to be called")
	}
	if stub.cfg.AdminEmail != "admin@example.com" || stub.cfg.APIPort != 8081 {
		t.Fatalf("unexpected config passed to setup manager: %+v", stub.cfg)
	}
}

func TestSetupWizardMissingManager(t *testing.T) {
	handler, _ := newTestHandler(t)
	user := domain.User{ID: "admin-1", DisplayName: "Admin", Roles: []string{"admin"}}
	body := map[string]any{
		"adminEmail":       "admin@example.com",
		"viewerUrl":        "https://viewer.example.com",
		"apiPort":          8080,
		"postgresPassword": "postgres-pass",
		"redisPassword":    "redis-pass",
		"srsToken":         "srs-token",
		"omeToken":         "ome-token",
		"transcoderToken":  "transcoder-token",
	}
	raw, _ := json.Marshal(body)

	req := withUser(httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewBuffer(raw)), user)
	rr := httptest.NewRecorder()

	handler.SetupWizard(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 service unavailable, got %d", rr.Code)
	}
}

func TestParseURL(t *testing.T) {
	if _, err := parseURL("http://example.com/path"); err != nil {
		t.Fatalf("expected parseURL to succeed: %v", err)
	}
	if _, err := parseURL("//missing-scheme"); err == nil {
		t.Fatalf("expected parseURL to fail when scheme missing")
	}
}
