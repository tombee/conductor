// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package llm provides LLM integration utilities for internal use.
package llm

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/providers"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

// ProviderAdapter adapts the full llm.Provider interface to the simpler
// workflow.LLMProvider interface expected by the step executor.
type ProviderAdapter struct {
	provider llm.Provider
}

// NewProviderAdapter creates a new adapter wrapping an llm.Provider.
func NewProviderAdapter(provider llm.Provider) *ProviderAdapter {
	return &ProviderAdapter{provider: provider}
}

// Complete implements workflow.LLMProvider interface.
// It converts the simple interface to the full llm.CompletionRequest.
func (a *ProviderAdapter) Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
	// Build messages from prompt
	messages := []llm.Message{
		{Role: llm.MessageRoleUser, Content: prompt},
	}

	// Handle system prompt if provided
	if system, ok := options["system"].(string); ok && system != "" {
		messages = append([]llm.Message{
			{Role: llm.MessageRoleSystem, Content: system},
		}, messages...)
	}

	// Build the request
	req := llm.CompletionRequest{
		Messages: messages,
	}

	// Handle model option
	if model, ok := options["model"].(string); ok {
		req.Model = model
	}

	// Handle temperature option
	if temp, ok := options["temperature"].(float64); ok {
		req.Temperature = &temp
	}

	// Handle max_tokens option
	if maxTokens, ok := options["max_tokens"].(int); ok {
		req.MaxTokens = &maxTokens
	}

	// Make the completion request
	resp, err := a.provider.Complete(ctx, req)

	if err != nil {
		return "", fmt.Errorf("LLM completion failed: %w", err)
	}

	return resp.Content, nil
}

// CreateProvider creates an llm.Provider from config.
// It instantiates the appropriate provider based on the provider type in the config,
// and optionally wraps it with retry and failover logic based on LLM configuration.
func CreateProvider(cfg *config.Config, providerName string) (llm.Provider, error) {
	providerCfg, exists := cfg.Providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %q not found in config", providerName)
	}

	// Create the base provider
	var baseProvider llm.Provider
	var err error

	switch providerCfg.Type {
	case "claude-code":
		var p *claudecode.Provider
		// Check if any model tiers are configured
		hasModels := providerCfg.Models.Fast != "" || providerCfg.Models.Balanced != "" || providerCfg.Models.Strategic != ""
		if hasModels {
			p = claudecode.NewWithModels(providerCfg.Models)
		} else {
			p = claudecode.New()
		}
		// Verify the CLI is available
		if found, err := p.Detect(); !found {
			if err != nil {
				return nil, fmt.Errorf("claude-code provider not available: %w", err)
			}
			return nil, fmt.Errorf("claude CLI not found in PATH")
		}
		baseProvider = p

	case "anthropic":
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("anthropic provider requires api_key in config")
		}
		baseProvider, err = providers.NewAnthropicProvider(providerCfg.APIKey)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", providerCfg.Type)
	}

	// Wrap with retry logic based on LLMConfig
	provider := wrapWithRetry(baseProvider, cfg.LLM)

	return provider, nil
}

// wrapWithRetry wraps a provider with retry logic based on LLM configuration.
func wrapWithRetry(provider llm.Provider, llmCfg config.LLMConfig) llm.Provider {
	// Build retry config from LLMConfig
	retryConfig := llm.RetryConfig{
		MaxRetries:      llmCfg.MaxRetries,
		InitialDelay:    llmCfg.RetryBackoffBase,
		MaxDelay:        10 * llmCfg.RetryBackoffBase, // 10x base as max
		Multiplier:      2.0,                           // Exponential backoff
		Jitter:          0.1,                           // 10% jitter
		AbsoluteTimeout: 2 * llmCfg.RequestTimeout,     // 2x request timeout as absolute max
		RetryableErrors: nil,                           // Use default retry logic
	}

	return llm.NewRetryableProvider(provider, retryConfig)
}
