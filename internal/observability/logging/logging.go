package logging

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

type Config struct {
	Level  string
	Writer io.Writer
}

// New creates a structured slog.Logger using the provided configuration.
func New(cfg Config) *slog.Logger {
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: parseLevel(cfg.Level)})
	return slog.New(handler)
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
