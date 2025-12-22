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
			expected: "claude-3-5-haiku-20241022",
		},
		{
			name:     "balanced tier",
			input:    "balanced",
			expected: "claude-sonnet-4-20250514",
		},
		{
			name:     "strategic tier",
			input:    "strategic",
			expected: "claude-opus-4-20250514",
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

	prompt := p.buildPrompt(messages)

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
		if !contains(prompt, part) {
			t.Errorf("buildPrompt() missing expected part: %q", part)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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

	if err != nil && !contains(err.Error(), "failed") && !contains(err.Error(), "not found") {
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

		if !contains(result.Message, "auth") && !contains(result.Message, "login") {
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
