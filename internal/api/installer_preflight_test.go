package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitriver-live/internal/domain"
)

type installerPreflightStub struct {
	req    InstallerPreflightRequest
	resp   InstallerPreflightResponse
	err    error
	called bool
}

func (s *installerPreflightStub) Check(_ context.Context, req InstallerPreflightRequest) (InstallerPreflightResponse, error) {
	s.called = true
	s.req = req
	return s.resp, s.err
}

type fakeFileInfo struct {
	name string
	dir  bool
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { if f.dir { return fs.ModeDir | 0o755 }; return 0o644 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() interface{}   { return nil }

func TestInstallerPreflightRequiresAdmin(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.InstallerPreflightService = &installerPreflightStub{}

	req := httptest.NewRequest(http.MethodPost, "/api/install/preflight", bytes.NewBufferString(`{}`))
	rr := httptest.NewRecorder()

	handler.InstallerPreflight(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", rr.Code)
	}
}

func TestInstallerPreflightBlocksNonAdmins(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.InstallerPreflightService = &installerPreflightStub{}

	user := domain.User{ID: "viewer-1", DisplayName: "Viewer", Roles: []string{"viewer"}}
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/install/preflight", bytes.NewBufferString(`{}`)), user)
	rr := httptest.NewRecorder()

	handler.InstallerPreflight(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden, got %d", rr.Code)
	}
}

func TestInstallerPreflightValidatesPayload(t *testing.T) {
	handler, _ := newTestHandler(t)
	handler.InstallerPreflightService = &installerPreflightStub{}

	user := domain.User{ID: "admin-1", DisplayName: "Admin", Roles: []string{"admin"}}
	req := withUser(httptest.NewRequest(http.MethodPost, "/api/install/preflight", bytes.NewBufferString("{")), user)
	rr := httptest.NewRecorder()

	handler.InstallerPreflight(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 bad request, got %d", rr.Code)
	}
}

func TestInstallerPreflightSuccess(t *testing.T) {
	handler, _ := newTestHandler(t)
	stub := &installerPreflightStub{
		resp: InstallerPreflightResponse{
			Status:    "warning",
			CheckedAt: "2026-03-28T12:00:00Z",
			Checks: []InstallerPreflightCheck{
				{ID: "supported-target", Title: "Supported target", Status: "pass", Summary: "Ubuntu detected"},
			},
		},
	}
	handler.InstallerPreflightService = stub

	user := domain.User{ID: "admin-1", DisplayName: "Admin", Roles: []string{"admin"}}
	body := map[string]any{
		"installDir": "/opt/bitriver-live",
		"dataDir":    "/var/lib/bitriver-live",
		"addr":       ":8080",
		"extraField": "ignored",
	}
	raw, _ := json.Marshal(body)

	req := withUser(httptest.NewRequest(http.MethodPost, "/api/install/preflight", bytes.NewBuffer(raw)), user)
	rr := httptest.NewRecorder()

	handler.InstallerPreflight(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 ok, got %d", rr.Code)
	}
	if !stub.called {
		t.Fatal("expected preflight checker to be called")
	}
	if stub.req.InstallDir != "/opt/bitriver-live" || stub.req.Addr != ":8080" {
		t.Fatalf("unexpected preflight request passed to checker: %+v", stub.req)
	}

	var resp InstallerPreflightResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "warning" || len(resp.Checks) != 1 {
		t.Fatalf("unexpected response payload: %+v", resp)
	}
}

func TestHostInstallerPreflightPassesUbuntuQuickDefaults(t *testing.T) {
	checker := hostInstallerPreflightChecker{
		lookPath: func(name string) (string, error) {
			return "/usr/bin/" + name, nil
		},
		stat: func(path string) (fs.FileInfo, error) {
			if path == "/run/systemd/system" {
				return fakeFileInfo{name: "system", dir: true}, nil
			}
			return nil, fs.ErrNotExist
		},
		readFile: func(string) ([]byte, error) {
			return []byte("ID=ubuntu\nVERSION_ID=\"22.04\"\n"), nil
		},
		dialAddress: func(context.Context, string, string) error {
			return nil
		},
		goos: "linux",
		now: func() time.Time {
			return time.Date(2026, time.March, 28, 12, 0, 0, 0, time.UTC)
		},
	}

	resp, err := checker.Check(context.Background(), InstallerPreflightRequest{})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if resp.Status != "pass" {
		t.Fatalf("expected overall pass, got %s", resp.Status)
	}

	checks := map[string]InstallerPreflightCheck{}
	for _, check := range resp.Checks {
		checks[check.ID] = check
	}

	if checks["supported-target"].Status != "pass" {
		t.Fatalf("expected supported-target pass, got %+v", checks["supported-target"])
	}
	if checks["port-readiness"].Status != "pass" {
		t.Fatalf("expected port-readiness pass, got %+v", checks["port-readiness"])
	}
	if checks["external-services"].Status != "pass" {
		t.Fatalf("expected external-services pass, got %+v", checks["external-services"])
	}
}

func TestHostInstallerPreflightFlagsPrivilegedPortAndServiceFailures(t *testing.T) {
	checker := hostInstallerPreflightChecker{
		lookPath: func(name string) (string, error) {
			switch name {
			case "bash", "curl", "sudo", "systemctl":
				return "/usr/bin/" + name, nil
			case "setcap":
				return "", errors.New("setcap missing")
			default:
				return "", errors.New("missing")
			}
		},
		stat: func(path string) (fs.FileInfo, error) {
			switch path {
			case "/run/systemd/system":
				return fakeFileInfo{name: "system", dir: true}, nil
			default:
				return nil, fs.ErrNotExist
			}
		},
		readFile: func(string) ([]byte, error) {
			return []byte("ID=ubuntu\nVERSION_ID=\"24.04\"\n"), nil
		},
		dialAddress: func(_ context.Context, _ string, address string) error {
			return errors.New("dial " + address + ": connection refused")
		},
		goos: "linux",
		now:  time.Now,
	}

	resp, err := checker.Check(context.Background(), InstallerPreflightRequest{
		Addr:          ":80",
		StorageDriver: "postgres",
		PostgresDsn:   "postgres://db.example.com:5432/bitriver?sslmode=disable",
		RedisAddr:     "redis.example.com:6379",
		TLSCert:       "/etc/ssl/certs/bitriver.pem",
	})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if resp.Status != "fail" {
		t.Fatalf("expected overall fail, got %s", resp.Status)
	}

	checks := map[string]InstallerPreflightCheck{}
	for _, check := range resp.Checks {
		checks[check.ID] = check
	}

	if checks["port-readiness"].Status != "fail" {
		t.Fatalf("expected port-readiness fail, got %+v", checks["port-readiness"])
	}
	if checks["external-services"].Status != "fail" {
		t.Fatalf("expected external-services fail, got %+v", checks["external-services"])
	}
	if checks["tls-assets"].Status != "fail" {
		t.Fatalf("expected tls-assets fail, got %+v", checks["tls-assets"])
	}
}
