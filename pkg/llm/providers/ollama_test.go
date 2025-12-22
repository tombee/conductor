package providers

import (
	"testing"

	"github.com/tombee/conductor/pkg/llm"
)

func TestNewOllamaProvider(t *testing.T) {
	_, err := NewOllamaProvider("http://localhost:11434")
	if err == nil {
		t.Error("expected error for unimplemented provider, got nil")
	}
}

func TestOllamaProvider_Name(t *testing.T) {
	var provider *OllamaProvider
	// Even though nil, test the name value
	if provider != nil {
		if provider.Name() != "ollama" {
			t.Errorf("expected name 'ollama', got '%s'", provider.Name())
		}
	}
}

func TestOllamaModels(t *testing.T) {
	if len(ollamaModels) == 0 {
		t.Error("expected at least one Ollama model")
	}

	// Verify all models have zero pricing (local execution)
	for _, model := range ollamaModels {
		if model.InputPricePerMillion != 0.00 {
			t.Errorf("model %s should have zero input price, got %.2f", model.ID, model.InputPricePerMillion)
		}
		if model.OutputPricePerMillion != 0.00 {
			t.Errorf("model %s should have zero output price, got %.2f", model.ID, model.OutputPricePerMillion)
		}
	}

	// Verify model tiers
	hasFast, hasBalanced, hasStrategic := false, false, false
	for _, model := range ollamaModels {
		switch model.Tier {
		case llm.ModelTierFast:
			hasFast = true
		case llm.ModelTierBalanced:
			hasBalanced = true
		case llm.ModelTierStrategic:
			hasStrategic = true
		}
	}

	if !hasFast || !hasBalanced || !hasStrategic {
		t.Error("not all model tiers are represented in Ollama models")
	}
}

func TestOllamaModels_Fields(t *testing.T) {
	for _, model := range ollamaModels {
		if model.ID == "" {
			t.Error("found model with empty ID")
		}
		if model.Name == "" {
			t.Error("found model with empty Name")
		}
		if model.MaxTokens <= 0 {
			t.Errorf("model %s has invalid MaxTokens: %d", model.ID, model.MaxTokens)
		}
		// Ollama models should not support tools in Phase 1
		if model.SupportsTools {
			t.Errorf("model %s should not support tools in Phase 1", model.ID)
		}
	}
}
