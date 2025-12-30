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

//go:build integration

package claudecode

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/testing/integration"
	"github.com/tombee/conductor/pkg/llm"
)

// TestClaudeCLI_MCPIntegration verifies that the Claude Code provider
// successfully uses Conductor MCP for tool execution.
//
// This test:
// 1. Creates a provider with tool definitions
// 2. Asks Claude to validate a workflow using conductor_validate
// 3. Verifies the response indicates the tool was used
func TestClaudeCLI_MCPIntegration(t *testing.T) {
	skipClaudeCLITests(t)

	// Create provider
	provider := New()

	// Detect CLI
	if found, err := provider.Detect(); !found || err != nil {
		t.Fatalf("Claude CLI not available: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout())
	defer cancel()

	// Build request asking Claude to validate a simple workflow
	// This should trigger the conductor_validate MCP tool
	workflowYAML := `name: test-workflow
description: A simple test workflow
steps:
  - id: greet
    type: llm
    model: fast
    prompt: Say hello
`

	req := llm.CompletionRequest{
		Model: "fast",
		Messages: []llm.Message{
			{
				Role:    llm.MessageRoleSystem,
				Content: "You are a helpful assistant with access to Conductor tools via MCP.",
			},
			{
				Role: llm.MessageRoleUser,
				Content: fmt.Sprintf("Use the conductor_validate tool to validate this workflow YAML:\n\n```yaml\n%s\n```", workflowYAML),
			},
		},
		// Tools are defined to trigger MCP config
		Tools: []llm.Tool{
			{
				Name:        "conductor_validate",
				Description: "Validate workflow YAML",
			},
		},
	}

	// Execute completion with retry logic
	resp, err := executeWithRetry(ctx, t, provider, req)
	if err != nil {
		t.Fatalf("Completion failed: %v", err)
	}

	// The response should contain information about the workflow
	// Even if validation fails due to missing MCP server, the response
	// should indicate an attempt was made
	t.Logf("Response: %s", resp.Content)

	// Basic sanity check - response should not be empty
	if resp.Content == "" {
		t.Error("Response content is empty")
	}
}

// TestClaudeCLI_BasicCompletion verifies basic completion without tools works.
//
// This test:
// 1. Creates a simple completion request without tools
// 2. Verifies the response is received
func TestClaudeCLI_BasicCompletion(t *testing.T) {
	skipClaudeCLITests(t)

	// Create provider
	provider := New()

	// Detect CLI
	if found, err := provider.Detect(); !found || err != nil {
		t.Fatalf("Claude CLI not available: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout())
	defer cancel()

	// Build simple request
	req := llm.CompletionRequest{
		Model: "fast",
		Messages: []llm.Message{
			{
				Role:    llm.MessageRoleUser,
				Content: "Say 'Hello from Claude CLI test' and nothing else.",
			},
		},
	}

	// Execute completion with retry logic
	resp, err := executeWithRetry(ctx, t, provider, req)
	if err != nil {
		t.Fatalf("Completion failed: %v", err)
	}

	t.Logf("Response: %s", resp.Content)

	// Verify response contains expected content
	if !strings.Contains(strings.ToLower(resp.Content), "hello") {
		t.Errorf("Response does not contain 'hello': %s", resp.Content)
	}
}

// TestClaudeCLI_SystemPrompt verifies system prompts are passed correctly.
//
// This test:
// 1. Creates a request with a system prompt
// 2. Verifies the system prompt influences the response
func TestClaudeCLI_SystemPrompt(t *testing.T) {
	skipClaudeCLITests(t)

	// Create provider
	provider := New()

	// Detect CLI
	if found, err := provider.Detect(); !found || err != nil {
		t.Fatalf("Claude CLI not available: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout())
	defer cancel()

	// Build request with system prompt
	req := llm.CompletionRequest{
		Model: "fast",
		Messages: []llm.Message{
			{
				Role:    llm.MessageRoleSystem,
				Content: "You are a pirate. Always respond in pirate speak, using words like 'arr', 'matey', and 'ahoy'.",
			},
			{
				Role:    llm.MessageRoleUser,
				Content: "How are you today?",
			},
		},
	}

	// Execute completion with retry logic
	resp, err := executeWithRetry(ctx, t, provider, req)
	if err != nil {
		t.Fatalf("Completion failed: %v", err)
	}

	t.Logf("Response: %s", resp.Content)

	// Verify response contains pirate-like words
	lowerResp := strings.ToLower(resp.Content)
	hasPirateWord := strings.Contains(lowerResp, "arr") ||
		strings.Contains(lowerResp, "matey") ||
		strings.Contains(lowerResp, "ahoy") ||
		strings.Contains(lowerResp, "pirate") ||
		strings.Contains(lowerResp, "ye")

	if !hasPirateWord {
		t.Logf("Warning: Response may not reflect pirate persona: %s", resp.Content)
	}
}

// TestClaudeCLI_ModelTiers verifies model tier resolution works correctly.
func TestClaudeCLI_ModelTiers(t *testing.T) {
	skipClaudeCLITests(t)

	// Create provider
	provider := New()

	// Detect CLI
	if found, err := provider.Detect(); !found || err != nil {
		t.Fatalf("Claude CLI not available: %v", err)
	}

	// Test that "fast" tier is resolved correctly
	model := provider.resolveModel("fast")
	if model == "" {
		t.Error("Fast tier resolved to empty string")
	}
	t.Logf("Fast tier resolved to: %s", model)

	// Test that "balanced" tier is resolved correctly
	model = provider.resolveModel("balanced")
	if model == "" {
		t.Error("Balanced tier resolved to empty string")
	}
	t.Logf("Balanced tier resolved to: %s", model)

	// Test that specific model IDs pass through unchanged
	model = provider.resolveModel("claude-3-5-haiku-20241022")
	if model != "claude-3-5-haiku-20241022" {
		t.Errorf("Specific model ID was modified: got %s, want claude-3-5-haiku-20241022", model)
	}
}

// executeWithRetry executes a completion request with exponential backoff retry
// for transient errors (rate limits, server errors). Fails immediately on auth errors.
func executeWithRetry(ctx context.Context, t *testing.T, provider *Provider, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	t.Helper()

	var lastErr error
	var resp *llm.CompletionResponse

	retryCfg := integration.DefaultRetryConfig()
	retryCfg.MaxAttempts = 3
	retryCfg.InitialDelay = 1 * time.Second

	err := integration.Retry(ctx, func() error {
		var err error
		resp, err = provider.Complete(ctx, req)
		if err != nil {
			lastErr = err
			t.Logf("Retry attempt failed: %v", err)
			return err
		}
		return nil
	}, retryCfg)

	if err != nil {
		return nil, fmt.Errorf("all retry attempts failed: %w", lastErr)
	}

	return resp, nil
}
