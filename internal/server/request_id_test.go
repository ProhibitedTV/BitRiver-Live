package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"bitriver-live/internal/observability/logging"
)

func TestRequestIDMiddlewareAnnotatesContextAndHeaders(t *testing.T) {
	t.Parallel()

	handler := requestIDMiddlewareWithGenerator(slog.Default(), func() string { return "generated" }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID, _ := logging.RequestIDFromContext(r.Context())
		if requestID != "incoming" {
			t.Fatalf("expected request id to be preserved, got %q", requestID)
		}
		streamID, _ := logging.StreamIDFromContext(r.Context())
		if streamID != "stream-123" {
			t.Fatalf("expected stream id \"stream-123\", got %q", streamID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "incoming")
	req.Header.Set("X-Stream-Id", "stream-123")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("X-Request-Id") != "incoming" {
		t.Fatalf("expected response header to carry request id, got %q", rr.Header().Get("X-Request-Id"))
	}
}

func TestLoggingMiddlewareEmitsRequestMetadata(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{AddSource: false}))

	handlerChain := requestIDMiddlewareWithGenerator(logger, func() string { return "generated-id" }, loggingMiddleware(logger, nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	req.Header.Set("X-Stream-Id", "stream-abc")

	handlerChain.ServeHTTP(httptest.NewRecorder(), req)

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal log line: %v", err)
	}

	if payload["request_id"] != "generated-id" {
		t.Fatalf("expected request_id to be propagated, got %v", payload["request_id"])
	}
	if payload["stream_id"] != "stream-abc" {
		t.Fatalf("expected stream_id to be propagated, got %v", payload["stream_id"])
	}
}
