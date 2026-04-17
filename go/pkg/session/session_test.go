package session

import (
	"testing"
	"time"

	"github.com/parlona/cloudapp/pkg/contracts"
)

func TestNewSession(t *testing.T) {
	sessionID := "test-session-123"
	traceID := "trace-456"
	transport := TransportTypeWebSocket

	session := NewSession(sessionID, traceID, transport)

	if session.SessionID != sessionID {
		t.Errorf("expected session ID %s, got %s", sessionID, session.SessionID)
	}
	if session.TraceID != traceID {
		t.Errorf("expected trace ID %s, got %s", traceID, session.TraceID)
	}
	if session.TransportType != transport {
		t.Errorf("expected transport type %v, got %v", transport, session.TransportType)
	}
	if session.CurrentState != StateIdle {
		t.Errorf("expected initial state %v, got %v", StateIdle, session.CurrentState)
	}
	if session.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if session.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSessionStateTransitions(t *testing.T) {
	session := NewSession("test-session", "trace-123", TransportTypeWebSocket)

	tests := []struct {
		name      string
		newState  SessionState
		wantError bool
	}{
		{"Idle -> Listening", StateListening, false},
		{"Listening -> Processing", StateProcessing, false},
		{"Processing -> Speaking", StateSpeaking, false},
		{"Speaking -> Idle", StateIdle, false},
		{"Idle -> Speaking (invalid)", StateSpeaking, true},
		{"Idle -> Processing (invalid)", StateProcessing, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := session.SetState(tt.newState)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected error for transition to %v", tt.newState)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if session.GetState() != tt.newState {
					t.Errorf("expected state %v, got %v", tt.newState, session.GetState())
				}
			}
		})
	}
}

func TestAssistantTurnCommit(t *testing.T) {
	turn := NewAssistantTurn("gen-123", 16000)

	// Simulate text generation and TTS queueing
	turn.AppendGeneratedText("Hello, this is a test response.")
	turn.QueueForTTS("Hello, this is a test response.")

	// Simulate playout progress (advance by some bytes)
	// At 16kHz, mono, PCM16: 2 bytes per sample, ~15 chars per second
	// For "Hello, this is a test response." (31 chars), we need about 2 seconds of audio
	// 2 seconds * 16000 samples/second * 2 bytes/sample = 64000 bytes
	turn.AdvancePlayout(64000)

	// Commit the spoken text
	committed := turn.CommitSpokenText()

	// Verify spoken text is returned
	if committed == "" {
		t.Error("expected committed text to not be empty")
	}

	// Verify spoken text is set
	if turn.GetSpokenText() != committed {
		t.Errorf("expected spoken text to match committed text: %s", committed)
	}

	// Verify generated text is trimmed (only unspoken portion remains)
	// Since we advanced enough bytes for all text, generated should be empty or minimal
	t.Logf("Generated text after commit: %q", turn.GetGeneratedText())
}

func TestAssistantTurnInterruption(t *testing.T) {
	turn := NewAssistantTurn("gen-456", 16000)

	// Generate some text
	fullText := "This is a long response that will be interrupted."
	turn.AppendGeneratedText(fullText)

	// Simulate partial playout (halfway through)
	// At 16kHz, mono, PCM16: ~15 chars per second
	// For 49 chars, we need about 3.3 seconds of audio
	// Halfway = 1.65 seconds * 16000 * 2 = ~52800 bytes
	turn.AdvancePlayout(26400)

	// Mark interruption at current playout position
	turn.MarkInterrupted(26400)

	// Verify interruption flag
	if !turn.IsInterrupted() {
		t.Error("expected turn to be marked as interrupted")
	}

	// Get committable text (should only be what was spoken)
	committable := turn.GetCommittableText()

	// Verify committable text is less than full text
	if len(committable) >= len(fullText) {
		t.Errorf("expected committable text (%d chars) to be less than full text (%d chars)",
			len(committable), len(fullText))
	}

	t.Logf("Full text: %q (%d chars)", fullText, len(fullText))
	t.Logf("Committable text: %q (%d chars)", committable, len(committable))
}

func TestConversationHistory(t *testing.T) {
	history := NewConversationHistory(10)

	// Append user messages
	history.AppendUserMessage("Hello, how are you?")
	history.AppendUserMessage("What's the weather like?")

	// Append assistant messages
	history.AppendAssistantMessage("I'm doing well, thank you!")
	history.AppendAssistantMessage("It's sunny today.")

	// Verify message count
	if history.Len() != 4 {
		t.Errorf("expected 4 messages, got %d", history.Len())
	}

	// Verify ordering
	messages := history.GetMessages()
	if messages[0].Role != "user" || messages[0].Content != "Hello, how are you?" {
		t.Errorf("expected first message to be user 'Hello, how are you?', got %s %q",
			messages[0].Role, messages[0].Content)
	}
	if messages[2].Role != "assistant" || messages[2].Content != "I'm doing well, thank you!" {
		t.Errorf("expected third message to be assistant 'I'm doing well, thank you!', got %s %q",
			messages[2].Role, messages[2].Content)
	}

	// Test max length truncation
	smallHistory := NewConversationHistory(3)
	smallHistory.AppendUserMessage("Message 1")
	smallHistory.AppendAssistantMessage("Response 1")
	smallHistory.AppendUserMessage("Message 2")
	smallHistory.AppendAssistantMessage("Response 2")
	smallHistory.AppendUserMessage("Message 3")

	// Should be truncated to max size
	if smallHistory.Len() > 3 {
		t.Errorf("expected history to be truncated to max size 3, got %d", smallHistory.Len())
	}

	// Most recent messages should be kept
	lastMsg, ok := smallHistory.GetLastUserMessage()
	if !ok || lastMsg != "Message 3" {
		t.Errorf("expected last user message to be 'Message 3', got %q", lastMsg)
	}
}

func TestSessionSetters(t *testing.T) {
	session := NewSession("test-session", "trace-123", TransportTypeWebSocket)

	// Test SetTenantID
	session.SetTenantID("tenant-456")
	if session.TenantID == nil || *session.TenantID != "tenant-456" {
		t.Error("expected tenant ID to be set")
	}

	// Test SetProviders
	providers := SelectedProviders{
		ASR: "google",
		LLM: "openai",
		TTS: "elevenlabs",
	}
	session.SetProviders(providers)
	if session.SelectedProviders.ASR != "google" {
		t.Errorf("expected ASR provider google, got %s", session.SelectedProviders.ASR)
	}

	// Test SetAudioProfile
	profile := contracts.AudioFormat{
		SampleRate: 16000,
		Channels:   1,
		Encoding:   contracts.PCM16,
	}
	session.SetAudioProfile(profile)
	if session.AudioProfile.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", session.AudioProfile.SampleRate)
	}

	// Test SetVoiceProfile
	voiceProfile := VoiceProfile{
		VoiceID: "voice-123",
		Speed:   1.2,
		Pitch:   1.0,
	}
	session.SetVoiceProfile(voiceProfile)
	if session.VoiceProfile.VoiceID != "voice-123" {
		t.Errorf("expected voice ID voice-123, got %s", session.VoiceProfile.VoiceID)
	}

	// Test SetModelOptions
	modelOpts := ModelOptions{
		ModelName:    "gpt-4",
		MaxTokens:    1024,
		Temperature:  0.7,
		SystemPrompt: "You are a helpful assistant.",
	}
	session.SetModelOptions(modelOpts)
	if session.ModelOptions.ModelName != "gpt-4" {
		t.Errorf("expected model name gpt-4, got %s", session.ModelOptions.ModelName)
	}
	if session.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("expected system prompt to be set, got %s", session.SystemPrompt)
	}
}

func TestSessionBotSpeaking(t *testing.T) {
	session := NewSession("test-session", "trace-123", TransportTypeWebSocket)

	if session.IsBotSpeaking() {
		t.Error("expected bot to not be speaking initially")
	}

	session.SetBotSpeaking(true)
	if !session.IsBotSpeaking() {
		t.Error("expected bot to be speaking after setting")
	}

	session.SetBotSpeaking(false)
	if session.IsBotSpeaking() {
		t.Error("expected bot to not be speaking after clearing")
	}
}

func TestSessionInterrupted(t *testing.T) {
	session := NewSession("test-session", "trace-123", TransportTypeWebSocket)

	if session.IsInterrupted() {
		t.Error("expected session to not be interrupted initially")
	}

	session.SetInterrupted(true)
	if !session.IsInterrupted() {
		t.Error("expected session to be interrupted after setting")
	}

	session.SetInterrupted(false)
	if session.IsInterrupted() {
		t.Error("expected session to not be interrupted after clearing")
	}
}

func TestSessionClone(t *testing.T) {
	original := NewSession("test-session", "trace-123", TransportTypeWebSocket)
	original.SetTenantID("tenant-456")
	original.SetProviders(SelectedProviders{ASR: "google", LLM: "openai", TTS: "elevenlabs"})

	activeTurn := NewAssistantTurn("gen-123", 16000)
	activeTurn.AppendGeneratedText("Test response")
	original.SetActiveTurn(activeTurn)

	clone := original.Clone()

	// Verify clone has same values
	if clone.SessionID != original.SessionID {
		t.Error("expected clone to have same session ID")
	}
	if *clone.TenantID != *original.TenantID {
		t.Error("expected clone to have same tenant ID")
	}

	// Verify clone is independent
	clone.SetTenantID("tenant-789")
	if *original.TenantID != "tenant-456" {
		t.Error("expected original to be unchanged when clone is modified")
	}

	// Verify active turn is cloned
	if clone.ActiveTurn == nil {
		t.Error("expected clone to have active turn")
	}
	if clone.ActiveTurn.GetGenerationID() != activeTurn.GetGenerationID() {
		t.Error("expected clone to have same generation ID")
	}

	// Verify turn is deeply cloned
	clone.ActiveTurn.AppendGeneratedText(" more text")
	if original.ActiveTurn.GetGeneratedText() != activeTurn.GetGeneratedText() {
		t.Error("expected original turn to be unchanged when clone is modified")
	}
}

func TestSessionTouch(t *testing.T) {
	session := NewSession("test-session", "trace-123", TransportTypeWebSocket)
	oldUpdatedAt := session.UpdatedAt

	// Wait a tiny bit to ensure time changes
	time.Sleep(1 * time.Millisecond)

	session.Touch()

	if !session.UpdatedAt.After(oldUpdatedAt) {
		t.Error("expected UpdatedAt to be updated after Touch()")
	}
}

func TestAssistantTurnPlayout(t *testing.T) {
	turn := NewAssistantTurn("gen-123", 16000)

	// Initial position should be 0
	if turn.GetPlayoutCursor() != 0 {
		t.Errorf("expected initial playout cursor to be 0, got %d", turn.GetPlayoutCursor())
	}

	// Advance playout
	turn.AdvancePlayout(32000) // 1 second at 16kHz mono PCM16

	if turn.GetPlayoutCursor() != 32000 {
		t.Errorf("expected playout cursor to be 32000, got %d", turn.GetPlayoutCursor())
	}

	// Check position as duration
	position := turn.CurrentPosition()
	expectedDuration := time.Second
	if position < expectedDuration-50*time.Millisecond || position > expectedDuration+50*time.Millisecond {
		t.Errorf("expected position ~1s, got %v", position)
	}

	// Reset
	turn.Reset()
	if turn.GetPlayoutCursor() != 0 {
		t.Errorf("expected playout cursor to be 0 after reset, got %d", turn.GetPlayoutCursor())
	}
}

func TestEstimateTextDuration(t *testing.T) {
	tests := []struct {
		text   string
		minDur time.Duration
		maxDur time.Duration
	}{
		{"Hello", 100 * time.Millisecond, 500 * time.Millisecond},
		{"Hello world this is a test", 500 * time.Millisecond, 1500 * time.Millisecond},
		{"", 100 * time.Millisecond, 100 * time.Millisecond}, // minimum duration
	}

	for _, tt := range tests {
		duration := EstimateTextDuration(tt.text)
		if duration < tt.minDur || duration > tt.maxDur {
			t.Errorf("text %q: expected duration between %v and %v, got %v",
				tt.text, tt.minDur, tt.maxDur, duration)
		}
	}
}
