package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"bitriver-live/internal/observability/logging"
)

type idGenerator func() string

func requestIDMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return requestIDMiddlewareWithGenerator(logger, newRequestID, next)
}

func requestIDMiddlewareWithGenerator(logger *slog.Logger, generator idGenerator, next http.Handler) http.Handler {
	if generator == nil {
		generator = newRequestID
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if requestID == "" {
			requestID = generator()
		}
		streamID := strings.TrimSpace(r.Header.Get("X-Stream-Id"))

		ctx := logging.ContextWithRequestID(r.Context(), requestID)
		if streamID != "" {
			ctx = logging.ContextWithStreamID(ctx, streamID)
		}
		ctxLogger := logging.WithContext(ctx, logger)
		ctx = logging.ContextWithLogger(ctx, ctxLogger)

		if requestID != "" {
			w.Header().Set("X-Request-Id", requestID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newRequestID() string {
	var buffer [16]byte
	if _, err := rand.Read(buffer[:]); err == nil {
		return hex.EncodeToString(buffer[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func loggerWithRequestContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if ctxLogger := logging.LoggerFromContext(ctx); ctxLogger != nil {
		return ctxLogger
	}
	return logging.WithContext(ctx, logger)
}
