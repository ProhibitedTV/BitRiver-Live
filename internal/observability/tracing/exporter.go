package tracing

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// SpanData is the exported representation of a span.
type SpanData struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Name         string
	StartTime    time.Time
	EndTime      time.Time
	Attributes   []Attribute
	Error        error
	Resource     resourceInfo
}

// Exporter writes span data to an external sink.
type Exporter interface {
	Export(ctx context.Context, span SpanData) error
	Shutdown(ctx context.Context) error
}

type httpExporter struct {
	endpoint string
	logger   *slog.Logger
	client   *http.Client
}

func newHTTPExporter(endpoint string, logger *slog.Logger) Exporter {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return nil
	}
	return &httpExporter{
		endpoint: strings.TrimRight(trimmed, "/") + "/v1/traces",
		logger:   logger,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

func (e *httpExporter) Export(ctx context.Context, span SpanData) error {
	if e == nil {
		return nil
	}
	payload := map[string]any{
		"resource": map[string]string{
			"service.name":           span.Resource.serviceName,
			"deployment.environment": span.Resource.environment,
		},
		"span": map[string]any{
			"trace_id":       span.TraceID,
			"span_id":        span.SpanID,
			"parent_span_id": span.ParentSpanID,
			"name":           span.Name,
			"start_time":     span.StartTime.Format(time.RFC3339Nano),
			"end_time":       span.EndTime.Format(time.RFC3339Nano),
			"attributes":     attributesToMap(span.Attributes),
			"error":          errorString(span.Error),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		if e.logger != nil {
			e.logger.Debug("trace export failed", "error", err)
		}
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (e *httpExporter) Shutdown(context.Context) error {
	return nil
}

func attributesToMap(attrs []Attribute) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	values := make(map[string]any, len(attrs))
	for _, attr := range attrs {
		if attr.Key == "" {
			continue
		}
		values[attr.Key] = attr.Value
	}
	return values
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
