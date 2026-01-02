package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewConfigFormat(t *testing.T) {
	yamlData := `
version: 1

providers:
  anthropic:
    type: anthropic
    api_key: $secret:anthropic/api_key
    models:
      claude-3-5-haiku-20241022:
        context_window: 200000
        input_price_per_mtok: 1.00
        output_price_per_mtok: 5.00
      claude-sonnet-4-20250514:
        context_window: 200000
        input_price_per_mtok: 3.00
        output_price_per_mtok: 15.00

  ollama:
    type: ollama
    base_url: http://localhost:11434
    models:
      llama3.2:
        context_window: 128000

tiers:
  fast: anthropic/claude-3-5-haiku-20241022
  balanced: anthropic/claude-sonnet-4-20250514
  strategic: anthropic/claude-sonnet-4-20250514
`

	var cfg Config
	if err := yaml.Unmarshal([]byte(yamlData), &cfg); err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Check version
	if cfg.Version != 1 {
		t.Errorf("Expected version 1, got %d", cfg.Version)
	}

	// Check providers
	if len(cfg.Providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(cfg.Providers))
	}

	// Check anthropic provider
	anthropic, ok := cfg.Providers["anthropic"]
	if !ok {
		t.Fatal("Anthropic provider not found")
	}
	if anthropic.Type != "anthropic" {
		t.Errorf("Expected type 'anthropic', got %q", anthropic.Type)
	}
	if len(anthropic.Models) != 2 {
		t.Errorf("Expected 2 models for anthropic, got %d", len(anthropic.Models))
	}

	// Check specific model
	haiku, ok := anthropic.Models["claude-3-5-haiku-20241022"]
	if !ok {
		t.Fatal("Haiku model not found")
	}
	if haiku.ContextWindow != 200000 {
		t.Errorf("Expected context window 200000, got %d", haiku.ContextWindow)
	}
	if haiku.InputPricePerMTok != 1.00 {
		t.Errorf("Expected input price 1.00, got %.2f", haiku.InputPricePerMTok)
	}
	if haiku.OutputPricePerMTok != 5.00 {
		t.Errorf("Expected output price 5.00, got %.2f", haiku.OutputPricePerMTok)
	}

	// Check tiers
	if len(cfg.Tiers) != 3 {
		t.Errorf("Expected 3 tiers, got %d", len(cfg.Tiers))
	}
	if cfg.Tiers["fast"] != "anthropic/claude-3-5-haiku-20241022" {
		t.Errorf("Expected fast tier to be 'anthropic/claude-3-5-haiku-20241022', got %q", cfg.Tiers["fast"])
	}
}
