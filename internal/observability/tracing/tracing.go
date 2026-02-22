package tracing

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"
)

type contextKey string

const traceContextKey contextKey = "trace_ctx"

// Config defines how traces are generated and exported.
type Config struct {
	ServiceName string
	Environment string
	Endpoint    string
	SampleRatio float64
	Logger      *slog.Logger
}

// Attribute represents a span attribute.
type Attribute struct {
	Key   string
	Value any
}

// StringAttr returns a string attribute.
func StringAttr(key, value string) Attribute {
	return Attribute{Key: key, Value: value}
}

// IntAttr returns an integer attribute.
func IntAttr(key string, value int) Attribute {
	return Attribute{Key: key, Value: value}
}

// FloatAttr returns a float attribute.
func FloatAttr(key string, value float64) Attribute {
	return Attribute{Key: key, Value: value}
}

// BoolAttr returns a boolean attribute.
func BoolAttr(key string, value bool) Attribute {
	return Attribute{Key: key, Value: value}
}

// TraceContext represents the current trace identifiers.
type TraceContext struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Sampled      bool
}

// TraceFromContext extracts trace metadata from a context.
func TraceFromContext(ctx context.Context) (TraceContext, bool) {
	if ctx == nil {
		return TraceContext{}, false
	}
	value, ok := ctx.Value(traceContextKey).(TraceContext)
	return value, ok && value.TraceID != ""
}

// ContextWithTrace stores trace metadata on the context.
func ContextWithTrace(ctx context.Context, tc TraceContext) context.Context {
	ctx = normalizeContext(ctx)
	if tc.TraceID == "" {
		return ctx
	}
	return context.WithValue(ctx, traceContextKey, tc)
}

// Tracer creates spans and exports them via the configured provider.
type Tracer struct {
	provider *Provider
}

// Span represents a unit of work in a trace.
type Span struct {
	name      string
	start     time.Time
	traceID   string
	spanID    string
	parentID  string
	attrs     []Attribute
	err       error
	exporter  Exporter
	resource  resourceInfo
	sampled   bool
	exportMu  *sync.Mutex
	endedOnce bool
}

// Provider owns exporter configuration and creates tracers.
type Provider struct {
	tracer      *Tracer
	exporter    Exporter
	resource    resourceInfo
	sampleRatio float64
	logger      *slog.Logger
	mu          sync.Mutex
}

type resourceInfo struct {
	serviceName string
	environment string
}

var defaultTracer = &Tracer{provider: &Provider{}}

// Default returns the default tracer.
func Default() *Tracer {
	return defaultTracer
}

// SetDefault installs the provided tracer as the package-level default.
func SetDefault(tracer *Tracer) {
	if tracer == nil {
		return
	}
	defaultTracer = tracer
}

// NewProvider constructs a tracing provider and a tracer configured for export.
func NewProvider(cfg Config) *Provider {
	ratio := cfg.SampleRatio
	switch {
	case ratio < 0:
		ratio = 0
	case ratio > 1:
		ratio = 1
	case ratio == 0:
		ratio = 1
	}
	provider := &Provider{
		exporter:    newHTTPExporter(cfg.Endpoint, cfg.Logger),
		resource:    resourceInfo{serviceName: strings.TrimSpace(cfg.ServiceName), environment: strings.TrimSpace(cfg.Environment)},
		sampleRatio: ratio,
		logger:      cfg.Logger,
	}
	provider.tracer = &Tracer{provider: provider}
	return provider
}

// Tracer returns the tracer for this provider.
func (p *Provider) Tracer() *Tracer {
	if p == nil || p.tracer == nil {
		return Default()
	}
	return p.tracer
}

// Shutdown flushes any buffered spans.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil || p.exporter == nil {
		return nil
	}
	return p.exporter.Shutdown(ctx)
}

// StartSpan begins a new span, inheriting trace information from ctx.
func (t *Tracer) StartSpan(ctx context.Context, name string, attrs ...Attribute) (context.Context, *Span) {
	ctx = normalizeContext(ctx)
	provider := t.provider
	if provider == nil {
		provider = &Provider{}
	}

	parent, _ := TraceFromContext(ctx)
	traceID := parent.TraceID
	if traceID == "" {
		traceID = newTraceID()
	}
	parentSpanID := parent.SpanID
	spanID := newSpanID()
	sampled := parent.Sampled
	if !sampled {
		sampled = provider.shouldSample()
	}

	span := &Span{
		name:     name,
		start:    time.Now().UTC(),
		traceID:  traceID,
		spanID:   spanID,
		parentID: parentSpanID,
		attrs:    attrs,
		exporter: provider.exporter,
		resource: provider.resource,
		sampled:  sampled,
		exportMu: &provider.mu,
	}

	nextCtx := ContextWithTrace(ctx, TraceContext{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		Sampled:      sampled,
	})
	return nextCtx, span
}

// AddAttributes appends attributes to the span.
func (s *Span) AddAttributes(attrs ...Attribute) {
	if s == nil {
		return
	}
	s.attrs = append(s.attrs, attrs...)
}

// RecordError attaches an error to the span.
func (s *Span) RecordError(err error) {
	if s == nil || err == nil {
		return
	}
	s.err = err
}

// End finalizes the span and exports it if sampled.
func (s *Span) End() {
	if s == nil || s.endedOnce {
		return
	}
	s.endedOnce = true
	if !s.sampled || s.exporter == nil {
		return
	}
	end := time.Now().UTC()
	spanData := SpanData{
		TraceID:      s.traceID,
		SpanID:       s.spanID,
		ParentSpanID: s.parentID,
		Name:         s.name,
		StartTime:    s.start,
		EndTime:      end,
		Attributes:   s.attrs,
		Error:        s.err,
		Resource:     s.resource,
	}
	s.exportMu.Lock()
	defer s.exportMu.Unlock()
	_ = s.exporter.Export(context.Background(), spanData)
}

func (p *Provider) shouldSample() bool {
	ratio := p.sampleRatio
	if ratio >= 1 {
		return true
	}
	if ratio <= 0 {
		return false
	}
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return false
	}
	return float64(n.Int64())/float64(math.MaxInt64) < ratio
}

func newTraceID() string {
	var buffer [16]byte
	if _, err := rand.Read(buffer[:]); err == nil {
		return hex.EncodeToString(buffer[:])
	}
	return ""
}

func newSpanID() string {
	var buffer [8]byte
	if _, err := rand.Read(buffer[:]); err == nil {
		return hex.EncodeToString(buffer[:])
	}
	return ""
}

// TraceParent encodes the trace context in W3C traceparent format.
func TraceParent(tc TraceContext) (string, error) {
	if tc.TraceID == "" || tc.SpanID == "" {
		return "", errors.New("missing trace identifiers")
	}
	flags := "00"
	if tc.Sampled {
		flags = "01"
	}
	return "00-" + tc.TraceID + "-" + tc.SpanID + "-" + flags, nil
}

// ParseTraceParent extracts trace identifiers from a W3C traceparent header.
func ParseTraceParent(value string) (TraceContext, error) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 4 {
		return TraceContext{}, errors.New("invalid traceparent")
	}
	if len(parts[1]) != 32 || len(parts[2]) != 16 {
		return TraceContext{}, errors.New("invalid trace identifiers")
	}
	sampled := false
	if len(parts[3]) == 2 && parts[3][1] == '1' {
		sampled = true
	}
	return TraceContext{
		TraceID: parts[1],
		SpanID:  parts[2],
		Sampled: sampled,
	}, nil
}
