package events

// SessionStartEvent is sent by the client to start a new session.
type SessionStartEvent struct {
	BaseEvent
	AudioProfile AudioProfileConfig `json:"audio_profile"`
	VoiceProfile VoiceProfileConfig `json:"voice_profile,omitempty"`
	SystemPrompt string             `json:"system_prompt,omitempty"`
	ModelOptions ModelOptionsConfig `json:"model_options,omitempty"`
	Providers    ProviderConfig     `json:"providers,omitempty"`
	TenantID     string             `json:"tenant_id,omitempty"`
}

// NewSessionStartEvent creates a new session start event.
func NewSessionStartEvent(sessionID string) *SessionStartEvent {
	return &SessionStartEvent{
		BaseEvent: NewBaseEvent(EventTypeSessionStart, sessionID),
	}
}

// AudioProfileConfig contains audio configuration.
type AudioProfileConfig struct {
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
	Encoding   string `json:"encoding"`
}

// VoiceProfileConfig contains voice configuration.
type VoiceProfileConfig struct {
	VoiceID string  `json:"voice_id"`
	Speed   float32 `json:"speed,omitempty"`
	Pitch   float32 `json:"pitch,omitempty"`
}

// ModelOptionsConfig contains model configuration.
type ModelOptionsConfig struct {
	ModelName     string   `json:"model_name,omitempty"`
	MaxTokens     int32    `json:"max_tokens,omitempty"`
	Temperature   float32  `json:"temperature,omitempty"`
	TopP          float32  `json:"top_p,omitempty"`
	StopSequences []string `json:"stop_sequences,omitempty"`
}

// ProviderConfig contains provider selection.
type ProviderConfig struct {
	ASR string `json:"asr,omitempty"`
	LLM string `json:"llm,omitempty"`
	TTS string `json:"tts,omitempty"`
	VAD string `json:"vad,omitempty"`
}

// AudioChunkEvent is sent by the client with audio data.
type AudioChunkEvent struct {
	BaseEvent
	AudioData string `json:"audio_data"` // base64 encoded
	IsFinal   bool   `json:"is_final,omitempty"`
}

// NewAudioChunkEvent creates a new audio chunk event.
func NewAudioChunkEvent(sessionID string, audioData []byte) *AudioChunkEvent {
	return &AudioChunkEvent{
		BaseEvent: NewBaseEvent(EventTypeAudioChunk, sessionID),
		AudioData: encodeAudio(audioData),
	}
}

// GetAudioData returns the decoded audio data.
func (e *AudioChunkEvent) GetAudioData() ([]byte, error) {
	return decodeAudio(e.AudioData)
}

// SessionUpdateEvent is sent by the client to update session configuration.
type SessionUpdateEvent struct {
	BaseEvent
	SystemPrompt string              `json:"system_prompt,omitempty"`
	VoiceProfile *VoiceProfileConfig `json:"voice_profile,omitempty"`
	ModelOptions *ModelOptionsConfig `json:"model_options,omitempty"`
	Providers    *ProviderConfig     `json:"providers,omitempty"`
}

// NewSessionUpdateEvent creates a new session update event.
func NewSessionUpdateEvent(sessionID string) *SessionUpdateEvent {
	return &SessionUpdateEvent{
		BaseEvent: NewBaseEvent(EventTypeSessionUpdate, sessionID),
	}
}

// SessionInterruptEvent is sent by the client to interrupt the current turn.
type SessionInterruptEvent struct {
	BaseEvent
	Reason string `json:"reason,omitempty"`
}

// NewSessionInterruptEvent creates a new session interrupt event.
func NewSessionInterruptEvent(sessionID string) *SessionInterruptEvent {
	return &SessionInterruptEvent{
		BaseEvent: NewBaseEvent(EventTypeSessionInterrupt, sessionID),
	}
}

// SessionStopEvent is sent by the client to end the session.
type SessionStopEvent struct {
	BaseEvent
	Reason string `json:"reason,omitempty"`
}

// NewSessionStopEvent creates a new session stop event.
func NewSessionStopEvent(sessionID string) *SessionStopEvent {
	return &SessionStopEvent{
		BaseEvent: NewBaseEvent(EventTypeSessionStop, sessionID),
	}
}
