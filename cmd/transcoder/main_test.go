package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

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

	srv, err := newServer("", tempDir)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv.routes())
	defer ts.Close()

	client := ts.Client()

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
	resp, err := client.Post(ts.URL+"/v1/jobs", "application/json", bytes.NewReader(body))
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
	jobID := jobResp.JobIDs[0]
	master := filepath.Join(tempDir, "live", jobID, "index.m3u8")

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
	resp2, err := client.Post(ts.URL+"/v1/jobs", "application/json", bytes.NewReader(body2))
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

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/v1/jobs/"+jobID2, nil)
	if err != nil {
		t.Fatalf("build delete: %v", err)
	}
	respDel, err := client.Do(req)
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
