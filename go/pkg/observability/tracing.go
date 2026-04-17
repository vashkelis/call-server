package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracerConfig contains tracer configuration.
type TracerConfig struct {
	ServiceName    string
	ServiceVersion string
	Endpoint       string
	Enabled        bool
}

// Tracer wraps OpenTelemetry tracer functionality.
type Tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	enabled  bool
}

// NewTracer creates a new tracer.
func NewTracer(cfg TracerConfig) (*Tracer, error) {
	if !cfg.Enabled {
		return &Tracer{enabled: false}, nil
	}

	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(provider)

	tracer := provider.Tracer(cfg.ServiceName)

	return &Tracer{
		provider: provider,
		tracer:   tracer,
		enabled:  true,
	}, nil
}

// StartSpan starts a new span.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, noop.Span{}
	}
	return t.tracer.Start(ctx, name, opts...)
}

// StartSpanWithAttributes starts a new span with attributes.
func (t *Tracer) StartSpanWithAttributes(ctx context.Context, name string, attrs map[string]string) (context.Context, trace.Span) {
	if !t.enabled {
		return ctx, noop.Span{}
	}

	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		otelAttrs = append(otelAttrs, attribute.String(k, v))
	}

	ctx, span := t.tracer.Start(ctx, name)
	span.SetAttributes(otelAttrs...)
	return ctx, span
}

// Shutdown shuts down the tracer provider.
func (t *Tracer) Shutdown(ctx context.Context) error {
	if !t.enabled || t.provider == nil {
		return nil
	}
	return t.provider.Shutdown(ctx)
}

// SpanFromContext returns the current span from context.
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// ContextWithSpan adds a span to the context.
func ContextWithSpan(ctx context.Context, span trace.Span) context.Context {
	return trace.ContextWithSpan(ctx, span)
}

// PipelineStage represents a stage in the voice processing pipeline.
type PipelineStage string

const (
	StageVAD          PipelineStage = "vad"
	StageASR          PipelineStage = "asr"
	StageLLM          PipelineStage = "llm"
	StageTTS          PipelineStage = "tts"
	StageMediaEdge    PipelineStage = "media_edge"
	StageOrchestrator PipelineStage = "orchestrator"
)

// TimestampTracker tracks timestamps for pipeline stages.
type TimestampTracker struct {
	mu         sync.RWMutex
	timestamps map[string]time.Time
	stage      PipelineStage
}

// NewTimestampTracker creates a new timestamp tracker.
func NewTimestampTracker(stage PipelineStage) *TimestampTracker {
	return &TimestampTracker{
		timestamps: make(map[string]time.Time),
		stage:      stage,
	}
}

// Record records a timestamp for the given event.
func (t *TimestampTracker) Record(event string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.timestamps[event] = time.Now().UTC()
}

// Get returns the timestamp for an event.
func (t *TimestampTracker) Get(event string) (time.Time, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	ts, ok := t.timestamps[event]
	return ts, ok
}

// Duration returns the duration between two events.
func (t *TimestampTracker) Duration(from, to string) (time.Duration, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	fromTime, fromOk := t.timestamps[from]
	toTime, toOk := t.timestamps[to]

	if !fromOk || !toOk {
		return 0, false
	}

	return toTime.Sub(fromTime), true
}

// LatencyMs returns the latency in milliseconds between two events.
func (t *TimestampTracker) LatencyMs(from, to string) (int64, bool) {
	duration, ok := t.Duration(from, to)
	if !ok {
		return 0, false
	}
	return duration.Milliseconds(), true
}

// AllTimestamps returns all recorded timestamps.
func (t *TimestampTracker) AllTimestamps() map[string]time.Time {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]time.Time, len(t.timestamps))
	for k, v := range t.timestamps {
		result[k] = v
	}
	return result
}

// PipelineTimestamps contains all timestamps defined in requirements Section 13.
type PipelineTimestamps struct {
	VADEnd                *time.Time
	ASRFinal              *time.Time
	LLMDispatch           *time.Time
	LLMFirstToken         *time.Time
	FirstSpeakableSegment *time.Time
	TTSDispatch           *time.Time
	TTSFirstChunk         *time.Time
	FirstAudioSent        *time.Time
	InterruptionDetected  *time.Time
	LLMCancelAck          *time.Time
	TTSCancelAck          *time.Time
}

// SessionTimestampTracker tracks timestamps for a session.
type SessionTimestampTracker struct {
	mu         sync.RWMutex
	sessionID  string
	timestamps PipelineTimestamps
}

// NewSessionTimestampTracker creates a new session timestamp tracker.
func NewSessionTimestampTracker(sessionID string) *SessionTimestampTracker {
	return &SessionTimestampTracker{
		sessionID: sessionID,
	}
}

// RecordVADEnd records the VAD end timestamp.
func (t *SessionTimestampTracker) RecordVADEnd() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.VADEnd = &now
}

// RecordASRFinal records the ASR final timestamp.
func (t *SessionTimestampTracker) RecordASRFinal() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.ASRFinal = &now
}

// RecordLLMDispatch records the LLM dispatch timestamp.
func (t *SessionTimestampTracker) RecordLLMDispatch() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.LLMDispatch = &now
}

// RecordLLMFirstToken records the LLM first token timestamp.
func (t *SessionTimestampTracker) RecordLLMFirstToken() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.LLMFirstToken = &now
}

// RecordFirstSpeakableSegment records the first speakable segment timestamp.
func (t *SessionTimestampTracker) RecordFirstSpeakableSegment() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.FirstSpeakableSegment = &now
}

// RecordTTSDispatch records the TTS dispatch timestamp.
func (t *SessionTimestampTracker) RecordTTSDispatch() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.TTSDispatch = &now
}

// RecordTTSFirstChunk records the TTS first chunk timestamp.
func (t *SessionTimestampTracker) RecordTTSFirstChunk() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.TTSFirstChunk = &now
}

// RecordFirstAudioSent records the first audio sent timestamp.
func (t *SessionTimestampTracker) RecordFirstAudioSent() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.FirstAudioSent = &now
}

// RecordInterruptionDetected records the interruption detected timestamp.
func (t *SessionTimestampTracker) RecordInterruptionDetected() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.InterruptionDetected = &now
}

// RecordLLMCancelAck records the LLM cancel acknowledgment timestamp.
func (t *SessionTimestampTracker) RecordLLMCancelAck() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.LLMCancelAck = &now
}

// RecordTTSCancelAck records the TTS cancel acknowledgment timestamp.
func (t *SessionTimestampTracker) RecordTTSCancelAck() {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now().UTC()
	t.timestamps.TTSCancelAck = &now
}

// GetTimestamps returns all recorded timestamps.
func (t *SessionTimestampTracker) GetTimestamps() PipelineTimestamps {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.timestamps
}

// CalculateLatency calculates the latency between two timestamps in milliseconds.
func (t *SessionTimestampTracker) CalculateLatency(from, to *time.Time) int64 {
	if from == nil || to == nil {
		return -1
	}
	return to.Sub(*from).Milliseconds()
}

// SpanHelper provides helper functions for creating spans.
type SpanHelper struct {
	tracer *Tracer
}

// NewSpanHelper creates a new span helper.
func NewSpanHelper(tracer *Tracer) *SpanHelper {
	return &SpanHelper{tracer: tracer}
}

// StartPipelineSpan starts a span for a pipeline stage.
func (h *SpanHelper) StartPipelineSpan(ctx context.Context, stage PipelineStage, sessionID string) (context.Context, trace.Span) {
	attrs := map[string]string{
		"pipeline_stage": string(stage),
		"session_id":     sessionID,
	}
	return h.tracer.StartSpanWithAttributes(ctx, string(stage), attrs)
}

// StartProviderSpan starts a span for a provider call.
func (h *SpanHelper) StartProviderSpan(ctx context.Context, provider, providerType, sessionID string) (context.Context, trace.Span) {
	attrs := map[string]string{
		"provider":      provider,
		"provider_type": providerType,
		"session_id":    sessionID,
	}
	return h.tracer.StartSpanWithAttributes(ctx, provider+"_"+providerType, attrs)
}

// InitMetrics initializes Prometheus metrics exporter.
func InitMetrics() (*metric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
	)

	return provider, nil
}
