package api

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bitriver-live/internal/models"
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

	upload := models.Upload{
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

	upload := models.Upload{
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
