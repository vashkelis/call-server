package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// sessions_active - Gauge of currently active sessions
	sessionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloudapp_sessions_active",
		Help: "Number of currently active sessions",
	})

	// turns_total - Counter of total turns processed
	turnsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "cloudapp_turns_total",
		Help: "Total number of turns processed",
	})

	// asr_latency_ms - Histogram of ASR latency
	asrLatencyMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cloudapp_asr_latency_ms",
		Help:    "ASR latency in milliseconds",
		Buckets: prometheus.ExponentialBuckets(10, 2, 10), // 10ms to ~10s
	})

	// llm_ttft_ms - Histogram of LLM time to first token
	llmTTFTMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cloudapp_llm_ttft_ms",
		Help:    "LLM time to first token in milliseconds",
		Buckets: prometheus.ExponentialBuckets(50, 2, 10), // 50ms to ~50s
	})

	// tts_first_chunk_ms - Histogram of TTS time to first audio chunk
	ttsFirstChunkMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cloudapp_tts_first_chunk_ms",
		Help:    "TTS time to first audio chunk in milliseconds",
		Buckets: prometheus.ExponentialBuckets(20, 2, 10), // 20ms to ~20s
	})

	// server_ttfa_ms - Histogram of server time to first audio (end-to-end)
	serverTTFAMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cloudapp_server_ttfa_ms",
		Help:    "Server time to first audio in milliseconds (end-to-end)",
		Buckets: prometheus.ExponentialBuckets(100, 2, 10), // 100ms to ~100s
	})

	// interruption_stop_ms - Histogram of interruption stop latency
	interruptionStopMs = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "cloudapp_interruption_stop_ms",
		Help:    "Time to stop after interruption in milliseconds",
		Buckets: prometheus.ExponentialBuckets(5, 2, 10), // 5ms to ~5s
	})

	// provider_errors_total - Counter of provider errors by provider
	providerErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloudapp_provider_errors_total",
		Help: "Total number of provider errors",
	}, []string{"provider", "type"})

	// provider_requests_total - Counter of provider requests by provider and type
	providerRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cloudapp_provider_requests_total",
		Help: "Total number of provider requests",
	}, []string{"provider", "type"})

	// provider_request_duration_ms - Histogram of provider request duration
	providerRequestDurationMs = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cloudapp_provider_request_duration_ms",
		Help:    "Provider request duration in milliseconds",
		Buckets: prometheus.ExponentialBuckets(10, 2, 10),
	}, []string{"provider", "type"})

	// websocket_connections_active - Gauge of active WebSocket connections
	websocketConnectionsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cloudapp_websocket_connections_active",
		Help: "Number of active WebSocket connections",
	})
)

// RecordSessionActive increments the active sessions gauge.
func RecordSessionActive() {
	sessionsActive.Inc()
}

// RecordSessionInactive decrements the active sessions gauge.
func RecordSessionInactive() {
	sessionsActive.Dec()
}

// RecordTurn increments the total turns counter.
func RecordTurn() {
	turnsTotal.Inc()
}

// RecordASRLatency records ASR latency.
func RecordASRLatency(duration time.Duration) {
	asrLatencyMs.Observe(float64(duration.Milliseconds()))
}

// RecordLLMTTFT records LLM time to first token.
func RecordLLMTTFT(duration time.Duration) {
	llmTTFTMs.Observe(float64(duration.Milliseconds()))
}

// RecordTTSFirstChunk records TTS time to first audio chunk.
func RecordTTSFirstChunk(duration time.Duration) {
	ttsFirstChunkMs.Observe(float64(duration.Milliseconds()))
}

// RecordServerTTFA records server time to first audio.
func RecordServerTTFA(duration time.Duration) {
	serverTTFAMs.Observe(float64(duration.Milliseconds()))
}

// RecordInterruptionStop records interruption stop latency.
func RecordInterruptionStop(duration time.Duration) {
	interruptionStopMs.Observe(float64(duration.Milliseconds()))
}

// RecordProviderError records a provider error.
func RecordProviderError(provider, providerType string) {
	providerErrorsTotal.WithLabelValues(provider, providerType).Inc()
}

// RecordProviderRequest records a provider request.
func RecordProviderRequest(provider, providerType string) {
	providerRequestsTotal.WithLabelValues(provider, providerType).Inc()
}

// RecordProviderRequestDuration records provider request duration.
func RecordProviderRequestDuration(provider, providerType string, duration time.Duration) {
	providerRequestDurationMs.WithLabelValues(provider, providerType).Observe(float64(duration.Milliseconds()))
}

// RecordWebSocketConnectionActive increments the active WebSocket connections gauge.
func RecordWebSocketConnectionActive() {
	websocketConnectionsActive.Inc()
}

// RecordWebSocketConnectionInactive decrements the active WebSocket connections gauge.
func RecordWebSocketConnectionInactive() {
	websocketConnectionsActive.Dec()
}

// MetricsCollector provides a convenient interface for recording metrics.
type MetricsCollector struct {
	provider string
}

// NewMetricsCollector creates a new metrics collector for a provider.
func NewMetricsCollector(provider string) *MetricsCollector {
	return &MetricsCollector{provider: provider}
}

// RecordASRRequest records an ASR request.
func (m *MetricsCollector) RecordASRRequest() {
	RecordProviderRequest(m.provider, "asr")
}

// RecordASRError records an ASR error.
func (m *MetricsCollector) RecordASRError() {
	RecordProviderError(m.provider, "asr")
}

// RecordASRDuration records ASR request duration.
func (m *MetricsCollector) RecordASRDuration(duration time.Duration) {
	RecordProviderRequestDuration(m.provider, "asr", duration)
}

// RecordLLMRequest records an LLM request.
func (m *MetricsCollector) RecordLLMRequest() {
	RecordProviderRequest(m.provider, "llm")
}

// RecordLLMError records an LLM error.
func (m *MetricsCollector) RecordLLMError() {
	RecordProviderError(m.provider, "llm")
}

// RecordLLMDuration records LLM request duration.
func (m *MetricsCollector) RecordLLMDuration(duration time.Duration) {
	RecordProviderRequestDuration(m.provider, "llm", duration)
}

// RecordTTSRequest records a TTS request.
func (m *MetricsCollector) RecordTTSRequest() {
	RecordProviderRequest(m.provider, "tts")
}

// RecordTTSError records a TTS error.
func (m *MetricsCollector) RecordTTSError() {
	RecordProviderError(m.provider, "tts")
}

// RecordTTSDuration records TTS request duration.
func (m *MetricsCollector) RecordTTSDuration(duration time.Duration) {
	RecordProviderRequestDuration(m.provider, "tts", duration)
}

// GetRegistry returns the Prometheus registry.
func GetRegistry() *prometheus.Registry {
	return prometheus.NewRegistry()
}

// DefaultRegistry returns the default Prometheus registry with our metrics.
func DefaultRegistry() *prometheus.Registry {
	// The promauto package automatically registers with the default registry
	return prometheus.DefaultRegisterer.(*prometheus.Registry)
}
