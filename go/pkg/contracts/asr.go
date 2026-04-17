package contracts

// ASRRequest contains audio chunks for speech recognition.
type ASRRequest struct {
	SessionContext *SessionContext `json:"session_context"`
	AudioChunk     []byte          `json:"audio_chunk"`
	AudioFormat    *AudioFormat    `json:"audio_format"`
	LanguageHint   string          `json:"language_hint"`
	IsFinal        bool            `json:"is_final"`
}

// WordTimestamp represents a word-level timestamp.
type WordTimestamp struct {
	Word    string `json:"word"`
	StartMs int64  `json:"start_ms"`
	EndMs   int64  `json:"end_ms"`
}

// ASRResponse contains transcripts from speech recognition.
type ASRResponse struct {
	SessionContext *SessionContext `json:"session_context"`
	Transcript     string          `json:"transcript"`
	IsPartial      bool            `json:"is_partial"`
	IsFinal        bool            `json:"is_final"`
	Confidence     float32         `json:"confidence"`
	Language       string          `json:"language"`
	WordTimestamps []WordTimestamp `json:"word_timestamps,omitempty"`
	Timing         *TimingMetadata `json:"timing,omitempty"`
}

// CapabilityRequest is used to request provider capabilities.
type CapabilityRequest struct {
	ProviderName string `json:"provider_name"`
}
