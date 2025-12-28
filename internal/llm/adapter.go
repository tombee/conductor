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
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/llm/cost"
	"github.com/tombee/conductor/pkg/llm/providers"
	"github.com/tombee/conductor/pkg/llm/providers/claudecode"
)

// ProviderAdapter adapts the full llm.Provider interface to the simpler
// workflow.LLMProvider interface expected by the step executor.
type ProviderAdapter struct {
	provider  llm.Provider
	costStore cost.CostStore
	mu        sync.RWMutex
}

// NewProviderAdapter creates a new adapter wrapping an llm.Provider.
func NewProviderAdapter(provider llm.Provider) *ProviderAdapter {
	return &ProviderAdapter{provider: provider}
}

// SetCostStore sets the cost store for tracking LLM costs.
func (a *ProviderAdapter) SetCostStore(store cost.CostStore) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.costStore = store
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
	startTime := time.Now()
	resp, err := a.provider.Complete(ctx, req)
	duration := time.Since(startTime)

	if err != nil {
		return "", fmt.Errorf("LLM completion failed: %w", err)
	}

	// Record cost after successful completion (non-blocking)
	a.recordCost(ctx, req, resp, duration, options)

	return resp.Content, nil
}

// recordCost records the cost of an LLM request in the cost store.
// This is non-blocking and logs errors without failing the request.
func (a *ProviderAdapter) recordCost(ctx context.Context, req llm.CompletionRequest, resp *llm.CompletionResponse, duration time.Duration, options map[string]interface{}) {
	a.mu.RLock()
	store := a.costStore
	a.mu.RUnlock()

	// Skip if no cost store configured
	if store == nil {
		return
	}

	// Extract context from options (RunID, StepName, WorkflowID)
	runID, _ := options["run_id"].(string)
	stepName, _ := options["step_name"].(string)
	workflowID, _ := options["workflow_id"].(string)

	// Build cost record
	record := llm.CostRecord{
		ID:          uuid.New().String(),
		RequestID:   resp.RequestID,
		RunID:       runID,
		StepName:    stepName,
		WorkflowID:  workflowID,
		Provider:    a.provider.Name(),
		Model:       req.Model,
		Timestamp:   time.Now(),
		Duration:    duration,
		Usage:       resp.Usage,
		// Cost calculation could be added here based on pricing tables
		// For now, record Usage which can be used for cost calculation later
		Cost: nil,
	}

	// Store the record (async to avoid blocking execution)
	go func() {
		if err := store.Store(context.Background(), record); err != nil {
			// Log warning but don't fail the request
			slog.Warn("failed to store cost record",
				slog.String("error", err.Error()),
				slog.String("run_id", runID),
				slog.String("step_name", stepName))
		}
	}()
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

	case "openai":
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("openai provider requires api_key in config")
		}
		baseProvider, err = providers.NewOpenAIProvider(providerCfg.APIKey)
		if err != nil {
			return nil, err
		}

	case "ollama":
		// Ollama doesn't require API key, uses localhost by default
		// The provider itself handles the baseURL
		baseProvider, err = providers.NewOllamaProvider("http://localhost:11434")
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
