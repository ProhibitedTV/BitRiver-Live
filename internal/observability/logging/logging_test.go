package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestNewUsesStdoutByDefault(t *testing.T) {
	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = originalStdout
		_ = w.Close()
		_ = r.Close()
	})

	logger := New(Config{})
	logger.Info("hello")

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if len(data) == 0 {
		t.Fatalf("expected output on stdout, got none")
	}
}

func TestNewRespectsCustomWriter(t *testing.T) {
	var buf bytes.Buffer

	logger := New(Config{Writer: &buf})
	logger.Info("custom writer")

	if buf.Len() == 0 {
		t.Fatalf("expected output in custom writer, got none")
	}
}

func TestParseLevel(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{name: "debug", input: "debug", expected: slog.LevelDebug},
		{name: "warning", input: "warning", expected: slog.LevelWarn},
		{name: "warn", input: "warn", expected: slog.LevelWarn},
		{name: "error", input: "error", expected: slog.LevelError},
		{name: "info", input: "info", expected: slog.LevelInfo},
		{name: "empty", input: "", expected: slog.LevelInfo},
		{name: "mixed case", input: " DeBuG ", expected: slog.LevelDebug},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			leveler := parseLevel(tc.input)
			if leveler == nil {
				t.Fatalf("expected leveler, got nil")
			}

			if got := leveler.Level(); got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestWithComponent(t *testing.T) {
	t.Run("adds component attribute", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))

		WithComponent(logger, "api").Info("component set")

		var payload map[string]any
		if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
			t.Fatalf("failed to unmarshal log output: %v", err)
		}

		if payload["component"] != "api" {
			t.Fatalf("expected component \"api\", got %v", payload["component"])
		}
	})

	t.Run("nil logger returns nil", func(t *testing.T) {
		if got := WithComponent(nil, "anything"); got != nil {
			t.Fatalf("expected nil logger, got %v", got)
		}
	})
}

func TestContextWithRequestAndStreamIDs(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "req-123")
	ctx = ContextWithStreamID(ctx, "stream-456")

	if id, ok := RequestIDFromContext(ctx); !ok || id != "req-123" {
		t.Fatalf("expected request id req-123, got %q", id)
	}
	if id, ok := StreamIDFromContext(ctx); !ok || id != "stream-456" {
		t.Fatalf("expected stream id stream-456, got %q", id)
	}
}

func TestWithContextAnnotatesLogger(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "req-1")
	ctx = ContextWithStreamID(ctx, "stream-1")

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	WithContext(ctx, logger).Info("hello")

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal log output: %v", err)
	}

	if payload["request_id"] != "req-1" {
		t.Fatalf("expected request_id to be set, got %v", payload["request_id"])
	}
	if payload["stream_id"] != "stream-1" {
		t.Fatalf("expected stream_id to be set, got %v", payload["stream_id"])
	}
}

func TestInitSetsDefaultLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := Init(Config{Writer: &buf, Format: string(FormatText), Level: "debug"})
	if logger != slog.Default() {
		t.Fatalf("expected Init to replace the default logger")
	}

	slog.Info("hello world")

	if !strings.Contains(buf.String(), "hello world") {
		t.Fatalf("expected text output to include message, got %q", buf.String())
	}
}

func TestRequestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	middleware := RequestLogger(RequestLoggerConfig{Logger: logger})

	req := httptest.NewRequest(http.MethodPost, "/widgets/abc123", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	recorder := httptest.NewRecorder()

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})).ServeHTTP(recorder, req)

	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode log entry: %v", err)
	}

	if payload["status"] != float64(http.StatusAccepted) {
		t.Fatalf("expected status %d, got %v", http.StatusAccepted, payload["status"])
	}
	if payload["remote_addr"] != "127.0.0.1:1234" {
		t.Fatalf("expected remote_addr to be recorded, got %v", payload["remote_addr"])
	}
	if payload["path"] != "/widgets/abc123" {
		t.Fatalf("expected path to be logged, got %v", payload["path"])
	}
}
