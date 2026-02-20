package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/storage"
)

func TestServeUploadMediaLogsOpenError(t *testing.T) {
	originalOpen := openUploadMediaFile
	originalStat := statUploadMediaFile
	t.Cleanup(func() {
		openUploadMediaFile = originalOpen
		statUploadMediaFile = originalStat
	})

	h, _ := newTestHandler(t)
	mediaDir := t.TempDir()
	h.UploadMediaDir = mediaDir

	var logs bytes.Buffer
	h.Logger = slog.New(slog.NewTextHandler(&logs, nil))

	upload := domain.Upload{
		ID:        "upload-123",
		ChannelID: "channel-1",
		Metadata: map[string]string{
			"mediaToken":       "token-abc",
			"mediaPath":        "myfile.mp4",
			"uploadedFilename": "myfile.mp4",
			"contentType":      "video/mp4",
		},
	}

	openErr := errors.New("disk offline")
	fullPath := filepath.Join(mediaDir, "myfile.mp4")
	openUploadMediaFile = func(path string) (*os.File, error) {
		if path != fullPath {
			t.Fatalf("open path = %q, want %q", path, fullPath)
		}
		return nil, openErr
	}

	req := httptest.NewRequest(http.MethodGet, "/api/uploads/upload-123/media?token=token-abc", nil)
	resp := httptest.NewRecorder()

	h.serveUploadMedia(resp, req, upload)

	response := resp.Result()
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}

	errResp := decodeAPIError(t, body)
	if errResp.Error.Message != "media unavailable" {
		t.Fatalf("message = %q, want %q", errResp.Error.Message, "media unavailable")
	}
	if strings.Contains(errResp.Error.Message, "failed") {
		t.Fatalf("unexpected opaque failure message: %q", errResp.Error.Message)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "upload-123") {
		t.Fatalf("log missing upload id: %s", logOutput)
	}
	if !strings.Contains(logOutput, fullPath) {
		t.Fatalf("log missing media path: %s", logOutput)
	}
	if !strings.Contains(logOutput, openErr.Error()) {
		t.Fatalf("log missing wrapped error: %s", logOutput)
	}
}

func TestServeUploadMediaLogsStatError(t *testing.T) {
	originalOpen := openUploadMediaFile
	originalStat := statUploadMediaFile
	t.Cleanup(func() {
		openUploadMediaFile = originalOpen
		statUploadMediaFile = originalStat
	})

	h, _ := newTestHandler(t)
	mediaDir := t.TempDir()
	h.UploadMediaDir = mediaDir

	var logs bytes.Buffer
	h.Logger = slog.New(slog.NewTextHandler(&logs, nil))

	mediaPath := filepath.Join(mediaDir, "stat-file.mp4")
	if err := os.WriteFile(mediaPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("write media: %v", err)
	}

	upload := domain.Upload{
		ID:        "upload-456",
		ChannelID: "channel-1",
		Metadata: map[string]string{
			"mediaToken":       "token-xyz",
			"mediaPath":        "stat-file.mp4",
			"uploadedFilename": "stat-file.mp4",
			"contentType":      "video/mp4",
		},
	}

	statErr := errors.New("stat blocked")
	openUploadMediaFile = func(path string) (*os.File, error) {
		return os.Open(path)
	}
	statUploadMediaFile = func(file *os.File) (os.FileInfo, error) {
		return nil, statErr
	}

	req := httptest.NewRequest(http.MethodGet, "/api/uploads/upload-456/media?token=token-xyz", nil)
	resp := httptest.NewRecorder()

	h.serveUploadMedia(resp, req, upload)

	response := resp.Result()
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()

	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusInternalServerError)
	}

	errResp := decodeAPIError(t, body)
	if errResp.Error.Message != "unable to serve media" {
		t.Fatalf("message = %q, want %q", errResp.Error.Message, "unable to serve media")
	}
	if strings.Contains(errResp.Error.Message, "failed") {
		t.Fatalf("unexpected opaque failure message: %q", errResp.Error.Message)
	}

	logOutput := logs.String()
	if !strings.Contains(logOutput, "upload-456") {
		t.Fatalf("log missing upload id: %s", logOutput)
	}
	if !strings.Contains(logOutput, mediaPath) {
		t.Fatalf("log missing media path: %s", logOutput)
	}
	if !strings.Contains(logOutput, statErr.Error()) {
		t.Fatalf("log missing wrapped error: %s", logOutput)
	}
}

func TestUploadMediaURLRespectsForwardedHost(t *testing.T) {
	t.Run("trusted proxy honors forwarded headers", func(t *testing.T) {
		h := &Handler{
			TrustedProxies: []string{"10.0.0.0/8"},
		}
		req := httptest.NewRequest(http.MethodGet, "http://internal.local/api/uploads/upload-123/media", nil)
		req.RemoteAddr = "10.1.2.3:1234"
		req.Header.Set("X-Forwarded-Host", "cdn.example.com, proxy.local")
		req.Header.Set("X-Forwarded-Proto", "https")

		got := h.uploadMediaURL(req, "upload-123", "token-abc")
		want := "https://cdn.example.com/api/uploads/upload-123/media?token=token-abc"

		if got != want {
			t.Fatalf("url = %q, want %q", got, want)
		}
	})

	t.Run("untrusted proxy ignores forwarded headers", func(t *testing.T) {
		h := &Handler{
			TrustedProxies: []string{"10.0.0.0/8"},
		}
		req := httptest.NewRequest(http.MethodGet, "http://internal.local/api/uploads/upload-123/media", nil)
		req.RemoteAddr = "192.168.1.10:4321"
		req.Header.Set("Forwarded", `for=192.0.2.60; proto=https; host="media.example.com:8443"`)
		req.Header.Set("X-Forwarded-Proto", "https")

		got := h.uploadMediaURL(req, "upload-123", "token-abc")
		want := "http://internal.local/api/uploads/upload-123/media?token=token-abc"

		if got != want {
			t.Fatalf("url = %q, want %q", got, want)
		}
	})

	t.Run("explicit trust flag honors forwarded headers", func(t *testing.T) {
		h := &Handler{
			TrustForwardedHeaders: true,
		}
		req := httptest.NewRequest(http.MethodGet, "http://internal.local/api/uploads/upload-123/media", nil)
		req.RemoteAddr = "192.168.1.10:4321"
		req.Header.Set("Forwarded", `for=192.0.2.60; proto=https; host="media.example.com:8443"`)
		req.Header.Set("X-Forwarded-Proto", "https")

		got := h.uploadMediaURL(req, "upload-123", "token-abc")
		want := "https://media.example.com:8443/api/uploads/upload-123/media?token=token-abc"

		if got != want {
			t.Fatalf("url = %q, want %q", got, want)
		}
	})
}

func TestCreateUploadFromMultipartUnderLimitSuccess(t *testing.T) {
	h, store := newTestHandler(t)
	h.UploadMediaDir = t.TempDir()
	h.UploadMaxBytes = 4096

	owner, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Owner", Email: "owner@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Owner Channel", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	req := newMultipartUploadRequest(t, channel.ID, "clip.mp4", bytes.Repeat([]byte("a"), 32), false)
	rec := httptest.NewRecorder()

	h.createUploadFromMultipart(rec, req, owner)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var resp uploadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.ChannelID != channel.ID {
		t.Fatalf("channelId = %q, want %q", resp.ChannelID, channel.ID)
	}
	if resp.SizeBytes != 32 {
		t.Fatalf("sizeBytes = %d, want %d", resp.SizeBytes, 32)
	}
}

func TestCreateUploadFromMultipartOverLimitRejected(t *testing.T) {
	h, store := newTestHandler(t)
	h.UploadMediaDir = t.TempDir()
	h.UploadMaxBytes = 32

	owner, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Owner", Email: "owner2@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Owner Channel", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	req := newMultipartUploadRequest(t, channel.ID, "big.mp4", bytes.Repeat([]byte("b"), 128), false)
	rec := httptest.NewRecorder()

	h.createUploadFromMultipart(rec, req, owner)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	errResp := decodeAPIError(t, rec.Body.Bytes())
	if errResp.Error.Code != "request_too_large" {
		t.Fatalf("code = %q, want %q", errResp.Error.Code, "request_too_large")
	}
	if !strings.Contains(errResp.Error.Message, "exceeds limit") {
		t.Fatalf("message = %q, want contains %q", errResp.Error.Message, "exceeds limit")
	}
}

func TestCreateUploadFromMultipartMalformedRejected(t *testing.T) {
	h, store := newTestHandler(t)
	h.UploadMaxBytes = 4096

	owner, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Owner", Email: "owner3@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Owner Channel", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	body := strings.NewReader("--bad\r\nContent-Disposition: form-data; name=\"channelId\"\r\n\r\n" + channel.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=bad")
	rec := httptest.NewRecorder()

	h.createUploadFromMultipart(rec, req, owner)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	errResp := decodeAPIError(t, rec.Body.Bytes())
	if errResp.Error.Message == "" {
		t.Fatal("expected error message")
	}
}

func newMultipartUploadRequest(t *testing.T, channelID, filename string, content []byte, malformed bool) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("channelId", channelID); err != nil {
		t.Fatalf("WriteField channelId: %v", err)
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("Write file part: %v", err)
	}
	if !malformed {
		if err := writer.Close(); err != nil {
			t.Fatalf("Close writer: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
