// Package session provides session management, state tracking, and storage abstractions.
package session

import (
	"sync"
	"time"

	"github.com/parlona/cloudapp/pkg/contracts"
)

// TransportType represents the transport mechanism for a session.
type TransportType int

const (
	TransportTypeWebSocket TransportType = iota
	TransportTypeSIP
	TransportTypeWebRTC
)

// String returns the string representation of the transport type.
func (t TransportType) String() string {
	switch t {
	case TransportTypeWebSocket:
		return "websocket"
	case TransportTypeSIP:
		return "sip"
	case TransportTypeWebRTC:
		return "webrtc"
	default:
		return "unknown"
	}
}

// SelectedProviders holds the provider names selected for a session.
type SelectedProviders struct {
	ASR string `json:"asr"`
	LLM string `json:"llm"`
	TTS string `json:"tts"`
	VAD string `json:"vad,omitempty"`
}

// VoiceProfile contains voice-related configuration for TTS.
type VoiceProfile struct {
	VoiceID string            `json:"voice_id"`
	Speed   float32           `json:"speed"`
	Pitch   float32           `json:"pitch"`
	Options map[string]string `json:"options,omitempty"`
}

// ModelOptions contains LLM model configuration.
type ModelOptions struct {
	ModelName       string            `json:"model_name"`
	MaxTokens       int32             `json:"max_tokens"`
	Temperature     float32           `json:"temperature"`
	TopP            float32           `json:"top_p"`
	StopSequences   []string          `json:"stop_sequences,omitempty"`
	SystemPrompt    string            `json:"system_prompt"`
	ProviderOptions map[string]string `json:"provider_options,omitempty"`
}

// Session represents a voice session with all its state.
type Session struct {
	mu sync.RWMutex

	SessionID         string                `json:"session_id"`
	TenantID          *string               `json:"tenant_id,omitempty"`
	TransportType     TransportType         `json:"transport_type"`
	SelectedProviders SelectedProviders     `json:"selected_providers"`
	AudioProfile      contracts.AudioFormat `json:"audio_profile"`
	VoiceProfile      VoiceProfile          `json:"voice_profile"`
	SystemPrompt      string                `json:"system_prompt"`
	ModelOptions      ModelOptions          `json:"model_options"`

	// Runtime state
	ActiveTurn   *AssistantTurn `json:"active_turn,omitempty"`
	BotSpeaking  bool           `json:"bot_speaking"`
	Interrupted  bool           `json:"interrupted"`
	CurrentState SessionState   `json:"current_state"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TraceID   string    `json:"trace_id"`
}

// NewSession creates a new session with the given parameters.
func NewSession(sessionID, traceID string, transport TransportType) *Session {
	now := time.Now().UTC()
	return &Session{
		SessionID:     sessionID,
		TransportType: transport,
		CurrentState:  StateIdle,
		CreatedAt:     now,
		UpdatedAt:     now,
		TraceID:       traceID,
	}
}

// SetTenantID sets the tenant ID for the session.
func (s *Session) SetTenantID(tenantID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TenantID = &tenantID
	s.UpdatedAt = time.Now().UTC()
}

// SetProviders sets the selected providers for the session.
func (s *Session) SetProviders(providers SelectedProviders) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SelectedProviders = providers
	s.UpdatedAt = time.Now().UTC()
}

// SetAudioProfile sets the audio profile for the session.
func (s *Session) SetAudioProfile(profile contracts.AudioFormat) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AudioProfile = profile
	s.UpdatedAt = time.Now().UTC()
}

// SetVoiceProfile sets the voice profile for the session.
func (s *Session) SetVoiceProfile(profile VoiceProfile) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.VoiceProfile = profile
	s.UpdatedAt = time.Now().UTC()
}

// SetModelOptions sets the model options for the session.
func (s *Session) SetModelOptions(options ModelOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ModelOptions = options
	s.SystemPrompt = options.SystemPrompt
	s.UpdatedAt = time.Now().UTC()
}

// SetActiveTurn sets the active assistant turn.
func (s *Session) SetActiveTurn(turn *AssistantTurn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveTurn = turn
	s.UpdatedAt = time.Now().UTC()
}

// GetActiveTurn returns the active assistant turn (thread-safe copy).
func (s *Session) GetActiveTurn() *AssistantTurn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ActiveTurn == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	return s.ActiveTurn.Clone()
}

// SetBotSpeaking sets the bot speaking flag.
func (s *Session) SetBotSpeaking(speaking bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BotSpeaking = speaking
	s.UpdatedAt = time.Now().UTC()
}

// IsBotSpeaking returns whether the bot is currently speaking.
func (s *Session) IsBotSpeaking() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.BotSpeaking
}

// SetInterrupted sets the interrupted flag.
func (s *Session) SetInterrupted(interrupted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Interrupted = interrupted
	s.UpdatedAt = time.Now().UTC()
}

// IsInterrupted returns whether the session has been interrupted.
func (s *Session) IsInterrupted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Interrupted
}

// SetState sets the current session state with validation.
func (s *Session) SetState(newState SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !IsValidTransition(s.CurrentState, newState) {
		return ErrInvalidStateTransition
	}

	s.CurrentState = newState
	s.UpdatedAt = time.Now().UTC()
	return nil
}

// GetState returns the current session state.
func (s *Session) GetState() SessionState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentState
}

// Clone creates a deep copy of the session.
func (s *Session) Clone() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := &Session{
		SessionID:         s.SessionID,
		TransportType:     s.TransportType,
		SelectedProviders: s.SelectedProviders,
		AudioProfile:      s.AudioProfile,
		VoiceProfile:      s.VoiceProfile,
		SystemPrompt:      s.SystemPrompt,
		ModelOptions:      s.ModelOptions,
		BotSpeaking:       s.BotSpeaking,
		Interrupted:       s.Interrupted,
		CurrentState:      s.CurrentState,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
		TraceID:           s.TraceID,
	}

	if s.TenantID != nil {
		tenantID := *s.TenantID
		clone.TenantID = &tenantID
	}

	if s.ActiveTurn != nil {
		clone.ActiveTurn = s.ActiveTurn.Clone()
	}

	return clone
}

// Touch updates the UpdatedAt timestamp.
func (s *Session) Touch() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpdatedAt = time.Now().UTC()
}
