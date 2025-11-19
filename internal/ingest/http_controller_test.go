package ingest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type flakyRoundTripper struct {
	failureReturned atomic.Bool
	transport       http.RoundTripper
}

func (f *flakyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !f.failureReturned.Load() {
		f.failureReturned.Store(true)
		return nil, errors.New("temporary DNS failure")
	}
	return f.transport.RoundTrip(req)
}

func TestHTTPControllerHealthChecksFailFastOnTransientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &flakyRoundTripper{transport: http.DefaultTransport}, Timeout: time.Second}

	controller := HTTPController{
		config: Config{
			SRSBaseURL:      server.URL,
			SRSToken:        "token",
			OMEBaseURL:      server.URL,
			OMEUsername:     "user",
			OMEPassword:     "pass",
			JobBaseURL:      server.URL,
			JobToken:        "token",
			HealthEndpoint:  "/healthz",
			HealthTimeout:   500 * time.Millisecond,
			HTTPClient:      client,
			LadderProfiles:  []Rendition{{Name: "720p", Bitrate: 1000}},
			HTTPMaxAttempts: 2,
		},
	}

	statuses := controller.HealthChecks(context.Background())

	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}

	statusMap := make(map[string]HealthStatus)
	for _, status := range statuses {
		statusMap[status.Component] = status
	}

	srsStatus, ok := statusMap["srs"]
	if !ok {
		t.Fatalf("missing SRS status")
	}
	if srsStatus.Status != "error" {
		t.Fatalf("expected SRS status error, got %s", srsStatus.Status)
	}
	if !strings.Contains(srsStatus.Detail, "temporary DNS failure") {
		t.Fatalf("expected transient failure detail, got %s", srsStatus.Detail)
	}

	for _, component := range []string{"ovenmediaengine", "transcoder"} {
		status, ok := statusMap[component]
		if !ok {
			t.Fatalf("missing status for %s", component)
		}
		if status.Status != "ok" {
			t.Fatalf("expected %s status ok, got %s", component, status.Status)
		}
	}
}
