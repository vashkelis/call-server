package statemachine

import (
	"sync"
	"time"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/session"
)

// TurnManager manages assistant turns within a session.
type TurnManager struct {
	mu          sync.RWMutex
	sessionID   string
	currentTurn *session.AssistantTurn
	history     *session.ConversationHistory
}

// NewTurnManager creates a new turn manager.
func NewTurnManager(sessionID string, history *session.ConversationHistory) *TurnManager {
	return &TurnManager{
		sessionID: sessionID,
		history:   history,
	}
}

// StartTurn starts a new assistant turn.
func (tm *TurnManager) StartTurn(generationID string, sampleRate int) *session.AssistantTurn {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.currentTurn = session.NewAssistantTurn(generationID, sampleRate)
	return tm.currentTurn
}

// AppendGenerated adds text to the generated buffer.
func (tm *TurnManager) AppendGenerated(text string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.currentTurn != nil {
		tm.currentTurn.AppendGeneratedText(text)
	}
}

// MarkQueued moves text from generated to queued_for_tts.
func (tm *TurnManager) MarkQueued(text string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.currentTurn != nil {
		tm.currentTurn.QueueForTTS(text)
	}
}

// MarkSpoken advances spoken_text based on playout progress.
// The duration parameter represents how much audio has been played.
func (tm *TurnManager) MarkSpoken(duration time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.currentTurn == nil {
		return
	}

	// Advance the playout cursor based on duration
	// Assuming 16kHz, mono, PCM16 = 2 bytes per sample
	sampleRate := 16000
	bytesPerSample := 2
	samples := int(duration.Seconds() * float64(sampleRate))
	bytes := samples * bytesPerSample

	tm.currentTurn.AdvancePlayout(bytes)
}

// AdvancePlayout advances the playout cursor by the given bytes.
func (tm *TurnManager) AdvancePlayout(bytes int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.currentTurn != nil {
		tm.currentTurn.AdvancePlayout(bytes)
	}
}

// HandleInterruption handles an interruption at the given playout position.
// Calls AssistantTurn.MarkInterrupted and trims to spoken text only.
func (tm *TurnManager) HandleInterruption(playoutPosition time.Duration) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.currentTurn == nil {
		return
	}

	// Calculate bytes for the playout position
	sampleRate := 16000
	bytesPerSample := 2
	samples := int(playoutPosition.Seconds() * float64(sampleRate))
	bytes := samples * bytesPerSample

	tm.currentTurn.MarkInterrupted(bytes)
}

// CommitTurn commits only spoken_text to conversation history.
// Returns the committed message.
func (tm *TurnManager) CommitTurn() contracts.ChatMessage {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.currentTurn == nil {
		return contracts.ChatMessage{}
	}

	// Get the committable text (only what was actually spoken)
	spokenText := tm.currentTurn.GetCommittableText()

	// Commit to history
	if spokenText != "" && tm.history != nil {
		tm.history.AppendAssistantMessage(spokenText)
	}

	// Clear the current turn
	tm.currentTurn = nil

	return contracts.ChatMessage{
		Role:    "assistant",
		Content: spokenText,
	}
}

// GetCurrentTurn returns the current assistant turn.
func (tm *TurnManager) GetCurrentTurn() *session.AssistantTurn {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.currentTurn == nil {
		return nil
	}

	return tm.currentTurn.Clone()
}

// HasActiveTurn returns true if there's an active turn.
func (tm *TurnManager) HasActiveTurn() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.currentTurn != nil
}

// GetCurrentPosition returns the current playout position.
func (tm *TurnManager) GetCurrentPosition() time.Duration {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.currentTurn == nil {
		return 0
	}

	return tm.currentTurn.CurrentPosition()
}

// GetGenerationID returns the generation ID of the current turn.
func (tm *TurnManager) GetGenerationID() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.currentTurn == nil {
		return ""
	}

	return tm.currentTurn.GetGenerationID()
}

// IsInterrupted returns true if the current turn was interrupted.
func (tm *TurnManager) IsInterrupted() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.currentTurn == nil {
		return false
	}

	return tm.currentTurn.IsInterrupted()
}

// Reset resets the turn manager.
func (tm *TurnManager) Reset() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.currentTurn = nil
}

// TurnStats contains statistics about the current turn.
type TurnStats struct {
	GenerationID    string
	GeneratedText   string
	QueuedForTTS    string
	SpokenText      string
	CurrentPosition time.Duration
	IsInterrupted   bool
}

// GetStats returns statistics about the current turn.
func (tm *TurnManager) GetStats() TurnStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if tm.currentTurn == nil {
		return TurnStats{}
	}

	return TurnStats{
		GenerationID:    tm.currentTurn.GetGenerationID(),
		GeneratedText:   tm.currentTurn.GetGeneratedText(),
		QueuedForTTS:    tm.currentTurn.GetQueuedForTTSText(),
		SpokenText:      tm.currentTurn.GetSpokenText(),
		CurrentPosition: tm.currentTurn.CurrentPosition(),
		IsInterrupted:   tm.currentTurn.IsInterrupted(),
	}
}

// TurnManagerRegistry manages turn managers for multiple sessions.
type TurnManagerRegistry struct {
	mu       sync.RWMutex
	managers map[string]*TurnManager
}

// NewTurnManagerRegistry creates a new turn manager registry.
func NewTurnManagerRegistry() *TurnManagerRegistry {
	return &TurnManagerRegistry{
		managers: make(map[string]*TurnManager),
	}
}

// GetOrCreate gets or creates a turn manager for a session.
func (r *TurnManagerRegistry) GetOrCreate(sessionID string, history *session.ConversationHistory) *TurnManager {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tm, ok := r.managers[sessionID]; ok {
		return tm
	}

	tm := NewTurnManager(sessionID, history)
	r.managers[sessionID] = tm
	return tm
}

// Get gets a turn manager for a session.
func (r *TurnManagerRegistry) Get(sessionID string) (*TurnManager, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tm, ok := r.managers[sessionID]
	return tm, ok
}

// Remove removes a turn manager.
func (r *TurnManagerRegistry) Remove(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.managers, sessionID)
}

// List returns all session IDs with turn managers.
func (r *TurnManagerRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.managers))
	for id := range r.managers {
		ids = append(ids, id)
	}
	return ids
}
