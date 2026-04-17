package statemachine

import (
	"testing"
	"time"

	"github.com/parlona/cloudapp/pkg/session"
)

func TestTurnLifecycle(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	// Start turn
	turn := tm.StartTurn("gen-123", 16000)
	if turn == nil {
		t.Fatal("expected non-nil turn")
	}
	if turn.GetGenerationID() != "gen-123" {
		t.Errorf("expected generation ID 'gen-123', got %s", turn.GetGenerationID())
	}

	if !tm.HasActiveTurn() {
		t.Error("expected HasActiveTurn to be true")
	}

	// Append generated text
	tm.AppendGenerated("Hello, ")
	tm.AppendGenerated("how can I help you?")

	currentTurn := tm.GetCurrentTurn()
	if currentTurn.GetGeneratedText() != "Hello, how can I help you?" {
		t.Errorf("expected generated text 'Hello, how can I help you?', got %s",
			currentTurn.GetGeneratedText())
	}

	// Mark queued
	tm.MarkQueued("Hello, how can I help you?")

	// Mark spoken (advance playout)
	tm.MarkSpoken(1 * time.Second) // 1 second of audio

	// Commit turn
	committed := tm.CommitTurn()

	// Verify committed message
	if committed.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %s", committed.Role)
	}

	// Verify turn is cleared
	if tm.HasActiveTurn() {
		t.Error("expected turn to be cleared after commit")
	}

	// Verify history was updated
	if history.Len() != 1 {
		t.Errorf("expected 1 message in history, got %d", history.Len())
	}

	messages := history.GetMessages()
	if messages[0].Role != "assistant" {
		t.Errorf("expected assistant message in history, got %s", messages[0].Role)
	}
}

func TestTurnInterruption(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	// Start turn and generate text
	tm.StartTurn("gen-456", 16000)

	// Generate a longer response
	fullText := "This is a long response that will be interrupted halfway through."
	tm.AppendGenerated(fullText)

	// Simulate partial playout (halfway through)
	// At 16kHz, mono, PCM16: ~15 chars per second
	// For ~65 chars, we need about 4.3 seconds of audio
	// Halfway = ~2.15 seconds
	tm.MarkSpoken(2150 * time.Millisecond)

	// Handle interruption at current position
	playoutPosition := tm.GetCurrentPosition()
	tm.HandleInterruption(playoutPosition)

	// Verify turn is marked interrupted
	if !tm.IsInterrupted() {
		t.Error("expected turn to be interrupted")
	}

	// Commit the turn
	committed := tm.CommitTurn()

	// Verify only spoken portion was committed
	if committed.Content == "" {
		t.Error("expected some content to be committed")
	}
	if len(committed.Content) >= len(fullText) {
		t.Errorf("expected committed text (%d chars) to be less than full text (%d chars)",
			len(committed.Content), len(fullText))
	}

	t.Logf("Full text: %q (%d chars)", fullText, len(fullText))
	t.Logf("Committed text: %q (%d chars)", committed.Content, len(committed.Content))
}

func TestMultipleTurns(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	// First turn
	tm.StartTurn("gen-1", 16000)
	tm.AppendGenerated("First response.")
	tm.MarkSpoken(2 * time.Second)
	tm.CommitTurn()

	// Second turn
	tm.StartTurn("gen-2", 16000)
	tm.AppendGenerated("Second response.")
	tm.MarkSpoken(2 * time.Second)
	tm.CommitTurn()

	// Third turn
	tm.StartTurn("gen-3", 16000)
	tm.AppendGenerated("Third response.")
	tm.MarkSpoken(2 * time.Second)
	tm.CommitTurn()

	// Verify history has all three turns
	if history.Len() != 3 {
		t.Errorf("expected 3 messages in history, got %d", history.Len())
	}

	messages := history.GetMessages()
	for i, msg := range messages {
		if msg.Role != "assistant" {
			t.Errorf("message %d: expected role 'assistant', got %s", i, msg.Role)
		}
	}
}

func TestNoUnspokenTextCommitted(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	// Start turn and generate text
	tm.StartTurn("gen-789", 16000)
	tm.AppendGenerated("This text was generated but never spoken.")

	// Don't advance playout at all (no audio was played)

	// Handle interruption at position 0
	tm.HandleInterruption(0)

	// Commit turn
	committed := tm.CommitTurn()

	// Verify nothing was committed (since nothing was spoken)
	if committed.Content != "" {
		t.Errorf("expected empty content (nothing spoken), got %q", committed.Content)
	}

	// Verify history is empty
	if history.Len() != 0 {
		t.Errorf("expected 0 messages in history, got %d", history.Len())
	}
}

func TestTurnManagerWithoutHistory(t *testing.T) {
	// Create turn manager without history
	tm := NewTurnManager("session-123", nil)

	// Start turn
	tm.StartTurn("gen-123", 16000)
	tm.AppendGenerated("Test response.")
	tm.MarkSpoken(1 * time.Second)

	// Commit should not panic even without history
	committed := tm.CommitTurn()

	if committed.Role != "assistant" {
		t.Errorf("expected role 'assistant', got %s", committed.Role)
	}
}

func TestAdvancePlayout(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	tm.StartTurn("gen-123", 16000)

	// Advance by bytes directly
	tm.AdvancePlayout(32000) // 1 second at 16kHz mono PCM16

	position := tm.GetCurrentPosition()
	expectedDuration := time.Second
	if position < expectedDuration-50*time.Millisecond || position > expectedDuration+50*time.Millisecond {
		t.Errorf("expected position ~%v, got %v", expectedDuration, position)
	}
}

func TestGetGenerationID(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	// No active turn
	if tm.GetGenerationID() != "" {
		t.Errorf("expected empty generation ID, got %s", tm.GetGenerationID())
	}

	// Start turn
	tm.StartTurn("gen-abc", 16000)

	if tm.GetGenerationID() != "gen-abc" {
		t.Errorf("expected generation ID 'gen-abc', got %s", tm.GetGenerationID())
	}
}

func TestGetStats(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	// No active turn
	stats := tm.GetStats()
	if stats.GenerationID != "" {
		t.Error("expected empty stats for no active turn")
	}

	// Start turn and add content
	tm.StartTurn("gen-stats", 16000)
	tm.AppendGenerated("Generated text.")
	tm.MarkQueued("Queued text.")
	tm.MarkSpoken(500 * time.Millisecond)

	stats = tm.GetStats()
	if stats.GenerationID != "gen-stats" {
		t.Errorf("expected generation ID 'gen-stats', got %s", stats.GenerationID)
	}
	if stats.GeneratedText != "Generated text." {
		t.Errorf("expected generated text 'Generated text.', got %s", stats.GeneratedText)
	}
	if stats.QueuedForTTS != "Queued text." {
		t.Errorf("expected queued text 'Queued text.', got %s", stats.QueuedForTTS)
	}
	if stats.CurrentPosition == 0 {
		t.Error("expected non-zero current position")
	}
}

func TestTurnManagerReset(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	tm.StartTurn("gen-123", 16000)
	tm.AppendGenerated("Test response.")

	if !tm.HasActiveTurn() {
		t.Error("expected active turn before reset")
	}

	tm.Reset()

	if tm.HasActiveTurn() {
		t.Error("expected no active turn after reset")
	}
}

func TestTurnManagerRegistry(t *testing.T) {
	registry := NewTurnManagerRegistry()
	history := session.NewConversationHistory(100)

	// Get or create
	tm1 := registry.GetOrCreate("session-1", history)
	if tm1 == nil {
		t.Fatal("expected non-nil turn manager")
	}

	// Get same instance
	tm2 := registry.GetOrCreate("session-1", history)
	if tm1 != tm2 {
		t.Error("expected same turn manager instance")
	}

	// Get different instance
	tm3 := registry.GetOrCreate("session-2", history)
	if tm1 == tm3 {
		t.Error("expected different turn manager instance")
	}

	// Get existing
	existing, ok := registry.Get("session-1")
	if !ok {
		t.Error("expected to find existing turn manager")
	}
	if existing != tm1 {
		t.Error("expected retrieved turn manager to match original")
	}

	// Get non-existent
	_, ok = registry.Get("nonexistent")
	if ok {
		t.Error("expected not to find non-existent turn manager")
	}

	// List
	ids := registry.List()
	if len(ids) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(ids))
	}

	// Remove
	registry.Remove("session-1")
	_, ok = registry.Get("session-1")
	if ok {
		t.Error("expected turn manager to be removed")
	}
}

func TestCurrentTurnClone(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	tm.StartTurn("gen-123", 16000)
	tm.AppendGenerated("Original text.")

	// Get clone
	clone := tm.GetCurrentTurn()
	if clone == nil {
		t.Fatal("expected non-nil clone")
	}

	// Modify clone
	clone.AppendGeneratedText(" More text.")

	// Original should be unchanged
	original := tm.GetCurrentTurn()
	if original.GetGeneratedText() != "Original text." {
		t.Errorf("expected original text unchanged, got %s", original.GetGeneratedText())
	}
}

func TestPartialCommit(t *testing.T) {
	history := session.NewConversationHistory(100)
	tm := NewTurnManager("session-123", history)

	// Start turn with text that takes ~2 seconds to speak
	// At 15 chars/second, 30 chars = 2 seconds
	tm.StartTurn("gen-123", 16000)
	tm.AppendGenerated("This is thirty characters long.")

	// Mark only 1 second as spoken
	tm.MarkSpoken(1 * time.Second)

	// Commit
	committed := tm.CommitTurn()

	// Should have committed roughly half the text
	// ~15 chars for 1 second of speech
	if len(committed.Content) == 0 {
		t.Error("expected some content to be committed")
	}
	if len(committed.Content) >= len("This is thirty characters long.") {
		t.Error("expected partial commit, not full text")
	}

	t.Logf("Full text: %q (%d chars)", "This is thirty characters long.", len("This is thirty characters long."))
	t.Logf("Committed: %q (%d chars)", committed.Content, len(committed.Content))
}
