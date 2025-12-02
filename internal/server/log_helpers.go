package server

import (
	"context"
	"log/slog"
	"net/http"

	"bitriver-live/internal/observability/logging"
)

// loggingWithRequest returns a logger annotated with request-scoped fields.
// The logger is enriched with request and stream IDs from the context alongside
// the HTTP path, the resolved client IP address, and the IP source so middleware
// logs stay aligned on shared keys.
func loggingWithRequest(base *slog.Logger, resolver *clientIPResolver, r *http.Request) *slog.Logger {
	if base == nil || r == nil {
		return nil
	}

	logger := loggerWithRequestContext(r.Context(), base)
	if logger == nil {
		return nil
	}

	ip, source := resolveClientIP(r, resolver)
	return logger.With(
		"path", r.URL.Path,
		"remote_ip", ip,
		"ip_source", source,
	)
}

func loggerWithRequestContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if ctxLogger := logging.LoggerFromContext(ctx); ctxLogger != nil {
		return ctxLogger
	}
	return logging.WithContext(ctx, logger)
}
