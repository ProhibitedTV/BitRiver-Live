package metrics

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"bitriver-live/internal/models"
)

func TestObserveRequestAndNormalizePath(t *testing.T) {
	recorder := New()

	type testCase struct {
		name     string
		method   string
		path     string
		status   int
		duration time.Duration
	}

	cases := []testCase{
		{
			name:     "root path",
			method:   "get",
			path:     "/",
			status:   200,
			duration: 50 * time.Millisecond,
		},
		{
			name:     "empty path",
			method:   "GET",
			path:     "",
			status:   200,
			duration: 25 * time.Millisecond,
		},
		{
			name:     "id segment",
			method:   "post",
			path:     "/users/123",
			status:   201,
			duration: 100 * time.Millisecond,
		},
		{
			name:     "trailing slash and alpha id",
			method:   "POST",
			path:     "/users/abc123def/",
			status:   201,
			duration: 50 * time.Millisecond,
		},
		{
			name:     "multi ids",
			method:   "PATCH",
			path:     "streams/abc/456/extra",
			status:   404,
			duration: 10 * time.Millisecond,
		},
	}

	expectedCounts := make(map[requestLabel]struct {
		count    uint64
		duration time.Duration
	})

	for _, tc := range cases {
		recorder.ObserveRequest(tc.method, tc.path, tc.status, tc.duration)

		label := requestLabel{
			method: strings.ToUpper(tc.method),
			path:   normalizePath(tc.path),
			status: fmt.Sprintf("%d", tc.status),
		}
		current := expectedCounts[label]
		current.count++
		current.duration += tc.duration
		expectedCounts[label] = current
	}

	if len(recorder.requestCount) != len(expectedCounts) {
		t.Fatalf("unexpected number of labels: got %d want %d", len(recorder.requestCount), len(expectedCounts))
	}

	for label, expected := range expectedCounts {
		gotCount := recorder.requestCount[label]
		gotDuration := recorder.requestDuration[label]
		if gotCount != expected.count {
			t.Errorf("count mismatch for %+v: got %d want %d", label, gotCount, expected.count)
		}
		if gotDuration != expected.duration {
			t.Errorf("duration mismatch for %+v: got %s want %s", label, gotDuration, expected.duration)
		}
	}

	labels := recorder.sortedRequestLabels()
	sortedExpected := make([]requestLabel, 0, len(expectedCounts))
	for label := range expectedCounts {
		sortedExpected = append(sortedExpected, label)
	}
	sort.Slice(sortedExpected, func(i, j int) bool {
		if sortedExpected[i].method != sortedExpected[j].method {
			return sortedExpected[i].method < sortedExpected[j].method
		}
		if sortedExpected[i].path != sortedExpected[j].path {
			return sortedExpected[i].path < sortedExpected[j].path
		}
		return sortedExpected[i].status < sortedExpected[j].status
	})

	if len(labels) != len(sortedExpected) {
		t.Fatalf("sorted labels length mismatch: got %d want %d", len(labels), len(sortedExpected))
	}

	for i := range labels {
		if labels[i] != sortedExpected[i] {
			t.Errorf("sorted label %d mismatch: got %+v want %+v", i, labels[i], sortedExpected[i])
		}
	}
}

func TestStreamGaugeConcurrent(t *testing.T) {
	recorder := New()

	var wg sync.WaitGroup
	starts := 100
	stops := 150

	wg.Add(starts + stops)
	for i := 0; i < starts; i++ {
		go func() {
			defer wg.Done()
			recorder.StreamStarted()
		}()
	}
	for i := 0; i < stops; i++ {
		go func() {
			defer wg.Done()
			recorder.StreamStopped()
		}()
	}

	wg.Wait()

	if active := recorder.ActiveStreams(); active != 0 {
		t.Fatalf("active streams should not go negative; got %d", active)
	}

	if count := recorder.streamEvents["start"]; count != uint64(starts) {
		t.Fatalf("unexpected start events: got %d want %d", count, starts)
	}
	if count := recorder.streamEvents["stop"]; count != uint64(stops) {
		t.Fatalf("unexpected stop events: got %d want %d", count, stops)
	}
}

func TestWriteAndHandlerOutput(t *testing.T) {
	recorder := New()

	recorder.ObserveRequest("GET", "/users/abc123", 200, 150*time.Millisecond)
	recorder.ObserveRequest("get", "/users/456/", 200, 50*time.Millisecond)
	recorder.ObserveRequest("POST", "/users", 201, time.Second)

	recorder.StreamStarted()
	recorder.StreamStarted()
	recorder.StreamStopped()

	recorder.SetIngestHealth(" Ingest-A ", "Healthy")
	recorder.SetIngestHealth("backup", "Degraded")

	recorder.ObserveChatEvent("message")
	recorder.ObserveChatEvent("message")

	recorder.ObserveMonetization("tip", models.MustParseMoney("1.5"))
	recorder.ObserveMonetization("tip", models.MustParseMoney("0.25"))
	recorder.ObserveMonetization("subscription", models.MustParseMoney("10"))

	var buf bytes.Buffer
	recorder.Write(&buf)

	expected := `# HELP bitriver_http_requests_total Total number of HTTP requests processed by the API
# TYPE bitriver_http_requests_total counter
bitriver_http_requests_total{method="GET",path="/users/:id",status="200"} 2
bitriver_http_requests_total{method="POST",path="/users",status="201"} 1
# HELP bitriver_http_request_duration_seconds_sum Cumulative duration of HTTP requests in seconds
# TYPE bitriver_http_request_duration_seconds_sum counter
bitriver_http_request_duration_seconds_sum{method="GET",path="/users/:id",status="200"} 0.200000
bitriver_http_request_duration_seconds_sum{method="POST",path="/users",status="201"} 1.000000
# HELP bitriver_http_request_duration_seconds_count Total number of observations for request durations
# TYPE bitriver_http_request_duration_seconds_count counter
bitriver_http_request_duration_seconds_count{method="GET",path="/users/:id",status="200"} 2
bitriver_http_request_duration_seconds_count{method="POST",path="/users",status="201"} 1
# HELP bitriver_stream_events_total Stream lifecycle events by type
# TYPE bitriver_stream_events_total counter
bitriver_stream_events_total{event="start"} 2
bitriver_stream_events_total{event="stop"} 1
# HELP bitriver_active_streams Current number of streams marked as live
# TYPE bitriver_active_streams gauge
bitriver_active_streams 1
# HELP bitriver_ingest_health Health status reported by ingest dependencies (1=ok,0=disabled,-1=degraded)
# TYPE bitriver_ingest_health gauge
bitriver_ingest_health{service="backup",status="degraded"} -1.000000
bitriver_ingest_health{service="ingest-a",status="healthy"} 1.000000
# HELP bitriver_chat_events_total Chat events by type
# TYPE bitriver_chat_events_total counter
bitriver_chat_events_total{event="message"} 2
# HELP bitriver_monetization_events_total Monetization events by type
# TYPE bitriver_monetization_events_total counter
bitriver_monetization_events_total{event="subscription"} 1
bitriver_monetization_events_total{event="tip"} 2
# HELP bitriver_monetization_amount_sum Total monetization amount by event type
# TYPE bitriver_monetization_amount_sum counter
bitriver_monetization_amount_sum{event="subscription"} 10
bitriver_monetization_amount_sum{event="tip"} 1.75`

	if diff := compareLines(buf.String(), expected); diff != "" {
		t.Fatalf("unexpected write output:\n%s", diff)
	}

	res := httptest.NewRecorder()
	recorder.Handler().ServeHTTP(res, httptest.NewRequest("GET", "/metrics", nil))

	if contentType := res.Result().Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("unexpected content type: %s", contentType)
	}

	if diff := compareLines(res.Body.String(), expected); diff != "" {
		t.Fatalf("unexpected handler output:\n%s", diff)
	}
}

func compareLines(actual, expected string) string {
	actualLines := strings.Split(strings.TrimSpace(actual), "\n")
	expectedLines := strings.Split(strings.TrimSpace(expected), "\n")
	if len(actualLines) != len(expectedLines) {
		return formatDiff(actualLines, expectedLines)
	}
	for i := range actualLines {
		if actualLines[i] != expectedLines[i] {
			return formatDiff(actualLines, expectedLines)
		}
	}
	return ""
}

func formatDiff(actual, expected []string) string {
	var b strings.Builder
	b.WriteString("expected\n")
	for _, line := range expected {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("got\n")
	for _, line := range actual {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
