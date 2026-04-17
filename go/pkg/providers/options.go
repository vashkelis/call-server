package providers

import (
	"github.com/parlona/cloudapp/pkg/contracts"
)

// ASROptions contains options for ASR recognition.
type ASROptions struct {
	SessionID        string
	LanguageHint     string
	EnableTimestamps bool
	AudioFormat      contracts.AudioFormat
	ProviderOptions  map[string]string
}

// NewASROptions creates default ASR options.
func NewASROptions(sessionID string) ASROptions {
	return ASROptions{
		SessionID: sessionID,
		AudioFormat: contracts.AudioFormat{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   contracts.PCM16,
		},
		ProviderOptions: make(map[string]string),
	}
}

// WithLanguageHint sets the language hint.
func (o ASROptions) WithLanguageHint(lang string) ASROptions {
	o.LanguageHint = lang
	return o
}

// WithTimestamps enables word timestamps.
func (o ASROptions) WithTimestamps(enable bool) ASROptions {
	o.EnableTimestamps = enable
	return o
}

// WithAudioFormat sets the audio format.
func (o ASROptions) WithAudioFormat(format contracts.AudioFormat) ASROptions {
	o.AudioFormat = format
	return o
}

// WithProviderOption sets a provider-specific option.
func (o ASROptions) WithProviderOption(key, value string) ASROptions {
	if o.ProviderOptions == nil {
		o.ProviderOptions = make(map[string]string)
	}
	o.ProviderOptions[key] = value
	return o
}

// LLMOptions contains options for LLM generation.
type LLMOptions struct {
	SessionID       string
	ModelName       string
	MaxTokens       int32
	Temperature     float32
	TopP            float32
	StopSequences   []string
	SystemPrompt    string
	ProviderOptions map[string]string
}

// NewLLMOptions creates default LLM options.
func NewLLMOptions(sessionID string) LLMOptions {
	return LLMOptions{
		SessionID:       sessionID,
		MaxTokens:       1024,
		Temperature:     0.7,
		TopP:            1.0,
		ProviderOptions: make(map[string]string),
	}
}

// WithModel sets the model name.
func (o LLMOptions) WithModel(model string) LLMOptions {
	o.ModelName = model
	return o
}

// WithMaxTokens sets the maximum tokens.
func (o LLMOptions) WithMaxTokens(tokens int32) LLMOptions {
	o.MaxTokens = tokens
	return o
}

// WithTemperature sets the temperature.
func (o LLMOptions) WithTemperature(temp float32) LLMOptions {
	o.Temperature = temp
	return o
}

// WithTopP sets the top-p value.
func (o LLMOptions) WithTopP(topP float32) LLMOptions {
	o.TopP = topP
	return o
}

// WithStopSequences sets the stop sequences.
func (o LLMOptions) WithStopSequences(sequences []string) LLMOptions {
	o.StopSequences = sequences
	return o
}

// WithSystemPrompt sets the system prompt.
func (o LLMOptions) WithSystemPrompt(prompt string) LLMOptions {
	o.SystemPrompt = prompt
	return o
}

// WithProviderOption sets a provider-specific option.
func (o LLMOptions) WithProviderOption(key, value string) LLMOptions {
	if o.ProviderOptions == nil {
		o.ProviderOptions = make(map[string]string)
	}
	o.ProviderOptions[key] = value
	return o
}

// TTSOptions contains options for TTS synthesis.
type TTSOptions struct {
	SessionID       string
	VoiceID         string
	Speed           float32
	Pitch           float32
	AudioFormat     contracts.AudioFormat
	SegmentIndex    int32
	ProviderOptions map[string]string
}

// NewTTSOptions creates default TTS options.
func NewTTSOptions(sessionID string) TTSOptions {
	return TTSOptions{
		SessionID: sessionID,
		Speed:     1.0,
		Pitch:     1.0,
		AudioFormat: contracts.AudioFormat{
			SampleRate: 16000,
			Channels:   1,
			Encoding:   contracts.PCM16,
		},
		ProviderOptions: make(map[string]string),
	}
}

// WithVoiceID sets the voice ID.
func (o TTSOptions) WithVoiceID(voiceID string) TTSOptions {
	o.VoiceID = voiceID
	return o
}

// WithSpeed sets the speech speed.
func (o TTSOptions) WithSpeed(speed float32) TTSOptions {
	o.Speed = speed
	return o
}

// WithPitch sets the voice pitch.
func (o TTSOptions) WithPitch(pitch float32) TTSOptions {
	o.Pitch = pitch
	return o
}

// WithAudioFormat sets the audio format.
func (o TTSOptions) WithAudioFormat(format contracts.AudioFormat) TTSOptions {
	o.AudioFormat = format
	return o
}

// WithSegmentIndex sets the segment index.
func (o TTSOptions) WithSegmentIndex(index int32) TTSOptions {
	o.SegmentIndex = index
	return o
}

// WithProviderOption sets a provider-specific option.
func (o TTSOptions) WithProviderOption(key, value string) TTSOptions {
	if o.ProviderOptions == nil {
		o.ProviderOptions = make(map[string]string)
	}
	o.ProviderOptions[key] = value
	return o
}
