package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateFixtureAndVerifyDurableState(t *testing.T) {
	t.Helper()
	const userID = "user-resilience"
	const channelID = "channel-resilience"
	var channelTitle string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/auth/signup":
			http.SetCookie(w, &http.Cookie{Name: "bitriver_session", Value: "private-cookie", Path: "/", Secure: true})
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"user":{"id":"` + userID + `"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/channels":
			if _, err := r.Cookie("bitriver_session"); err != nil {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}
			var request struct {
				Title string `json:"title"`
			}
			_ = json.NewDecoder(r.Body).Decode(&request)
			channelTitle = request.Title
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": channelID, "ownerId": userID, "title": channelTitle})
		case r.Method == http.MethodGet && r.URL.Path == "/api/viewer/me":
			if _, err := r.Cookie("bitriver_session"); err != nil {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]string{"id": userID}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/channels/"+channelID:
			_ = json.NewEncoder(w).Encode(map[string]string{"id": channelID, "ownerId": userID, "title": channelTitle})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newAPIClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := client.createFixture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := client.verifyDurableState(context.Background(), fixture)
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.SessionPreserved || !evidence.ChannelPreserved {
		t.Fatalf("unexpected durable evidence: %+v", evidence)
	}
}

func TestObservationClassifiers(t *testing.T) {
	ready := httpObservation{StatusCode: http.StatusOK, Body: []byte(`{"status":"ok"}`)}
	if !readyRecovered(ready) || readyDegraded(ready) {
		t.Fatal("ready response was classified incorrectly")
	}
	degraded := httpObservation{StatusCode: http.StatusServiceUnavailable, Body: []byte(`{"status":"degraded"}`)}
	if !readyDegraded(degraded) || readyRecovered(degraded) {
		t.Fatal("degraded response was classified incorrectly")
	}
	status := httpObservation{StatusCode: http.StatusOK, Body: []byte(`{"status":"down","checks":[{"name":"transcoder","status":"down"}]}`)}
	if !statusComponent(status, "transcoder", false) || statusComponent(status, "transcoder", true) {
		t.Fatal("component response was classified incorrectly")
	}
}

func TestWaitForIsBounded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := waitFor(ctx, time.Millisecond, func(context.Context) bool { return false }); err == nil {
		t.Fatal("expected bounded wait failure")
	}
}

func TestReportValidation(t *testing.T) {
	report := completeTestReport()
	if err := report.validate([]string{"private-sentinel"}); err != nil {
		t.Fatalf("expected valid report: %v", err)
	}

	report.Scenarios = report.Scenarios[:len(report.Scenarios)-1]
	if err := report.validate(nil); err == nil {
		t.Fatal("expected missing scenario refusal")
	}

	report = completeTestReport()
	report.Scenarios[0].RecoverySeconds = -1
	if err := report.validate(nil); err == nil {
		t.Fatal("expected invalid timing refusal")
	}

	report = completeTestReport()
	report.RemainingAcceptance = []string{"private-sentinel"}
	if err := report.validate([]string{"private-sentinel"}); err == nil {
		t.Fatal("expected private sentinel refusal")
	}
}

func TestRestartCountsStable(t *testing.T) {
	if !restartCountsStable(map[string]int{"api": 1, "db": 0}, map[string]int{"db": 0, "api": 1}) {
		t.Fatal("equal restart counts should be stable")
	}
	if restartCountsStable(map[string]int{"api": 1}, map[string]int{"api": 2}) {
		t.Fatal("growing restart count should be unstable")
	}
}

func TestPrivateSentinelsAndStagedEnvironment(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "source.env")
	destination := filepath.Join(directory, "staged.env")
	privatePassword := "private-password-value"
	payload := "BITRIVER_POSTGRES_PASSWORD=" + privatePassword + "\nBITRIVER_LIVE_PORT=8080\n"
	if err := os.WriteFile(source, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	sentinels, err := privateSentinels(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(sentinels) != 1 || sentinels[0] != privatePassword {
		t.Fatalf("unexpected sentinel inventory: %v", sentinels)
	}
	commit := strings.Repeat("a", 40)
	if err := stagePrivateEnvironment(source, destination, commit); err != nil {
		t.Fatal(err)
	}
	staged, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	text := string(staged)
	if !strings.Contains(text, "BITRIVER_LIVE_PORT=28080") ||
		!strings.Contains(text, "BITRIVER_OME_API=http://ome:28081") ||
		!strings.Contains(text, "BITRIVER_LIVE_IMAGE_DIGEST=\n") {
		t.Fatal("staged environment lacks isolated port or mutable-image override")
	}
	if !strings.Contains(text, privatePassword) {
		t.Fatal("staged environment did not preserve private contract values")
	}
}

func TestCappedTailAndRedaction(t *testing.T) {
	buffer := &cappedTail{limit: 13}
	_, _ = buffer.Write([]byte("prefix-private-value"))
	if got := buffer.String(); got != "private-value" {
		t.Fatalf("unexpected tail %q", got)
	}
	if got := redact("error private-value", []string{"private-value"}); strings.Contains(got, "private-value") {
		t.Fatal("redaction retained private sentinel")
	}
}

func TestCleanEnvironmentDoesNotLeakBuildPolicy(t *testing.T) {
	cleaned := cleanEnvironment([]string{
		"PATH=/usr/bin",
		"BITRIVER_PRIVATE=value",
		"COMPOSE_PROFILES=test",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOTOOLCHAIN=local",
		"GOCACHE=/private/cache",
	})
	if len(cleaned) != 1 || cleaned[0] != "PATH=/usr/bin" {
		t.Fatalf("unexpected cleaned environment: %v", cleaned)
	}
}

func TestResolveReportPathProtectsRepositoryFiles(t *testing.T) {
	repository := filepath.Join(t.TempDir(), "repository")
	if err := os.MkdirAll(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	allowed, err := resolveReportPath(repository, filepath.FromSlash(".artifacts/resilience/report.json"))
	if err != nil {
		t.Fatalf("expected artifact path to be allowed: %v", err)
	}
	if !pathWithin(filepath.Join(repository, ".artifacts"), allowed) {
		t.Fatalf("artifact path escaped expected root: %s", allowed)
	}
	if _, err := resolveReportPath(repository, "TASKS.md"); err == nil {
		t.Fatal("expected repository file destination to be refused")
	}
	external := filepath.Join(t.TempDir(), "report.json")
	if got, err := resolveReportPath(repository, external); err != nil || got != external {
		t.Fatalf("expected external destination to be allowed, got %q: %v", got, err)
	}
}

func completeTestReport() resilienceReport {
	report := newReport("0123456789abcdef")
	report.Isolation.OperatorEnvironmentUnchanged = true
	report.Isolation.OperatorOMEConfigUnchanged = true
	report.Isolation.TeardownComplete = true
	for _, name := range requiredScenarioNames {
		report.Scenarios = append(report.Scenarios, scenarioEvidence{
			Name:                name,
			Targets:             []string{name},
			ExpectedSignal:      "bounded test signal",
			DegradationObserved: true,
			DegradationSeconds:  0.1,
			RecoverySeconds:     0.2,
			DurableState: durableEvidence{
				SessionPreserved: true,
				ChannelPreserved: true,
			},
			RestartCountsStable: true,
			Result:              "passed",
		})
	}
	return report
}
