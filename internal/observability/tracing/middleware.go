package tracing

import (
	"net/http"
	"time"

	"bitriver-live/internal/observability/logging"
	"bitriver-live/internal/observability/metrics"
)

// HTTPMiddleware wraps HTTP handlers with tracing spans.
func HTTPMiddleware(tracer *Tracer, next http.Handler) http.Handler {
	if tracer == nil {
		tracer = Default()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if parent := r.Header.Get("traceparent"); parent != "" {
			if parsed, err := ParseTraceParent(parent); err == nil {
				ctx = ContextWithTrace(ctx, parsed)
			}
		}

		attrs := []Attribute{
			StringAttr("http.method", r.Method),
			StringAttr("http.path", r.URL.Path),
			StringAttr("http.scheme", scheme(r)),
		}
		ctx, span := tracer.StartSpan(ctx, "http.request", attrs...)

		if traceCtx, ok := TraceFromContext(ctx); ok {
			if header, err := TraceParent(traceCtx); err == nil {
				w.Header().Set("traceparent", header)
			}
			if logger := logging.LoggerFromContext(ctx); logger != nil {
				ctx = logging.ContextWithLogger(ctx, logger.With("trace_id", traceCtx.TraceID, "span_id", traceCtx.SpanID))
			}
		}

		recorder := metrics.NewResponseRecorder(w)
		start := time.Now()
		next.ServeHTTP(recorder, r.WithContext(ctx))
		span.AddAttributes(IntAttr("http.status_code", recorder.Status()))
		span.AddAttributes(FloatAttr("http.duration_ms", float64(time.Since(start).Milliseconds())))
		span.End()
	})
}

func scheme(r *http.Request) string {
	if r == nil {
		return ""
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
