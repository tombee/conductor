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

package claudecode

import (
	"context"
	"strings"
	"testing"

	"github.com/tombee/conductor/pkg/llm"
)

func TestProvider_Name(t *testing.T) {
	p := New()
	if p.Name() != "claude-code" {
		t.Errorf("expected name 'claude-code', got %q", p.Name())
	}
}

func TestProvider_Capabilities(t *testing.T) {
	p := New()
	caps := p.Capabilities()

	if !caps.Streaming {
		t.Error("expected streaming to be supported")
	}

	if !caps.Tools {
		t.Error("expected tools to be supported")
	}

	if len(caps.Models) == 0 {
		t.Error("expected models to be listed")
	}
}

func TestProvider_Detect(t *testing.T) {
	p := New()

	// This test will pass or fail based on whether Claude CLI is installed
	// We're just checking that Detect() doesn't panic
	found, err := p.Detect()
	if err != nil {
		t.Errorf("Detect() returned error: %v", err)
	}

	// If found, verify that cliCommand was set
	if found && p.cliCommand == "" {
		t.Error("Detect() returned true but cliCommand not set")
	}
}

func TestProvider_HealthCheck(t *testing.T) {
	p := New()
	ctx := context.Background()

	// This test will vary based on local environment
	// We're just checking that HealthCheck() doesn't panic
	result := p.HealthCheck(ctx)

	// Verify that result structure is populated correctly
	if result.Installed && result.Authenticated && result.Working {
		if !result.Healthy() {
			t.Error("all checks passed but Healthy() returned false")
		}
	}

	if !result.Installed && result.ErrorStep != llm.HealthCheckStepInstalled {
		t.Error("installation failed but ErrorStep not set to installed")
	}
}

func TestProvider_ResolveModel(t *testing.T) {
	p := New()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "fast tier",
			input:    "fast",
			expected: "haiku",
		},
		{
			name:     "balanced tier",
			input:    "balanced",
			expected: "sonnet",
		},
		{
			name:     "strategic tier",
			input:    "strategic",
			expected: "opus",
		},
		{
			name:     "explicit model ID",
			input:    "claude-3-opus-20240229",
			expected: "claude-3-opus-20240229",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.resolveModel(tt.input)
			if result != tt.expected {
				t.Errorf("resolveModel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestProvider_BuildPrompt(t *testing.T) {
	p := New()

	messages := []llm.Message{
		{Role: llm.MessageRoleSystem, Content: "You are a helpful assistant."},
		{Role: llm.MessageRoleUser, Content: "Hello!"},
		{Role: llm.MessageRoleAssistant, Content: "Hi there!"},
		{Role: llm.MessageRoleUser, Content: "How are you?"},
	}

	prompt := p.buildPrompt(messages, nil)

	// Verify all messages are included
	if prompt == "" {
		t.Error("buildPrompt returned empty string")
	}

	expectedParts := []string{
		"System: You are a helpful assistant.",
		"User: Hello!",
		"Assistant: Hi there!",
		"User: How are you?",
	}

	for _, part := range expectedParts {
		if !strings.Contains(prompt, part) {
			t.Errorf("buildPrompt() missing expected part: %q", part)
		}
	}
}

func TestProvider_Complete_CLINotFound(t *testing.T) {
	p := New()
	// Set an invalid CLI command
	p.cliCommand = "nonexistent-claude-cli-binary"

	ctx := context.Background()
	req := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello"},
		},
	}

	_, err := p.Complete(ctx, req)
	if err == nil {
		t.Error("expected error when CLI not found, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "failed") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error about CLI failure, got: %v", err)
	}
}

func TestHealthCheck_AuthenticationFails(t *testing.T) {
	// This test assumes claude CLI is not installed or not authenticated
	// It validates the error handling path
	p := New()
	ctx := context.Background()

	result := p.HealthCheck(ctx)

	// The health check should complete without panic
	// If installed but not authenticated, we should get an appropriate error
	if result.Installed && !result.Authenticated {
		if result.ErrorStep != llm.HealthCheckStepAuthenticated {
			t.Error("authentication failure should set ErrorStep to Authenticated")
		}

		if !strings.Contains(result.Message, "auth") && !strings.Contains(result.Message, "login") {
			t.Error("authentication failure message should mention auth or login")
		}
	}
}

func TestHealthCheck_ConnectivityFails(t *testing.T) {
	// This test checks that connectivity failures are handled
	// In practice, this is hard to test without mocking the CLI
	// So we just verify the structure of results
	p := New()
	ctx := context.Background()

	result := p.HealthCheck(ctx)

	// Verify that if working=false but installed and authenticated,
	// the error step is correct
	if result.Installed && result.Authenticated && !result.Working {
		if result.ErrorStep != llm.HealthCheckStepWorking {
			t.Error("connectivity failure should set ErrorStep to Working")
		}
	}
}

func TestHealthCheck_UnknownCommand(t *testing.T) {
	// Test that we handle the case where "claude auth status" is not available
	// This is tested implicitly by the checkAuthentication logic
	// Here we just verify it doesn't panic
	p := New()
	ctx := context.Background()

	_ = p.HealthCheck(ctx)
	// Test passes if no panic occurs
}

func TestProvider_BuildCLIArgs_WithTools(t *testing.T) {
	p := New()

	tests := []struct {
		name          string
		req           llm.CompletionRequest
		wantMCPConfig bool
	}{
		{
			name: "with tools - should add MCP config",
			req: llm.CompletionRequest{
				Model: "claude-sonnet-4-20250514",
				Messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "Hello"},
				},
				Tools: []llm.Tool{
					{Name: "file.read"},
					{Name: "shell.run"},
				},
			},
			wantMCPConfig: true,
		},
		{
			name: "nil tools - no MCP config",
			req: llm.CompletionRequest{
				Model: "claude-sonnet-4-20250514",
				Messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "Hello"},
				},
				Tools: nil,
			},
			wantMCPConfig: false,
		},
		{
			name: "empty tools slice - no MCP config",
			req: llm.CompletionRequest{
				Model: "claude-sonnet-4-20250514",
				Messages: []llm.Message{
					{Role: llm.MessageRoleUser, Content: "Hello"},
				},
				Tools: []llm.Tool{},
			},
			wantMCPConfig: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := p.buildCLIArgs(tt.req, false)

			hasMCPConfig := false

			for i, arg := range args {
				if arg == "--mcp-config" {
					hasMCPConfig = true
					// Check that next arg is a JSON config containing conductor MCP server
					if i+1 < len(args) {
						cfg := args[i+1]
						if !strings.Contains(cfg, "conductor") || !strings.Contains(cfg, "mcp-server") {
							t.Errorf("--mcp-config should contain conductor mcp-server config, got %q", cfg)
						}
					}
				}
			}

			if hasMCPConfig != tt.wantMCPConfig {
				t.Errorf("--mcp-config presence = %v, want %v", hasMCPConfig, tt.wantMCPConfig)
			}
		})
	}
}

func TestProvider_NewWithModels(t *testing.T) {
	customModels := llm.ModelTierMap{
		Fast:      "claude-custom-fast",
		Balanced:  "claude-custom-balanced",
		Strategic: "claude-custom-strategic",
	}

	p := NewWithModels(customModels)

	tests := []struct {
		tier     string
		expected string
	}{
		{"fast", "claude-custom-fast"},
		{"balanced", "claude-custom-balanced"},
		{"strategic", "claude-custom-strategic"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			result := p.resolveModel(tt.tier)
			if result != tt.expected {
				t.Errorf("resolveModel(%q) = %q, want %q", tt.tier, result, tt.expected)
			}
		})
	}
}

func TestProvider_Stream_CLINotFound(t *testing.T) {
	p := New()
	// Set an invalid CLI command
	p.cliCommand = "nonexistent-claude-cli-binary"

	ctx := context.Background()
	req := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello"},
		},
	}

	_, err := p.Stream(ctx, req)
	if err == nil {
		t.Error("expected error when CLI not found, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "failed") {
		t.Errorf("expected error about CLI failure, got: %v", err)
	}
}

func TestProvider_Stream_ContextCancel(t *testing.T) {
	p := New()

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := llm.CompletionRequest{
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "Hello"},
		},
	}

	// Ensure CLI is detected first (so we don't fail on detection)
	if found, _ := p.Detect(); !found {
		t.Skip("Claude CLI not found, skipping context cancellation test")
	}

	chunks, err := p.Stream(ctx, req)
	if err != nil {
		// Error at start is acceptable for cancelled context
		return
	}

	// If we got a channel, verify it handles cancellation
	for chunk := range chunks {
		if chunk.Error != nil {
			// Context cancellation error is expected
			if strings.Contains(chunk.Error.Error(), "context") {
				return
			}
		}
	}
}
