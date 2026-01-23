package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bitriver-live/internal/observability/metrics"
)

func TestViewerQoE(t *testing.T) {
	handler, _ := newTestHandler(t)
	recorder := metrics.New()
	original := metrics.Default()
	metrics.SetDefault(recorder)
	t.Cleanup(func() { metrics.SetDefault(original) })

	payload := `{"channelId":"chan-123","sessionId":"sess-1","event":"playing","player":"hls.js","protocol":"hls","rendition":"720p","latencyMode":"low-latency"}`
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/qoe", bytes.NewBufferString(payload))
	rec := httptest.NewRecorder()

	handler.ViewerQoE(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected accepted response, got %d", rec.Code)
	}

	var output bytes.Buffer
	recorder.Write(&output)
	if !strings.Contains(output.String(), "bitriver_viewer_qoe_events_total") {
		t.Fatalf("expected viewer QoE metrics to be recorded")
	}
}
