// Package providers contains concrete implementations of LLM providers.
package providers

import (
	"context"
	"errors"
	"sync"

	"github.com/tombee/conductor/pkg/llm"
)

// OpenAIProvider is a placeholder for the OpenAI provider implementation.
// This will be implemented in a future phase.
//
// Phase 1 Status: PLACEHOLDER - Not Implemented
// Planned for: Phase 2 or later
type OpenAIProvider struct {
	apiKey    string
	lastUsage *llm.TokenUsage
	usageMu   sync.RWMutex
}

// NewOpenAIProvider creates a placeholder OpenAI provider.
// Returns an error indicating this provider is not yet implemented.
func NewOpenAIProvider(apiKey string) (*OpenAIProvider, error) {
	return nil, errors.New("OpenAI provider not implemented in Phase 1")
}

// Name returns the provider identifier.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// Capabilities returns placeholder capabilities.
func (p *OpenAIProvider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		Streaming: true,
		Tools:     true,
		Models:    openAIModels,
	}
}

// Complete is not implemented in Phase 1.
func (p *OpenAIProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, errors.New("OpenAI provider not implemented in Phase 1")
}

// Stream is not implemented in Phase 1.
func (p *OpenAIProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	return nil, errors.New("OpenAI provider not implemented in Phase 1")
}

// GetLastUsage returns the token usage from the most recent request.
// Implements the UsageTrackable interface for cost tracking.
func (p *OpenAIProvider) GetLastUsage() *llm.TokenUsage {
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
func (p *OpenAIProvider) setLastUsage(usage llm.TokenUsage) {
	p.usageMu.Lock()
	defer p.usageMu.Unlock()
	p.lastUsage = &usage
}

// openAIModels contains placeholder model metadata for OpenAI.
// These will be updated when the provider is fully implemented.
var openAIModels = []llm.ModelInfo{
	{
		ID:                    "gpt-4-turbo",
		Name:                  "GPT-4 Turbo",
		Tier:                  llm.ModelTierStrategic,
		MaxTokens:             128000,
		MaxOutputTokens:       4096,
		InputPricePerMillion:  10.00,
		OutputPricePerMillion: 30.00,
		SupportsTools:         true,
		SupportsVision:        true,
		Description:           "Most capable GPT-4 model for complex tasks.",
	},
	{
		ID:                    "gpt-4",
		Name:                  "GPT-4",
		Tier:                  llm.ModelTierBalanced,
		MaxTokens:             8192,
		MaxOutputTokens:       4096,
		InputPricePerMillion:  30.00,
		OutputPricePerMillion: 60.00,
		SupportsTools:         true,
		SupportsVision:        false,
		Description:           "Balanced model for most tasks.",
	},
	{
		ID:                    "gpt-3.5-turbo",
		Name:                  "GPT-3.5 Turbo",
		Tier:                  llm.ModelTierFast,
		MaxTokens:             16385,
		MaxOutputTokens:       4096,
		InputPricePerMillion:  0.50,
		OutputPricePerMillion: 1.50,
		SupportsTools:         true,
		SupportsVision:        false,
		Description:           "Fast and cost-effective for simple tasks.",
	},
}
