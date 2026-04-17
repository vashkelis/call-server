package handler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/parlona/cloudapp/media-edge/internal/transport"
	"github.com/parlona/cloudapp/media-edge/internal/vad"
	"github.com/parlona/cloudapp/pkg/audio"
	"github.com/parlona/cloudapp/pkg/events"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/session"
)

// SessionHandler manages the audio pipeline for a single session.
type SessionHandler struct {
	sessionID      string
	session        *session.Session
	transport      transport.Transport
	bridge         OrchestratorBridge
	vadProcessor   *vad.VADProcessor
	normalizer     *audio.PCM16Normalizer
	chunker        *audio.Chunker
	playoutTracker *audio.PlayoutTracker
	logger         *observability.Logger

	// Audio pipeline
	inputBuffer  *audio.JitterBuffer
	outputBuffer *audio.JitterBuffer

	// State management
	mu          sync.RWMutex
	state       session.SessionState
	botSpeaking bool
	interrupted bool
	ctx         context.Context
	cancel      context.CancelFunc

	// Event channels
	eventCh        chan events.Event
	orchestratorCh <-chan events.Event

	// Audio accumulation for ASR
	speechBuffer   []byte
	isSpeechActive bool

	// Metrics
	metrics *observability.MetricsCollector
}

// SessionHandlerConfig contains configuration for the session handler.
type SessionHandlerConfig struct {
	SessionID    string
	Session      *session.Session
	Transport    transport.Transport
	Bridge       OrchestratorBridge
	Logger       *observability.Logger
	AudioProfile audio.AudioProfile
	VADConfig    vad.VADConfig
}

// NewSessionHandler creates a new session handler.
func NewSessionHandler(config SessionHandlerConfig) (*SessionHandler, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create VAD detector
	vadDetector := vad.NewEnergyVAD(config.VADConfig)
	vadProcessor := vad.NewVADProcessor(vadDetector)

	// Create normalizer
	normalizer := audio.NewPCM16Normalizer()

	// Create chunker for 10ms frames at 16kHz
	frameSize := 160 * 2 // 160 samples * 2 bytes (PCM16)
	chunker := audio.NewChunker(frameSize, nil)

	// Create playout tracker
	playoutTracker := audio.NewPlayoutTracker(16000, 1)

	// Create buffers
	inputBuffer := audio.NewJitterBuffer(100)
	outputBuffer := audio.NewJitterBuffer(100)

	logger := config.Logger.WithSession(config.SessionID, config.Session.TraceID)

	sh := &SessionHandler{
		sessionID:      config.SessionID,
		session:        config.Session,
		transport:      config.Transport,
		bridge:         config.Bridge,
		vadProcessor:   vadProcessor,
		normalizer:     normalizer,
		chunker:        chunker,
		playoutTracker: playoutTracker,
		logger:         logger,
		inputBuffer:    inputBuffer,
		outputBuffer:   outputBuffer,
		state:          session.StateIdle,
		ctx:            ctx,
		cancel:         cancel,
		eventCh:        make(chan events.Event, 100),
		speechBuffer:   make([]byte, 0, 16000*2), // 1 second buffer
		metrics:        observability.NewMetricsCollector("media_edge"),
	}

	// Set up VAD callbacks
	vadProcessor.SetOnSpeechStart(sh.onSpeechStart)
	vadProcessor.SetOnSpeechEnd(sh.onSpeechEnd)

	// Set up playout tracker callbacks
	playoutTracker.SetOnProgress(sh.onPlayoutProgress)
	playoutTracker.SetOnComplete(sh.onPlayoutComplete)

	return sh, nil
}

// Start starts the session handler.
func (sh *SessionHandler) Start() error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if sh.state != session.StateIdle {
		return fmt.Errorf("session already started")
	}

	// Start orchestrator event receiver
	orchEvents, err := sh.bridge.ReceiveEvents(sh.ctx, sh.sessionID)
	if err != nil {
		return fmt.Errorf("failed to receive orchestrator events: %w", err)
	}
	sh.orchestratorCh = orchEvents

	// Start event processing goroutine
	go sh.processEvents()

	// Start audio output goroutine
	go sh.processAudioOutput()

	sh.state = session.StateListening
	sh.session.SetState(session.StateListening)

	sh.logger.Info("Session handler started")

	return nil
}

// Stop stops the session handler.
func (sh *SessionHandler) Stop() error {
	sh.mu.Lock()
	if sh.state == session.StateIdle {
		sh.mu.Unlock()
		return nil
	}
	sh.state = session.StateIdle
	sh.mu.Unlock()

	// Cancel context
	sh.cancel()

	// Close buffers
	sh.inputBuffer.Close()
	sh.outputBuffer.Close()

	// Stop bridge session
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sh.bridge.StopSession(ctx, sh.sessionID)

	sh.logger.Info("Session handler stopped")

	return nil
}

// ProcessAudioChunk processes an incoming audio chunk.
func (sh *SessionHandler) ProcessAudioChunk(data []byte, profile audio.AudioProfile) error {
	sh.mu.RLock()
	if sh.state == session.StateIdle {
		sh.mu.RUnlock()
		return fmt.Errorf("session not active")
	}
	sh.mu.RUnlock()

	// Normalize audio to canonical format
	normalized, err := sh.normalizer.Normalize(data, profile)
	if err != nil {
		sh.logger.WithError(err).Warn("Failed to normalize audio")
		// Continue with original data
		normalized = data
	}

	// Feed to chunker to get fixed-size frames
	frames := audio.ChunkStatic(normalized, 160*2) // 10ms frames at 16kHz

	// Process each frame through VAD
	for _, frame := range frames.Frames {
		result := sh.vadProcessor.Process(frame)

		// Handle speech detection
		if result.IsSpeech || result.SpeechStart {
			sh.handleSpeechFrame(frame)
		} else if result.SpeechEnd {
			sh.handleSpeechEnd()
		}

		// Check for interruption
		if sh.botSpeaking && result.SpeechStart {
			sh.handleInterruption()
		}
	}

	// Handle partial frame
	if len(frames.PartialFrame) > 0 {
		// Buffer for next chunk
		sh.chunker.Write(frames.PartialFrame)
	}

	// Send audio to orchestrator
	if err := sh.bridge.SendAudio(sh.ctx, sh.sessionID, normalized); err != nil {
		sh.logger.WithError(err).Debug("Failed to send audio to orchestrator")
	}

	return nil
}

// handleSpeechFrame handles a frame detected as speech.
func (sh *SessionHandler) handleSpeechFrame(frame []byte) {
	if !sh.isSpeechActive {
		sh.isSpeechActive = true
		sh.speechBuffer = sh.speechBuffer[:0]

		// Send VAD event to client
		vadEvent := events.NewVADEvent(sh.sessionID, "speech_start")
		sh.sendEventToClient(vadEvent)

		sh.logger.Debug("Speech started")
	}

	// Accumulate audio for ASR
	sh.speechBuffer = append(sh.speechBuffer, frame...)
}

// handleSpeechEnd handles speech end detection.
func (sh *SessionHandler) handleSpeechEnd() {
	if !sh.isSpeechActive {
		return
	}

	sh.isSpeechActive = false

	// Send VAD event to client
	vadEvent := events.NewVADEvent(sh.sessionID, "speech_end")
	sh.sendEventToClient(vadEvent)

	sh.logger.Debug("Speech ended")

	// Transition state
	sh.mu.Lock()
	if sh.state == session.StateListening {
		sh.state = session.StateProcessing
		sh.session.SetState(session.StateProcessing)
	}
	sh.mu.Unlock()
}

// onSpeechStart is called when VAD detects speech start.
func (sh *SessionHandler) onSpeechStart() {
	sh.logger.Debug("VAD: Speech start detected")
}

// onSpeechEnd is called when VAD detects speech end.
func (sh *SessionHandler) onSpeechEnd(duration time.Duration) {
	sh.logger.WithFields(map[string]interface{}{
		"duration_ms": duration.Milliseconds(),
	}).Debug("VAD: Speech end detected")
}

// handleInterruption handles user interruption during bot speech.
func (sh *SessionHandler) handleInterruption() {
	sh.mu.Lock()
	if !sh.botSpeaking {
		sh.mu.Unlock()
		return
	}

	sh.interrupted = true
	sh.botSpeaking = false
	sh.state = session.StateInterrupted
	sh.mu.Unlock()

	// Notify orchestrator
	if err := sh.bridge.Interrupt(sh.ctx, sh.sessionID); err != nil {
		sh.logger.WithError(err).Warn("Failed to send interrupt to orchestrator")
	}

	// Send interruption event to client
	interruptionEvent := events.NewInterruptionEvent(sh.sessionID, "user_speech")
	interruptionEvent.SpokenText = sh.session.GetActiveTurn().GetSpokenText()
	interruptionEvent.PlayoutPosition = sh.playoutTracker.CurrentPosition().Milliseconds()
	sh.sendEventToClient(interruptionEvent)

	// Clear output buffer
	sh.outputBuffer.Clear()
	sh.playoutTracker.Reset()

	sh.logger.Info("Interruption handled")

	// Transition back to listening
	sh.mu.Lock()
	sh.state = session.StateListening
	sh.session.SetState(session.StateListening)
	sh.mu.Unlock()
}

// processEvents processes events from the orchestrator.
func (sh *SessionHandler) processEvents() {
	for {
		select {
		case <-sh.ctx.Done():
			return

		case event, ok := <-sh.orchestratorCh:
			if !ok {
				sh.logger.Info("Orchestrator event channel closed")
				return
			}

			sh.handleOrchestratorEvent(event)
		}
	}
}

// handleOrchestratorEvent handles an event from the orchestrator.
func (sh *SessionHandler) handleOrchestratorEvent(event events.Event) {
	switch e := event.(type) {
	case *events.ASRPartialEvent:
		// Forward to client
		sh.sendEventToClient(e)

	case *events.ASRFinalEvent:
		// Forward to client and send to orchestrator for LLM processing
		sh.sendEventToClient(e)

		sh.mu.Lock()
		sh.state = session.StateProcessing
		sh.mu.Unlock()

		// Send utterance to orchestrator
		if err := sh.bridge.SendUserUtterance(sh.ctx, sh.sessionID, e.Transcript); err != nil {
			sh.logger.WithError(err).Warn("Failed to send utterance to orchestrator")
		}

	case *events.LLMPartialTextEvent:
		// Forward to client
		sh.sendEventToClient(e)

		sh.mu.Lock()
		if sh.state == session.StateProcessing {
			sh.state = session.StateSpeaking
			sh.session.SetState(session.StateSpeaking)
			sh.botSpeaking = true
		}
		sh.mu.Unlock()

	case *events.TTSAudioChunkEvent:
		// Decode and queue for playout
		audioData, err := e.GetAudioData()
		if err != nil {
			sh.logger.WithError(err).Warn("Failed to decode TTS audio")
			return
		}

		// Add to output buffer
		if err := sh.outputBuffer.Write(audioData); err != nil {
			sh.logger.WithError(err).Debug("Output buffer full, dropping audio")
		}

	case *events.TurnEvent:
		// Forward to client
		sh.sendEventToClient(e)

		// Update session state
		if e.Event == "completed" {
			sh.mu.Lock()
			sh.botSpeaking = false
			sh.state = session.StateListening
			sh.session.SetState(session.StateListening)
			sh.mu.Unlock()
		}

	case *events.ErrorEvent:
		// Forward error to client
		sh.sendEventToClient(e)
		sh.logger.WithFields(map[string]interface{}{
			"error_code":    e.Code,
			"error_message": e.Message,
		}).Error("Orchestrator error")

	default:
		sh.logger.WithField("event_type", event.GetType()).Debug("Unknown event type from orchestrator")
	}
}

// processAudioOutput processes audio output to the client.
func (sh *SessionHandler) processAudioOutput() {
	ticker := time.NewTicker(10 * time.Millisecond) // 10ms frames
	defer ticker.Stop()

	for {
		select {
		case <-sh.ctx.Done():
			return

		case <-ticker.C:
			// Try to read from output buffer
			frame, ok := sh.outputBuffer.TryRead()
			if !ok {
				continue
			}

			// Send to client
			if err := sh.transport.SendAudio(frame); err != nil {
				sh.logger.WithError(err).Debug("Failed to send audio to client")
				continue
			}

			// Update playout tracker
			sh.playoutTracker.Advance(len(frame))
		}
	}
}

// sendEventToClient sends an event to the client via the transport.
func (sh *SessionHandler) sendEventToClient(event events.Event) {
	if err := sh.transport.SendEvent(event); err != nil {
		sh.logger.WithError(err).Debug("Failed to send event to client")
	}
}

// onPlayoutProgress is called when playout advances.
func (sh *SessionHandler) onPlayoutProgress(position time.Duration) {
	// Update active turn playout cursor
	if turn := sh.session.GetActiveTurn(); turn != nil {
		turn.AdvancePlayout(160 * 2) // 10ms frame
	}
}

// onPlayoutComplete is called when playout completes.
func (sh *SessionHandler) onPlayoutComplete() {
	sh.mu.Lock()
	sh.botSpeaking = false
	if sh.state == session.StateSpeaking {
		sh.state = session.StateListening
		sh.session.SetState(session.StateListening)
	}
	sh.mu.Unlock()

	sh.logger.Debug("Playout complete")
}

// HandleInterrupt handles a manual interrupt request.
func (sh *SessionHandler) HandleInterrupt() error {
	sh.mu.Lock()
	if !sh.botSpeaking {
		sh.mu.Unlock()
		return nil
	}
	sh.mu.Unlock()

	sh.handleInterruption()
	return nil
}

// UpdateConfig updates the session configuration.
func (sh *SessionHandler) UpdateConfig(systemPrompt string, voiceProfile *events.VoiceProfileConfig, modelOptions *events.ModelOptionsConfig, providers *events.ProviderConfig) error {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	if systemPrompt != "" {
		sh.session.SystemPrompt = systemPrompt
	}

	if voiceProfile != nil {
		sh.session.SetVoiceProfile(session.VoiceProfile{
			VoiceID: voiceProfile.VoiceID,
			Speed:   voiceProfile.Speed,
			Pitch:   voiceProfile.Pitch,
		})
	}

	if modelOptions != nil {
		sh.session.SetModelOptions(session.ModelOptions{
			ModelName:     modelOptions.ModelName,
			MaxTokens:     modelOptions.MaxTokens,
			Temperature:   modelOptions.Temperature,
			TopP:          modelOptions.TopP,
			StopSequences: modelOptions.StopSequences,
			SystemPrompt:  modelOptions.ModelName,
		})
	}

	if providers != nil {
		sh.session.SetProviders(session.SelectedProviders{
			ASR: providers.ASR,
			LLM: providers.LLM,
			TTS: providers.TTS,
			VAD: providers.VAD,
		})
	}

	sh.logger.Info("Session configuration updated")

	return nil
}

// State returns the current session state.
func (sh *SessionHandler) State() session.SessionState {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.state
}

// IsBotSpeaking returns true if the bot is currently speaking.
func (sh *SessionHandler) IsBotSpeaking() bool {
	sh.mu.RLock()
	defer sh.mu.RUnlock()
	return sh.botSpeaking
}

// SessionID returns the session ID.
func (sh *SessionHandler) SessionID() string {
	return sh.sessionID
}

// PlayoutPosition returns the current playout position.
func (sh *SessionHandler) PlayoutPosition() time.Duration {
	return sh.playoutTracker.CurrentPosition()
}
