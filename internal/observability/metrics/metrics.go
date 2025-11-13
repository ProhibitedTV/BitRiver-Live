package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bitriver-live/internal/models"
)

type requestLabel struct {
	method string
	path   string
	status string
}

// Recorder aggregates in-memory metrics counters and gauges for HTTP requests,
// stream lifecycle events, ingest health, chat activity, and monetization
// signals. It coordinates concurrent writers via a RWMutex while exposing a
// thread-safe gauge for active stream tracking.
type Recorder struct {
	mu                sync.RWMutex
	requestCount      map[requestLabel]uint64
	requestDuration   map[requestLabel]time.Duration
	streamEvents      map[string]uint64
	ingestHealthValue map[string]float64
	ingestHealthState map[string]string
	activeStreams     atomic.Int64
	chatEvents        map[string]uint64
	monetizationCount map[string]uint64
	monetizationTotal map[string]models.Money
}

var defaultRecorder = New()

// New constructs an empty Recorder with initialized backing maps so callers can
// immediately record metrics without additional setup.
func New() *Recorder {
	return &Recorder{
		requestCount:      make(map[requestLabel]uint64),
		requestDuration:   make(map[requestLabel]time.Duration),
		streamEvents:      make(map[string]uint64),
		ingestHealthValue: make(map[string]float64),
		ingestHealthState: make(map[string]string),
		chatEvents:        make(map[string]uint64),
		monetizationCount: make(map[string]uint64),
		monetizationTotal: make(map[string]models.Money),
	}
}

// Default returns the singleton Recorder instance shared across helper
// functions for packages that do not require custom instrumentation pipelines.
func Default() *Recorder {
	return defaultRecorder
}

// ObserveRequest normalizes the request label set and accumulates totals for
// request count and cumulative duration by HTTP method, normalized path, and
// status code.
func (r *Recorder) ObserveRequest(method, path string, status int, duration time.Duration) {
	label := requestLabel{
		method: strings.ToUpper(method),
		path:   normalizePath(path),
		status: fmt.Sprintf("%d", status),
	}
	r.mu.Lock()
	r.requestCount[label]++
	r.requestDuration[label] += duration
	r.mu.Unlock()
}

// StreamStarted records a start lifecycle event and increments the active
// stream gauge atomically so concurrent sessions remain consistent.
func (r *Recorder) StreamStarted() {
	r.incrementStreamEvent("start")
	r.activeStreams.Add(1)
}

// StreamStopped records a stop lifecycle event and decrements the active
// stream gauge, guarding against negative counts when concurrent updates race.
func (r *Recorder) StreamStopped() {
	r.incrementStreamEvent("stop")
	for {
		current := r.activeStreams.Load()
		if current <= 0 {
			return
		}
		if r.activeStreams.CompareAndSwap(current, current-1) {
			return
		}
	}
}

func (r *Recorder) incrementStreamEvent(event string) {
	normalized := strings.ToLower(strings.TrimSpace(event))
	if normalized == "" {
		normalized = "unknown"
	}
	r.mu.Lock()
	r.streamEvents[normalized]++
	r.mu.Unlock()
}

// ObserveChatEvent records a chat event type for throughput monitoring.
func (r *Recorder) ObserveChatEvent(event string) {
	normalized := strings.ToLower(strings.TrimSpace(event))
	if normalized == "" {
		normalized = "unknown"
	}
	r.mu.Lock()
	r.chatEvents[normalized]++
	r.mu.Unlock()
}

// ObserveMonetization tracks monetization events, capturing counts and total amounts.
func (r *Recorder) ObserveMonetization(event string, amount models.Money) {
	normalized := strings.ToLower(strings.TrimSpace(event))
	if normalized == "" {
		normalized = "unknown"
	}
	r.mu.Lock()
	r.monetizationCount[normalized]++
	total := r.monetizationTotal[normalized]
	r.monetizationTotal[normalized] = total.Add(amount)
	r.mu.Unlock()
}

// ActiveStreams exposes the current gauge of concurrently active streams.
func (r *Recorder) ActiveStreams() int64 {
	return r.activeStreams.Load()
}

// SetIngestHealth normalizes ingest service identifiers, maps status strings to
// numeric health values, and stores both representations for export.
func (r *Recorder) SetIngestHealth(service, status string) {
	normalizedService := strings.ToLower(strings.TrimSpace(service))
	if normalizedService == "" {
		normalizedService = "unknown"
	}
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	value := 0.0
	switch normalizedStatus {
	case "ok", "healthy":
		value = 1
	case "disabled":
		value = 0
	default:
		value = -1
	}
	r.mu.Lock()
	r.ingestHealthValue[normalizedService] = value
	r.ingestHealthState[normalizedService] = normalizedStatus
	r.mu.Unlock()
}

// Handler exposes the Recorder as an http.Handler that writes Prometheus text
// exposition data with the appropriate content type.
func (r *Recorder) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		r.Write(w)
	})
}

// Write renders the Recorder's metrics in Prometheus text format, sorting label
// sets to provide stable output for scrapes and tests.
func (r *Recorder) Write(w io.Writer) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	requestLabels := r.sortedRequestLabels()
	streamEvents := r.sortedStreamEvents()
	ingestServices := r.sortedIngestServices()
	chatEvents := r.sortedChatEvents()
	monetizationEvents := r.sortedMonetizationEvents()

	fmt.Fprintln(w, "# HELP bitriver_http_requests_total Total number of HTTP requests processed by the API")
	fmt.Fprintln(w, "# TYPE bitriver_http_requests_total counter")
	for _, label := range requestLabels {
		count := r.requestCount[label]
		fmt.Fprintf(w, "bitriver_http_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n", label.method, label.path, label.status, count)
	}

	fmt.Fprintln(w, "# HELP bitriver_http_request_duration_seconds_sum Cumulative duration of HTTP requests in seconds")
	fmt.Fprintln(w, "# TYPE bitriver_http_request_duration_seconds_sum counter")
	for _, label := range requestLabels {
		duration := r.requestDuration[label].Seconds()
		fmt.Fprintf(w, "bitriver_http_request_duration_seconds_sum{method=\"%s\",path=\"%s\",status=\"%s\"} %f\n", label.method, label.path, label.status, duration)
	}

	fmt.Fprintln(w, "# HELP bitriver_http_request_duration_seconds_count Total number of observations for request durations")
	fmt.Fprintln(w, "# TYPE bitriver_http_request_duration_seconds_count counter")
	for _, label := range requestLabels {
		count := r.requestCount[label]
		fmt.Fprintf(w, "bitriver_http_request_duration_seconds_count{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n", label.method, label.path, label.status, count)
	}

	fmt.Fprintln(w, "# HELP bitriver_stream_events_total Stream lifecycle events by type")
	fmt.Fprintln(w, "# TYPE bitriver_stream_events_total counter")
	for _, event := range streamEvents {
		value := r.streamEvents[event]
		fmt.Fprintf(w, "bitriver_stream_events_total{event=\"%s\"} %d\n", event, value)
	}

	fmt.Fprintln(w, "# HELP bitriver_active_streams Current number of streams marked as live")
	fmt.Fprintln(w, "# TYPE bitriver_active_streams gauge")
	fmt.Fprintf(w, "bitriver_active_streams %d\n", r.activeStreams.Load())

	fmt.Fprintln(w, "# HELP bitriver_ingest_health Health status reported by ingest dependencies (1=ok,0=disabled,-1=degraded)")
	fmt.Fprintln(w, "# TYPE bitriver_ingest_health gauge")
	for _, service := range ingestServices {
		value := r.ingestHealthValue[service]
		status := r.ingestHealthState[service]
		fmt.Fprintf(w, "bitriver_ingest_health{service=\"%s\",status=\"%s\"} %f\n", service, status, value)
	}

	fmt.Fprintln(w, "# HELP bitriver_chat_events_total Chat events by type")
	fmt.Fprintln(w, "# TYPE bitriver_chat_events_total counter")
	for _, event := range chatEvents {
		count := r.chatEvents[event]
		fmt.Fprintf(w, "bitriver_chat_events_total{event=\"%s\"} %d\n", event, count)
	}

	fmt.Fprintln(w, "# HELP bitriver_monetization_events_total Monetization events by type")
	fmt.Fprintln(w, "# TYPE bitriver_monetization_events_total counter")
	for _, event := range monetizationEvents {
		count := r.monetizationCount[event]
		fmt.Fprintf(w, "bitriver_monetization_events_total{event=\"%s\"} %d\n", event, count)
	}

	fmt.Fprintln(w, "# HELP bitriver_monetization_amount_sum Total monetization amount by event type")
	fmt.Fprintln(w, "# TYPE bitriver_monetization_amount_sum counter")
	for _, event := range monetizationEvents {
		total := r.monetizationTotal[event]
		fmt.Fprintf(w, "bitriver_monetization_amount_sum{event=\"%s\"} %s\n", event, total.DecimalString())
	}
}

func (r *Recorder) sortedRequestLabels() []requestLabel {
	labels := make([]requestLabel, 0, len(r.requestCount))
	for label := range r.requestCount {
		labels = append(labels, label)
	}
	sort.Slice(labels, func(i, j int) bool {
		if labels[i].method != labels[j].method {
			return labels[i].method < labels[j].method
		}
		if labels[i].path != labels[j].path {
			return labels[i].path < labels[j].path
		}
		return labels[i].status < labels[j].status
	})
	return labels
}

func (r *Recorder) sortedStreamEvents() []string {
	events := make([]string, 0, len(r.streamEvents))
	for event := range r.streamEvents {
		events = append(events, event)
	}
	sort.Strings(events)
	return events
}

func (r *Recorder) sortedIngestServices() []string {
	services := make([]string, 0, len(r.ingestHealthValue))
	for service := range r.ingestHealthValue {
		services = append(services, service)
	}
	sort.Strings(services)
	return services
}

func (r *Recorder) sortedChatEvents() []string {
	events := make([]string, 0, len(r.chatEvents))
	for event := range r.chatEvents {
		events = append(events, event)
	}
	sort.Strings(events)
	return events
}

func (r *Recorder) sortedMonetizationEvents() []string {
	totalEvents := len(r.monetizationCount) + len(r.monetizationTotal)
	seen := make(map[string]struct{}, totalEvents)
	events := make([]string, 0, totalEvents)
	for event := range r.monetizationCount {
		if _, exists := seen[event]; exists {
			continue
		}
		seen[event] = struct{}{}
		events = append(events, event)
	}
	for event := range r.monetizationTotal {
		if _, exists := seen[event]; exists {
			continue
		}
		seen[event] = struct{}{}
		events = append(events, event)
	}
	sort.Strings(events)
	return events
}

func normalizePath(path string) string {
	if path == "" || path == "/" {
		return "/"
	}
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if part == "" {
			continue
		}
		if looksLikeIdentifier(part) {
			parts[i] = ":id"
			continue
		}
	}
	normalized := strings.Join(parts, "/")
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}
	if strings.HasSuffix(normalized, "/") && len(normalized) > 1 {
		normalized = strings.TrimSuffix(normalized, "/")
	}
	return normalized
}

func looksLikeIdentifier(segment string) bool {
	if len(segment) >= 8 {
		return true
	}
	digitCount := 0
	for _, r := range segment {
		if r >= '0' && r <= '9' {
			digitCount++
		}
	}
	return digitCount >= 3
}

// ObserveRequest is a helper on the default recorder.
func ObserveRequest(method, path string, status int, duration time.Duration) {
	defaultRecorder.ObserveRequest(method, path, status, duration)
}

// StreamStarted increments counters on the default recorder.
func StreamStarted() {
	defaultRecorder.StreamStarted()
}

// StreamStopped decrements active streams on the default recorder.
func StreamStopped() {
	defaultRecorder.StreamStopped()
}

// SetIngestHealth updates ingest health for the default recorder.
func SetIngestHealth(service, status string) {
	defaultRecorder.SetIngestHealth(service, status)
}

// Handler exposes the default recorder as an HTTP handler.
func Handler() http.Handler {
	return defaultRecorder.Handler()
}
