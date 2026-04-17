package session

import (
	"context"
	"sync"
	"time"

	"github.com/parlona/cloudapp/pkg/contracts"
)

// ConversationHistory manages the dialogue history for a session.
// Critical: Only spoken_text is committed to history, not unspoken generated text.
type ConversationHistory struct {
	mu       sync.RWMutex
	messages []contracts.ChatMessage
	maxSize  int
}

// NewConversationHistory creates a new conversation history.
func NewConversationHistory(maxSize int) *ConversationHistory {
	if maxSize <= 0 {
		maxSize = 100 // Default max history size
	}
	return &ConversationHistory{
		messages: make([]contracts.ChatMessage, 0),
		maxSize:  maxSize,
	}
}

// AppendUserMessage adds a user message to the history.
func (h *ConversationHistory) AppendUserMessage(content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, contracts.ChatMessage{
		Role:    "user",
		Content: content,
	})

	h.trimIfNeeded()
}

// AppendAssistantMessage adds an assistant message to the history.
// This should only be called with text that was actually spoken.
func (h *ConversationHistory) AppendAssistantMessage(content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if content == "" {
		return
	}

	h.messages = append(h.messages, contracts.ChatMessage{
		Role:    "assistant",
		Content: content,
	})

	h.trimIfNeeded()
}

// AppendSystemMessage adds a system message to the history.
func (h *ConversationHistory) AppendSystemMessage(content string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.messages = append(h.messages, contracts.ChatMessage{
		Role:    "system",
		Content: content,
	})

	h.trimIfNeeded()
}

// GetMessages returns a copy of all messages in the history.
func (h *ConversationHistory) GetMessages() []contracts.ChatMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]contracts.ChatMessage, len(h.messages))
	copy(result, h.messages)
	return result
}

// GetPromptContext returns messages formatted for LLM prompt context.
// This includes system message, recent conversation history.
func (h *ConversationHistory) GetPromptContext(systemPrompt string, maxContextMessages int) []contracts.ChatMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var context []contracts.ChatMessage

	// Add system prompt if provided
	if systemPrompt != "" {
		context = append(context, contracts.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Add existing system messages from history
	for _, msg := range h.messages {
		if msg.Role == "system" {
			context = append(context, msg)
		}
	}

	// Add recent user/assistant messages
	userAssistantMessages := h.getUserAssistantMessages()
	if maxContextMessages > 0 && len(userAssistantMessages) > maxContextMessages {
		userAssistantMessages = userAssistantMessages[len(userAssistantMessages)-maxContextMessages:]
	}

	context = append(context, userAssistantMessages...)
	return context
}

// GetLastUserMessage returns the most recent user message.
func (h *ConversationHistory) GetLastUserMessage() (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := len(h.messages) - 1; i >= 0; i-- {
		if h.messages[i].Role == "user" {
			return h.messages[i].Content, true
		}
	}
	return "", false
}

// GetLastAssistantMessage returns the most recent assistant message.
func (h *ConversationHistory) GetLastAssistantMessage() (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := len(h.messages) - 1; i >= 0; i-- {
		if h.messages[i].Role == "assistant" {
			return h.messages[i].Content, true
		}
	}
	return "", false
}

// Clear removes all messages from history.
func (h *ConversationHistory) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = h.messages[:0]
}

// Len returns the number of messages in history.
func (h *ConversationHistory) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.messages)
}

// trimIfNeeded removes oldest messages if history exceeds max size.
func (h *ConversationHistory) trimIfNeeded() {
	if len(h.messages) > h.maxSize {
		// Keep system messages, trim oldest user/assistant
		h.trimOldestNonSystem()
	}
}

// trimOldestNonSystem removes oldest non-system messages.
func (h *ConversationHistory) trimOldestNonSystem() {
	var newMessages []contracts.ChatMessage
	systemMessages := 0

	// First pass: count system messages
	for _, msg := range h.messages {
		if msg.Role == "system" {
			systemMessages++
		}
	}

	// Calculate how many non-system messages to keep
	nonSystemToKeep := h.maxSize - systemMessages
	if nonSystemToKeep < 0 {
		nonSystemToKeep = 0
	}

	// Collect system messages
	for _, msg := range h.messages {
		if msg.Role == "system" {
			newMessages = append(newMessages, msg)
		}
	}

	// Collect recent non-system messages
	nonSystem := h.getUserAssistantMessages()
	if len(nonSystem) > nonSystemToKeep {
		nonSystem = nonSystem[len(nonSystem)-nonSystemToKeep:]
	}
	newMessages = append(newMessages, nonSystem...)

	h.messages = newMessages
}

// getUserAssistantMessages returns only user and assistant messages.
func (h *ConversationHistory) getUserAssistantMessages() []contracts.ChatMessage {
	var result []contracts.ChatMessage
	for _, msg := range h.messages {
		if msg.Role == "user" || msg.Role == "assistant" {
			result = append(result, msg)
		}
	}
	return result
}

// HistoryEntry represents a single entry in the conversation history with metadata.
type HistoryEntry struct {
	SessionID  string    `json:"session_id"`
	TurnID     string    `json:"turn_id"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	Timestamp  time.Time `json:"timestamp"`
	SpokenOnly bool      `json:"spoken_only"` // true if this was committed after interruption
	DurationMs int64     `json:"duration_ms,omitempty"`
}

// HistoryStore defines the interface for persistent history storage.
type HistoryStore interface {
	// AppendEntry appends an entry to the persistent history.
	AppendEntry(ctx context.Context, entry HistoryEntry) error

	// GetHistory retrieves history for a session.
	GetHistory(ctx context.Context, sessionID string, limit int) ([]HistoryEntry, error)

	// GetFullTranscript returns the complete conversation transcript.
	GetFullTranscript(ctx context.Context, sessionID string) (string, error)
}
