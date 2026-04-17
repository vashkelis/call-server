package events

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestParseSessionStartEvent(t *testing.T) {
	// Create a session.start event
	event := &SessionStartEvent{
		BaseEvent: NewBaseEvent(EventTypeSessionStart, "session-123"),
		AudioProfile: AudioProfileConfig{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   "pcm16",
		},
		VoiceProfile: VoiceProfileConfig{
			VoiceID: "voice-123",
			Speed:   1.0,
			Pitch:   1.0,
		},
		SystemPrompt: "You are a helpful assistant.",
		ModelOptions: ModelOptionsConfig{
			ModelName:   "gpt-4",
			MaxTokens:   1024,
			Temperature: 0.7,
		},
		Providers: ProviderConfig{
			ASR: "google",
			LLM: "openai",
			TTS: "elevenlabs",
		},
		TenantID: "tenant-456",
	}

	// Marshal to JSON
	data, err := MarshalEvent(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Parse it back
	parsed, err := ParseEvent(data)
	if err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	// Verify type
	if parsed.GetType() != EventTypeSessionStart {
		t.Errorf("expected type %s, got %s", EventTypeSessionStart, parsed.GetType())
	}

	// Verify it's a SessionStartEvent
	startEvent, ok := parsed.(*SessionStartEvent)
	if !ok {
		t.Fatalf("expected *SessionStartEvent, got %T", parsed)
	}

	// Verify fields
	if startEvent.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", startEvent.SessionID)
	}
	if startEvent.AudioProfile.SampleRate != 16000 {
		t.Errorf("expected sample rate 16000, got %d", startEvent.AudioProfile.SampleRate)
	}
	if startEvent.VoiceProfile.VoiceID != "voice-123" {
		t.Errorf("expected voice ID 'voice-123', got %s", startEvent.VoiceProfile.VoiceID)
	}
	if startEvent.SystemPrompt != "You are a helpful assistant." {
		t.Errorf("expected system prompt, got %s", startEvent.SystemPrompt)
	}
	if startEvent.ModelOptions.ModelName != "gpt-4" {
		t.Errorf("expected model name 'gpt-4', got %s", startEvent.ModelOptions.ModelName)
	}
	if startEvent.Providers.ASR != "google" {
		t.Errorf("expected ASR provider 'google', got %s", startEvent.Providers.ASR)
	}
	if startEvent.TenantID != "tenant-456" {
		t.Errorf("expected tenant ID 'tenant-456', got %s", startEvent.TenantID)
	}
}

func TestParseAudioChunkEvent(t *testing.T) {
	// Create audio data
	audioData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	encodedAudio := base64.StdEncoding.EncodeToString(audioData)

	// Create event JSON manually to test parsing
	eventJSON := `{
		"type": "audio.chunk",
		"timestamp": 1234567890,
		"session_id": "session-456",
		"audio_data": "` + encodedAudio + `",
		"is_final": true
	}`

	// Parse the event
	parsed, err := ParseEvent([]byte(eventJSON))
	if err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	// Verify type
	if parsed.GetType() != EventTypeAudioChunk {
		t.Errorf("expected type %s, got %s", EventTypeAudioChunk, parsed.GetType())
	}

	// Verify it's an AudioChunkEvent
	audioEvent, ok := parsed.(*AudioChunkEvent)
	if !ok {
		t.Fatalf("expected *AudioChunkEvent, got %T", parsed)
	}

	// Verify fields
	if audioEvent.SessionID != "session-456" {
		t.Errorf("expected session ID 'session-456', got %s", audioEvent.SessionID)
	}
	if !audioEvent.IsFinal {
		t.Error("expected IsFinal to be true")
	}

	// Verify audio data decoding
	decoded, err := audioEvent.GetAudioData()
	if err != nil {
		t.Fatalf("failed to decode audio data: %v", err)
	}
	if len(decoded) != len(audioData) {
		t.Errorf("expected decoded length %d, got %d", len(audioData), len(decoded))
	}
	for i, b := range decoded {
		if b != audioData[i] {
			t.Errorf("byte %d: expected %d, got %d", i, audioData[i], b)
		}
	}
}

func TestMarshalServerEvents(t *testing.T) {
	tests := []struct {
		name  string
		event Event
		check func(t *testing.T, data []byte)
	}{
		{
			name: "SessionStartedEvent",
			event: NewSessionStartedEvent("session-123", AudioProfileConfig{
				SampleRate: 16000,
				Channels:   1,
				Encoding:   "pcm16",
			}),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeSessionStarted) {
					t.Errorf("expected type 'session.started', got %v", result["type"])
				}
				if result["session_id"] != "session-123" {
					t.Errorf("expected session_id 'session-123', got %v", result["session_id"])
				}
				profile, ok := result["audio_profile"].(map[string]interface{})
				if !ok {
					t.Fatal("expected audio_profile to be an object")
				}
				if profile["sample_rate"].(float64) != 16000 {
					t.Errorf("expected sample_rate 16000, got %v", profile["sample_rate"])
				}
			},
		},
		{
			name:  "VADEvent",
			event: NewVADEvent("session-123", "speech_start"),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeVAD) {
					t.Errorf("expected type 'vad.event', got %v", result["type"])
				}
				if result["event"] != "speech_start" {
					t.Errorf("expected event 'speech_start', got %v", result["event"])
				}
			},
		},
		{
			name:  "ASRPartialEvent",
			event: NewASRPartialEvent("session-123", "Hello world"),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeASRPartial) {
					t.Errorf("expected type 'asr.partial', got %v", result["type"])
				}
				if result["transcript"] != "Hello world" {
					t.Errorf("expected transcript 'Hello world', got %v", result["transcript"])
				}
			},
		},
		{
			name:  "ASRFinalEvent",
			event: NewASRFinalEvent("session-123", "Hello world"),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeASRFinal) {
					t.Errorf("expected type 'asr.final', got %v", result["type"])
				}
				if result["transcript"] != "Hello world" {
					t.Errorf("expected transcript 'Hello world', got %v", result["transcript"])
				}
			},
		},
		{
			name:  "LLMPartialTextEvent",
			event: NewLLMPartialTextEvent("session-123", "This is a response"),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeLLMPartialText) {
					t.Errorf("expected type 'llm.partial_text', got %v", result["type"])
				}
				if result["text"] != "This is a response" {
					t.Errorf("expected text 'This is a response', got %v", result["text"])
				}
			},
		},
		{
			name:  "TTSAudioChunkEvent",
			event: NewTTSAudioChunkEvent("session-123", []byte{0x01, 0x02, 0x03}, 0, false),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeTTSAudioChunk) {
					t.Errorf("expected type 'tts.audio_chunk', got %v", result["type"])
				}
				if result["segment_index"].(float64) != 0 {
					t.Errorf("expected segment_index 0, got %v", result["segment_index"])
				}
				if result["is_final"].(bool) != false {
					t.Errorf("expected is_final false, got %v", result["is_final"])
				}
				// Verify audio data is base64 encoded
				if result["audio_data"] == "" {
					t.Error("expected audio_data to be set")
				}
			},
		},
		{
			name:  "TurnEvent",
			event: NewTurnEvent("session-123", "assistant", "started"),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeTurn) {
					t.Errorf("expected type 'turn.event', got %v", result["type"])
				}
				if result["turn_type"] != "assistant" {
					t.Errorf("expected turn_type 'assistant', got %v", result["turn_type"])
				}
				if result["event"] != "started" {
					t.Errorf("expected event 'started', got %v", result["event"])
				}
			},
		},
		{
			name:  "InterruptionEvent",
			event: NewInterruptionEvent("session-123", "user_barge_in"),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeInterruption) {
					t.Errorf("expected type 'interruption.event', got %v", result["type"])
				}
				if result["reason"] != "user_barge_in" {
					t.Errorf("expected reason 'user_barge_in', got %v", result["reason"])
				}
			},
		},
		{
			name:  "ErrorEvent",
			event: NewErrorEvent("session-123", "INVALID_REQUEST", "Invalid audio format"),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeError) {
					t.Errorf("expected type 'error', got %v", result["type"])
				}
				if result["code"] != "INVALID_REQUEST" {
					t.Errorf("expected code 'INVALID_REQUEST', got %v", result["code"])
				}
				if result["message"] != "Invalid audio format" {
					t.Errorf("expected message 'Invalid audio format', got %v", result["message"])
				}
			},
		},
		{
			name:  "SessionEndedEvent",
			event: NewSessionEndedEvent("session-123", "user_disconnected"),
			check: func(t *testing.T, data []byte) {
				var result map[string]interface{}
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if result["type"] != string(EventTypeSessionEnded) {
					t.Errorf("expected type 'session.ended', got %v", result["type"])
				}
				if result["reason"] != "user_disconnected" {
					t.Errorf("expected reason 'user_disconnected', got %v", result["reason"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := MarshalEvent(tt.event)
			if err != nil {
				t.Fatalf("failed to marshal event: %v", err)
			}
			tt.check(t, data)
		})
	}
}

func TestUnknownEventType(t *testing.T) {
	// Create JSON with unknown event type
	unknownJSON := `{
		"type": "unknown.event.type",
		"timestamp": 1234567890,
		"session_id": "session-123"
	}`

	_, err := ParseEvent([]byte(unknownJSON))
	if err == nil {
		t.Error("expected error for unknown event type")
	}
}

func TestInvalidJSON(t *testing.T) {
	// Invalid JSON
	invalidJSON := `{invalid json`

	_, err := ParseEvent([]byte(invalidJSON))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestBaseEventMethods(t *testing.T) {
	event := NewBaseEvent(EventTypeSessionStart, "session-123")

	if event.GetType() != EventTypeSessionStart {
		t.Errorf("expected type %s, got %s", EventTypeSessionStart, event.GetType())
	}
	if event.GetSessionID() != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", event.GetSessionID())
	}
	if event.GetTimestamp() == 0 {
		t.Error("expected non-zero timestamp")
	}
}

func TestNow(t *testing.T) {
	timestamp := Now()
	if timestamp == 0 {
		t.Error("expected non-zero timestamp from Now()")
	}

	// Should be a reasonable Unix timestamp (after 2020)
	if timestamp < 1577836800000 { // Jan 1, 2020
		t.Error("expected timestamp to be after 2020")
	}
}

func TestNewSessionStartEvent(t *testing.T) {
	event := NewSessionStartEvent("session-123")

	if event.Type != EventTypeSessionStart {
		t.Errorf("expected type %s, got %s", EventTypeSessionStart, event.Type)
	}
	if event.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", event.SessionID)
	}
	if event.Timestamp == 0 {
		t.Error("expected non-zero timestamp")
	}
}

func TestNewAudioChunkEvent(t *testing.T) {
	audioData := []byte{0x01, 0x02, 0x03}
	event := NewAudioChunkEvent("session-123", audioData)

	if event.Type != EventTypeAudioChunk {
		t.Errorf("expected type %s, got %s", EventTypeAudioChunk, event.Type)
	}

	// Verify audio data is base64 encoded
	decoded, err := event.GetAudioData()
	if err != nil {
		t.Fatalf("failed to decode audio data: %v", err)
	}
	if len(decoded) != len(audioData) {
		t.Errorf("expected decoded length %d, got %d", len(audioData), len(decoded))
	}
}

func TestNewSessionUpdateEvent(t *testing.T) {
	event := NewSessionUpdateEvent("session-123")

	if event.Type != EventTypeSessionUpdate {
		t.Errorf("expected type %s, got %s", EventTypeSessionUpdate, event.Type)
	}
	if event.SessionID != "session-123" {
		t.Errorf("expected session ID 'session-123', got %s", event.SessionID)
	}
}

func TestNewSessionInterruptEvent(t *testing.T) {
	event := NewSessionInterruptEvent("session-123")

	if event.Type != EventTypeSessionInterrupt {
		t.Errorf("expected type %s, got %s", EventTypeSessionInterrupt, event.Type)
	}
}

func TestNewSessionStopEvent(t *testing.T) {
	event := NewSessionStopEvent("session-123")

	if event.Type != EventTypeSessionStop {
		t.Errorf("expected type %s, got %s", EventTypeSessionStop, event.Type)
	}
}

func TestMustMarshalEvent(t *testing.T) {
	event := NewSessionStartEvent("session-123")

	// Should not panic
	data := MustMarshalEvent(event)
	if len(data) == 0 {
		t.Error("expected non-empty data")
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("expected valid JSON, got error: %v", err)
	}
}

func TestParseAllClientEvents(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected EventType
	}{
		{
			name:     "session.start",
			json:     `{"type":"session.start","timestamp":1234567890,"session_id":"sess-1"}`,
			expected: EventTypeSessionStart,
		},
		{
			name:     "audio.chunk",
			json:     `{"type":"audio.chunk","timestamp":1234567890,"session_id":"sess-1","audio_data":"AQIDBA=="}`,
			expected: EventTypeAudioChunk,
		},
		{
			name:     "session.update",
			json:     `{"type":"session.update","timestamp":1234567890,"session_id":"sess-1"}`,
			expected: EventTypeSessionUpdate,
		},
		{
			name:     "session.interrupt",
			json:     `{"type":"session.interrupt","timestamp":1234567890,"session_id":"sess-1"}`,
			expected: EventTypeSessionInterrupt,
		},
		{
			name:     "session.stop",
			json:     `{"type":"session.stop","timestamp":1234567890,"session_id":"sess-1"}`,
			expected: EventTypeSessionStop,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseEvent([]byte(tt.json))
			if err != nil {
				t.Fatalf("failed to parse event: %v", err)
			}
			if event.GetType() != tt.expected {
				t.Errorf("expected type %s, got %s", tt.expected, event.GetType())
			}
		})
	}
}

func TestParseAllServerEvents(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected EventType
	}{
		{
			name:     "session.started",
			json:     `{"type":"session.started","timestamp":1234567890,"session_id":"sess-1"}`,
			expected: EventTypeSessionStarted,
		},
		{
			name:     "vad.event",
			json:     `{"type":"vad.event","timestamp":1234567890,"session_id":"sess-1","event":"speech_start"}`,
			expected: EventTypeVAD,
		},
		{
			name:     "asr.partial",
			json:     `{"type":"asr.partial","timestamp":1234567890,"session_id":"sess-1","transcript":"hello"}`,
			expected: EventTypeASRPartial,
		},
		{
			name:     "asr.final",
			json:     `{"type":"asr.final","timestamp":1234567890,"session_id":"sess-1","transcript":"hello world"}`,
			expected: EventTypeASRFinal,
		},
		{
			name:     "llm.partial_text",
			json:     `{"type":"llm.partial_text","timestamp":1234567890,"session_id":"sess-1","text":"response"}`,
			expected: EventTypeLLMPartialText,
		},
		{
			name:     "tts.audio_chunk",
			json:     `{"type":"tts.audio_chunk","timestamp":1234567890,"session_id":"sess-1","audio_data":"","segment_index":0,"is_final":true}`,
			expected: EventTypeTTSAudioChunk,
		},
		{
			name:     "turn.event",
			json:     `{"type":"turn.event","timestamp":1234567890,"session_id":"sess-1","turn_type":"assistant","event":"started"}`,
			expected: EventTypeTurn,
		},
		{
			name:     "interruption.event",
			json:     `{"type":"interruption.event","timestamp":1234567890,"session_id":"sess-1","reason":"barge_in"}`,
			expected: EventTypeInterruption,
		},
		{
			name:     "error",
			json:     `{"type":"error","timestamp":1234567890,"session_id":"sess-1","code":"ERROR","message":"test"}`,
			expected: EventTypeError,
		},
		{
			name:     "session.ended",
			json:     `{"type":"session.ended","timestamp":1234567890,"session_id":"sess-1"}`,
			expected: EventTypeSessionEnded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseEvent([]byte(tt.json))
			if err != nil {
				t.Fatalf("failed to parse event: %v", err)
			}
			if event.GetType() != tt.expected {
				t.Errorf("expected type %s, got %s", tt.expected, event.GetType())
			}
		})
	}
}
