package logging

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"bitriver-live/internal/observability/metrics"
)

type Config struct {
	Level  string
	Writer io.Writer
	Format string
}

type LogFormat string

const (
	FormatJSON LogFormat = "json"
	FormatText LogFormat = "text"
)

// Init creates a slog.Logger using the provided configuration and installs it
// as the process-wide default logger.
func Init(cfg Config) *slog.Logger {
	logger := New(cfg)
	slog.SetDefault(logger)
	return logger
}

// New creates a structured slog.Logger using the provided configuration.
func New(cfg Config) *slog.Logger {
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	handler := newHandler(cfg, writer)
	return slog.New(handler)
}

func newHandler(cfg Config, writer io.Writer) slog.Handler {
	options := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}
	switch LogFormat(strings.ToLower(strings.TrimSpace(cfg.Format))) {
	case FormatText:
		return slog.NewTextHandler(writer, options)
	default:
		return slog.NewJSONHandler(writer, options)
	}
}

func parseLevel(level string) slog.Leveler {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		l := slog.LevelDebug
		return &l
	case "warn", "warning":
		l := slog.LevelWarn
		return &l
	case "error":
		l := slog.LevelError
		return &l
	case "info", "":
		fallthrough
	default:
		l := slog.LevelInfo
		return &l
	}
}

// WithComponent returns a logger annotated with the provided component field.
func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	if logger == nil {
		return nil
	}
	return logger.With("component", component)
}

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	streamIDKey  contextKey = "stream_id"
	loggerKey    contextKey = "logger"
)

// ContextWithRequestID adds the provided request ID to the context when it is non-empty.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, trimmed)
}

// RequestIDFromContext extracts the request ID previously stored on the context.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(requestIDKey).(string)
	return value, ok && value != ""
}

// ContextWithStreamID adds the provided stream ID to the context when it is non-empty.
func ContextWithStreamID(ctx context.Context, id string) context.Context {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return ctx
	}
	return context.WithValue(ctx, streamIDKey, trimmed)
}

// StreamIDFromContext extracts the stream ID previously stored on the context.
func StreamIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	value, ok := ctx.Value(streamIDKey).(string)
	return value, ok && value != ""
}

// ContextWithLogger attaches a logger to the context when available.
func ContextWithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	if logger == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey, logger)
}

// LoggerFromContext retrieves a logger previously stored on the context.
func LoggerFromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return nil
	}
	if logger, ok := ctx.Value(loggerKey).(*slog.Logger); ok {
		return logger
	}
	return nil
}

// WithContext returns a logger annotated with request and stream IDs held in the context.
func WithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return nil
	}
	if requestID, ok := RequestIDFromContext(ctx); ok {
		logger = logger.With("request_id", requestID)
	}
	if streamID, ok := StreamIDFromContext(ctx); ok {
		logger = logger.With("stream_id", streamID)
	}
	return logger
}

// RequestLoggerConfig configures the HTTP request logging middleware.
type RequestLoggerConfig struct {
	Logger            *slog.Logger
	DisableRemoteAddr bool
	AdditionalFields  func(*http.Request, int, time.Duration) []any
}

// RequestLogger returns middleware that logs HTTP requests using the provided
// configuration. It captures method, path, status, duration, and optionally the
// remote address alongside any additional fields supplied by the caller.
func RequestLogger(cfg RequestLoggerConfig) func(http.Handler) http.Handler {
	baseLogger := cfg.Logger
	if baseLogger == nil {
		baseLogger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := metrics.NewResponseRecorder(w)
			start := time.Now()
			next.ServeHTTP(recorder, r)

			duration := time.Since(start)
			requestLogger := WithContext(r.Context(), baseLogger)
			if requestLogger == nil {
				return
			}

			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.Status(),
				"duration_ms", duration.Milliseconds(),
			}

			if !cfg.DisableRemoteAddr {
				attrs = append(attrs, "remote_addr", r.RemoteAddr)
			}

			if cfg.AdditionalFields != nil {
				attrs = append(attrs, cfg.AdditionalFields(r, recorder.Status(), duration)...)
			}

			requestLogger.Info("request completed", attrs...)
		})
	}
}
