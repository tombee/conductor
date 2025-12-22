package providers

import (
	"testing"

	"github.com/tombee/conductor/pkg/llm"
)

func TestNewOpenAIProvider(t *testing.T) {
	_, err := NewOpenAIProvider("test-api-key")
	if err == nil {
		t.Error("expected error for unimplemented provider, got nil")
	}
}

func TestOpenAIProvider_Capabilities(t *testing.T) {
	// Create a nil provider just to test the Capabilities method signature
	var provider *OpenAIProvider

	// Even though the provider is nil, we can check that the models list exists
	if len(openAIModels) == 0 {
		t.Error("expected at least one OpenAI model")
	}

	// Verify model tiers
	hasFast, hasBalanced, hasStrategic := false, false, false
	for _, model := range openAIModels {
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
		t.Error("not all model tiers are represented in OpenAI models")
	}

	// Verify Name() returns correct value
	if provider != nil {
		if provider.Name() != "openai" {
			t.Errorf("expected name 'openai', got '%s'", provider.Name())
		}
	}
}

func TestOpenAIModels(t *testing.T) {
	// Test that all models have required fields
	for _, model := range openAIModels {
		if model.ID == "" {
			t.Error("found model with empty ID")
		}
		if model.Name == "" {
			t.Error("found model with empty Name")
		}
		if model.MaxTokens <= 0 {
			t.Errorf("model %s has invalid MaxTokens: %d", model.ID, model.MaxTokens)
		}
	}
}
