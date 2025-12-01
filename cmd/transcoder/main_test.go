package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"bitriver-live/internal/observability/metrics"
)

const testToken = "test-token"

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type healthResponse struct {
	Status     string                    `json:"status"`
	Components map[string]map[string]any `json:"components"`
	Running    int                       `json:"runningJobs"`
}

func startStubTranscoder(t *testing.T, tempDir string, exitErr *atomic.Pointer[error]) (*server, *httptest.Server) {
	t.Helper()
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "https://cdn.example.com/hls")
	srv, err := newServer(testToken, tempDir, newTestLogger())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	srv.launchProcess = func(id string, plan *transcodePlan, onExit func(error)) (*processState, error) {
		done := make(chan struct{})
		var once atomic.Bool
		cancel := func() {
			if once.CompareAndSwap(false, true) {
				close(done)
			}
		}
		go func() {
			time.Sleep(10 * time.Millisecond)
			if onExit != nil {
				e := exitErr.Load()
				if e != nil {
					onExit(*e)
				} else {
					onExit(nil)
				}
			}
			cancel()
		}()
		return &processState{cancel: cancel, done: done}, nil
	}
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return srv, ts
}

func TestJobProducesSegmentsAndCanBeStopped(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires ffmpeg")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	tempDir := t.TempDir()
	sample := filepath.Join(tempDir, "sample.mp4")
	generate := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=5",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-shortest", "-t", "5",
		"-pix_fmt", "yuv420p",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac",
		sample,
	)
	if out, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate sample: %v (%s)", err, out)
	}

	publicDir := filepath.Join(tempDir, "public")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "https://cdn.example.com/hls")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_DIR", publicDir)

	srv, err := newServer(testToken, tempDir, newTestLogger())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()
	publicSrv := httptest.NewServer(http.FileServer(http.Dir(publicDir)))
	defer publicSrv.Close()

	client := ts.Client()
	authHeader := "Bearer " + testToken

	renditions := []map[string]any{
		{"name": "720p", "bitrate": 2800},
		{"name": "480p", "bitrate": 1500},
	}
	body, err := json.Marshal(map[string]any{
		"channelId":  "channel-1",
		"sessionId":  "session-1",
		"originUrl":  sample,
		"renditions": renditions,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/jobs", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build job request: %v", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post job: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	var jobResp jobResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(jobResp.JobIDs) == 0 {
		t.Fatalf("expected job id")
	}
	if jobResp.JobID == "" {
		t.Fatalf("expected jobId field to be populated")
	}
	jobID := jobResp.JobIDs[0]
	if jobResp.JobID != jobID {
		t.Fatalf("expected jobId %s, got %s", jobID, jobResp.JobID)
	}
	if len(jobResp.Renditions) == 0 {
		t.Fatalf("expected renditions in response")
	}
	var jobRenditions []rendition
	if err := json.Unmarshal(jobResp.Renditions, &jobRenditions); err != nil {
		t.Fatalf("decode job renditions: %v", err)
	}
	if len(jobRenditions) == 0 {
		t.Fatalf("expected at least one rendition in job response")
	}
	livePrefix := fmt.Sprintf("https://cdn.example.com/hls/live/%s/", jobID)
	if !strings.HasPrefix(jobRenditions[0].ManifestURL, livePrefix) {
		t.Fatalf("unexpected live rendition manifest: %s", jobRenditions[0].ManifestURL)
	}
	master := filepath.Join(tempDir, "live", jobID, "index.m3u8")
	liveLink := filepath.Join(publicDir, "live", jobID)

	waitFor(t, 30*time.Second, func() bool {
		resp, err := http.Get(publicSrv.URL + fmt.Sprintf("/live/%s/index.m3u8", jobID))
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return false
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}
		return len(data) > 0
	})

	info, err := os.Lstat(liveLink)
	if err != nil {
		t.Fatalf("stat live symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected live mirror to be a symlink, got %s", info.Mode())
	}
	target, err := os.Readlink(liveLink)
	if err != nil {
		t.Fatalf("read live symlink: %v", err)
	}
	expectedTarget, err := filepath.Abs(filepath.Join(tempDir, "live", jobID))
	if err != nil {
		t.Fatalf("resolve expected target: %v", err)
	}
	resolvedTarget, err := filepath.Abs(target)
	if err != nil {
		t.Fatalf("resolve live symlink target: %v", err)
	}
	if resolvedTarget != expectedTarget {
		t.Fatalf("unexpected live symlink target: %s", resolvedTarget)
	}

	waitFor(t, 30*time.Second, func() bool {
		_, err := os.Stat(master)
		return err == nil
	})
	waitFor(t, 30*time.Second, func() bool {
		srv.mu.RLock()
		_, running := srv.processes[jobID]
		srv.mu.RUnlock()
		return !running
	})

	waitFor(t, 5*time.Second, func() bool {
		_, err := os.Lstat(liveLink)
		return errors.Is(err, os.ErrNotExist)
	})

	metaPath := filepath.Join(tempDir, "live", jobID, "metadata.json")
	waitFor(t, 5*time.Second, func() bool {
		_, err := os.Stat(metaPath)
		return err == nil
	})
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var persisted job
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if persisted.StoppedAt == nil {
		t.Fatalf("expected stopped timestamp")
	}
	if persisted.Playback != filepath.ToSlash(master) {
		t.Fatalf("unexpected playback path: %s", persisted.Playback)
	}
	variants, err := filepath.Glob(filepath.Join(tempDir, "live", jobID, "*", "index.m3u8"))
	if err != nil {
		t.Fatalf("glob variants: %v", err)
	}
	if len(variants) == 0 {
		t.Fatalf("expected variant playlists")
	}

	masterData, err := os.ReadFile(master)
	if err != nil {
		t.Fatalf("read master playlist: %v", err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(masterData))
	bandwidths := make(map[int]struct{})
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#EXT-X-STREAM-INF:") {
			continue
		}
		attrs := strings.Split(strings.TrimPrefix(line, "#EXT-X-STREAM-INF:"), ",")
		for _, attr := range attrs {
			trimmed := strings.TrimSpace(attr)
			if !strings.HasPrefix(trimmed, "BANDWIDTH=") {
				continue
			}
			value := strings.TrimPrefix(trimmed, "BANDWIDTH=")
			parsed, err := strconv.Atoi(value)
			if err != nil {
				t.Fatalf("parse bandwidth %q: %v", value, err)
			}
			bandwidths[parsed] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan master playlist: %v", err)
	}
	if len(bandwidths) != len(renditions) {
		t.Fatalf("expected %d unique bandwidth entries, got %d", len(renditions), len(bandwidths))
	}
	for bw := range bandwidths {
		if bw <= 0 {
			t.Fatalf("expected positive bandwidth, got %d", bw)
		}
	}

	if len(persisted.Renditions) != len(renditions) {
		t.Fatalf("expected %d renditions in metadata, got %d", len(renditions), len(persisted.Renditions))
	}
	seenBitrates := make(map[int]struct{})
	for _, variant := range persisted.Renditions {
		if variant.ManifestURL == "" {
			t.Fatalf("expected manifest url for rendition %s", variant.Name)
		}
		if variant.Bitrate <= 0 {
			t.Fatalf("expected positive total bitrate for rendition %s", variant.Name)
		}
		if variant.VideoBitrate <= 0 {
			t.Fatalf("expected video bitrate for rendition %s", variant.Name)
		}
		if variant.AudioBitrate <= 0 {
			t.Fatalf("expected audio bitrate for rendition %s", variant.Name)
		}
		if variant.Width <= 0 || variant.Height <= 0 {
			t.Fatalf("expected resolution for rendition %s", variant.Name)
		}
		if variant.VideoProfile == "" {
			t.Fatalf("expected video profile for rendition %s", variant.Name)
		}
		if _, exists := seenBitrates[variant.Bitrate]; exists {
			t.Fatalf("expected unique bitrate per rendition, found duplicate %d", variant.Bitrate)
		}
		seenBitrates[variant.Bitrate] = struct{}{}
	}

	// start a second job and cancel it via DELETE
	body2, err := json.Marshal(map[string]any{
		"channelId":  "channel-2",
		"sessionId":  "session-2",
		"originUrl":  sample,
		"renditions": []map[string]any{{"name": "360p", "bitrate": 800}},
	})
	if err != nil {
		t.Fatalf("marshal request 2: %v", err)
	}
	req2, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/jobs", bytes.NewReader(body2))
	if err != nil {
		t.Fatalf("build job request 2: %v", err)
	}
	req2.Header.Set("Authorization", authHeader)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("post job 2: %v", err)
	}
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status 2: %d", resp2.StatusCode)
	}
	var jobResp2 jobResponse
	if err := json.NewDecoder(resp2.Body).Decode(&jobResp2); err != nil {
		t.Fatalf("decode response 2: %v", err)
	}
	resp2.Body.Close()
	if len(jobResp2.JobIDs) == 0 {
		t.Fatalf("expected second job id")
	}
	jobID2 := jobResp2.JobIDs[0]
	liveLink2 := filepath.Join(publicDir, "live", jobID2)

	reqDel, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/jobs/"+jobID2, nil)
	if err != nil {
		t.Fatalf("build delete: %v", err)
	}
	reqDel.Header.Set("Authorization", authHeader)
	respDel, err := client.Do(reqDel)
	if err != nil {
		t.Fatalf("delete job: %v", err)
	}
	respDel.Body.Close()
	if respDel.StatusCode != http.StatusNoContent {
		t.Fatalf("unexpected delete status: %d", respDel.StatusCode)
	}

	waitFor(t, 30*time.Second, func() bool {
		srv.mu.RLock()
		_, running := srv.processes[jobID2]
		srv.mu.RUnlock()
		return !running
	})

	waitFor(t, 5*time.Second, func() bool {
		_, err := os.Lstat(liveLink2)
		return errors.Is(err, os.ErrNotExist)
	})

	metaPath2 := filepath.Join(tempDir, "live", jobID2, "metadata.json")
	waitFor(t, 5*time.Second, func() bool {
		_, err := os.Stat(metaPath2)
		return err == nil
	})
	data2, err := os.ReadFile(metaPath2)
	if err != nil {
		t.Fatalf("read metadata 2: %v", err)
	}
	var persisted2 job
	if err := json.Unmarshal(data2, &persisted2); err != nil {
		t.Fatalf("decode metadata 2: %v", err)
	}
	if persisted2.StoppedAt == nil {
		t.Fatalf("expected stopped timestamp for cancelled job")
	}
}

func TestNewServerRequiresPublicBaseURL(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_DIR", "")

	if _, err := newServer(testToken, tempDir, newTestLogger()); err == nil {
		t.Fatal("expected error when public base URL is unset")
	} else if !strings.Contains(err.Error(), "BITRIVER_TRANSCODER_PUBLIC_BASE_URL must be configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUploadPublishesHTTPPlayback(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires ffmpeg")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	tempDir := t.TempDir()
	workDir := filepath.Join(tempDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir work: %v", err)
	}
	sample := filepath.Join(tempDir, "sample.mp4")
	generate := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=5",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100",
		"-shortest", "-t", "3",
		"-pix_fmt", "yuv420p",
		"-c:v", "libx264", "-preset", "ultrafast",
		"-c:a", "aac",
		sample,
	)
	if out, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("generate sample: %v (%s)", err, out)
	}

	publicDir := filepath.Join(tempDir, "public")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "https://cdn.example.com/hls")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_DIR", publicDir)

	srv, err := newServer(testToken, workDir, newTestLogger())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	client := ts.Client()

	renditions := []map[string]any{
		{"name": "720p", "bitrate": 2800},
	}
	body, err := json.Marshal(map[string]any{
		"channelId":  "channel-1",
		"uploadId":   "upload-1",
		"sourceUrl":  sample,
		"filename":   "sample.mp4",
		"renditions": renditions,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	authHeader := "Bearer " + testToken
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/uploads", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build upload request: %v", err)
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("post upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	var uploadResp uploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&uploadResp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}
	if uploadResp.JobID == "" {
		t.Fatal("expected job id in upload response")
	}
	expectedPlayback := fmt.Sprintf("https://cdn.example.com/hls/uploads/%s/index.m3u8", uploadResp.JobID)
	if uploadResp.PlaybackURL != expectedPlayback {
		t.Fatalf("unexpected playback url: %s", uploadResp.PlaybackURL)
	}
	if len(uploadResp.Renditions) == 0 {
		t.Fatal("expected rendition metadata in response")
	}
	var responseRenditions []rendition
	if err := json.Unmarshal(uploadResp.Renditions, &responseRenditions); err != nil {
		t.Fatalf("decode rendition response: %v", err)
	}
	if len(responseRenditions) == 0 {
		t.Fatal("expected at least one rendition in response payload")
	}
	prefix := fmt.Sprintf("https://cdn.example.com/hls/uploads/%s/", uploadResp.JobID)
	if !strings.HasPrefix(responseRenditions[0].ManifestURL, prefix) {
		t.Fatalf("unexpected rendition manifest url: %s", responseRenditions[0].ManifestURL)
	}

	metadataPath := filepath.Join(workDir, "uploads", uploadResp.JobID, "metadata.json")
	waitFor(t, 45*time.Second, func() bool {
		_, err := os.Stat(metadataPath)
		return err == nil
	})
	waitFor(t, 30*time.Second, func() bool {
		srv.mu.RLock()
		proc := srv.processes[uploadResp.JobID]
		srv.mu.RUnlock()
		return proc == nil
	})

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	var persisted uploadJob
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	if persisted.CompletedAt == nil {
		t.Fatal("expected completed timestamp for upload")
	}
	if persisted.Playback != expectedPlayback {
		t.Fatalf("unexpected persisted playback url: %s", persisted.Playback)
	}
	for _, rendition := range persisted.Renditions {
		if !strings.HasPrefix(rendition.ManifestURL, prefix) {
			t.Fatalf("unexpected rendition url: %s", rendition.ManifestURL)
		}
	}
	publishedMaster := filepath.Join(publicDir, "uploads", uploadResp.JobID, "index.m3u8")
	if _, err := os.Stat(publishedMaster); err != nil {
		t.Fatalf("expected published master playlist: %v", err)
	}
}

func TestHandleJobsRecordsMetrics(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	tempDir := t.TempDir()
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "https://cdn.example.com/hls")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_DIR", filepath.Join(tempDir, "public"))

	srv, err := newServer(testToken, tempDir, newTestLogger())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	srv.launchProcess = func(jobID string, plan *transcodePlan, onExit func(error)) (*processState, error) {
		return &processState{cancel: func() {}, done: make(chan struct{})}, nil
	}

	body, err := json.Marshal(map[string]any{
		"channelId":  "channel-1",
		"sessionId":  "session-1",
		"originUrl":  "https://cdn/source.m3u8",
		"renditions": []map[string]any{{"name": "720p", "bitrate": 2000}},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	res := httptest.NewRecorder()

	srv.handleJobs(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("unexpected status: %d", res.Code)
	}

	events, active := metrics.Default().TranscoderJobCounts()
	if events[metrics.TranscoderJobLabel{Kind: "live", Status: "start"}] != 1 {
		t.Fatalf("expected one live start, got %d", events[metrics.TranscoderJobLabel{Kind: "live", Status: "start"}])
	}
	if events[metrics.TranscoderJobLabel{Kind: "live", Status: "fail"}] != 0 {
		t.Fatalf("expected zero live failures, got %d", events[metrics.TranscoderJobLabel{Kind: "live", Status: "fail"}])
	}
	if active != 1 {
		t.Fatalf("expected active jobs gauge of 1, got %d", active)
	}
}

func TestHandleJobsMetricsOnFailure(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	tempDir := t.TempDir()
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "https://cdn.example.com/hls")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_DIR", filepath.Join(tempDir, "public"))

	srv, err := newServer(testToken, tempDir, newTestLogger())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	srv.launchProcess = func(jobID string, plan *transcodePlan, onExit func(error)) (*processState, error) {
		return nil, errors.New("ffmpeg missing")
	}

	body, err := json.Marshal(map[string]any{
		"channelId":  "channel-1",
		"sessionId":  "session-1",
		"originUrl":  "https://cdn/source.m3u8",
		"renditions": []map[string]any{{"name": "720p", "bitrate": 2000}},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/jobs", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	res := httptest.NewRecorder()

	srv.handleJobs(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", res.Code)
	}

	events, active := metrics.Default().TranscoderJobCounts()
	if events[metrics.TranscoderJobLabel{Kind: "live", Status: "fail"}] != 1 {
		t.Fatalf("expected one live failure, got %d", events[metrics.TranscoderJobLabel{Kind: "live", Status: "fail"}])
	}
	if active != 0 {
		t.Fatalf("expected active jobs gauge of 0, got %d", active)
	}
}

func TestHandleUploadsRecordsMetrics(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	tempDir := t.TempDir()
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "https://cdn.example.com/hls")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_DIR", filepath.Join(tempDir, "public"))

	srv, err := newServer(testToken, tempDir, newTestLogger())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	srv.launchProcess = func(jobID string, plan *transcodePlan, onExit func(error)) (*processState, error) {
		return &processState{cancel: func() {}, done: make(chan struct{})}, nil
	}

	body, err := json.Marshal(map[string]any{
		"channelId":  "channel-1",
		"uploadId":   "upload-1",
		"sourceUrl":  "https://cdn/source.mp4",
		"filename":   "source.mp4",
		"renditions": []map[string]any{{"name": "720p", "bitrate": 2000}},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/uploads", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	res := httptest.NewRecorder()

	srv.handleUploads(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", res.Code)
	}

	events, active := metrics.Default().TranscoderJobCounts()
	if events[metrics.TranscoderJobLabel{Kind: "upload", Status: "start"}] != 1 {
		t.Fatalf("expected one upload start, got %d", events[metrics.TranscoderJobLabel{Kind: "upload", Status: "start"}])
	}
	if events[metrics.TranscoderJobLabel{Kind: "upload", Status: "fail"}] != 0 {
		t.Fatalf("expected zero upload failures, got %d", events[metrics.TranscoderJobLabel{Kind: "upload", Status: "fail"}])
	}
	if active != 1 {
		t.Fatalf("expected active jobs gauge of 1, got %d", active)
	}
}

func TestHandleUploadsMetricsOnFailure(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	tempDir := t.TempDir()
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "https://cdn.example.com/hls")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_DIR", filepath.Join(tempDir, "public"))

	srv, err := newServer(testToken, tempDir, newTestLogger())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	srv.launchProcess = func(jobID string, plan *transcodePlan, onExit func(error)) (*processState, error) {
		return nil, errors.New("ffmpeg missing")
	}

	body, err := json.Marshal(map[string]any{
		"channelId":  "channel-1",
		"uploadId":   "upload-1",
		"sourceUrl":  "https://cdn/source.mp4",
		"filename":   "source.mp4",
		"renditions": []map[string]any{{"name": "720p", "bitrate": 2000}},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/uploads", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+testToken)
	res := httptest.NewRecorder()

	srv.handleUploads(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", res.Code)
	}

	events, active := metrics.Default().TranscoderJobCounts()
	if events[metrics.TranscoderJobLabel{Kind: "upload", Status: "fail"}] != 1 {
		t.Fatalf("expected one upload failure, got %d", events[metrics.TranscoderJobLabel{Kind: "upload", Status: "fail"}])
	}
	if active != 0 {
		t.Fatalf("expected active jobs gauge of 0, got %d", active)
	}
}

func TestExitHandlersRecordMetrics(t *testing.T) {
	metrics.Default().Reset()
	t.Cleanup(metrics.Default().Reset)

	tempDir := t.TempDir()
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_BASE_URL", "https://cdn.example.com/hls")
	t.Setenv("BITRIVER_TRANSCODER_PUBLIC_DIR", filepath.Join(tempDir, "public"))

	srv, err := newServer(testToken, tempDir, newTestLogger())
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	liveID := "job-exit"
	uploadID := "upload-exit"
	srv.jobs[liveID] = &job{ID: liveID}
	srv.uploads[uploadID] = &uploadJob{ID: uploadID}

	metrics.TranscoderJobStarted("live")
	metrics.TranscoderJobStarted("upload")

	srv.makeJobExitHandler(liveID)(nil)
	srv.makeUploadExitHandler(uploadID)(errors.New("ffmpeg error"))

	events, active := metrics.Default().TranscoderJobCounts()
	if events[metrics.TranscoderJobLabel{Kind: "live", Status: "complete"}] != 1 {
		t.Fatalf("expected one live completion, got %d", events[metrics.TranscoderJobLabel{Kind: "live", Status: "complete"}])
	}
	if events[metrics.TranscoderJobLabel{Kind: "upload", Status: "fail"}] != 1 {
		t.Fatalf("expected one upload failure, got %d", events[metrics.TranscoderJobLabel{Kind: "upload", Status: "fail"}])
	}
	if active != 0 {
		t.Fatalf("expected active jobs gauge of 0, got %d", active)
	}
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if fn() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("condition not met within %s", timeout)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func TestHealthTracksFFmpegFailuresAndRecovery(t *testing.T) {
	tempDir := t.TempDir()
	var exitPtr atomic.Pointer[error]
	initialErr := errors.New("ffmpeg crashed")
	exitPtr.Store(&initialErr)
	_, ts := startStubTranscoder(t, tempDir, &exitPtr)

	submitJob(t, ts, "file:///tmp/input.mp4")

	waitFor(t, 2*time.Second, func() bool {
		status, _ := fetchHealth(t, ts)
		return status.Status == "degraded"
	})

	exitPtr.Store(nil)
	submitJob(t, ts, "file:///tmp/input.mp4")

	waitFor(t, 2*time.Second, func() bool {
		status, _ := fetchHealth(t, ts)
		return status.Status == "ok"
	})
}

func submitJob(t *testing.T, ts *httptest.Server, origin string) {
	t.Helper()
	renditions := []map[string]any{{"name": "720p", "bitrate": 2800}}
	body, err := json.Marshal(map[string]any{
		"channelId":  "channel-1",
		"sessionId":  "session-1",
		"originUrl":  origin,
		"renditions": renditions,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/jobs", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("post job: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}

func fetchHealth(t *testing.T, ts *httptest.Server) (healthResponse, int) {
	t.Helper()
	resp, err := ts.Client().Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz request: %v", err)
	}
	defer resp.Body.Close()
	var payload healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	return payload, resp.StatusCode
}
