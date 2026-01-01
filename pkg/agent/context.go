package agent

import (
	"strings"
)

// ContextManager manages the conversation context window and token limits.
type ContextManager struct {
	// maxTokens is the maximum context window size
	maxTokens int

	// pruneThreshold is the token count at which pruning should occur
	pruneThreshold int
}

// NewContextManager creates a new context manager.
func NewContextManager(maxTokens int) *ContextManager {
	return &ContextManager{
		maxTokens:      maxTokens,
		pruneThreshold: int(float64(maxTokens) * 0.8), // Prune at 80% capacity
	}
}

// ShouldPrune checks if the message history should be pruned.
func (cm *ContextManager) ShouldPrune(messages []Message) bool {
	totalTokens := cm.EstimateTokens(messages)
	return totalTokens > cm.pruneThreshold
}

// Prune reduces the message history to fit within the context window.
// It preserves the system message and recent messages, dropping older messages.
func (cm *ContextManager) Prune(messages []Message) []Message {
	if len(messages) == 0 {
		return messages
	}

	// Always keep the system message (first message)
	pruned := []Message{messages[0]}

	// Calculate tokens for system message
	systemTokens := cm.EstimateTokens(pruned)

	// Calculate remaining token budget
	remainingTokens := cm.maxTokens - systemTokens

	// Add messages from newest to oldest until we hit the limit
	for i := len(messages) - 1; i > 0; i-- {
		msgTokens := cm.estimateMessageTokens(&messages[i])
		if remainingTokens-msgTokens < 0 {
			break
		}
		remainingTokens -= msgTokens
		// Prepend to maintain chronological order
		pruned = append([]Message{messages[i]}, pruned[1:]...)
	}

	return pruned
}

// EstimateTokens estimates the total token count for a list of messages.
// Phase 1: Uses simple heuristic (4 characters per token).
// Future: Use tiktoken or similar for accurate tokenization.
func (cm *ContextManager) EstimateTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += cm.estimateMessageTokens(&msg)
	}
	return total
}

// estimateMessageTokens estimates tokens for a single message.
func (cm *ContextManager) estimateMessageTokens(msg *Message) int {
	// Rough estimate: 4 characters per token
	// This is a simplification; actual tokenization is model-specific
	tokens := len(msg.Content) / 4

	// Add overhead for role and structure
	tokens += 10

	// Add tokens for tool calls if present
	for _, toolCall := range msg.ToolCalls {
		tokens += len(toolCall.Name) / 4
		tokens += 20 // Overhead for tool call structure

		// Estimate tokens for arguments
		switch args := toolCall.Arguments.(type) {
		case string:
			tokens += len(args) / 4
		case map[string]interface{}:
			// Rough estimate for JSON structure
			tokens += cm.estimateMapTokens(args)
		}
	}

	return tokens
}

// estimateMapTokens estimates tokens for a map structure.
func (cm *ContextManager) estimateMapTokens(m map[string]interface{}) int {
	tokens := 0
	for key, value := range m {
		tokens += len(key) / 4
		tokens += cm.estimateValueTokens(value)
	}
	return tokens
}

// estimateValueTokens estimates tokens for an arbitrary value.
func (cm *ContextManager) estimateValueTokens(value interface{}) int {
	switch v := value.(type) {
	case string:
		return len(v) / 4
	case int, int64, float64, bool:
		return 1
	case map[string]interface{}:
		return cm.estimateMapTokens(v)
	case []interface{}:
		tokens := 0
		for _, item := range v {
			tokens += cm.estimateValueTokens(item)
		}
		return tokens
	default:
		return 10 // Default estimate for unknown types
	}
}

// TruncateContent truncates message content to fit within a token budget.
func (cm *ContextManager) TruncateContent(content string, maxTokens int) string {
	// Estimate max characters based on token budget
	maxChars := maxTokens * 4

	if len(content) <= maxChars {
		return content
	}

	// Truncate and add ellipsis
	truncated := content[:maxChars-3]
	// Try to truncate at a word boundary
	if lastSpace := strings.LastIndex(truncated, " "); lastSpace > 0 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}

// GetContextStats returns statistics about the current context usage.
type ContextStats struct {
	MessageCount    int
	EstimatedTokens int
	MaxTokens       int
	UtilizationPct  float64
}

// GetStats returns statistics about the context usage.
func (cm *ContextManager) GetStats(messages []Message) ContextStats {
	estimatedTokens := cm.EstimateTokens(messages)
	utilizationPct := float64(estimatedTokens) / float64(cm.maxTokens) * 100

	return ContextStats{
		MessageCount:    len(messages),
		EstimatedTokens: estimatedTokens,
		MaxTokens:       cm.maxTokens,
		UtilizationPct:  utilizationPct,
	}
}
