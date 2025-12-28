package llm

// ModelTier represents performance/cost trade-offs for model selection.
// Applications can request a tier without knowing provider-specific model IDs.
type ModelTier string

const (
	// ModelTierFast prioritizes speed and cost-efficiency.
	// Best for simple tasks, high-volume requests, or quick responses.
	// Example: Claude Haiku, GPT-3.5-turbo
	ModelTierFast ModelTier = "fast"

	// ModelTierBalanced offers a balance between capability and cost.
	// Best for most general-purpose tasks requiring reasoning.
	// Example: Claude Sonnet, GPT-4
	ModelTierBalanced ModelTier = "balanced"

	// ModelTierStrategic provides maximum capability for complex reasoning.
	// Best for difficult tasks requiring deep analysis or expert knowledge.
	// Example: Claude Opus, GPT-4-turbo
	ModelTierStrategic ModelTier = "strategic"
)

// ModelInfo describes a specific model's capabilities and pricing.
type ModelInfo struct {
	// ID is the provider-specific model identifier (e.g., "claude-3-opus-20240229").
	ID string

	// Name is the human-readable model name (e.g., "Claude 3 Opus").
	Name string

	// Tier indicates the performance/cost category.
	Tier ModelTier

	// MaxTokens is the maximum context window size in tokens.
	MaxTokens int

	// MaxOutputTokens is the maximum tokens the model can generate in one response.
	// If 0, uses provider default or MaxTokens.
	MaxOutputTokens int

	// InputPricePerMillion is the cost in USD per million input tokens.
	InputPricePerMillion float64

	// OutputPricePerMillion is the cost in USD per million output tokens.
	OutputPricePerMillion float64

	// CacheCreationPricePerMillion is the cost in USD per million tokens for cache writes.
	// This is typically the same as InputPricePerMillion.
	// If 0, cache creation costs are not tracked separately.
	CacheCreationPricePerMillion float64

	// CacheReadPricePerMillion is the cost in USD per million tokens for cache reads.
	// This is typically lower than InputPricePerMillion (e.g., 25% for Anthropic).
	// If 0, cache read costs are not tracked separately.
	CacheReadPricePerMillion float64

	// SupportsTools indicates whether this model can use function calling.
	SupportsTools bool

	// SupportsVision indicates whether this model can process images.
	SupportsVision bool

	// Description provides additional context about the model's strengths.
	Description string
}

// CalculateCost computes the cost for a request based on token usage.
// Returns CostInfo with accuracy=measured since this uses provider-reported tokens.
// This function does not include cache token costs - use CalculateCostWithCache for that.
func (m ModelInfo) CalculateCost(usage TokenUsage) *CostInfo {
	inputCost := float64(usage.PromptTokens) / 1_000_000.0 * m.InputPricePerMillion
	outputCost := float64(usage.CompletionTokens) / 1_000_000.0 * m.OutputPricePerMillion
	totalCost := inputCost + outputCost

	return &CostInfo{
		Amount:   totalCost,
		Currency: "USD",
		Accuracy: CostMeasured,
		Source:   SourcePricingTable,
	}
}

// CalculateCostWithCache computes the cost for a request including cache token costs.
// It accounts for cache creation and cache read tokens separately from regular input tokens.
// Returns CostInfo with accuracy=measured since this uses provider-reported tokens.
// If the model doesn't have cache pricing configured (both fields are 0), it falls back
// to CalculateCost behavior.
func (m ModelInfo) CalculateCostWithCache(usage TokenUsage) *CostInfo {
	// Check if cache pricing is configured
	hasCachePricing := m.CacheCreationPricePerMillion > 0 || m.CacheReadPricePerMillion > 0

	if !hasCachePricing {
		// No cache pricing - use standard calculation
		return m.CalculateCost(usage)
	}

	// Calculate regular input/output costs
	inputCost := float64(usage.PromptTokens) / 1_000_000.0 * m.InputPricePerMillion
	outputCost := float64(usage.CompletionTokens) / 1_000_000.0 * m.OutputPricePerMillion

	// Calculate cache costs
	cacheCreationCost := float64(usage.CacheCreationTokens) / 1_000_000.0 * m.CacheCreationPricePerMillion
	cacheReadCost := float64(usage.CacheReadTokens) / 1_000_000.0 * m.CacheReadPricePerMillion

	totalCost := inputCost + outputCost + cacheCreationCost + cacheReadCost

	return &CostInfo{
		Amount:   totalCost,
		Currency: "USD",
		Accuracy: CostMeasured,
		Source:   SourcePricingTable,
	}
}

// GetModelByTier returns the first model matching the specified tier.
// Returns nil if no model matches the tier.
func GetModelByTier(models []ModelInfo, tier ModelTier) *ModelInfo {
	for i := range models {
		if models[i].Tier == tier {
			return &models[i]
		}
	}
	return nil
}

// GetModelByID returns the model with the specified ID.
// Returns nil if no model matches the ID.
func GetModelByID(models []ModelInfo, id string) *ModelInfo {
	for i := range models {
		if models[i].ID == id {
			return &models[i]
		}
	}
	return nil
}
