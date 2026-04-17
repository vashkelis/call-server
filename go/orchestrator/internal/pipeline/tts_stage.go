package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/providers"
)

// TTSStage wraps a TTS provider with circuit breaker and metrics.
type TTSStage struct {
	provider       providers.TTSProvider
	circuitBreaker *CircuitBreaker
	logger         *observability.Logger
	metrics        *observability.MetricsCollector

	// Track active syntheses for cancellation
	activeSyntheses sync.Map // map[string]context.CancelFunc
}

// NewTTSStage creates a new TTS pipeline stage.
func NewTTSStage(
	provider providers.TTSProvider,
	cbRegistry *CircuitBreakerRegistry,
	logger *observability.Logger,
) *TTSStage {
	return &TTSStage{
		provider:       provider,
		circuitBreaker: cbRegistry.Get(provider.Name()),
		logger:         logger.WithField("stage", "tts"),
		metrics:        observability.NewMetricsCollector(provider.Name()),
	}
}

// Synthesize sends text to the TTS provider and streams audio chunks back.
// Records tts_dispatch, tts_first_chunk timestamps.
func (s *TTSStage) Synthesize(
	ctx context.Context,
	sessionID string,
	text string,
	opts providers.TTSOptions,
) (<-chan []byte, error) {
	logger := s.logger.WithField("session_id", sessionID)
	logger.WithField("text_length", len(text)).Debug("starting TTS synthesis")

	// Create result channel
	audioCh := make(chan []byte, 10)

	// Track timing
	timing := observability.NewTimestampTracker(observability.StageTTS)
	timing.Record("tts_dispatch")

	// Create cancellable context for this synthesis
	synthCtx, cancel := context.WithCancel(ctx)
	s.activeSyntheses.Store(sessionID, cancel)

	// Start TTS synthesis with circuit breaker protection
	var providerAudioCh chan []byte
	var providerErr error

	err := s.circuitBreaker.Execute(synthCtx, func() error {
		s.metrics.RecordTTSRequest()
		start := time.Now()

		providerAudioCh, providerErr = s.provider.StreamSynthesize(synthCtx, text, opts)
		if providerErr != nil {
			s.metrics.RecordTTSError()
			return fmt.Errorf("TTS stream synthesize failed: %w", providerErr)
		}

		s.metrics.RecordTTSDuration(time.Since(start))
		return nil
	})

	if err != nil {
		close(audioCh)
		s.activeSyntheses.Delete(sessionID)
		if err == ErrCircuitOpen {
			logger.Error("TTS circuit breaker is open")
		}
		return nil, fmt.Errorf("TTS synthesis failed: %w", err)
	}

	// Process audio chunks
	firstChunk := true
	go func() {
		defer close(audioCh)
		defer s.activeSyntheses.Delete(sessionID)

		for {
			select {
			case <-synthCtx.Done():
				logger.Debug("TTS synthesis cancelled")
				return

			case audioChunk, ok := <-providerAudioCh:
				if !ok {
					logger.Debug("TTS audio channel closed")
					return
				}

				// Record first chunk timing
				if firstChunk {
					firstChunk = false
					timing.Record("tts_first_chunk")
					if dispatchTime, ok := timing.Get("tts_dispatch"); ok {
						observability.RecordTTSFirstChunk(time.Since(dispatchTime))
					}
				}

				select {
				case <-synthCtx.Done():
					return
				case audioCh <- audioChunk:
				}
			}
		}
	}()

	return audioCh, nil
}

// SynthesizeIncremental batches incoming tokens into speakable segments (sentence boundaries)
// and dispatches each segment to TTS. Enables overlapping LLM generation and TTS synthesis.
func (s *TTSStage) SynthesizeIncremental(
	ctx context.Context,
	sessionID string,
	tokenChan <-chan string,
	opts providers.TTSOptions,
) (<-chan []byte, error) {
	logger := s.logger.WithField("session_id", sessionID)
	logger.Debug("starting incremental TTS synthesis")

	// Create result channel
	audioCh := make(chan []byte, 10)

	// Track timing
	timing := observability.NewTimestampTracker(observability.StageTTS)

	// Buffer for accumulating tokens
	var buffer strings.Builder
	var segmentIndex int32
	var firstSegment bool = true

	// Create cancellable context
	synthCtx, cancel := context.WithCancel(ctx)
	s.activeSyntheses.Store(sessionID, cancel)

	// Process tokens and synthesize segments
	go func() {
		defer close(audioCh)
		defer s.activeSyntheses.Delete(sessionID)

		// Segment synthesizer
		synthesizeSegment := func(text string, isFinal bool) error {
			if text == "" {
				return nil
			}

			segmentOpts := opts
			segmentOpts.SegmentIndex = segmentIndex
			if isFinal {
				segmentOpts = segmentOpts.WithProviderOption("is_final", "true")
			}

			// Record dispatch time for first segment
			if firstSegment {
				firstSegment = false
				timing.Record("tts_dispatch")
			}

			// Synthesize this segment
			segmentAudioCh, err := s.Synthesize(synthCtx, sessionID, text, segmentOpts)
			if err != nil {
				return fmt.Errorf("failed to synthesize segment %d: %w", segmentIndex, err)
			}

			segmentIndex++

			// Forward audio chunks
			for audioChunk := range segmentAudioCh {
				select {
				case <-synthCtx.Done():
					return synthCtx.Err()
				case audioCh <- audioChunk:
				}
			}

			return nil
		}

		for {
			select {
			case <-synthCtx.Done():
				logger.Debug("incremental TTS cancelled")
				return

			case token, ok := <-tokenChan:
				if !ok {
					// Token channel closed, synthesize remaining buffer
					if buffer.Len() > 0 {
						if err := synthesizeSegment(buffer.String(), true); err != nil {
							logger.WithError(err).Error("failed to synthesize final segment")
						}
					}
					return
				}

				buffer.WriteString(token)

				// Check if we have a complete speakable segment
				if text := buffer.String(); isSpeakableSegment(text) {
					segment := extractSpeakableSegment(text)
					if segment != "" {
						if err := synthesizeSegment(segment, false); err != nil {
							logger.WithError(err).Error("failed to synthesize segment")
							continue
						}
						// Remove synthesized portion from buffer
						remaining := text[len(segment):]
						buffer.Reset()
						buffer.WriteString(remaining)
					}
				}
			}
		}
	}()

	return audioCh, nil
}

// Cancel cancels an ongoing synthesis for the given session.
func (s *TTSStage) Cancel(ctx context.Context, sessionID string) error {
	logger := s.logger.WithField("session_id", sessionID)
	logger.Debug("cancelling TTS synthesis")

	// Cancel the local context
	if cancelFunc, ok := s.activeSyntheses.Load(sessionID); ok {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			cancel()
		}
		s.activeSyntheses.Delete(sessionID)
	}

	// Also notify the provider
	if err := s.provider.Cancel(ctx, sessionID); err != nil {
		logger.WithError(err).Warn("failed to cancel TTS at provider")
		// Don't return error - we've already cancelled locally
	}

	return nil
}

// Name returns the provider name.
func (s *TTSStage) Name() string {
	return s.provider.Name()
}

// Capabilities returns the provider capabilities.
func (s *TTSStage) Capabilities() contracts.ProviderCapability {
	return s.provider.Capabilities()
}

// isSpeakableSegment checks if the text contains a complete speakable segment.
// A speakable segment is typically a sentence or phrase that can be synthesized.
func isSpeakableSegment(text string) bool {
	// Check for sentence-ending punctuation
	if idx := strings.LastIndexAny(text, ".!?"); idx != -1 {
		// Make sure there's enough content before the punctuation
		return idx > 10 // At least 10 characters before punctuation
	}

	// Check for phrase boundaries (comma, semicolon) with enough content
	if idx := strings.LastIndexAny(text, ",;:"); idx != -1 {
		return idx > 20 // At least 20 characters before boundary
	}

	// Check if we have a long enough chunk
	return len(text) > 50
}

// extractSpeakableSegment extracts the speakable portion of text up to a sentence boundary.
func extractSpeakableSegment(text string) string {
	// Find the last sentence boundary
	if idx := strings.LastIndexAny(text, ".!?"); idx != -1 {
		// Include the punctuation
		return strings.TrimSpace(text[:idx+1])
	}

	// Find phrase boundary
	if idx := strings.LastIndexAny(text, ",;:"); idx != -1 {
		return strings.TrimSpace(text[:idx+1])
	}

	// Return the whole text if it's long enough
	if len(text) > 50 {
		// Try to break at a word boundary
		for i := len(text) - 1; i >= 0; i-- {
			if unicode.IsSpace(rune(text[i])) {
				return strings.TrimSpace(text[:i])
			}
		}
	}

	return ""
}
