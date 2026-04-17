package contracts

// TTSRequest contains text to synthesize.
type TTSRequest struct {
	SessionContext  *SessionContext   `json:"session_context"`
	Text            string            `json:"text"`
	VoiceID         string            `json:"voice_id"`
	AudioFormat     *AudioFormat      `json:"audio_format"`
	SegmentIndex    int32             `json:"segment_index"`
	ProviderOptions map[string]string `json:"provider_options,omitempty"`
}

// TTSResponse contains audio chunks from text-to-speech.
type TTSResponse struct {
	SessionContext *SessionContext `json:"session_context"`
	AudioChunk     []byte          `json:"audio_chunk"`
	AudioFormat    *AudioFormat    `json:"audio_format"`
	SegmentIndex   int32           `json:"segment_index"`
	IsFinal        bool            `json:"is_final"`
	Timing         *TimingMetadata `json:"timing,omitempty"`
}
