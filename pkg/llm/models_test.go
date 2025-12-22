package llm

import (
	"testing"
)

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
