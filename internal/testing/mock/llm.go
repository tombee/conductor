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

package mock

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/tombee/conductor/internal/testing/fixture"
	"github.com/tombee/conductor/pkg/workflow"
)

// LLMProvider is a mock LLM provider that returns fixture-based responses.
type LLMProvider struct {
	fixtureLoader *fixture.Loader
	realProvider  workflow.LLMProvider
	logger        *slog.Logger
}

// NewLLMProvider creates a new mock LLM provider.
func NewLLMProvider(loader *fixture.Loader, realProvider workflow.LLMProvider, logger *slog.Logger) *LLMProvider {
	return &LLMProvider{
		fixtureLoader: loader,
		realProvider:  realProvider,
		logger:        logger,
	}
}

// Complete returns a fixture-based completion response.
func (m *LLMProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (*workflow.CompletionResult, error) {
	// Extract step ID from context
	stepID := extractStepIDFromContext(ctx)
	m.logger.Info("[MOCK] LLM completion request", "step_id", stepID)

	// Load fixture for this step
	fixtureData, err := m.fixtureLoader.LoadLLMFixture(stepID)
	if err != nil {
		// If no fixture found and we have a real provider, fall back to it
		if m.realProvider != nil {
			m.logger.Debug("[MOCK] No fixture found, using real provider", "step_id", stepID, "error", err)
			return m.realProvider.Complete(ctx, prompt, options)
		}
		return nil, fmt.Errorf("mock mode: %w", err)
	}

	// Find matching response
	responseText, err := m.findMatchingResponse(fixtureData, stepID, prompt, options)
	if err != nil {
		return nil, err
	}

	// Return as CompletionResult (mock usage is nil - not applicable for mocks)
	return &workflow.CompletionResult{
		Content: responseText,
		Model:   "mock",
	}, nil
}

// findMatchingResponse finds the appropriate response from fixture data.
func (m *LLMProvider) findMatchingResponse(fixtureData *fixture.LLMFixture, stepID, prompt string, options map[string]interface{}) (string, error) {
	// If simple response is set, use it
	if fixtureData.Response != "" {
		return fixtureData.Response, nil
	}

	// Try conditional responses
	var defaultResponse *fixture.LLMResponse
	for i := range fixtureData.Responses {
		resp := &fixtureData.Responses[i]

		// Check if this is the default response
		if resp.Default {
			defaultResponse = resp
			continue
		}

		// Check conditions
		if resp.When != nil {
			// Check step ID match
			if resp.When.StepID != "" && resp.When.StepID != stepID {
				continue
			}

			// Check prompt contains
			if resp.When.PromptContains != "" {
				promptText := extractPromptText(prompt, options)
				if !strings.Contains(strings.ToLower(promptText), strings.ToLower(resp.When.PromptContains)) {
					continue
				}
			}
		}

		// All conditions matched, use this response
		m.logger.Debug("[MOCK] Matched conditional response", "step_id", stepID)
		return resp.Return, nil
	}

	// Use default response if available
	if defaultResponse != nil {
		m.logger.Debug("[MOCK] Using default response", "step_id", stepID)
		return defaultResponse.Return, nil
	}

	// No matching response found
	return "", fmt.Errorf("no matching response in fixture for step %q", stepID)
}

// extractPromptText extracts the full prompt text including system prompts.
func extractPromptText(prompt string, options map[string]interface{}) string {
	var parts []string

	// Add system prompt if present
	if system, ok := options["system"].(string); ok && system != "" {
		parts = append(parts, system)
	}

	// Add main prompt
	parts = append(parts, prompt)

	return strings.Join(parts, "\n")
}

// extractStepIDFromContext extracts the step ID from the context.
// The step ID should be set in the context by the workflow executor.
func extractStepIDFromContext(ctx context.Context) string {
	// Try to get step ID from context value
	if stepID, ok := ctx.Value(stepIDKey{}).(string); ok {
		return stepID
	}
	return "unknown"
}

// stepIDKey is a type for context keys to avoid collisions.
type stepIDKey struct{}

// WithStepID returns a new context with the step ID set.
func WithStepID(ctx context.Context, stepID string) context.Context {
	return context.WithValue(ctx, stepIDKey{}, stepID)
}
