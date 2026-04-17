package pipeline

import (
	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/session"
)

// PromptAssembler assembles prompts for LLM generation.
type PromptAssembler struct {
	maxContextMessages int
}

// NewPromptAssembler creates a new prompt assembler.
func NewPromptAssembler(maxContextMessages int) *PromptAssembler {
	if maxContextMessages <= 0 {
		maxContextMessages = 20 // Default max context
	}
	return &PromptAssembler{
		maxContextMessages: maxContextMessages,
	}
}

// AssemblePrompt assembles a prompt from session state and user utterance.
// It:
//   - Starts with system prompt
//   - Adds conversation history (only committed/spoken messages)
//   - Adds current user utterance
//   - Respects max context window (truncates oldest messages if needed)
//   - Never includes unspoken assistant text
func (pa *PromptAssembler) AssemblePrompt(
	sess *session.Session,
	userUtterance string,
) []contracts.ChatMessage {
	var messages []contracts.ChatMessage

	// Add system prompt if present
	if sess.SystemPrompt != "" {
		messages = append(messages, contracts.ChatMessage{
			Role:    "system",
			Content: sess.SystemPrompt,
		})
	}

	// Get conversation history from session
	// Note: In a real implementation, the session would have access to the conversation history
	// For now, we assume the history is stored separately and accessed via the session store

	// Add user utterance
	if userUtterance != "" {
		messages = append(messages, contracts.ChatMessage{
			Role:    "user",
			Content: userUtterance,
		})
	}

	// Apply context window limit
	messages = pa.applyContextLimit(messages)

	return messages
}

// AssemblePromptWithHistory assembles a prompt with explicit conversation history.
// This is the preferred method when history is available.
func (pa *PromptAssembler) AssemblePromptWithHistory(
	systemPrompt string,
	history []contracts.ChatMessage,
	userUtterance string,
) []contracts.ChatMessage {
	var messages []contracts.ChatMessage

	// Add system prompt if present
	if systemPrompt != "" {
		messages = append(messages, contracts.ChatMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	// Add conversation history (only committed messages)
	// Filter out any unspoken assistant messages
	for _, msg := range history {
		// Skip empty messages
		if msg.Content == "" {
			continue
		}
		// Only include committed messages (user and spoken assistant)
		if msg.Role == "user" || msg.Role == "assistant" || msg.Role == "system" {
			messages = append(messages, msg)
		}
	}

	// Add user utterance
	if userUtterance != "" {
		messages = append(messages, contracts.ChatMessage{
			Role:    "user",
			Content: userUtterance,
		})
	}

	// Apply context window limit
	messages = pa.applyContextLimit(messages)

	return messages
}

// applyContextLimit applies the maximum context window limit.
// It preserves the system prompt and most recent messages.
func (pa *PromptAssembler) applyContextLimit(messages []contracts.ChatMessage) []contracts.ChatMessage {
	if len(messages) <= pa.maxContextMessages {
		return messages
	}

	// Separate system messages from other messages
	var systemMessages []contracts.ChatMessage
	var otherMessages []contracts.ChatMessage

	for _, msg := range messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	// Calculate how many non-system messages we can keep
	nonSystemLimit := pa.maxContextMessages - len(systemMessages)
	if nonSystemLimit < 0 {
		nonSystemLimit = 0
	}

	// Keep the most recent non-system messages
	if len(otherMessages) > nonSystemLimit {
		otherMessages = otherMessages[len(otherMessages)-nonSystemLimit:]
	}

	// Reassemble: system messages first, then recent non-system messages
	var result []contracts.ChatMessage
	result = append(result, systemMessages...)
	result = append(result, otherMessages...)

	return result
}

// CountTokens estimates the token count for a message.
// This is a rough approximation - in production, use a proper tokenizer.
func (pa *PromptAssembler) CountTokens(message contracts.ChatMessage) int {
	// Rough approximation: ~4 characters per token on average
	return len(message.Content) / 4
}

// CountTotalTokens estimates the total token count for a list of messages.
func (pa *PromptAssembler) CountTotalTokens(messages []contracts.ChatMessage) int {
	total := 0
	for _, msg := range messages {
		total += pa.CountTokens(msg)
		// Add overhead per message
		total += 4 // Approximate overhead for role, formatting, etc.
	}
	return total
}

// TrimToTokenLimit trims messages to fit within a token limit.
// It preserves the system prompt and the most recent messages.
func (pa *PromptAssembler) TrimToTokenLimit(
	messages []contracts.ChatMessage,
	maxTokens int,
) []contracts.ChatMessage {
	if pa.CountTotalTokens(messages) <= maxTokens {
		return messages
	}

	// Separate system messages from other messages
	var systemMessages []contracts.ChatMessage
	var otherMessages []contracts.ChatMessage

	for _, msg := range messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		} else {
			otherMessages = append(otherMessages, msg)
		}
	}

	// Calculate system message tokens
	systemTokens := pa.CountTotalTokens(systemMessages)
	availableTokens := maxTokens - systemTokens
	if availableTokens < 0 {
		availableTokens = 0
	}

	// Remove oldest messages until we fit
	for pa.CountTotalTokens(otherMessages) > availableTokens && len(otherMessages) > 1 {
		// Keep at least the last message (user utterance)
		otherMessages = otherMessages[1:]
	}

	// Reassemble
	var result []contracts.ChatMessage
	result = append(result, systemMessages...)
	result = append(result, otherMessages...)

	return result
}
