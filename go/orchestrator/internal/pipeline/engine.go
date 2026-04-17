package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/parlona/cloudapp/orchestrator/internal/persistence"
	"github.com/parlona/cloudapp/orchestrator/internal/statemachine"
	"github.com/parlona/cloudapp/pkg/events"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/providers"
	"github.com/parlona/cloudapp/pkg/session"
)

// Engine is the core orchestration engine that manages the ASR->LLM->TTS pipeline.
type Engine struct {
	providerRegistry *providers.ProviderRegistry
	sessionStore     session.SessionStore
	redisPersistence *persistence.RedisPersistence
	postgres         *persistence.PostgresPersistence
	config           *Config
	logger           *observability.Logger
	metrics          *observability.MetricsCollector

	// Pipeline stages
	asrStage *ASRStage
	llmStage *LLMStage
	ttsStage *TTSStage

	// State management
	fsmManager      *statemachine.FSMManager
	turnManagerReg  *statemachine.TurnManagerRegistry
	promptAssembler *PromptAssembler

	// Active sessions
	activeSessions sync.Map // map[string]*SessionContext
}

// Config contains engine configuration.
type Config struct {
	MaxSessionDuration   time.Duration
	EnableInterruptions  bool
	MaxContextMessages   int
	CircuitBreakerConfig CircuitBreakerConfig
}

// DefaultConfig returns default engine configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxSessionDuration:   time.Hour,
		EnableInterruptions:  true,
		MaxContextMessages:   20,
		CircuitBreakerConfig: DefaultCircuitBreakerConfig(),
	}
}

// SessionContext holds runtime context for an active session.
type SessionContext struct {
	SessionID      string
	Session        *session.Session
	History        *session.ConversationHistory
	TurnManager    *statemachine.TurnManager
	FSM            *statemachine.SessionFSM
	CancelFunc     context.CancelFunc
	TimestampTrack *observability.SessionTimestampTracker
}

// NewEngine creates a new orchestration engine.
func NewEngine(
	providerRegistry *providers.ProviderRegistry,
	sessionStore session.SessionStore,
	redisPersistence *persistence.RedisPersistence,
	postgres *persistence.PostgresPersistence,
	config *Config,
	logger *observability.Logger,
) *Engine {
	if config == nil {
		config = DefaultConfig()
	}

	// Create circuit breaker registry
	cbRegistry := NewCircuitBreakerRegistry(config.CircuitBreakerConfig)

	// Get default providers (these would be configured)
	asrProvider, _ := providerRegistry.GetASR("default")
	llmProvider, _ := providerRegistry.GetLLM("default")
	ttsProvider, _ := providerRegistry.GetTTS("default")

	return &Engine{
		providerRegistry: providerRegistry,
		sessionStore:     sessionStore,
		redisPersistence: redisPersistence,
		postgres:         postgres,
		config:           config,
		logger:           logger.WithField("component", "engine"),
		metrics:          observability.NewMetricsCollector("orchestrator"),
		asrStage:         NewASRStage(asrProvider, cbRegistry, logger),
		llmStage:         NewLLMStage(llmProvider, cbRegistry, logger),
		ttsStage:         NewTTSStage(ttsProvider, cbRegistry, logger),
		fsmManager:       statemachine.NewFSMManager(),
		turnManagerReg:   statemachine.NewTurnManagerRegistry(),
		promptAssembler:  NewPromptAssembler(config.MaxContextMessages),
	}
}

// ProcessSession is the main loop that receives audio chunks and manages the ASR->LLM->TTS pipeline.
func (e *Engine) ProcessSession(
	ctx context.Context,
	sessionID string,
	audioStream <-chan []byte,
	eventSink chan<- interface{},
) error {
	logger := e.logger.WithField("session_id", sessionID)
	logger.Info("starting session processing")

	// Get or create session
	sess, err := e.sessionStore.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Create session context
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Create FSM
	fsm := e.fsmManager.GetOrCreate(sessionID, eventSink)

	// Create conversation history
	history := session.NewConversationHistory(100)

	// Create turn manager
	turnManager := e.turnManagerReg.GetOrCreate(sessionID, history)

	// Create timestamp tracker
	timestampTrack := observability.NewSessionTimestampTracker(sessionID)

	// Store session context
	ctxData := &SessionContext{
		SessionID:      sessionID,
		Session:        sess,
		History:        history,
		TurnManager:    turnManager,
		FSM:            fsm,
		CancelFunc:     cancel,
		TimestampTrack: timestampTrack,
	}
	e.activeSessions.Store(sessionID, ctxData)
	defer e.activeSessions.Delete(sessionID)

	// Set up FSM handlers
	e.setupFSMHandlers(fsm, ctxData, eventSink)

	// Start ASR processing
	asrOpts := providers.NewASROptions(sessionID)
	asrResultCh, err := e.asrStage.ProcessAudioStream(sessionCtx, sessionID, audioStream, asrOpts)
	if err != nil {
		return fmt.Errorf("failed to start ASR processing: %w", err)
	}

	// Process ASR results
	for {
		select {
		case <-sessionCtx.Done():
			logger.Info("session processing cancelled")
			return nil

		case result, ok := <-asrResultCh:
			if !ok {
				logger.Info("ASR result channel closed")
				return nil
			}

			if result.Error != nil {
				logger.WithError(result.Error).Error("ASR error")
				continue
			}

			if result.IsPartial {
				// Emit partial transcript event
				event := events.NewASRPartialEvent(sessionID, result.Transcript)
				select {
				case eventSink <- event:
				default:
				}
			}

			if result.IsFinal {
				// Record timestamp
				timestampTrack.RecordASRFinal()

				// Emit final transcript event
				event := events.NewASRFinalEvent(sessionID, result.Transcript)
				select {
				case eventSink <- event:
				default:
				}

				// Process the utterance
				if err := e.ProcessUserUtterance(sessionCtx, sessionID, result.Transcript, eventSink); err != nil {
					logger.WithError(err).Error("failed to process user utterance")
				}
			}
		}
	}
}

// ProcessUserUtterance processes a final ASR transcript through LLM and TTS.
func (e *Engine) ProcessUserUtterance(
	ctx context.Context,
	sessionID string,
	transcript string,
	eventSink chan<- interface{},
) error {
	logger := e.logger.WithFields(map[string]interface{}{
		"session_id": sessionID,
		"transcript": transcript,
	})
	logger.Info("processing user utterance")

	// Get session context
	ctxData, ok := e.activeSessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session context not found")
	}
	sessionCtx := ctxData.(*SessionContext)

	// Transition FSM to processing
	if err := sessionCtx.FSM.Transition(statemachine.ASRFinalEvent); err != nil {
		logger.WithError(err).Warn("FSM transition failed")
	}

	// Add user message to history
	sessionCtx.History.AppendUserMessage(transcript)

	// Assemble prompt
	messages := e.promptAssembler.AssemblePromptWithHistory(
		sessionCtx.Session.SystemPrompt,
		sessionCtx.History.GetMessages(),
		transcript,
	)

	// Start LLM generation
	llmOpts := providers.NewLLMOptions(sessionID)
	if sessionCtx.Session.ModelOptions.ModelName != "" {
		llmOpts.ModelName = sessionCtx.Session.ModelOptions.ModelName
	}
	llmOpts.Temperature = sessionCtx.Session.ModelOptions.Temperature
	llmOpts.MaxTokens = sessionCtx.Session.ModelOptions.MaxTokens

	tokenCh, err := e.llmStage.Generate(ctx, sessionID, messages, llmOpts)
	if err != nil {
		return fmt.Errorf("failed to start LLM generation: %w", err)
	}

	// Start turn
	generationID := generateID()
	turn := sessionCtx.TurnManager.StartTurn(generationID, 16000)
	_ = turn

	// Transition FSM when first token arrives
	firstToken := true

	// Create token channel for incremental TTS
	ttsTokenCh := make(chan string, 10)

	// Start incremental TTS
	ttsOpts := providers.NewTTSOptions(sessionID)
	if sessionCtx.Session.VoiceProfile.VoiceID != "" {
		ttsOpts.VoiceID = sessionCtx.Session.VoiceProfile.VoiceID
	}
	ttsOpts.Speed = sessionCtx.Session.VoiceProfile.Speed

	ttsAudioCh, err := e.ttsStage.SynthesizeIncremental(ctx, sessionID, ttsTokenCh, ttsOpts)
	if err != nil {
		return fmt.Errorf("failed to start TTS synthesis: %w", err)
	}

	// Process LLM tokens and TTS audio concurrently
	var wg sync.WaitGroup

	// LLM token processor
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ttsTokenCh)

		var accumulatedText string
		for token := range tokenCh {
			if token.Error != nil {
				logger.WithError(token.Error).Error("LLM token error")
				return
			}

			if firstToken {
				firstToken = false
				sessionCtx.TimestampTrack.RecordLLMFirstToken()
				if err := sessionCtx.FSM.Transition(statemachine.FirstTTSAudioEvent); err != nil {
					logger.WithError(err).Warn("FSM transition failed")
				}
			}

			accumulatedText += token.Token
			sessionCtx.TurnManager.AppendGenerated(token.Token)

			// Emit LLM partial text event
			event := events.NewLLMPartialTextEvent(sessionID, token.Token)
			select {
			case eventSink <- event:
			default:
			}

			// Send to TTS
			select {
			case <-ctx.Done():
				return
			case ttsTokenCh <- token.Token:
			}

			if token.IsFinal {
				// Commit turn to history
				sessionCtx.TurnManager.CommitTurn()
				break
			}
		}

		logger.WithField("generated_text", accumulatedText).Debug("LLM generation complete")
	}()

	// TTS audio processor
	wg.Add(1)
	go func() {
		defer wg.Done()

		segmentIndex := int32(0)
		firstChunk := true

		for audioChunk := range ttsAudioCh {
			if firstChunk {
				firstChunk = false
				sessionCtx.TimestampTrack.RecordTTSFirstChunk()
				if asrFinal := sessionCtx.TimestampTrack.GetTimestamps().ASRFinal; asrFinal != nil {
					observability.RecordServerTTFA(time.Since(*asrFinal))
				}
			}

			// Emit TTS audio chunk event
			event := events.NewTTSAudioChunkEvent(sessionID, audioChunk, segmentIndex, false)
			select {
			case eventSink <- event:
			default:
			}

			segmentIndex++
		}

		// Emit final audio chunk
		event := events.NewTTSAudioChunkEvent(sessionID, []byte{}, segmentIndex, true)
		select {
		case eventSink <- event:
		default:
		}
	}()

	wg.Wait()

	// Transition FSM to idle
	if err := sessionCtx.FSM.Transition(statemachine.BotFinishedEvent); err != nil {
		logger.WithError(err).Warn("FSM transition failed")
	}

	return nil
}

// HandleInterruption handles an interruption (barge-in) for a session.
func (e *Engine) HandleInterruption(
	ctx context.Context,
	sessionID string,
) error {
	logger := e.logger.WithField("session_id", sessionID)
	logger.Info("handling interruption")

	// Get session context
	ctxData, ok := e.activeSessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("session context not found")
	}
	sessionCtx := ctxData.(*SessionContext)

	// Record timestamp
	sessionCtx.TimestampTrack.RecordInterruptionDetected()

	// Transition FSM to interrupted
	if err := sessionCtx.FSM.Transition(statemachine.InterruptionEvent); err != nil {
		logger.WithError(err).Warn("FSM transition failed")
	}

	// Cancel active LLM generation
	if generationID := sessionCtx.TurnManager.GetGenerationID(); generationID != "" {
		if err := e.llmStage.Cancel(ctx, sessionID, generationID); err != nil {
			logger.WithError(err).Warn("failed to cancel LLM generation")
		}
		sessionCtx.TimestampTrack.RecordLLMCancelAck()
	}

	// Cancel active TTS synthesis
	if err := e.ttsStage.Cancel(ctx, sessionID); err != nil {
		logger.WithError(err).Warn("failed to cancel TTS synthesis")
	}
	sessionCtx.TimestampTrack.RecordTTSCancelAck()

	// Get current playout position and handle interruption in turn manager
	playoutPosition := sessionCtx.TurnManager.GetCurrentPosition()
	sessionCtx.TurnManager.HandleInterruption(playoutPosition)

	// Commit only spoken text to history
	committedMsg := sessionCtx.TurnManager.CommitTurn()
	if committedMsg.Content != "" {
		sessionCtx.History.AppendAssistantMessage(committedMsg.Content)
		logger.WithField("committed_text", committedMsg.Content).Debug("committed spoken text to history")
	}

	// Record interruption stop latency
	if interruptionTime := sessionCtx.TimestampTrack.GetTimestamps().InterruptionDetected; interruptionTime != nil {
		observability.RecordInterruptionStop(time.Since(*interruptionTime))
	}

	// Transition FSM back to listening
	if err := sessionCtx.FSM.Transition(statemachine.SpeechStartEvent); err != nil {
		logger.WithError(err).Warn("FSM transition failed")
	}

	return nil
}

// StopSession stops a session and cleans up resources.
func (e *Engine) StopSession(ctx context.Context, sessionID string) error {
	logger := e.logger.WithField("session_id", sessionID)
	logger.Info("stopping session")

	// Get session context
	ctxData, ok := e.activeSessions.Load(sessionID)
	if ok {
		sessionCtx := ctxData.(*SessionContext)

		// Cancel context
		if sessionCtx.CancelFunc != nil {
			sessionCtx.CancelFunc()
		}

		// Transition FSM to idle
		sessionCtx.FSM.Transition(statemachine.SessionStopEvent)
	}

	// Clean up
	e.activeSessions.Delete(sessionID)
	e.fsmManager.Remove(sessionID)
	e.turnManagerReg.Remove(sessionID)

	// Clean up Redis
	if e.redisPersistence != nil {
		if err := e.redisPersistence.DeleteSession(ctx, sessionID); err != nil {
			logger.WithError(err).Warn("failed to delete session from Redis")
		}
	}

	return nil
}

// setupFSMHandlers sets up FSM state change handlers.
func (e *Engine) setupFSMHandlers(
	fsm *statemachine.SessionFSM,
	ctxData *SessionContext,
	eventSink chan<- interface{},
) {
	fsm.SetOnTransition(func(change statemachine.StateChange) {
		e.logger.WithFields(map[string]interface{}{
			"from":  change.From,
			"to":    change.To,
			"event": change.Event,
		}).Debug("FSM state transition")

		// Emit turn event
		turnEvent := events.NewTurnEvent(ctxData.SessionID, "assistant", change.To.String())
		turnEvent.Event = fmt.Sprintf("%v", change.Event)
		select {
		case eventSink <- turnEvent:
		default:
		}
	})
}

// GetActiveSessions returns a list of active session IDs.
func (e *Engine) GetActiveSessions() []string {
	var sessions []string
	e.activeSessions.Range(func(key, value interface{}) bool {
		sessions = append(sessions, key.(string))
		return true
	})
	return sessions
}

// IsSessionActive returns true if the session is active.
func (e *Engine) IsSessionActive(sessionID string) bool {
	_, ok := e.activeSessions.Load(sessionID)
	return ok
}
