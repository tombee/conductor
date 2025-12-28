package llm

import (
	"testing"
)

func TestModelInfo_CalculateCost(t *testing.T) {
	model := ModelInfo{
		InputPricePerMillion:  3.00,
		OutputPricePerMillion: 15.00,
	}

	usage := TokenUsage{
		PromptTokens:     1000,
		CompletionTokens: 500,
		TotalTokens:      1500,
	}

	costInfo := model.CalculateCost(usage)

	// Expected: (1000/1000000 * 3.00) + (500/1000000 * 15.00) = 0.003 + 0.0075 = 0.0105
	expected := 0.0105
	// Use a small epsilon for floating point comparison
	epsilon := 0.000001
	if costInfo.Amount < expected-epsilon || costInfo.Amount > expected+epsilon {
		t.Errorf("expected cost %.6f, got %.6f", expected, costInfo.Amount)
	}

	// Verify cost info fields
	if costInfo.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", costInfo.Currency)
	}
	if costInfo.Accuracy != CostMeasured {
		t.Errorf("expected accuracy measured, got %s", costInfo.Accuracy)
	}
	if costInfo.Source != SourcePricingTable {
		t.Errorf("expected source pricing_table, got %s", costInfo.Source)
	}
}

func TestGetModelByTier(t *testing.T) {
	models := []ModelInfo{
		{ID: "fast-model", Tier: ModelTierFast},
		{ID: "balanced-model", Tier: ModelTierBalanced},
		{ID: "strategic-model", Tier: ModelTierStrategic},
	}

	// Test finding each tier
	tests := []struct {
		tier     ModelTier
		expected string
	}{
		{ModelTierFast, "fast-model"},
		{ModelTierBalanced, "balanced-model"},
		{ModelTierStrategic, "strategic-model"},
	}

	for _, tt := range tests {
		model := GetModelByTier(models, tt.tier)
		if model == nil {
			t.Errorf("expected model for tier %s, got nil", tt.tier)
			continue
		}
		if model.ID != tt.expected {
			t.Errorf("expected model ID %s for tier %s, got %s", tt.expected, tt.tier, model.ID)
		}
	}
}

func TestGetModelByTier_NotFound(t *testing.T) {
	models := []ModelInfo{
		{ID: "fast-model", Tier: ModelTierFast},
	}

	model := GetModelByTier(models, ModelTierStrategic)
	if model != nil {
		t.Error("expected nil for non-existent tier, got a model")
	}
}

func TestGetModelByID(t *testing.T) {
	models := []ModelInfo{
		{ID: "model-1", Name: "Model 1"},
		{ID: "model-2", Name: "Model 2"},
		{ID: "model-3", Name: "Model 3"},
	}

	// Test finding existing model
	model := GetModelByID(models, "model-2")
	if model == nil {
		t.Fatal("expected to find model-2, got nil")
	}
	if model.Name != "Model 2" {
		t.Errorf("expected name 'Model 2', got '%s'", model.Name)
	}
}

func TestGetModelByID_NotFound(t *testing.T) {
	models := []ModelInfo{
		{ID: "model-1", Name: "Model 1"},
	}

	model := GetModelByID(models, "nonexistent")
	if model != nil {
		t.Error("expected nil for non-existent model ID, got a model")
	}
}

func TestModelTierConstants(t *testing.T) {
	// Verify tier constants are defined
	tiers := []ModelTier{
		ModelTierFast,
		ModelTierBalanced,
		ModelTierStrategic,
	}

	expectedValues := []string{"fast", "balanced", "strategic"}

	for i, tier := range tiers {
		if string(tier) != expectedValues[i] {
			t.Errorf("expected tier value %s, got %s", expectedValues[i], string(tier))
		}
	}
}

func TestModelInfo_CalculateCostWithCache(t *testing.T) {
	tests := []struct {
		name     string
		model    ModelInfo
		usage    TokenUsage
		expected float64
	}{
		{
			name: "with cache pricing configured",
			model: ModelInfo{
				InputPricePerMillion:         3.00,
				OutputPricePerMillion:        15.00,
				CacheCreationPricePerMillion: 3.00,
				CacheReadPricePerMillion:     0.75, // 25% of input
			},
			usage: TokenUsage{
				PromptTokens:        1000,
				CompletionTokens:    500,
				CacheCreationTokens: 2000,
				CacheReadTokens:     4000,
				TotalTokens:         7500,
			},
			// Expected: (1000/1M * 3.00) + (500/1M * 15.00) + (2000/1M * 3.00) + (4000/1M * 0.75)
			// = 0.003 + 0.0075 + 0.006 + 0.003 = 0.0195
			expected: 0.0195,
		},
		{
			name: "without cache pricing falls back to CalculateCost",
			model: ModelInfo{
				InputPricePerMillion:         3.00,
				OutputPricePerMillion:        15.00,
				CacheCreationPricePerMillion: 0.0, // Not configured
				CacheReadPricePerMillion:     0.0,
			},
			usage: TokenUsage{
				PromptTokens:        1000,
				CompletionTokens:    500,
				CacheCreationTokens: 2000, // Should be ignored
				CacheReadTokens:     4000, // Should be ignored
				TotalTokens:         7500,
			},
			// Expected: (1000/1M * 3.00) + (500/1M * 15.00) = 0.003 + 0.0075 = 0.0105
			expected: 0.0105,
		},
		{
			name: "zero cache tokens",
			model: ModelInfo{
				InputPricePerMillion:         3.00,
				OutputPricePerMillion:        15.00,
				CacheCreationPricePerMillion: 3.00,
				CacheReadPricePerMillion:     0.75,
			},
			usage: TokenUsage{
				PromptTokens:        1000,
				CompletionTokens:    500,
				CacheCreationTokens: 0,
				CacheReadTokens:     0,
				TotalTokens:         1500,
			},
			// Expected: (1000/1M * 3.00) + (500/1M * 15.00) = 0.003 + 0.0075 = 0.0105
			expected: 0.0105,
		},
	}

	epsilon := 0.000001
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			costInfo := tt.model.CalculateCostWithCache(tt.usage)

			if costInfo.Amount < tt.expected-epsilon || costInfo.Amount > tt.expected+epsilon {
				t.Errorf("expected cost %.6f, got %.6f", tt.expected, costInfo.Amount)
			}

			// Verify cost info fields
			if costInfo.Currency != "USD" {
				t.Errorf("expected currency USD, got %s", costInfo.Currency)
			}
			if costInfo.Accuracy != CostMeasured {
				t.Errorf("expected accuracy measured, got %s", costInfo.Accuracy)
			}
			if costInfo.Source != SourcePricingTable {
				t.Errorf("expected source pricing_table, got %s", costInfo.Source)
			}
		})
	}
}
