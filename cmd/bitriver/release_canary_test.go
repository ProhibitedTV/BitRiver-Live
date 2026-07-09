package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseCanaryPassesWithHealthyStackEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/readyz", "/healthz", "/api/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","token":"secret-value","components":{"srs":{"status":"ok"}}}`))
		case "/viewer":
			_, _ = w.Write([]byte("<html>viewer</html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	logs := filepath.Join(dir, "logs.txt")
	writeTestFile(t, logs, "bitriver-live started\ntranscoder healthy\n")
	rollback := filepath.Join(dir, "rollback.md")
	writeTestFile(t, rollback, `
Previous version/tag: v1.2.2
Data/migration note: no schema rollback is required.
Env/config note: reuse the previous .env file.
Artifact path: redeploy image tag v1.2.2 from the release artifact.
`)

	artifactDir := filepath.Join(dir, "artifacts")
	err := runReleaseCanary([]string{
		"--base-url", server.URL,
		"--logs-file", logs,
		"--rollback-notes", rollback,
		"--require-rollback-notes",
		"--artifact-dir", artifactDir,
	})
	if err != nil {
		t.Fatalf("release canary failed: %v", err)
	}

	report := readCanaryReport(t, filepath.Join(artifactDir, "canary-report.json"))
	if report.Status != "passed" {
		t.Fatalf("expected passed report, got %#v", report)
	}
	response := readJSONMap(t, filepath.Join(artifactDir, "api-health.json"))
	body := response["body"].(map[string]any)
	if body["token"] != "[redacted]" {
		t.Fatalf("expected token to be redacted, got %#v", body["token"])
	}
}

func TestReleaseCanaryFailsOnUnhealthyEndpointStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/healthz" {
			_, _ = w.Write([]byte(`{"status":"ok","components":{"transcoder":{"status":"degraded"}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	dir := t.TempDir()
	logs := filepath.Join(dir, "logs.txt")
	writeTestFile(t, logs, "clean logs\n")
	rollback := filepath.Join(dir, "rollback.md")
	writeTestFile(t, rollback, "previous version v1\ndata migration no rollback\nenv config previous file\nartifact image tag v1\n")

	err := runReleaseCanary([]string{
		"--base-url", server.URL,
		"--logs-file", logs,
		"--rollback-notes", rollback,
		"--require-rollback-notes",
		"--artifact-dir", filepath.Join(dir, "artifacts"),
	})
	if err == nil {
		t.Fatal("expected release canary to fail on degraded health status")
	}
}

func TestReleaseCanaryLogScanFindsHighConfidencePatterns(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "logs.txt")
	writeTestFile(t, logs, "ok\npanic: cannot initialize\n")

	matches, err := scanCanaryLogFile(logs)
	if err != nil {
		t.Fatalf("scan logs: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one log match, got %#v", matches)
	}
}

func TestReleaseCanaryRollbackNotesRequireCoverage(t *testing.T) {
	missing := missingRollbackSections("previous version v1\nartifact image tag v1\n")
	if !containsString(missing, "data/migration note") {
		t.Fatalf("expected missing data/migration note, got %#v", missing)
	}
	if !containsString(missing, "env/config note") {
		t.Fatalf("expected missing env/config note, got %#v", missing)
	}

	complete := missingRollbackSections("previous version v1\ndata migration no rollback\nenv config previous file\nartifact image tag v1\n")
	if len(complete) != 0 {
		t.Fatalf("expected complete rollback notes, got %#v", complete)
	}
}

func readCanaryReport(t *testing.T, path string) releaseCanaryReport {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report releaseCanaryReport
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	return report
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read json: %v", err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return value
}
