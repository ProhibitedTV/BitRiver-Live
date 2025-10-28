package logging

import (
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
