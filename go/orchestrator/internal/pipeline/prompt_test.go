package pipeline

import (
	"testing"

	"github.com/parlona/cloudapp/pkg/contracts"
	"github.com/parlona/cloudapp/pkg/session"
)

func TestBasicPromptAssembly(t *testing.T) {
	pa := NewPromptAssembler(20)

	sess := session.NewSession("session-123", "trace-456", session.TransportTypeWebSocket)
	sess.SystemPrompt = "You are a helpful assistant."

	messages := pa.AssemblePrompt(sess, "Hello, how are you?")

	// Should have system prompt + user utterance
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	if messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got %s", messages[0].Role)
	}
	if messages[0].Content != "You are a helpful assistant." {
		t.Errorf("expected system prompt content, got %s", messages[0].Content)
	}

	if messages[1].Role != "user" {
		t.Errorf("expected second message role 'user', got %s", messages[1].Role)
	}
	if messages[1].Content != "Hello, how are you?" {
		t.Errorf("expected user utterance, got %s", messages[1].Content)
	}
}

func TestPromptTruncation(t *testing.T) {
	pa := NewPromptAssembler(5) // Small context window

	sess := session.NewSession("session-123", "trace-456", session.TransportTypeWebSocket)
	sess.SystemPrompt = "You are a helpful assistant."

	// Create many messages in history
	history := session.NewConversationHistory(100)
	for i := 0; i < 10; i++ {
		history.AppendUserMessage("User message " + string(rune('0'+i)))
		history.AppendAssistantMessage("Assistant response " + string(rune('0'+i)))
	}

	messages := pa.AssemblePromptWithHistory(
		sess.SystemPrompt,
		history.GetMessages(),
		"Current question?",
	)

	// Should be truncated to maxContextMessages
	if len(messages) > 5 {
		t.Errorf("expected at most 5 messages, got %d", len(messages))
	}

	// System prompt should be preserved
	foundSystem := false
	for _, msg := range messages {
		if msg.Role == "system" {
			foundSystem = true
			break
		}
	}
	if !foundSystem {
		t.Error("expected system prompt to be preserved after truncation")
	}

	// Current user utterance should be preserved (last message)
	if messages[len(messages)-1].Content != "Current question?" {
		t.Errorf("expected last message to be current question, got %s",
			messages[len(messages)-1].Content)
	}
}

func TestEmptyHistory(t *testing.T) {
	pa := NewPromptAssembler(20)

	sess := session.NewSession("session-123", "trace-456", session.TransportTypeWebSocket)
	sess.SystemPrompt = "You are a helpful assistant."

	messages := pa.AssemblePromptWithHistory(
		sess.SystemPrompt,
		[]contracts.ChatMessage{}, // Empty history
		"Hello!",
	)

	// Should have system prompt + user utterance
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}

	if messages[0].Role != "system" {
		t.Errorf("expected first message role 'system', got %s", messages[0].Role)
	}

	if messages[1].Role != "user" {
		t.Errorf("expected second message role 'user', got %s", messages[1].Role)
	}
}

func TestNoSystemPrompt(t *testing.T) {
	pa := NewPromptAssembler(20)

	history := session.NewConversationHistory(100)
	history.AppendUserMessage("Previous question")
	history.AppendAssistantMessage("Previous answer")

	messages := pa.AssemblePromptWithHistory(
		"", // No system prompt
		history.GetMessages(),
		"Current question?",
	)

	// Should not have system message
	for _, msg := range messages {
		if msg.Role == "system" {
			t.Error("expected no system message when system prompt is empty")
		}
	}

	// Should have history + current question
	if len(messages) != 3 {
		t.Errorf("expected 3 messages (2 history + 1 current), got %d", len(messages))
	}
}

func TestEmptyUserUtterance(t *testing.T) {
	pa := NewPromptAssembler(20)

	sess := session.NewSession("session-123", "trace-456", session.TransportTypeWebSocket)
	sess.SystemPrompt = "You are a helpful assistant."

	messages := pa.AssemblePrompt(sess, "") // Empty user utterance

	// Should only have system prompt
	if len(messages) != 1 {
		t.Errorf("expected 1 message (system only), got %d", len(messages))
	}
}

func TestCountTokens(t *testing.T) {
	pa := NewPromptAssembler(20)

	// Rough approximation: ~4 chars per token
	message := contracts.ChatMessage{
		Role:    "user",
		Content: "This is a test message with about 40 characters.",
	}

	tokens := pa.CountTokens(message)
	expectedTokens := len(message.Content) / 4

	if tokens != expectedTokens {
		t.Errorf("expected %d tokens, got %d", expectedTokens, tokens)
	}
}

func TestCountTotalTokens(t *testing.T) {
	pa := NewPromptAssembler(20)

	messages := []contracts.ChatMessage{
		{Role: "system", Content: "You are helpful."}, // ~4 tokens + 4 overhead
		{Role: "user", Content: "Hello"},              // ~1 token + 4 overhead
	}

	total := pa.CountTotalTokens(messages)
	// Rough check: should be positive and reasonable
	if total <= 0 {
		t.Errorf("expected positive token count, got %d", total)
	}

	// With ~4 chars/token + 4 overhead per message:
	// system: 16/4 + 4 = 8
	// user: 5/4 + 4 = 5
	// total: ~13
	if total < 5 || total > 20 {
		t.Errorf("expected token count around 13, got %d", total)
	}
}

func TestTrimToTokenLimit(t *testing.T) {
	pa := NewPromptAssembler(20)

	messages := []contracts.ChatMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Message 2"},
		{Role: "assistant", Content: "Response 2"},
		{Role: "user", Content: "Current question?"},
	}

	// Trim to a small token limit
	trimmed := pa.TrimToTokenLimit(messages, 20)

	// System message should be preserved
	foundSystem := false
	for _, msg := range trimmed {
		if msg.Role == "system" {
			foundSystem = true
			break
		}
	}
	if !foundSystem {
		t.Error("expected system message to be preserved")
	}

	// Current question should be preserved (last message)
	if trimmed[len(trimmed)-1].Content != "Current question?" {
		t.Errorf("expected last message to be current question, got %s",
			trimmed[len(trimmed)-1].Content)
	}

	// Should have fewer messages than original
	if len(trimmed) >= len(messages) {
		t.Error("expected trimmed messages to be fewer than original")
	}
}

func TestTrimToTokenLimitNoTrimNeeded(t *testing.T) {
	pa := NewPromptAssembler(20)

	messages := []contracts.ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hello?"},
	}

	// Trim to a large token limit (no trimming needed)
	trimmed := pa.TrimToTokenLimit(messages, 1000)

	// Should have same messages
	if len(trimmed) != len(messages) {
		t.Errorf("expected %d messages, got %d", len(messages), len(trimmed))
	}
}

func TestDefaultMaxContextMessages(t *testing.T) {
	// Create assembler with 0 (should use default of 20)
	pa := NewPromptAssembler(0)

	if pa.maxContextMessages != 20 {
		t.Errorf("expected default maxContextMessages 20, got %d", pa.maxContextMessages)
	}
}

func TestAssemblePromptWithHistoryFiltersEmpty(t *testing.T) {
	pa := NewPromptAssembler(20)

	messages := []contracts.ChatMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: ""}, // Empty content
		{Role: "assistant", Content: "Response."},
		{Role: "user", Content: "Valid question?"},
	}

	result := pa.AssemblePromptWithHistory("", messages, "Current?")

	// Empty messages should be filtered out
	for _, msg := range result {
		if msg.Content == "" {
			t.Error("expected empty messages to be filtered out")
		}
	}
}

func TestAssemblePromptWithHistoryIncludesAllRoles(t *testing.T) {
	pa := NewPromptAssembler(20)

	history := []contracts.ChatMessage{
		{Role: "system", Content: "System message."},
		{Role: "user", Content: "User message."},
		{Role: "assistant", Content: "Assistant message."},
	}

	result := pa.AssemblePromptWithHistory("New system.", history, "Current?")

	// Should include system, user, and assistant messages
	roles := make(map[string]int)
	for _, msg := range result {
		roles[msg.Role]++
	}

	if roles["system"] != 2 { // New system + old system
		t.Errorf("expected 2 system messages, got %d", roles["system"])
	}
	if roles["user"] != 2 { // History user + current
		t.Errorf("expected 2 user messages, got %d", roles["user"])
	}
	if roles["assistant"] != 1 {
		t.Errorf("expected 1 assistant message, got %d", roles["assistant"])
	}
}

func TestContextLimitPreservesSystem(t *testing.T) {
	pa := NewPromptAssembler(3) // Very small limit

	messages := []contracts.ChatMessage{
		{Role: "system", Content: "System prompt."},
		{Role: "user", Content: "Message 1"},
		{Role: "assistant", Content: "Response 1"},
		{Role: "user", Content: "Message 2"},
		{Role: "assistant", Content: "Response 2"},
	}

	result := pa.applyContextLimit(messages)

	// Should preserve system message even with small limit
	foundSystem := false
	for _, msg := range result {
		if msg.Role == "system" {
			foundSystem = true
			break
		}
	}
	if !foundSystem {
		t.Error("expected system message to be preserved even with small context limit")
	}
}

func TestContextLimitOnlySystem(t *testing.T) {
	pa := NewPromptAssembler(5)

	// Only system messages
	messages := []contracts.ChatMessage{
		{Role: "system", Content: "System 1."},
		{Role: "system", Content: "System 2."},
	}

	result := pa.applyContextLimit(messages)

	// Should keep all system messages
	if len(result) != 2 {
		t.Errorf("expected 2 system messages, got %d", len(result))
	}
}
