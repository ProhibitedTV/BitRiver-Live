package ingest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func TestHTTPControllerHealthChecksRetriesOnTransientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := &http.Client{Transport: &flakyRoundTripper{transport: http.DefaultTransport}, Timeout: time.Second}

	controller := HTTPController{
		config: Config{
			SRSBaseURL:        server.URL,
			SRSToken:          "token",
			OMEBaseURL:        server.URL,
			OMEUsername:       "user",
			OMEPassword:       "pass",
			JobBaseURL:        server.URL,
			JobToken:          "token",
			HealthEndpoint:    "/healthz",
			HTTPClient:        client,
			HTTPMaxAttempts:   2,
			HTTPRetryInterval: 0,
			LadderProfiles:    []Rendition{{Name: "720p", Bitrate: 1000}},
		},
		retryAttempts: 2,
	}

	statuses := controller.HealthChecks(context.Background())

	if len(statuses) != 3 {
		t.Fatalf("expected 3 statuses, got %d", len(statuses))
	}
	for _, status := range statuses {
		if status.Status != "ok" {
			t.Fatalf("expected status ok for %s, got %s (%s)", status.Component, status.Status, status.Detail)
		}
	}

	if !controller.config.HTTPClient.Transport.(*flakyRoundTripper).failureReturned.Load() {
		t.Fatalf("expected at least one retry due to transient failure")
	}
}
