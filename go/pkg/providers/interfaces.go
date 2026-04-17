// Package providers defines the provider interfaces and registry for ASR, LLM, and TTS services.
package providers

import (
	"context"

	"github.com/parlona/cloudapp/pkg/contracts"
)

// ASRResult represents a result from ASR streaming recognition.
type ASRResult struct {
	Transcript     string
	IsPartial      bool
	IsFinal        bool
	Confidence     float32
	Language       string
	WordTimestamps []contracts.WordTimestamp
	Error          error
}

// ASRProvider defines the interface for Automatic Speech Recognition providers.
type ASRProvider interface {
	// StreamRecognize performs streaming speech recognition.
	// The audioStream channel receives audio chunks, and results are sent on the returned channel.
	StreamRecognize(ctx context.Context, audioStream chan []byte, opts ASROptions) (chan ASRResult, error)

	// Cancel cancels an ongoing recognition for the given session.
	Cancel(ctx context.Context, sessionID string) error

	// Capabilities returns the provider's capabilities.
	Capabilities() contracts.ProviderCapability

	// Name returns the provider name.
	Name() string
}

// LLMToken represents a token from LLM streaming generation.
type LLMToken struct {
	Token        string
	IsFinal      bool
	FinishReason string
	Usage        *contracts.UsageMetadata
	Error        error
}

// LLMProvider defines the interface for Language Model providers.
type LLMProvider interface {
	// StreamGenerate performs streaming text generation.
	// Results are sent on the returned channel.
	StreamGenerate(ctx context.Context, messages []contracts.ChatMessage, opts LLMOptions) (chan LLMToken, error)

	// Cancel cancels an ongoing generation for the given session.
	Cancel(ctx context.Context, sessionID string) error

	// Capabilities returns the provider's capabilities.
	Capabilities() contracts.ProviderCapability

	// Name returns the provider name.
	Name() string
}

// TTSProvider defines the interface for Text-to-Speech providers.
type TTSProvider interface {
	// StreamSynthesize performs streaming text-to-speech synthesis.
	// Audio chunks are sent on the returned channel.
	StreamSynthesize(ctx context.Context, text string, opts TTSOptions) (chan []byte, error)

	// Cancel cancels an ongoing synthesis for the given session.
	Cancel(ctx context.Context, sessionID string) error

	// Capabilities returns the provider's capabilities.
	Capabilities() contracts.ProviderCapability

	// Name returns the provider name.
	Name() string
}

// Provider is the common interface for all provider types.
type Provider interface {
	// Capabilities returns the provider's capabilities.
	Capabilities() contracts.ProviderCapability

	// Name returns the provider name.
	Name() string
}

// VADProvider defines the interface for Voice Activity Detection providers.
type VADProvider interface {
	// ProcessAudio processes audio and returns voice activity events.
	ProcessAudio(ctx context.Context, audio []byte) (VADResult, error)

	// Reset resets the VAD state for a new session.
	Reset(sessionID string)

	// Name returns the provider name.
	Name() string
}

// VADResult represents the result of VAD processing.
type VADResult struct {
	IsSpeech    bool
	Confidence  float32
	SpeechStart bool
	SpeechEnd   bool
	Position    int64 // milliseconds
}
