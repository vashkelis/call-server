package contracts

// ChatMessage represents a message in an LLM conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// UsageMetadata contains token usage information.
type UsageMetadata struct {
	PromptTokens     int32 `json:"prompt_tokens"`
	CompletionTokens int32 `json:"completion_tokens"`
	TotalTokens      int32 `json:"total_tokens"`
}

// LLMRequest contains conversation messages for generation.
type LLMRequest struct {
	SessionContext  *SessionContext   `json:"session_context"`
	Messages        []ChatMessage     `json:"messages"`
	MaxTokens       int32             `json:"max_tokens"`
	Temperature     float32           `json:"temperature"`
	TopP            float32           `json:"top_p"`
	StopSequences   []string          `json:"stop_sequences,omitempty"`
	ProviderOptions map[string]string `json:"provider_options,omitempty"`
}

// LLMResponse contains generated tokens from the LLM.
type LLMResponse struct {
	SessionContext *SessionContext `json:"session_context"`
	Token          string          `json:"token"`
	IsFinal        bool            `json:"is_final"`
	FinishReason   string          `json:"finish_reason"`
	Usage          *UsageMetadata  `json:"usage,omitempty"`
	Timing         *TimingMetadata `json:"timing,omitempty"`
}
