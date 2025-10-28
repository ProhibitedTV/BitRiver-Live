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
)

type requestLabel struct {
	method string
	path   string
	status string
}

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
	monetizationTotal map[string]float64
}

var defaultRecorder = New()

func New() *Recorder {
	return &Recorder{
		requestCount:      make(map[requestLabel]uint64),
		requestDuration:   make(map[requestLabel]time.Duration),
		streamEvents:      make(map[string]uint64),
		ingestHealthValue: make(map[string]float64),
		ingestHealthState: make(map[string]string),
		chatEvents:        make(map[string]uint64),
		monetizationCount: make(map[string]uint64),
		monetizationTotal: make(map[string]float64),
	}
}

func Default() *Recorder {
	return defaultRecorder
}

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

func (r *Recorder) StreamStarted() {
	r.incrementStreamEvent("start")
	r.activeStreams.Add(1)
}

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
func (r *Recorder) ObserveMonetization(event string, amount float64) {
	normalized := strings.ToLower(strings.TrimSpace(event))
	if normalized == "" {
		normalized = "unknown"
	}
	r.mu.Lock()
	r.monetizationCount[normalized]++
	r.monetizationTotal[normalized] += amount
	r.mu.Unlock()
}

func (r *Recorder) ActiveStreams() int64 {
	return r.activeStreams.Load()
}

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

func (r *Recorder) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		r.Write(w)
	})
}

func (r *Recorder) Write(w io.Writer) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	fmt.Fprintln(w, "# HELP bitriver_http_requests_total Total number of HTTP requests processed by the API")
	fmt.Fprintln(w, "# TYPE bitriver_http_requests_total counter")
	for _, label := range r.sortedRequestLabels() {
		count := r.requestCount[label]
		fmt.Fprintf(w, "bitriver_http_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n", label.method, label.path, label.status, count)
	}

	fmt.Fprintln(w, "# HELP bitriver_http_request_duration_seconds_sum Cumulative duration of HTTP requests in seconds")
	fmt.Fprintln(w, "# TYPE bitriver_http_request_duration_seconds_sum counter")
	for _, label := range r.sortedRequestLabels() {
		duration := r.requestDuration[label].Seconds()
		fmt.Fprintf(w, "bitriver_http_request_duration_seconds_sum{method=\"%s\",path=\"%s\",status=\"%s\"} %f\n", label.method, label.path, label.status, duration)
	}

	fmt.Fprintln(w, "# HELP bitriver_http_request_duration_seconds_count Total number of observations for request durations")
	fmt.Fprintln(w, "# TYPE bitriver_http_request_duration_seconds_count counter")
	for _, label := range r.sortedRequestLabels() {
		count := r.requestCount[label]
		fmt.Fprintf(w, "bitriver_http_request_duration_seconds_count{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n", label.method, label.path, label.status, count)
	}

	fmt.Fprintln(w, "# HELP bitriver_stream_events_total Stream lifecycle events by type")
	fmt.Fprintln(w, "# TYPE bitriver_stream_events_total counter")
	for _, event := range r.sortedStreamEvents() {
		value := r.streamEvents[event]
		fmt.Fprintf(w, "bitriver_stream_events_total{event=\"%s\"} %d\n", event, value)
	}

	fmt.Fprintln(w, "# HELP bitriver_active_streams Current number of streams marked as live")
	fmt.Fprintln(w, "# TYPE bitriver_active_streams gauge")
	fmt.Fprintf(w, "bitriver_active_streams %d\n", r.activeStreams.Load())

	fmt.Fprintln(w, "# HELP bitriver_ingest_health Health status reported by ingest dependencies (1=ok,0=disabled,-1=degraded)")
	fmt.Fprintln(w, "# TYPE bitriver_ingest_health gauge")
	for _, service := range r.sortedIngestServices() {
		value := r.ingestHealthValue[service]
		status := r.ingestHealthState[service]
		fmt.Fprintf(w, "bitriver_ingest_health{service=\"%s\",status=\"%s\"} %f\n", service, status, value)
	}

	fmt.Fprintln(w, "# HELP bitriver_chat_events_total Chat events by type")
	fmt.Fprintln(w, "# TYPE bitriver_chat_events_total counter")
	for _, event := range r.sortedChatEvents() {
		count := r.chatEvents[event]
		fmt.Fprintf(w, "bitriver_chat_events_total{event=\"%s\"} %d\n", event, count)
	}

	fmt.Fprintln(w, "# HELP bitriver_monetization_events_total Monetization events by type")
	fmt.Fprintln(w, "# TYPE bitriver_monetization_events_total counter")
	for _, event := range r.sortedMonetizationEvents() {
		count := r.monetizationCount[event]
		fmt.Fprintf(w, "bitriver_monetization_events_total{event=\"%s\"} %d\n", event, count)
	}

	fmt.Fprintln(w, "# HELP bitriver_monetization_amount_sum Total monetization amount by event type")
	fmt.Fprintln(w, "# TYPE bitriver_monetization_amount_sum counter")
	for _, event := range r.sortedMonetizationEvents() {
		total := r.monetizationTotal[event]
		fmt.Fprintf(w, "bitriver_monetization_amount_sum{event=\"%s\"} %f\n", event, total)
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
	events := make([]string, 0, len(r.monetizationCount))
	for event := range r.monetizationCount {
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
