package ingest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestHTTPChannelAdapterCreateAndDelete verifies that the channel adapter
// correctly calls the SRS controller to create and delete channels with the
// appropriate authorization header and payload.
func TestHTTPChannelAdapterCreateAndDelete(t *testing.T) {
	t.Helper()
	var created bool
	var deleted bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/channels":
			created = true
			if got := r.Header.Get("Authorization"); got != "Bearer token" {
				t.Fatalf("expected bearer token, got %q", got)
			}
			var payload srsChannelRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload.ChannelID != "channel-123" || payload.StreamKey != "stream-key" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			if err := json.NewEncoder(w).Encode(srsChannelResponse{
				PrimaryIngest: "rtmp://primary",
				BackupIngest:  "rtmp://backup",
			}); err != nil {
				t.Fatalf("encode response: %v", err)
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/channels/channel-123":
			deleted = true
			if got := r.Header.Get("Authorization"); got != "Bearer token" {
				t.Fatalf("expected bearer token on delete, got %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := newHTTPChannelAdapter(server.URL, "token", server.Client(), nil, 3, time.Nanosecond)

	primary, backup, err := adapter.CreateChannel(context.Background(), "channel-123", "stream-key")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if primary != "rtmp://primary" || backup != "rtmp://backup" {
		t.Fatalf("unexpected ingest endpoints: %q, %q", primary, backup)
	}
	if !created {
		t.Fatal("expected create endpoint to be invoked")
	}

	if err := adapter.DeleteChannel(context.Background(), "channel-123"); err != nil {
		t.Fatalf("DeleteChannel: %v", err)
	}
	if !deleted {
		t.Fatal("expected delete endpoint to be invoked")
	}
}

// TestHTTPChannelAdapterRetries verifies that the channel adapter retries
// on a 5xx error (which is treated as transient by doWithRetry).
func TestHTTPChannelAdapterRetries(t *testing.T) {
	t.Helper()
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/channels" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if attempts == 1 {
			http.Error(w, "temporary", http.StatusInternalServerError)
			return
		}
		if err := json.NewEncoder(w).Encode(srsChannelResponse{
			PrimaryIngest: "rtmp://primary",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	adapter := newHTTPChannelAdapter(server.URL, "token", server.Client(), nil, 2, time.Nanosecond)
	primary, backup, err := adapter.CreateChannel(context.Background(), "channel-123", "stream-key")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if primary != "rtmp://primary" || backup != "" {
		t.Fatalf("unexpected ingest endpoints: %q, %q", primary, backup)
	}
	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

// TestHTTPChannelAdapterDoesNotRetryOn4xx verifies that 4xx responses
// other than 429 are treated as permanent errors and are not retried.
func TestHTTPChannelAdapterDoesNotRetryOn4xx(t *testing.T) {
	t.Helper()
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/channels" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	adapter := newHTTPChannelAdapter(server.URL, "token", server.Client(), nil, 3, time.Nanosecond)
	_, _, err := adapter.CreateChannel(context.Background(), "channel-123", "stream-key")
	if err == nil {
		t.Fatal("expected error for 4xx response, got nil")
	}
	if attempts != 1 {
		t.Fatalf("expected exactly 1 attempt for 4xx, got %d", attempts)
	}
}

// TestHTTPChannelAdapterRetriesOn429 verifies that HTTP 429 (Too Many Requests)
// is treated as retryable and retried until success.
func TestHTTPChannelAdapterRetriesOn429(t *testing.T) {
	t.Helper()
	var attempts int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/channels" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if attempts < 3 {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		if err := json.NewEncoder(w).Encode(srsChannelResponse{
			PrimaryIngest: "rtmp://primary",
			BackupIngest:  "rtmp://backup",
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	adapter := newHTTPChannelAdapter(server.URL, "token", server.Client(), nil, 5, time.Nanosecond)
	primary, backup, err := adapter.CreateChannel(context.Background(), "channel-123", "stream-key")
	if err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts (2x 429 + 1x success), got %d", attempts)
	}
	if primary != "rtmp://primary" || backup != "rtmp://backup" {
		t.Fatalf("unexpected ingest endpoints: %q, %q", primary, backup)
	}
}

// TestHTTPApplicationAdapterLifecycle verifies that the application adapter
// correctly uses basic auth and round-trips the origin and playback URLs.
func TestHTTPApplicationAdapterLifecycle(t *testing.T) {
	t.Helper()
	var created bool
	var deleted bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/applications":
			created = true
			user, pass, ok := r.BasicAuth()
			if !ok || user != "admin" || pass != "password" {
				t.Fatalf("expected basic auth credentials, got %q/%q", user, pass)
			}
			var payload omeApplicationRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload.ChannelID != "channel-123" {
				t.Fatalf("unexpected channel id %q", payload.ChannelID)
			}
			if err := json.NewEncoder(w).Encode(omeApplicationResponse{
				OriginURL:   "http://origin",
				PlaybackURL: "https://playback",
			}); err != nil {
				t.Fatalf("encode response: %v", err)
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/applications/channel-123":
			deleted = true
			user, pass, ok := r.BasicAuth()
			if !ok || user != "admin" || pass != "password" {
				t.Fatalf("expected basic auth on delete, got %q/%q", user, pass)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := newHTTPApplicationAdapter(server.URL, "admin", "password", server.Client(), nil, 3, time.Nanosecond)
	origin, playback, err := adapter.CreateApplication(context.Background(), "channel-123", []string{"1080p"})
	if err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}
	if origin != "http://origin" || playback != "https://playback" {
		t.Fatalf("unexpected playback URLs: %q %q", origin, playback)
	}
	if !created {
		t.Fatal("expected application creation to be invoked")
	}
	if err := adapter.DeleteApplication(context.Background(), "channel-123"); err != nil {
		t.Fatalf("DeleteApplication: %v", err)
	}
	if !deleted {
		t.Fatal("expected application deletion to be invoked")
	}
}

// TestHTTPTranscoderAdapterStartStop verifies that the transcoder adapter
// correctly starts jobs, merges JobID and JobIDs, and stops a job using
// the expected authorization header.
func TestHTTPTranscoderAdapterStartStop(t *testing.T) {
	t.Helper()
	var started bool
	var stopped bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/jobs":
			started = true
			if got := r.Header.Get("Authorization"); got != "Bearer job-token" {
				t.Fatalf("expected bearer token, got %q", got)
			}
			var payload ffmpegJobRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			if payload.ChannelID != "channel-123" || payload.SessionID != "session-abc" {
				t.Fatalf("unexpected payload: %+v", payload)
			}
			if err := json.NewEncoder(w).Encode(ffmpegJobResponse{
				JobID:  "job-primary",
				JobIDs: []string{"job-a", "job-b"},
				Renditions: []Rendition{{
					Name:        "1080p",
					ManifestURL: "https://cdn/1080p.m3u8",
					Bitrate:     6000,
				}},
			}); err != nil {
				t.Fatalf("encode response: %v", err)
			}
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/jobs/job-a":
			stopped = true
			if got := r.Header.Get("Authorization"); got != "Bearer job-token" {
				t.Fatalf("expected bearer token on delete, got %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	adapter := newHTTPTranscoderAdapter(server.URL, "job-token", server.Client(), nil, 3, time.Nanosecond)
	ladder := []Rendition{{Name: "1080p", Bitrate: 6000}}
	jobIDs, renditions, err := adapter.StartJobs(context.Background(), "channel-123", "session-abc", "http://origin", ladder)
	if err != nil {
		t.Fatalf("StartJobs: %v", err)
	}
	if len(jobIDs) != 3 {
		t.Fatalf("expected three job IDs (including legacy field), got %d", len(jobIDs))
	}
	if len(renditions) != 1 || renditions[0].ManifestURL != "https://cdn/1080p.m3u8" {
		t.Fatalf("unexpected renditions: %+v", renditions)
	}
	if !started {
		t.Fatal("expected start endpoint to be invoked")
	}
	// Ensure ladder input is not mutated.
	if ladder[0].ManifestURL != "" {
		t.Fatalf("expected input ladder to remain unchanged, got manifest %q", ladder[0].ManifestURL)
	}

	if err := adapter.StopJob(context.Background(), "job-a"); err != nil {
		t.Fatalf("StopJob: %v", err)
	}
	if !stopped {
		t.Fatal("expected stop endpoint to be invoked")
	}
}

// TestHTTPTranscoderAdapterStartUpload verifies that the transcoder adapter
// correctly starts an upload/VOD job and returns the expected job result.
func TestHTTPTranscoderAdapterStartUpload(t *testing.T) {
	t.Helper()
	var started bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/uploads" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		started = true
		if got := r.Header.Get("Authorization"); got != "Bearer job-token" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		var payload ffmpegUploadRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.ChannelID != "channel-123" || payload.UploadID != "upload-abc" || payload.SourceURL != "https://cdn/source.mp4" {
			t.Fatalf("unexpected payload: %+v", payload)
		}
		if err := json.NewEncoder(w).Encode(ffmpegUploadResponse{
			JobID:       "job-upload",
			PlaybackURL: "https://cdn/hls/index.m3u8",
			Renditions: []Rendition{{
				Name:        "720p",
				ManifestURL: "https://cdn/hls/720p.m3u8",
				Bitrate:     3000,
			}},
		}); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	adapter := newHTTPTranscoderAdapter(server.URL, "job-token", server.Client(), nil, 3, time.Nanosecond)
	result, err := adapter.StartUpload(context.Background(), uploadJobRequest{
		ChannelID: "channel-123",
		UploadID:  "upload-abc",
		SourceURL: "https://cdn/source.mp4",
		Filename:  "source.mp4",
		Renditions: []Rendition{{
			Name:    "720p",
			Bitrate: 3000,
		}},
	})
	if err != nil {
		t.Fatalf("StartUpload: %v", err)
	}
	if !started {
		t.Fatal("expected upload endpoint to be invoked")
	}
	if result.JobID != "job-upload" || result.PlaybackURL != "https://cdn/hls/index.m3u8" {
		t.Fatalf("unexpected upload result: %+v", result)
	}
	if len(result.Renditions) != 1 || result.Renditions[0].ManifestURL != "https://cdn/hls/720p.m3u8" {
		t.Fatalf("unexpected renditions: %+v", result.Renditions)
	}
}
