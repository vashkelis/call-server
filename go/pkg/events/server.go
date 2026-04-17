package events

import (
	"github.com/parlona/cloudapp/pkg/contracts"
)

// SessionStartedEvent is sent by the server when a session is started.
type SessionStartedEvent struct {
	BaseEvent
	SessionID    string             `json:"session_id"`
	AudioProfile AudioProfileConfig `json:"audio_profile"`
	ServerTime   int64              `json:"server_time"`
}

// NewSessionStartedEvent creates a new session started event.
func NewSessionStartedEvent(sessionID string, profile AudioProfileConfig) *SessionStartedEvent {
	return &SessionStartedEvent{
		BaseEvent:    NewBaseEvent(EventTypeSessionStarted, sessionID),
		SessionID:    sessionID,
		AudioProfile: profile,
		ServerTime:   Now(),
	}
}

// VADEvent is sent by the server for voice activity detection events.
type VADEvent struct {
	BaseEvent
	Event      string  `json:"event"` // "speech_start", "speech_end"
	Confidence float32 `json:"confidence,omitempty"`
	Position   int64   `json:"position,omitempty"` // milliseconds
}

// NewVADEvent creates a new VAD event.
func NewVADEvent(sessionID, event string) *VADEvent {
	return &VADEvent{
		BaseEvent: NewBaseEvent(EventTypeVAD, sessionID),
		Event:     event,
	}
}

// ASRPartialEvent is sent by the server for partial ASR transcripts.
type ASRPartialEvent struct {
	BaseEvent
	Transcript string  `json:"transcript"`
	Language   string  `json:"language,omitempty"`
	Confidence float32 `json:"confidence,omitempty"`
}

// NewASRPartialEvent creates a new ASR partial event.
func NewASRPartialEvent(sessionID, transcript string) *ASRPartialEvent {
	return &ASRPartialEvent{
		BaseEvent:  NewBaseEvent(EventTypeASRPartial, sessionID),
		Transcript: transcript,
	}
}

// ASRFinalEvent is sent by the server for final ASR transcripts.
type ASRFinalEvent struct {
	BaseEvent
	Transcript     string                    `json:"transcript"`
	Language       string                    `json:"language,omitempty"`
	Confidence     float32                   `json:"confidence,omitempty"`
	WordTimestamps []contracts.WordTimestamp `json:"word_timestamps,omitempty"`
}

// NewASRFinalEvent creates a new ASR final event.
func NewASRFinalEvent(sessionID, transcript string) *ASRFinalEvent {
	return &ASRFinalEvent{
		BaseEvent:  NewBaseEvent(EventTypeASRFinal, sessionID),
		Transcript: transcript,
	}
}

// LLMPartialTextEvent is sent by the server for partial LLM output.
type LLMPartialTextEvent struct {
	BaseEvent
	Text       string `json:"text"`
	IsComplete bool   `json:"is_complete,omitempty"`
}

// NewLLMPartialTextEvent creates a new LLM partial text event.
func NewLLMPartialTextEvent(sessionID, text string) *LLMPartialTextEvent {
	return &LLMPartialTextEvent{
		BaseEvent: NewBaseEvent(EventTypeLLMPartialText, sessionID),
		Text:      text,
	}
}

// TTSAudioChunkEvent is sent by the server with synthesized audio.
type TTSAudioChunkEvent struct {
	BaseEvent
	AudioData    string `json:"audio_data"` // base64 encoded
	SegmentIndex int32  `json:"segment_index"`
	IsFinal      bool   `json:"is_final"`
}

// NewTTSAudioChunkEvent creates a new TTS audio chunk event.
func NewTTSAudioChunkEvent(sessionID string, audioData []byte, segmentIndex int32, isFinal bool) *TTSAudioChunkEvent {
	return &TTSAudioChunkEvent{
		BaseEvent:    NewBaseEvent(EventTypeTTSAudioChunk, sessionID),
		AudioData:    encodeAudio(audioData),
		SegmentIndex: segmentIndex,
		IsFinal:      isFinal,
	}
}

// GetAudioData returns the decoded audio data.
func (e *TTSAudioChunkEvent) GetAudioData() ([]byte, error) {
	return decodeAudio(e.AudioData)
}

// TurnEvent is sent by the server to indicate turn state changes.
type TurnEvent struct {
	BaseEvent
	TurnType     string `json:"turn_type"` // "user", "assistant"
	Event        string `json:"event"`     // "started", "completed", "cancelled"
	Text         string `json:"text,omitempty"`
	GenerationID string `json:"generation_id,omitempty"`
}

// NewTurnEvent creates a new turn event.
func NewTurnEvent(sessionID, turnType, event string) *TurnEvent {
	return &TurnEvent{
		BaseEvent: NewBaseEvent(EventTypeTurn, sessionID),
		TurnType:  turnType,
		Event:     event,
	}
}

// InterruptionEvent is sent by the server when an interruption occurs.
type InterruptionEvent struct {
	BaseEvent
	Reason          string `json:"reason"`
	SpokenText      string `json:"spoken_text,omitempty"`
	UnspokenText    string `json:"unspoken_text,omitempty"`
	PlayoutPosition int64  `json:"playout_position_ms,omitempty"`
}

// NewInterruptionEvent creates a new interruption event.
func NewInterruptionEvent(sessionID, reason string) *InterruptionEvent {
	return &InterruptionEvent{
		BaseEvent: NewBaseEvent(EventTypeInterruption, sessionID),
		Reason:    reason,
	}
}

// ErrorEvent is sent by the server when an error occurs.
type ErrorEvent struct {
	BaseEvent
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// NewErrorEvent creates a new error event.
func NewErrorEvent(sessionID, code, message string) *ErrorEvent {
	return &ErrorEvent{
		BaseEvent: NewBaseEvent(EventTypeError, sessionID),
		Code:      code,
		Message:   message,
	}
}

// SessionEndedEvent is sent by the server when a session ends.
type SessionEndedEvent struct {
	BaseEvent
	Reason   string `json:"reason,omitempty"`
	Duration int64  `json:"duration_ms,omitempty"`
}

// NewSessionEndedEvent creates a new session ended event.
func NewSessionEndedEvent(sessionID, reason string) *SessionEndedEvent {
	return &SessionEndedEvent{
		BaseEvent: NewBaseEvent(EventTypeSessionEnded, sessionID),
		Reason:    reason,
	}
}
