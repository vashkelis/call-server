package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/providers"
)

// generateID generates a unique ID for generation tracking.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// LLMToken represents a token from LLM streaming generation.
type LLMToken struct {
	Token        string
	IsFinal      bool
	FinishReason string
	Usage        *contracts.UsageMetadata
	GenerationID string
	Error        error
}

// LLMStage wraps an LLM provider with circuit breaker and metrics.
type LLMStage struct {
	provider       providers.LLMProvider
	circuitBreaker *CircuitBreaker
	logger         *observability.Logger
	metrics        *observability.MetricsCollector

	// Track active generations for cancellation
	activeGens sync.Map // map[string]context.CancelFunc
}

// NewLLMStage creates a new LLM pipeline stage.
func NewLLMStage(
	provider providers.LLMProvider,
	cbRegistry *CircuitBreakerRegistry,
	logger *observability.Logger,
) *LLMStage {
	return &LLMStage{
		provider:       provider,
		circuitBreaker: cbRegistry.Get(provider.Name()),
		logger:         logger.WithField("stage", "llm"),
		metrics:        observability.NewMetricsCollector(provider.Name()),
	}
}

// Generate sends a prompt to the LLM provider and streams tokens back.
// Tracks generation_id for cancellation.
// Records llm_dispatch, llm_first_token timestamps.
func (s *LLMStage) Generate(
	ctx context.Context,
	sessionID string,
	messages []contracts.ChatMessage,
	opts providers.LLMOptions,
) (<-chan LLMToken, error) {
	logger := s.logger.WithField("session_id", sessionID)

	// Generate a unique generation ID
	generationID := generateID()
	logger = logger.WithField("generation_id", generationID)
	logger.Debug("starting LLM generation")

	// Create result channel
	tokenCh := make(chan LLMToken, 10)

	// Track timing
	timing := observability.NewTimestampTracker(observability.StageLLM)
	timing.Record("llm_dispatch")

	// Create cancellable context for this generation
	genCtx, cancel := context.WithCancel(ctx)

	// Store cancel function for later cancellation
	s.activeGens.Store(generationID, cancel)
	defer func() {
		// Clean up when done
		if genCtx.Err() != nil {
			s.activeGens.Delete(generationID)
		}
	}()

	// Start LLM generation with circuit breaker protection
	var providerTokenCh chan providers.LLMToken
	var providerErr error

	err := s.circuitBreaker.Execute(genCtx, func() error {
		s.metrics.RecordLLMRequest()
		start := time.Now()

		providerTokenCh, providerErr = s.provider.StreamGenerate(genCtx, messages, opts)
		if providerErr != nil {
			s.metrics.RecordLLMError()
			return fmt.Errorf("LLM stream generate failed: %w", providerErr)
		}

		s.metrics.RecordLLMDuration(time.Since(start))
		return nil
	})

	if err != nil {
		close(tokenCh)
		s.activeGens.Delete(generationID)
		if err == ErrCircuitOpen {
			logger.Error("LLM circuit breaker is open")
		}
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Process tokens
	firstToken := true
	go func() {
		defer close(tokenCh)
		defer s.activeGens.Delete(generationID)

		for {
			select {
			case <-genCtx.Done():
				if genCtx.Err() == context.Canceled {
					logger.Debug("LLM generation cancelled")
				} else {
					logger.WithError(genCtx.Err()).Debug("LLM generation context done")
				}
				return

			case token, ok := <-providerTokenCh:
				if !ok {
					logger.Debug("LLM token channel closed")
					return
				}

				if token.Error != nil {
					logger.WithError(token.Error).Error("LLM provider error")
					select {
					case <-genCtx.Done():
						return
					case tokenCh <- LLMToken{Error: token.Error, GenerationID: generationID}:
					}
					continue
				}

				// Record first token timing
				if firstToken {
					firstToken = false
					timing.Record("llm_first_token")
					if dispatchTime, ok := timing.Get("llm_dispatch"); ok {
						observability.RecordLLMTTFT(time.Since(dispatchTime))
					}
				}

				// Convert provider token to stage token
				stageToken := LLMToken{
					Token:        token.Token,
					IsFinal:      token.IsFinal,
					FinishReason: token.FinishReason,
					Usage:        token.Usage,
					GenerationID: generationID,
				}

				select {
				case <-genCtx.Done():
					return
				case tokenCh <- stageToken:
				}

				if token.IsFinal {
					logger.WithField("finish_reason", token.FinishReason).Debug("LLM generation complete")
					return
				}
			}
		}
	}()

	return tokenCh, nil
}

// Cancel cancels an ongoing generation for the given session and generation ID.
// Idempotent cancellation.
func (s *LLMStage) Cancel(ctx context.Context, sessionID string, generationID string) error {
	logger := s.logger.WithFields(map[string]interface{}{
		"session_id":    sessionID,
		"generation_id": generationID,
	})
	logger.Debug("cancelling LLM generation")

	// Cancel the local context
	if cancelFunc, ok := s.activeGens.Load(generationID); ok {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			cancel()
		}
		s.activeGens.Delete(generationID)
	}

	// Also notify the provider
	if err := s.provider.Cancel(ctx, sessionID); err != nil {
		logger.WithError(err).Warn("failed to cancel LLM at provider")
		// Don't return error - we've already cancelled locally
	}

	return nil
}

// Name returns the provider name.
func (s *LLMStage) Name() string {
	return s.provider.Name()
}

// Capabilities returns the provider capabilities.
func (s *LLMStage) Capabilities() contracts.ProviderCapability {
	return s.provider.Capabilities()
}

// GetActiveGeneration returns the generation ID if there's an active generation for the session.
// Note: This is a simplified version - in production you'd want session-scoped tracking.
func (s *LLMStage) GetActiveGeneration(sessionID string) (string, bool) {
	var foundID string
	var found bool

	s.activeGens.Range(func(key, value interface{}) bool {
		genID := key.(string)
		// In a real implementation, you'd check if this generation belongs to the session
		// For now, we just return the first active one
		foundID = genID
		found = true
		return false // stop iteration
	})

	return foundID, found
}
