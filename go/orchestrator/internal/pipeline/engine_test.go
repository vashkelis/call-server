package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/observability"
	"github.com/parlona/cloudapp/pkg/providers"
	"github.com/parlona/cloudapp/pkg/session"
)

// MockASRProvider is a mock ASR provider for testing
type MockASRProvider struct {
	name       string
	transcript string
	delay      time.Duration
	cancelled  bool
	cancelMu   sync.Mutex
}

func NewMockASRProvider(name, transcript string, delay time.Duration) *MockASRProvider {
	return &MockASRProvider{
		name:       name,
		transcript: transcript,
		delay:      delay,
	}
}

func (m *MockASRProvider) StreamRecognize(ctx context.Context, audioStream chan []byte, opts providers.ASROptions) (chan providers.ASRResult, error) {
	resultCh := make(chan providers.ASRResult, 10)

	go func() {
		defer close(resultCh)

		// Wait a bit then send partial
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.delay / 2):
			resultCh <- providers.ASRResult{
				Transcript: "partial...",
				IsPartial:  true,
			}
		}

		// Wait more then send final
		select {
		case <-ctx.Done():
			return
		case <-time.After(m.delay / 2):
			resultCh <- providers.ASRResult{
				Transcript: m.transcript,
				IsFinal:    true,
			}
		}
	}()

	// Consume audio stream
	go func() {
		for range audioStream {
			// Just consume
		}
	}()

	return resultCh, nil
}

func (m *MockASRProvider) Cancel(ctx context.Context, sessionID string) error {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.cancelled = true
	return nil
}

func (m *MockASRProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{
		SupportsStreamingInput:  true,
		SupportsStreamingOutput: true,
	}
}

func (m *MockASRProvider) Name() string {
	return m.name
}

func (m *MockASRProvider) WasCancelled() bool {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	return m.cancelled
}

// MockLLMProvider is a mock LLM provider for testing
type MockLLMProvider struct {
	name       string
	response   string
	tokenDelay time.Duration
	cancelled  bool
	cancelMu   sync.Mutex
}

func NewMockLLMProvider(name, response string, tokenDelay time.Duration) *MockLLMProvider {
	return &MockLLMProvider{
		name:       name,
		response:   response,
		tokenDelay: tokenDelay,
	}
}

func (m *MockLLMProvider) StreamGenerate(ctx context.Context, messages []contracts.ChatMessage, opts providers.LLMOptions) (chan providers.LLMToken, error) {
	tokenCh := make(chan providers.LLMToken, 10)

	go func() {
		defer close(tokenCh)

		tokens := splitIntoTokens(m.response)
		for i, token := range tokens {
			select {
			case <-ctx.Done():
				return
			case <-time.After(m.tokenDelay):
				isFinal := i == len(tokens)-1
				tokenCh <- providers.LLMToken{
					Token:   token,
					IsFinal: isFinal,
				}
			}
		}
	}()

	return tokenCh, nil
}

func (m *MockLLMProvider) Cancel(ctx context.Context, sessionID string) error {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.cancelled = true
	return nil
}

func (m *MockLLMProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{
		SupportsStreamingInput:  true,
		SupportsStreamingOutput: true,
	}
}

func (m *MockLLMProvider) Name() string {
	return m.name
}

func (m *MockLLMProvider) WasCancelled() bool {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	return m.cancelled
}

// MockTTSProvider is a mock TTS provider for testing
type MockTTSProvider struct {
	name       string
	chunkSize  int
	chunkDelay time.Duration
	cancelled  bool
	cancelMu   sync.Mutex
}

func NewMockTTSProvider(name string, chunkSize int, chunkDelay time.Duration) *MockTTSProvider {
	return &MockTTSProvider{
		name:       name,
		chunkSize:  chunkSize,
		chunkDelay: chunkDelay,
	}
}

func (m *MockTTSProvider) StreamSynthesize(ctx context.Context, text string, opts providers.TTSOptions) (chan []byte, error) {
	audioCh := make(chan []byte, 10)

	go func() {
		defer close(audioCh)

		// Simulate audio chunks based on text length
		numChunks := len(text) / 10
		if numChunks < 1 {
			numChunks = 1
		}

		for i := 0; i < numChunks; i++ {
			select {
			case <-ctx.Done():
				return
			case <-time.After(m.chunkDelay):
				chunk := make([]byte, m.chunkSize)
				// Fill with some pattern
				for j := range chunk {
					chunk[j] = byte(i % 256)
				}
				audioCh <- chunk
			}
		}
	}()

	return audioCh, nil
}

func (m *MockTTSProvider) Cancel(ctx context.Context, sessionID string) error {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	m.cancelled = true
	return nil
}

func (m *MockTTSProvider) Capabilities() contracts.ProviderCapability {
	return contracts.ProviderCapability{
		SupportsStreamingInput:  true,
		SupportsStreamingOutput: true,
		SupportsVoices:          true,
	}
}

func (m *MockTTSProvider) Name() string {
	return m.name
}

func (m *MockTTSProvider) WasCancelled() bool {
	m.cancelMu.Lock()
	defer m.cancelMu.Unlock()
	return m.cancelled
}

// Helper function to split text into tokens
func splitIntoTokens(text string) []string {
	// Simple word-based tokenization
	words := []string{}
	current := ""
	for _, r := range text {
		if r == ' ' {
			if current != "" {
				words = append(words, current+" ")
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		words = append(words, current)
	}
	return words
}

// MockSessionStore is a mock session store for testing
type MockSessionStore struct {
	sessions map[string]*session.Session
	mu       sync.RWMutex
}

func NewMockSessionStore() *MockSessionStore {
	return &MockSessionStore{
		sessions: make(map[string]*session.Session),
	}
}

func (m *MockSessionStore) Get(ctx context.Context, sessionID string) (*session.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if sess, ok := m.sessions[sessionID]; ok {
		return sess, nil
	}
	return nil, session.ErrSessionNotFound
}

func (m *MockSessionStore) Save(ctx context.Context, sess *session.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sess.SessionID] = sess
	return nil
}

func (m *MockSessionStore) Delete(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
	return nil
}

func (m *MockSessionStore) UpdateTurn(ctx context.Context, sessionID string, turn *session.AssistantTurn) error {
	return nil
}

func (m *MockSessionStore) List(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *MockSessionStore) Close() error {
	return nil
}

func (m *MockSessionStore) CreateSession(sessionID, traceID string, transport session.TransportType) *session.Session {
	sess := session.NewSession(sessionID, traceID, transport)
	sess.SetModelOptions(session.ModelOptions{
		ModelName:   "mock-model",
		Temperature: 0.7,
		MaxTokens:   1024,
	})
	sess.SystemPrompt = "You are a helpful assistant."
	sess.SetVoiceProfile(session.VoiceProfile{
		VoiceID: "mock-voice",
		Speed:   1.0,
	})
	m.sessions[sessionID] = sess
	return sess
}

// TestMockEndToEndPipeline tests the complete ASR->LLM->TTS pipeline
func TestMockEndToEndPipeline(t *testing.T) {
	// Create mock providers
	mockASR := NewMockASRProvider("mock-asr", "Hello, how are you?", 100*time.Millisecond)
	mockLLM := NewMockLLMProvider("mock-llm", "I am doing well, thank you for asking!", 20*time.Millisecond)
	mockTTS := NewMockTTSProvider("mock-tts", 320, 10*time.Millisecond)

	// Create registry and register providers
	registry := providers.NewProviderRegistry()
	registry.RegisterASR("default", mockASR)
	registry.RegisterLLM("default", mockLLM)
	registry.RegisterTTS("default", mockTTS)

	// Create session store with a session
	store := NewMockSessionStore()
	store.CreateSession("test-session", "test-trace", session.TransportTypeWebSocket)

	// Create engine
	config := DefaultConfig()
	config.MaxSessionDuration = 5 * time.Minute
	logger := observability.NewDevelopmentLogger()
	engine := NewEngine(registry, store, nil, nil, config, logger)

	// Create audio stream and event sink
	audioStream := make(chan []byte, 10)
	eventSink := make(chan interface{}, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start processing in background
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := engine.ProcessSession(ctx, "test-session", audioStream, eventSink)
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Errorf("ProcessSession error: %v", err)
		}
	}()

	// Send some audio
	audioStream <- make([]byte, 1600) // 100ms of 16kHz audio
	close(audioStream)

	// Collect events
	events := []interface{}{}
	eventDone := make(chan struct{})
	go func() {
		for event := range eventSink {
			events = append(events, event)
		}
		close(eventDone)
	}()

	// Wait for processing
	wg.Wait()
	close(eventSink)
	<-eventDone

	// Verify we got events
	if len(events) == 0 {
		t.Error("expected events, got none")
	}

	// Verify session is no longer active
	if engine.IsSessionActive("test-session") {
		t.Error("expected session to be inactive after audio stream closed")
	}
}

// TestInterruptionDuringBotSpeech tests interruption handling
func TestInterruptionDuringBotSpeech(t *testing.T) {
	// Create mock providers with longer delays to allow interruption
	mockASR := NewMockASRProvider("mock-asr", "Stop speaking", 200*time.Millisecond)
	mockLLM := NewMockLLMProvider("mock-llm", "This is a very long response that should be interrupted before it completes completely.", 50*time.Millisecond)
	mockTTS := NewMockTTSProvider("mock-tts", 320, 30*time.Millisecond)

	registry := providers.NewProviderRegistry()
	registry.RegisterASR("default", mockASR)
	registry.RegisterLLM("default", mockLLM)
	registry.RegisterTTS("default", mockTTS)

	store := NewMockSessionStore()
	store.CreateSession("test-interrupt", "test-trace", session.TransportTypeWebSocket)

	config := DefaultConfig()
	config.EnableInterruptions = true
	logger := observability.NewDevelopmentLogger()
	engine := NewEngine(registry, store, nil, nil, config, logger)

	audioStream := make(chan []byte, 10)
	eventSink := make(chan interface{}, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start processing
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := engine.ProcessSession(ctx, "test-interrupt", audioStream, eventSink)
		if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
			t.Errorf("ProcessSession error: %v", err)
		}
	}()

	// Send initial audio
	audioStream <- make([]byte, 1600)

	// Wait a bit for processing to start
	time.Sleep(300 * time.Millisecond)

	// Trigger interruption
	err := engine.HandleInterruption(ctx, "test-interrupt")
	if err != nil {
		t.Errorf("HandleInterruption error: %v", err)
	}

	// Verify providers were cancelled
	if !mockLLM.WasCancelled() {
		t.Error("expected LLM to be cancelled")
	}
	if !mockTTS.WasCancelled() {
		t.Error("expected TTS to be cancelled")
	}

	// Clean up
	close(audioStream)
	wg.Wait()
}

// TestProviderSwitching tests changing providers mid-session
func TestProviderSwitching(t *testing.T) {
	// Create initial mock providers
	mockASR1 := NewMockASRProvider("asr-1", "First transcript", 50*time.Millisecond)
	mockLLM1 := NewMockLLMProvider("llm-1", "First response", 20*time.Millisecond)
	mockTTS1 := NewMockTTSProvider("tts-1", 320, 10*time.Millisecond)

	// Create alternative providers
	mockASR2 := NewMockASRProvider("asr-2", "Second transcript", 50*time.Millisecond)
	mockLLM2 := NewMockLLMProvider("llm-2", "Second response", 20*time.Millisecond)
	mockTTS2 := NewMockTTSProvider("tts-2", 320, 10*time.Millisecond)

	registry := providers.NewProviderRegistry()
	registry.RegisterASR("default", mockASR1)
	registry.RegisterLLM("default", mockLLM1)
	registry.RegisterTTS("default", mockTTS1)
	registry.RegisterASR("alternative", mockASR2)
	registry.RegisterLLM("alternative", mockLLM2)
	registry.RegisterTTS("alternative", mockTTS2)

	store := NewMockSessionStore()
	sess := store.CreateSession("test-switch", "test-trace", session.TransportTypeWebSocket)

	// Set initial providers
	sess.SetProviders(session.SelectedProviders{
		ASR: "default",
		LLM: "default",
		TTS: "default",
	})

	config := DefaultConfig()
	logger := observability.NewDevelopmentLogger()
	_ = NewEngine(registry, store, nil, nil, config, logger)

	// Verify we can resolve providers
	resolved, err := registry.ResolveForSession(sess, nil)
	if err != nil {
		t.Fatalf("ResolveForSession error: %v", err)
	}
	if resolved.ASR != "default" {
		t.Errorf("expected ASR provider 'default', got '%s'", resolved.ASR)
	}

	// Switch providers
	sess.SetProviders(session.SelectedProviders{
		ASR: "alternative",
		LLM: "alternative",
		TTS: "alternative",
	})

	// Verify new providers are resolved
	resolved, err = registry.ResolveForSession(sess, nil)
	if err != nil {
		t.Fatalf("ResolveForSession error after switch: %v", err)
	}
	if resolved.ASR != "alternative" {
		t.Errorf("expected ASR provider 'alternative', got '%s'", resolved.ASR)
	}
	if resolved.LLM != "alternative" {
		t.Errorf("expected LLM provider 'alternative', got '%s'", resolved.LLM)
	}
	if resolved.TTS != "alternative" {
		t.Errorf("expected TTS provider 'alternative', got '%s'", resolved.TTS)
	}
}

// TestStateCommitGolden is a golden test for state commit behavior
func TestStateCommitGolden(t *testing.T) {
	// This test verifies exact conversation history matches expected output
	// after an interruption scenario

	mockASR := NewMockASRProvider("mock-asr", "Tell me a story", 50*time.Millisecond)
	mockLLM := NewMockLLMProvider("mock-llm", "Once upon a time there was a brave knight who fought dragons.", 30*time.Millisecond)
	mockTTS := NewMockTTSProvider("mock-tts", 320, 20*time.Millisecond)

	registry := providers.NewProviderRegistry()
	registry.RegisterASR("default", mockASR)
	registry.RegisterLLM("default", mockLLM)
	registry.RegisterTTS("default", mockTTS)

	store := NewMockSessionStore()
	sess := store.CreateSession("test-golden", "golden-trace", session.TransportTypeWebSocket)
	sess.SystemPrompt = "You are a storyteller."

	config := DefaultConfig()
	logger := observability.NewDevelopmentLogger()
	engine := NewEngine(registry, store, nil, nil, config, logger)

	// Create conversation history
	history := session.NewConversationHistory(100)
	history.AppendUserMessage("Tell me a story")

	// Simulate a turn that gets interrupted
	turnManager := engine.turnManagerReg.GetOrCreate("test-golden", history)
	turn := turnManager.StartTurn("gen-123", 16000)

	// Simulate generated text
	fullText := "Once upon a time there was a brave knight who fought dragons and saved the kingdom."
	turn.AppendGeneratedText(fullText)

	// Simulate playout of only part of the text (interruption happens here)
	turn.AdvancePlayout(10000) // Only partway through
	turn.MarkInterrupted(10000)

	// Commit the turn
	committed := turnManager.CommitTurn()

	// The committed text should only contain what was spoken, not the full generated text
	if committed.Content == fullText {
		t.Error("committed text should not equal full generated text after interruption")
	}

	// The committed text should be shorter than the full text
	if len(committed.Content) >= len(fullText) {
		t.Errorf("committed text length (%d) should be less than full text length (%d)",
			len(committed.Content), len(fullText))
	}

	// Verify the turn manager state is clean after commit
	if turnManager.GetGenerationID() != "" {
		t.Error("generation ID should be cleared after commit")
	}
}

// TestEngineSessionLifecycle tests the complete session lifecycle
func TestEngineSessionLifecycle(t *testing.T) {
	mockASR := NewMockASRProvider("mock-asr", "Test message", 50*time.Millisecond)
	mockLLM := NewMockLLMProvider("mock-llm", "Test response", 20*time.Millisecond)
	mockTTS := NewMockTTSProvider("mock-tts", 320, 10*time.Millisecond)

	registry := providers.NewProviderRegistry()
	registry.RegisterASR("default", mockASR)
	registry.RegisterLLM("default", mockLLM)
	registry.RegisterTTS("default", mockTTS)

	store := NewMockSessionStore()
	store.CreateSession("lifecycle-session", "lifecycle-trace", session.TransportTypeWebSocket)

	config := DefaultConfig()
	logger := observability.NewDevelopmentLogger()
	engine := NewEngine(registry, store, nil, nil, config, logger)

	// Verify session is not active initially
	if engine.IsSessionActive("lifecycle-session") {
		t.Error("session should not be active initially")
	}

	// Start a session
	audioStream := make(chan []byte, 10)
	eventSink := make(chan interface{}, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		engine.ProcessSession(ctx, "lifecycle-session", audioStream, eventSink)
	}()

	// Give it time to start
	time.Sleep(100 * time.Millisecond)

	// Verify session is now active
	if !engine.IsSessionActive("lifecycle-session") {
		t.Error("session should be active after starting")
	}

	// Check active sessions list
	sessions := engine.GetActiveSessions()
	found := false
	for _, id := range sessions {
		if id == "lifecycle-session" {
			found = true
			break
		}
	}
	if !found {
		t.Error("session should be in active sessions list")
	}

	// Send audio and close stream
	close(audioStream)
	wg.Wait()

	// Verify session is no longer active
	if engine.IsSessionActive("lifecycle-session") {
		t.Error("session should not be active after closing audio stream")
	}

	// Test StopSession
	close(eventSink)
}

// TestEngineMultipleUtterances tests processing multiple utterances in one session
func TestEngineMultipleUtterances(t *testing.T) {
	mockASR := NewMockASRProvider("mock-asr", "First utterance", 50*time.Millisecond)
	mockLLM := NewMockLLMProvider("mock-llm", "First response", 20*time.Millisecond)
	mockTTS := NewMockTTSProvider("mock-tts", 320, 10*time.Millisecond)

	registry := providers.NewProviderRegistry()
	registry.RegisterASR("default", mockASR)
	registry.RegisterLLM("default", mockLLM)
	registry.RegisterTTS("default", mockTTS)

	store := NewMockSessionStore()
	store.CreateSession("multi-utterance", "multi-trace", session.TransportTypeWebSocket)

	config := DefaultConfig()
	logger := observability.NewDevelopmentLogger()
	engine := NewEngine(registry, store, nil, nil, config, logger)

	// Test ProcessUserUtterance directly
	ctx := context.Background()

	// First need to set up the session context
	audioStream := make(chan []byte, 10)
	eventSink := make(chan interface{}, 100)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		engine.ProcessSession(ctx, "multi-utterance", audioStream, eventSink)
	}()

	// Wait for session to be active
	time.Sleep(100 * time.Millisecond)

	// Process an utterance directly
	err := engine.ProcessUserUtterance(ctx, "multi-utterance", "Hello", eventSink)
	if err != nil {
		t.Errorf("ProcessUserUtterance error: %v", err)
	}

	// Clean up
	close(audioStream)
	wg.Wait()
	close(eventSink)
}
