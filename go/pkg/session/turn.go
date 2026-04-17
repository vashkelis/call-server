package session

import (
	"strings"
	"sync"
	"time"
)

// AssistantTurn tracks the state of an assistant response turn.
type AssistantTurn struct {
	mu sync.RWMutex

	GenerationID     string     `json:"generation_id"`
	GeneratedText    string     `json:"generated_text"`
	QueuedForTTSText string     `json:"queued_for_tts_text"`
	SpokenText       string     `json:"spoken_text"`
	Interrupted      bool       `json:"interrupted"`
	PlayoutCursor    int        `json:"playout_cursor"` // bytes sent
	PlayoutStartTime *time.Time `json:"playout_start_time,omitempty"`

	// Internal tracking
	totalAudioBytes int
	sampleRate      int
	bytesPerSample  int
}

// NewAssistantTurn creates a new assistant turn.
func NewAssistantTurn(generationID string, sampleRate int) *AssistantTurn {
	return &AssistantTurn{
		GenerationID:   generationID,
		sampleRate:     sampleRate,
		bytesPerSample: 2, // PCM16 = 2 bytes per sample
	}
}

// AppendGeneratedText adds text to the generated buffer.
func (t *AssistantTurn) AppendGeneratedText(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.GeneratedText += text
}

// SetGeneratedText sets the entire generated text.
func (t *AssistantTurn) SetGeneratedText(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.GeneratedText = text
}

// GetGeneratedText returns the generated text.
func (t *AssistantTurn) GetGeneratedText() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.GeneratedText
}

// QueueForTTS moves text from generated to queued for TTS.
func (t *AssistantTurn) QueueForTTS(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.QueuedForTTSText += text
}

// GetQueuedForTTSText returns the text queued for TTS.
func (t *AssistantTurn) GetQueuedForTTSText() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.QueuedForTTSText
}

// CommitSpokenText marks text as spoken up to the current playout cursor.
// This trims the generated text to only what has actually been spoken.
func (t *AssistantTurn) CommitSpokenText() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Calculate how much text corresponds to the audio sent
	// This is a simplified approximation - in production, you'd use
	// word timestamps or character-to-audio mapping
	spokenDuration := t.calculateDurationFromBytes(t.PlayoutCursor)
	spokenChars := int(spokenDuration.Seconds() * 15) // ~15 chars per second

	if spokenChars > len(t.GeneratedText) {
		spokenChars = len(t.GeneratedText)
	}

	committed := t.GeneratedText[:spokenChars]
	t.SpokenText = committed

	// Trim the generated text to only what was spoken
	t.GeneratedText = t.GeneratedText[spokenChars:]
	t.QueuedForTTSText = ""

	return committed
}

// MarkInterrupted marks the turn as interrupted at the given playout position.
func (t *AssistantTurn) MarkInterrupted(playoutPosition int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Interrupted = true
	if playoutPosition < t.PlayoutCursor {
		t.PlayoutCursor = playoutPosition
	}
}

// GetCommittableText returns the text that can be committed to history.
// This is only the text that corresponds to audio that was actually sent.
func (t *AssistantTurn) GetCommittableText() string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Calculate text corresponding to playout cursor
	duration := t.calculateDurationFromBytes(t.PlayoutCursor)
	chars := int(duration.Seconds() * 15) // ~15 chars per second

	allText := t.GeneratedText
	if len(allText) > chars {
		return allText[:chars]
	}
	return allText
}

// AdvancePlayout advances the playout cursor by the given number of bytes.
func (t *AssistantTurn) AdvancePlayout(bytes int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.PlayoutStartTime == nil {
		now := time.Now().UTC()
		t.PlayoutStartTime = &now
	}

	t.PlayoutCursor += bytes
	t.totalAudioBytes += bytes
}

// GetPlayoutCursor returns the current playout cursor position in bytes.
func (t *AssistantTurn) GetPlayoutCursor() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.PlayoutCursor
}

// CurrentPosition returns the current playout position as a duration.
func (t *AssistantTurn) CurrentPosition() time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.calculateDurationFromBytes(t.PlayoutCursor)
}

// Reset resets the playout tracking.
func (t *AssistantTurn) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.PlayoutCursor = 0
	t.PlayoutStartTime = nil
}

// IsInterrupted returns whether the turn was interrupted.
func (t *AssistantTurn) IsInterrupted() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.Interrupted
}

// GetSpokenText returns the spoken text.
func (t *AssistantTurn) GetSpokenText() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.SpokenText
}

// GetGenerationID returns the generation ID.
func (t *AssistantTurn) GetGenerationID() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.GenerationID
}

// Clone creates a deep copy of the turn.
func (t *AssistantTurn) Clone() *AssistantTurn {
	t.mu.RLock()
	defer t.mu.RUnlock()

	clone := &AssistantTurn{
		GenerationID:     t.GenerationID,
		GeneratedText:    t.GeneratedText,
		QueuedForTTSText: t.QueuedForTTSText,
		SpokenText:       t.SpokenText,
		Interrupted:      t.Interrupted,
		PlayoutCursor:    t.PlayoutCursor,
		sampleRate:       t.sampleRate,
		bytesPerSample:   t.bytesPerSample,
		totalAudioBytes:  t.totalAudioBytes,
	}

	if t.PlayoutStartTime != nil {
		startTime := *t.PlayoutStartTime
		clone.PlayoutStartTime = &startTime
	}

	return clone
}

// calculateDurationFromBytes converts audio bytes to duration.
func (t *AssistantTurn) calculateDurationFromBytes(bytes int) time.Duration {
	if t.sampleRate == 0 || t.bytesPerSample == 0 {
		return 0
	}
	samples := bytes / t.bytesPerSample
	seconds := float64(samples) / float64(t.sampleRate)
	return time.Duration(seconds * float64(time.Second))
}

// EstimateTextDuration estimates the duration of spoken text.
// This is a rough approximation for text-to-audio mapping.
func EstimateTextDuration(text string) time.Duration {
	// Average speaking rate: ~150 words per minute = ~2.5 words per second
	// Average word length: ~5 characters
	// So ~12.5 characters per second
	words := len(strings.Fields(text))
	duration := time.Duration(words) * 400 * time.Millisecond // ~400ms per word
	if duration < 100*time.Millisecond {
		duration = 100 * time.Millisecond
	}
	return duration
}
