package pricing

import (
	"fmt"
	"strings"
)

// TokenUsage tracks token consumption for cost calculation.
// This is a copy of llm.TokenUsage to avoid circular import.
type TokenUsage struct {
	PromptTokens        int
	CompletionTokens    int
	TotalTokens         int
	CacheCreationTokens int
	CacheReadTokens     int
}

// CostAccuracy indicates reliability of cost value.
type CostAccuracy string

const (
	CostMeasured    CostAccuracy = "measured"
	CostEstimated   CostAccuracy = "estimated"
	CostUnavailable CostAccuracy = "unavailable"
)

// CostInfo contains cost details with accuracy tracking.
type CostInfo struct {
	Amount   float64
	Currency string
	Accuracy CostAccuracy
	Source   string
}

// Cost sources.
const (
	SourceProvider     = "provider"
	SourcePricingTable = "pricing_table"
	SourceEstimated    = "estimated"
)

// CalculateCost computes the cost for a request using pricing configuration.
// Supports cache tokens and returns accuracy information.
func CalculateCost(pricing *ModelPricing, usage TokenUsage) *CostInfo {
	if pricing == nil {
		return &CostInfo{
			Amount:   0,
			Currency: "USD",
			Accuracy: CostUnavailable,
			Source:   SourcePricingTable,
		}
	}

	// Subscription models have no per-token cost
	if pricing.IsSubscription {
		return &CostInfo{
			Amount:   0,
			Currency: "USD",
			Accuracy: CostMeasured,
			Source:   SourcePricingTable,
		}
	}

	// Calculate regular token costs
	inputCost := float64(usage.PromptTokens) / 1_000_000.0 * pricing.InputPricePerMillion
	outputCost := float64(usage.CompletionTokens) / 1_000_000.0 * pricing.OutputPricePerMillion

	// Calculate cache token costs if supported
	var cacheCreationCost float64
	var cacheReadCost float64

	if pricing.CacheCreationPricePerMillion > 0 && usage.CacheCreationTokens > 0 {
		cacheCreationCost = float64(usage.CacheCreationTokens) / 1_000_000.0 * pricing.CacheCreationPricePerMillion
	}

	if pricing.CacheReadPricePerMillion > 0 && usage.CacheReadTokens > 0 {
		cacheReadCost = float64(usage.CacheReadTokens) / 1_000_000.0 * pricing.CacheReadPricePerMillion
	}

	totalCost := inputCost + outputCost + cacheCreationCost + cacheReadCost

	// Determine accuracy based on token source
	accuracy := determineAccuracy(usage)

	return &CostInfo{
		Amount:   totalCost,
		Currency: "USD",
		Accuracy: accuracy,
		Source:   SourcePricingTable,
	}
}

// determineAccuracy determines cost accuracy based on token usage data.
func determineAccuracy(usage TokenUsage) CostAccuracy {
	// If we have provider-reported tokens, this is measured
	if usage.PromptTokens > 0 || usage.CompletionTokens > 0 {
		return CostMeasured
	}

	// If we only have total tokens (estimated from characters), this is estimated
	if usage.TotalTokens > 0 {
		return CostEstimated
	}

	// No token data available
	return CostUnavailable
}

// EstimateTokensFromText estimates token count from text using character-based approximation.
// This is a fallback when provider doesn't report token counts.
// Uses the common approximation: ~4 characters per token for English text.
func EstimateTokensFromText(text string) int {
	// Simple character-based estimation
	// Real tokenizers are more complex, but this provides a reasonable approximation
	charCount := len(text)

	// Average ~4 characters per token for English text
	// This is conservative (tends to overestimate tokens)
	estimatedTokens := charCount / 4

	// Minimum 1 token for non-empty text
	if estimatedTokens == 0 && charCount > 0 {
		estimatedTokens = 1
	}

	return estimatedTokens
}

// EstimateTokensFromMessages estimates tokens from a list of messages.
// Each message has a role and content. This adds overhead for message formatting.
func EstimateTokensFromMessages(messages []Message) int {
	totalTokens := 0

	// Add base overhead for message formatting
	// OpenAI/Anthropic add tokens for role markers and formatting
	messageOverhead := 3 // tokens per message for role and separators

	for _, msg := range messages {
		// Estimate content tokens
		contentTokens := EstimateTokensFromText(msg.Content)

		// Add role tokens
		roleTokens := EstimateTokensFromText(msg.Role)

		totalTokens += contentTokens + roleTokens + messageOverhead
	}

	// Add base overhead for the conversation
	totalTokens += 3

	return totalTokens
}

// Message represents a chat message for token estimation.
type Message struct {
	Role    string
	Content string
}

// FormatCost formats a cost value with accuracy indicator for display.
// Returns strings like "$0.0045", "~$0.0045", or "--" for unavailable.
func FormatCost(cost *CostInfo) string {
	if cost == nil || cost.Accuracy == CostUnavailable {
		return "--"
	}

	// Format the cost amount
	formatted := fmt.Sprintf("$%.4f", cost.Amount)

	// Add prefix for estimated costs
	if cost.Accuracy == CostEstimated {
		formatted = "~" + formatted
	}

	return formatted
}

// FormatTokens formats token count for display with units.
func FormatTokens(tokens int) string {
	if tokens >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(tokens)/1_000_000.0)
	}
	if tokens >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1_000.0)
	}
	return fmt.Sprintf("%d", tokens)
}

// ParseModel extracts provider and model from a model string.
// Supports formats like "anthropic:claude-3-opus" or just "claude-3-opus".
func ParseModel(modelStr string) (provider, model string) {
	parts := strings.SplitN(modelStr, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	// Try to infer provider from model name
	if strings.HasPrefix(modelStr, "claude-") {
		return "anthropic", modelStr
	}
	if strings.HasPrefix(modelStr, "gpt-") || strings.HasPrefix(modelStr, "o1-") {
		return "openai", modelStr
	}

	// Default to unknown provider
	return "unknown", modelStr
}
