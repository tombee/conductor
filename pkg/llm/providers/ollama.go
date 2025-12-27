// Package providers contains concrete implementations of LLM providers.
package providers

import (
	"context"
	"net/http"
	"sync"

	"github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/llm"
)

// OllamaProvider is a placeholder for the Ollama provider implementation.
// This will be implemented in a future phase.
//
// Phase 1 Status: PLACEHOLDER - Not Implemented
// Planned for: Phase 2 or later
type OllamaProvider struct {
	baseURL   string
	lastUsage *llm.TokenUsage
	usageMu   sync.RWMutex
}

// NewOllamaProvider creates a placeholder Ollama provider.
// Returns an error indicating this provider is not yet implemented.
func NewOllamaProvider(baseURL string) (*OllamaProvider, error) {
	return nil, &errors.ProviderError{
		Provider:   "ollama",
		StatusCode: http.StatusNotImplemented,
		Message:    "Ollama provider not implemented in Phase 1",
		Suggestion: "Use the Anthropic provider or wait for Ollama implementation in Phase 2",
	}
}

// Name returns the provider identifier.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Capabilities returns placeholder capabilities.
func (p *OllamaProvider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		Streaming: true,
		Tools:     false, // Ollama has limited tool support
		Models:    ollamaModels,
	}
}

// Complete is not implemented in Phase 1.
func (p *OllamaProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, &errors.ProviderError{
		Provider:   "ollama",
		StatusCode: http.StatusNotImplemented,
		Message:    "Ollama provider not implemented in Phase 1",
		Suggestion: "Use the Anthropic provider or wait for Ollama implementation in Phase 2",
	}
}

// Stream is not implemented in Phase 1.
func (p *OllamaProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	return nil, &errors.ProviderError{
		Provider:   "ollama",
		StatusCode: http.StatusNotImplemented,
		Message:    "Ollama provider not implemented in Phase 1",
		Suggestion: "Use the Anthropic provider or wait for Ollama implementation in Phase 2",
	}
}

// GetLastUsage returns the token usage from the most recent request.
// Implements the UsageTrackable interface for cost tracking.
// For Ollama, token counts may be estimated when not provided by the API.
func (p *OllamaProvider) GetLastUsage() *llm.TokenUsage {
	p.usageMu.RLock()
	defer p.usageMu.RUnlock()

	if p.lastUsage == nil {
		return nil
	}

	// Return a copy to prevent mutation
	usage := *p.lastUsage
	return &usage
}

// setLastUsage updates the cached usage from a response.
func (p *OllamaProvider) setLastUsage(usage llm.TokenUsage) {
	p.usageMu.Lock()
	defer p.usageMu.Unlock()
	p.lastUsage = &usage
}

// ollamaModels contains placeholder model metadata for Ollama.
// These will be updated when the provider is fully implemented.
// Note: Ollama pricing is $0 as it runs locally.
var ollamaModels = []llm.ModelInfo{
	{
		ID:                    "llama3.1:70b",
		Name:                  "Llama 3.1 70B",
		Tier:                  llm.ModelTierStrategic,
		MaxTokens:             128000,
		MaxOutputTokens:       4096,
		InputPricePerMillion:  0.00, // Local execution
		OutputPricePerMillion: 0.00,
		SupportsTools:         false,
		SupportsVision:        false,
		Description:           "Large local model for complex reasoning.",
	},
	{
		ID:                    "llama3.1:8b",
		Name:                  "Llama 3.1 8B",
		Tier:                  llm.ModelTierBalanced,
		MaxTokens:             128000,
		MaxOutputTokens:       4096,
		InputPricePerMillion:  0.00,
		OutputPricePerMillion: 0.00,
		SupportsTools:         false,
		SupportsVision:        false,
		Description:           "Medium local model for general tasks.",
	},
	{
		ID:                    "phi3:mini",
		Name:                  "Phi-3 Mini",
		Tier:                  llm.ModelTierFast,
		MaxTokens:             4096,
		MaxOutputTokens:       2048,
		InputPricePerMillion:  0.00,
		OutputPricePerMillion: 0.00,
		SupportsTools:         false,
		SupportsVision:        false,
		Description:           "Fast local model for quick responses.",
	},
}
