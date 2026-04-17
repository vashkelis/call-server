package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/providers"
)

// ASRResult represents a result from ASR streaming recognition.
type ASRResult struct {
	Transcript string
	IsPartial  bool
	IsFinal    bool
	Confidence float32
	Language   string
	Timing     *observability.TimestampTracker
	Error      error
}

// ASRStage wraps an ASR provider with circuit breaker and metrics.
type ASRStage struct {
	provider       providers.ASRProvider
	circuitBreaker *CircuitBreaker
	logger         *observability.Logger
	metrics        *observability.MetricsCollector
}

// NewASRStage creates a new ASR pipeline stage.
func NewASRStage(
	provider providers.ASRProvider,
	cbRegistry *CircuitBreakerRegistry,
	logger *observability.Logger,
) *ASRStage {
	return &ASRStage{
		provider:       provider,
		circuitBreaker: cbRegistry.Get(provider.Name()),
		logger:         logger.WithField("stage", "asr"),
		metrics:        observability.NewMetricsCollector(provider.Name()),
	}
}

// ProcessAudio streams audio to the ASR provider and returns results.
// It emits partial transcripts as they arrive and returns final transcript when complete.
// Supports cancellation via context.
func (s *ASRStage) ProcessAudio(
	ctx context.Context,
	sessionID string,
	audio []byte,
	opts providers.ASROptions,
) (<-chan ASRResult, error) {
	logger := s.logger.WithField("session_id", sessionID)
	logger.Debug("starting ASR processing")

	// Create result channel
	resultCh := make(chan ASRResult, 10)

	// Create audio stream channel
	audioStream := make(chan []byte, 10)

	// Track timing
	timing := observability.NewTimestampTracker(observability.StageASR)

	// Start ASR recognition with circuit breaker protection
	var providerResultCh chan providers.ASRResult
	var providerErr error

	err := s.circuitBreaker.Execute(ctx, func() error {
		s.metrics.RecordASRRequest()
		start := time.Now()

		providerResultCh, providerErr = s.provider.StreamRecognize(ctx, audioStream, opts)
		if providerErr != nil {
			s.metrics.RecordASRError()
			return fmt.Errorf("ASR stream recognize failed: %w", providerErr)
		}

		s.metrics.RecordASRDuration(time.Since(start))
		return nil
	})

	if err != nil {
		close(resultCh)
		if err == ErrCircuitOpen {
			logger.Error("ASR circuit breaker is open")
		}
		return nil, fmt.Errorf("ASR processing failed: %w", err)
	}

	// Send audio data
	go func() {
		defer close(audioStream)
		select {
		case <-ctx.Done():
			return
		case audioStream <- audio:
		}
	}()

	// Process results
	go func() {
		defer close(resultCh)

		for {
			select {
			case <-ctx.Done():
				logger.Debug("ASR processing cancelled")
				return

			case result, ok := <-providerResultCh:
				if !ok {
					logger.Debug("ASR result channel closed")
					return
				}

				if result.Error != nil {
					logger.WithError(result.Error).Error("ASR provider error")
					select {
					case <-ctx.Done():
						return
					case resultCh <- ASRResult{Error: result.Error, Timing: timing}:
					}
					continue
				}

				// Convert provider result to stage result
				stageResult := ASRResult{
					Transcript: result.Transcript,
					IsPartial:  result.IsPartial,
					IsFinal:    result.IsFinal,
					Confidence: result.Confidence,
					Language:   result.Language,
					Timing:     timing,
				}

				if result.IsFinal {
					timing.Record("asr_final")
					if startTime, ok := timing.Get("asr_start"); ok {
						observability.RecordASRLatency(time.Since(startTime))
					}
				}

				select {
				case <-ctx.Done():
					return
				case resultCh <- stageResult:
				}

				if result.IsFinal {
					return
				}
			}
		}
	}()

	timing.Record("asr_start")
	return resultCh, nil
}

// ProcessAudioStream processes a continuous audio stream for ASR recognition.
// This is used for real-time streaming recognition where audio chunks arrive over time.
func (s *ASRStage) ProcessAudioStream(
	ctx context.Context,
	sessionID string,
	audioStream <-chan []byte,
	opts providers.ASROptions,
) (<-chan ASRResult, error) {
	logger := s.logger.WithField("session_id", sessionID)
	logger.Debug("starting ASR stream processing")

	// Create result channel
	resultCh := make(chan ASRResult, 10)

	// Create internal audio stream channel
	internalAudioStream := make(chan []byte, 10)

	// Track timing
	timing := observability.NewTimestampTracker(observability.StageASR)

	// Start ASR recognition with circuit breaker protection
	var providerResultCh chan providers.ASRResult
	var providerErr error

	err := s.circuitBreaker.Execute(ctx, func() error {
		s.metrics.RecordASRRequest()
		start := time.Now()

		providerResultCh, providerErr = s.provider.StreamRecognize(ctx, internalAudioStream, opts)
		if providerErr != nil {
			s.metrics.RecordASRError()
			return fmt.Errorf("ASR stream recognize failed: %w", providerErr)
		}

		s.metrics.RecordASRDuration(time.Since(start))
		return nil
	})

	if err != nil {
		close(resultCh)
		if err == ErrCircuitOpen {
			logger.Error("ASR circuit breaker is open")
		}
		return nil, fmt.Errorf("ASR processing failed: %w", err)
	}

	// Forward audio from input stream to internal stream
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(internalAudioStream)

		for {
			select {
			case <-ctx.Done():
				return
			case audio, ok := <-audioStream:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case internalAudioStream <- audio:
				}
			}
		}
	}()

	// Process results
	go func() {
		defer close(resultCh)
		defer wg.Wait()

		for {
			select {
			case <-ctx.Done():
				logger.Debug("ASR stream processing cancelled")
				return

			case result, ok := <-providerResultCh:
				if !ok {
					logger.Debug("ASR result channel closed")
					return
				}

				if result.Error != nil {
					logger.WithError(result.Error).Error("ASR provider error")
					select {
					case <-ctx.Done():
						return
					case resultCh <- ASRResult{Error: result.Error, Timing: timing}:
					}
					continue
				}

				// Convert provider result to stage result
				stageResult := ASRResult{
					Transcript: result.Transcript,
					IsPartial:  result.IsPartial,
					IsFinal:    result.IsFinal,
					Confidence: result.Confidence,
					Language:   result.Language,
					Timing:     timing,
				}

				if result.IsFinal {
					timing.Record("asr_final")
					if startTime, ok := timing.Get("asr_start"); ok {
						observability.RecordASRLatency(time.Since(startTime))
					}
				}

				select {
				case <-ctx.Done():
					return
				case resultCh <- stageResult:
				}
			}
		}
	}()

	timing.Record("asr_start")
	return resultCh, nil
}

// Cancel cancels an ongoing ASR recognition for the given session.
func (s *ASRStage) Cancel(ctx context.Context, sessionID string) error {
	logger := s.logger.WithField("session_id", sessionID)
	logger.Debug("cancelling ASR")

	if err := s.provider.Cancel(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to cancel ASR: %w", err)
	}

	return nil
}

// Name returns the provider name.
func (s *ASRStage) Name() string {
	return s.provider.Name()
}

// Capabilities returns the provider capabilities.
func (s *ASRStage) Capabilities() contracts.ProviderCapability {
	return s.provider.Capabilities()
}
