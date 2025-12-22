package pricing

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestNewPricingManager(t *testing.T) {
	pm := NewPricingManager()
	if pm == nil {
		t.Fatal("expected pricing manager, got nil")
	}

	// Should have built-in pricing
	config := pm.GetConfig()
	if len(config.Models) == 0 {
		t.Error("expected built-in pricing models")
	}

	// Should have Anthropic and OpenAI models
	hasAnthropic := false
	hasOpenAI := false
	for _, mp := range config.Models {
		if mp.Provider == "anthropic" {
			hasAnthropic = true
		}
		if mp.Provider == "openai" {
			hasOpenAI = true
		}
	}

	if !hasAnthropic {
		t.Error("expected Anthropic models in built-in pricing")
	}
	if !hasOpenAI {
		t.Error("expected OpenAI models in built-in pricing")
	}
}

func TestGetPricing(t *testing.T) {
	pm := NewPricingManager()

	tests := []struct {
		name     string
		provider string
		model    string
		wantNil  bool
	}{
		{
			name:     "anthropic claude-3-opus exists",
			provider: "anthropic",
			model:    "claude-3-opus-20240229",
			wantNil:  false,
		},
		{
			name:     "openai gpt-4o exists",
			provider: "openai",
			model:    "gpt-4o",
			wantNil:  false,
		},
		{
			name:     "nonexistent model",
			provider: "anthropic",
			model:    "claude-99",
			wantNil:  true,
		},
		{
			name:     "nonexistent provider",
			provider: "unknown",
			model:    "model-1",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := pm.GetPricing(tt.provider, tt.model)
			if tt.wantNil && pricing != nil {
				t.Errorf("expected nil, got pricing: %+v", pricing)
			}
			if !tt.wantNil && pricing == nil {
				t.Error("expected pricing, got nil")
			}
			if pricing != nil {
				if pricing.Provider != tt.provider {
					t.Errorf("expected provider %s, got %s", tt.provider, pricing.Provider)
				}
				if pricing.Model != tt.model {
					t.Errorf("expected model %s, got %s", tt.model, pricing.Model)
				}
			}
		})
	}
}

func TestGetPricingWithWarning(t *testing.T) {
	pm := NewPricingManager()

	// Test with fresh pricing (no warning expected)
	t.Run("fresh pricing", func(t *testing.T) {
		pricing, warning := pm.GetPricingWithWarning("anthropic", "claude-3-opus-20240229")
		if pricing == nil {
			t.Fatal("expected pricing, got nil")
		}
		if warning != "" {
			t.Errorf("expected no warning, got: %s", warning)
		}
	})

	// Test with stale pricing
	t.Run("stale pricing", func(t *testing.T) {
		// Create pricing with old effective date
		pm.mu.Lock()
		oldDate := time.Now().Add(-60 * 24 * time.Hour) // 60 days ago
		for i := range pm.config.Models {
			pm.config.Models[i].EffectiveDate = oldDate
		}
		pm.mu.Unlock()

		pricing, warning := pm.GetPricingWithWarning("anthropic", "claude-3-opus-20240229")
		if pricing == nil {
			t.Fatal("expected pricing, got nil")
		}
		if warning == "" {
			t.Error("expected staleness warning, got empty string")
		}
	})

	// Test with nonexistent model
	t.Run("nonexistent model", func(t *testing.T) {
		pricing, warning := pm.GetPricingWithWarning("unknown", "model-99")
		if pricing != nil {
			t.Errorf("expected nil pricing, got: %+v", pricing)
		}
		if warning != "" {
			t.Errorf("expected no warning for missing model, got: %s", warning)
		}
	})
}

func TestCachePricing(t *testing.T) {
	pm := NewPricingManager()

	// Anthropic models should have cache pricing
	tests := []struct {
		name     string
		provider string
		model    string
		wantCache bool
	}{
		{
			name:      "claude-3-opus has cache",
			provider:  "anthropic",
			model:     "claude-3-opus-20240229",
			wantCache: true,
		},
		{
			name:      "claude-3-5-sonnet has cache",
			provider:  "anthropic",
			model:     "claude-3-5-sonnet-20241022",
			wantCache: true,
		},
		{
			name:      "gpt-4o no cache",
			provider:  "openai",
			model:     "gpt-4o",
			wantCache: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := pm.GetPricing(tt.provider, tt.model)
			if pricing == nil {
				t.Fatal("expected pricing, got nil")
			}

			hasCache := pricing.CacheCreationPricePerMillion > 0 || pricing.CacheReadPricePerMillion > 0
			if hasCache != tt.wantCache {
				t.Errorf("expected cache support=%v, got cache_creation=%f, cache_read=%f",
					tt.wantCache, pricing.CacheCreationPricePerMillion, pricing.CacheReadPricePerMillion)
			}
		})
	}
}

func TestListProviders(t *testing.T) {
	pm := NewPricingManager()
	providers := pm.ListProviders()

	if len(providers) == 0 {
		t.Fatal("expected providers, got empty list")
	}

	// Should include major providers
	expectedProviders := []string{"anthropic", "openai", "ollama"}
	for _, expected := range expectedProviders {
		found := false
		for _, provider := range providers {
			if provider == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected provider %s in list, not found", expected)
		}
	}
}

func TestListModels(t *testing.T) {
	pm := NewPricingManager()

	t.Run("anthropic models", func(t *testing.T) {
		models := pm.ListModels("anthropic")
		if len(models) == 0 {
			t.Error("expected Anthropic models, got empty list")
		}

		// Should include Claude models
		expectedModels := []string{"claude-3-opus-20240229", "claude-3-5-sonnet-20241022"}
		for _, expected := range expectedModels {
			found := false
			for _, model := range models {
				if model == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected model %s in Anthropic models, not found", expected)
			}
		}
	})

	t.Run("openai models", func(t *testing.T) {
		models := pm.ListModels("openai")
		if len(models) == 0 {
			t.Error("expected OpenAI models, got empty list")
		}
	})

	t.Run("nonexistent provider", func(t *testing.T) {
		models := pm.ListModels("unknown-provider")
		if len(models) != 0 {
			t.Errorf("expected empty list for unknown provider, got %d models", len(models))
		}
	})
}

func TestLoadUserConfig(t *testing.T) {
	// Create temp directory for test config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "pricing.yaml")

	// Create test user config
	userConfig := PricingConfig{
		Version:   "1.0",
		UpdatedAt: time.Now(),
		Models: []ModelPricing{
			{
				Provider:              "anthropic",
				Model:                 "claude-test-model",
				InputPricePerMillion:  1.00,
				OutputPricePerMillion: 2.00,
				EffectiveDate:         time.Now(),
			},
			{
				Provider:              "openai",
				Model:                 "gpt-test-model",
				InputPricePerMillion:  0.50,
				OutputPricePerMillion: 1.00,
				EffectiveDate:         time.Now(),
			},
		},
	}

	data, err := yaml.Marshal(userConfig)
	if err != nil {
		t.Fatalf("failed to marshal test config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Test loading user config
	pm, err := NewPricingManagerWithConfig(configPath)
	if err != nil {
		t.Fatalf("failed to create pricing manager with config: %v", err)
	}

	// Should have both built-in and user models
	config := pm.GetConfig()
	if len(config.Models) == 0 {
		t.Fatal("expected models after loading config")
	}

	// User models should be present
	testModel := pm.GetPricing("anthropic", "claude-test-model")
	if testModel == nil {
		t.Error("expected user test model to be loaded")
	}

	// Built-in models should still be present
	builtInModel := pm.GetPricing("anthropic", "claude-3-opus-20240229")
	if builtInModel == nil {
		t.Error("expected built-in model to still be present")
	}
}

func TestLoadUserConfigNonexistent(t *testing.T) {
	// Test with nonexistent config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent.yaml")

	pm, err := NewPricingManagerWithConfig(configPath)
	if err != nil {
		t.Fatalf("should not error on nonexistent config: %v", err)
	}

	// Should still have built-in pricing
	config := pm.GetConfig()
	if len(config.Models) == 0 {
		t.Error("expected built-in pricing when user config doesn't exist")
	}
}

func TestMergePricing(t *testing.T) {
	pm := NewPricingManager()

	builtIn := &PricingConfig{
		Version:   "1.0",
		UpdatedAt: time.Now(),
		Models: []ModelPricing{
			{
				Provider:              "anthropic",
				Model:                 "claude-3-opus-20240229",
				InputPricePerMillion:  15.00,
				OutputPricePerMillion: 75.00,
				EffectiveDate:         time.Now(),
			},
			{
				Provider:              "openai",
				Model:                 "gpt-4o",
				InputPricePerMillion:  2.50,
				OutputPricePerMillion: 10.00,
				EffectiveDate:         time.Now(),
			},
		},
	}

	user := &PricingConfig{
		Version:   "1.1",
		UpdatedAt: time.Now(),
		Models: []ModelPricing{
			{
				Provider:              "anthropic",
				Model:                 "claude-3-opus-20240229",
				InputPricePerMillion:  20.00, // Override
				OutputPricePerMillion: 80.00, // Override
				EffectiveDate:         time.Now(),
			},
			{
				Provider:              "custom",
				Model:                 "custom-model",
				InputPricePerMillion:  5.00,
				OutputPricePerMillion: 10.00,
				EffectiveDate:         time.Now(),
			},
		},
	}

	merged := pm.mergePricing(builtIn, user)

	// Should have 3 models: overridden opus, built-in gpt-4o, custom model
	if len(merged.Models) != 3 {
		t.Errorf("expected 3 models, got %d", len(merged.Models))
	}

	// Check opus was overridden
	var opus *ModelPricing
	for i := range merged.Models {
		if merged.Models[i].Provider == "anthropic" && merged.Models[i].Model == "claude-3-opus-20240229" {
			opus = &merged.Models[i]
			break
		}
	}
	if opus == nil {
		t.Fatal("expected opus model in merged config")
	}
	if opus.InputPricePerMillion != 20.00 {
		t.Errorf("expected opus override price 20.00, got %f", opus.InputPricePerMillion)
	}

	// Check custom model was added
	var custom *ModelPricing
	for i := range merged.Models {
		if merged.Models[i].Provider == "custom" && merged.Models[i].Model == "custom-model" {
			custom = &merged.Models[i]
			break
		}
	}
	if custom == nil {
		t.Error("expected custom model in merged config")
	}

	// Check gpt-4o is still present
	var gpt4o *ModelPricing
	for i := range merged.Models {
		if merged.Models[i].Provider == "openai" && merged.Models[i].Model == "gpt-4o" {
			gpt4o = &merged.Models[i]
			break
		}
	}
	if gpt4o == nil {
		t.Error("expected gpt-4o in merged config")
	}
}

func TestSetStalenessThreshold(t *testing.T) {
	pm := NewPricingManager()

	// Set custom threshold
	customThreshold := 10 * 24 * time.Hour // 10 days
	pm.SetStalenessThreshold(customThreshold)

	// Create pricing with old effective date
	pm.mu.Lock()
	oldDate := time.Now().Add(-15 * 24 * time.Hour) // 15 days ago
	for i := range pm.config.Models {
		pm.config.Models[i].EffectiveDate = oldDate
	}
	pm.mu.Unlock()

	// Should get warning with 10-day threshold (pricing is 15 days old)
	_, warning := pm.GetPricingWithWarning("anthropic", "claude-3-opus-20240229")
	if warning == "" {
		t.Error("expected staleness warning with custom threshold")
	}

	// Set threshold to 20 days
	pm.SetStalenessThreshold(20 * 24 * time.Hour)

	// Should not get warning now (pricing is 15 days old, threshold is 20)
	_, warning = pm.GetPricingWithWarning("anthropic", "claude-3-opus-20240229")
	if warning != "" {
		t.Errorf("expected no warning with 20-day threshold, got: %s", warning)
	}
}

func TestSubscriptionModels(t *testing.T) {
	pm := NewPricingManager()

	// Ollama models should be marked as subscription
	ollama := pm.GetPricing("ollama", "llama2")
	if ollama == nil {
		t.Fatal("expected ollama llama2 pricing")
	}

	if !ollama.IsSubscription {
		t.Error("expected ollama models to be marked as subscription")
	}

	if ollama.InputPricePerMillion != 0 || ollama.OutputPricePerMillion != 0 {
		t.Error("expected subscription models to have zero pricing")
	}
}
