package metrics

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPMiddlewareRecordsRequests(t *testing.T) {
	recorder := New()
	handler := HTTPMiddleware(recorder, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/widgets/abc123", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	var buf bytes.Buffer
	recorder.Write(&buf)
	body := buf.String()

	expected := `bitriver_http_requests_total{method="GET",path="/widgets/:id",status="418"} 1`
	if !strings.Contains(body, expected) {
		t.Fatalf("expected metrics output to contain %q, got %q", expected, body)
	}
}

func TestNewRegistrySetsDefaultRecorder(t *testing.T) {
	original := Default()
	t.Cleanup(func() {
		SetDefault(original)
	})

	registry := NewRegistry()
	registry.Recorder.Reset()

	ObserveRequest("POST", "/jobs/123", http.StatusCreated, 150*time.Millisecond)

	var buf bytes.Buffer
	registry.Recorder.Write(&buf)
	body := buf.String()

	expected := `bitriver_http_requests_total{method="POST",path="/jobs/:id",status="201"} 1`
	if !strings.Contains(body, expected) {
		t.Fatalf("expected registry metrics to include %q, got %q", expected, body)
	}
}
