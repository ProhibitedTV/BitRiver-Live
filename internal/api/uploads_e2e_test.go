//go:build e2e

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bitriver-live/internal/domain"
	"bitriver-live/internal/storage"
)

func TestE2EUploadMultipartAppearsInList(t *testing.T) {
	h, store := newTestHandler(t)
	h.UploadMediaDir = t.TempDir()
	h.UploadMaxBytes = 64 * 1024

	owner, err := store.CreateUser(storage.CreateUserParams{DisplayName: "Owner", Email: "owner-e2e@example.com"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	channel, err := store.CreateChannel(owner.ID, "Owner Channel", "gaming", nil)
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/uploads", func(w http.ResponseWriter, r *http.Request) {
		h.Uploads(w, withUser(r, owner))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	uploadResp := postMultipartUpload(t, server.URL+"/api/uploads", owner, channel.ID, "tiny.mp4", validMP4Sample())

	persisted, ok := store.GetUpload(uploadResp.ID)
	if !ok {
		t.Fatalf("expected upload %q to persist", uploadResp.ID)
	}
	if persisted.ChannelID != channel.ID {
		t.Fatalf("persisted channel = %q, want %q", persisted.ChannelID, channel.ID)
	}
	if persisted.Status != "pending" {
		t.Fatalf("persisted status = %q, want %q", persisted.Status, "pending")
	}
	if persisted.PlaybackURL != "" {
		t.Fatalf("persisted playback url = %q, want empty", persisted.PlaybackURL)
	}

	listed := waitForUploadInList(t, server.Client(), server.URL+"/api/uploads", owner, channel.ID, uploadResp.ID, 3*time.Second)
	if listed.Status != "pending" {
		t.Fatalf("listed status = %q, want %q", listed.Status, "pending")
	}
	if listed.PlaybackURL != "" {
		t.Fatalf("listed playback url = %q, want empty", listed.PlaybackURL)
	}
}

func waitForUploadInList(t *testing.T, client *http.Client, endpoint string, actor domain.User, channelID, uploadID string, timeout time.Duration) uploadResponse {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		resp, ok := getUploadFromList(t, client, endpoint, actor, channelID, uploadID)
		if ok {
			return resp
		}

		select {
		case <-ctx.Done():
			t.Fatalf("upload %q did not appear in list within %s", uploadID, timeout)
		case <-ticker.C:
		}
	}
}

func getUploadFromList(t *testing.T, client *http.Client, endpoint string, actor domain.User, channelID, uploadID string) (uploadResponse, bool) {
	t.Helper()

	listReq, err := http.NewRequest(http.MethodGet, endpoint+"?channelId="+channelID, nil)
	if err != nil {
		t.Fatalf("build list request: %v", err)
	}
	listResp, err := client.Do(withUser(listReq, actor))
	if err != nil {
		t.Fatalf("list uploads: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		t.Fatalf("list status = %d, want %d body=%s", listResp.StatusCode, http.StatusOK, string(body))
	}

	var listed []uploadResponse
	if err := json.NewDecoder(listResp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	for _, item := range listed {
		if item.ID == uploadID {
			return item, true
		}
	}

	return uploadResponse{}, false
}

func postMultipartUpload(t *testing.T, endpoint string, owner domain.User, channelID, filename string, fixture []byte) uploadResponse {
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
	if _, err := part.Write(fixture); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, &body)
	if err != nil {
		t.Fatalf("build upload request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(withUser(req, owner))
	if err != nil {
		t.Fatalf("post upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload status = %d, want %d body=%s", resp.StatusCode, http.StatusCreated, string(payload))
	}

	var uploaded uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploaded); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if uploaded.ID == "" {
		t.Fatal("expected upload id")
	}
	return uploaded
}
