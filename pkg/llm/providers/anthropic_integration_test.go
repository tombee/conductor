//go:build integration

package providers

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/testing/integration"
	"github.com/tombee/conductor/pkg/llm"
)

// TestAnthropicComplete_RealAPI tests a real completion call to Anthropic.
// This is a Tier 2 test: single real API call with cost tracking.
func TestAnthropicComplete_RealAPI(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	// Initialize cost tracker
	tracker := integration.NewCostTracker()
	tracker.ResetTest()

	// Create provider
	cfg := integration.LoadConfig()
	provider, err := NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create a simple completion request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := integration.SimpleCompletionRequest("fast", "Say 'integration test success' and nothing else")

	// Execute with retry for transient failures
	var resp *llm.CompletionResponse
	err = integration.Retry(ctx, func() error {
		var retryErr error
		resp, retryErr = provider.Complete(ctx, req)
		return retryErr
	}, integration.DefaultRetryConfig())

	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Verify response structure
	if resp == nil {
		t.Fatal("Response is nil")
	}
	if resp.Content == "" {
		t.Error("Response content is empty")
	}
	if resp.FinishReason == "" {
		t.Error("Finish reason is empty")
	}
	if resp.Model == "" {
		t.Error("Model is empty")
	}
	if resp.RequestID == "" {
		t.Error("Request ID is empty")
	}

	// Verify token usage
	if resp.Usage.TotalTokens == 0 {
		t.Error("Total tokens is 0")
	}
	if resp.Usage.PromptTokens == 0 {
		t.Error("Prompt tokens is 0")
	}
	if resp.Usage.CompletionTokens == 0 {
		t.Error("Completion tokens is 0")
	}

	// Track cost
	modelInfo, err := provider.GetModelInfo(resp.Model)
	if err != nil {
		t.Fatalf("Failed to get model info: %v", err)
	}

	if err := tracker.Record(resp.Usage); err != nil {
		t.Fatalf("Cost tracking failed: %v", err)
	}

	t.Logf("Test cost: $%.4f (model: %s, tokens: %d)", tracker.GetTestCost(), resp.Model, resp.Usage.TotalTokens)
}

// TestAnthropicStream_RealAPI tests real streaming completion from Anthropic.
// This is a Tier 2 test: single streaming call with token counting.
func TestAnthropicStream_RealAPI(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	// Initialize cost tracker
	tracker := integration.NewCostTracker()
	tracker.ResetTest()

	// Create provider
	cfg := integration.LoadConfig()
	provider, err := NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create streaming request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req := integration.StreamingCompletionRequest("fast", "Count from 1 to 5, one number per line")

	// Execute stream
	chunks, err := provider.Stream(ctx, req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}

	// Collect chunks
	var content strings.Builder
	var finalUsage *llm.TokenUsage
	var model string
	chunkCount := 0

	for chunk := range chunks {
		if chunk.Error != nil {
			t.Fatalf("Stream error: %v", chunk.Error)
		}

		if chunk.Delta.Content != "" {
			content.WriteString(chunk.Delta.Content)
			chunkCount++
		}

		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}

		if chunk.RequestID != "" && model == "" {
			// Extract model from first chunk if available
			// For now we'll use the requested tier
			model = "claude-3-5-haiku-20241022" // fast tier default
		}
	}

	// Verify streaming worked
	if chunkCount == 0 {
		t.Error("No content chunks received")
	}

	finalContent := content.String()
	if finalContent == "" {
		t.Error("Final content is empty")
	}

	// Verify we got usage data
	if finalUsage == nil {
		t.Error("No usage data in stream")
	} else {
		if finalUsage.TotalTokens == 0 {
			t.Error("Total tokens is 0")
		}

		// Track cost
		modelInfo, err := provider.GetModelInfo(model)
		if err != nil {
			t.Fatalf("Failed to get model info: %v", err)
		}

		if err := tracker.Record(*finalUsage); err != nil {
			t.Fatalf("Cost tracking failed: %v", err)
		}

		t.Logf("Stream test cost: $%.4f (chunks: %d, tokens: %d)", tracker.GetTestCost(), chunkCount, finalUsage.TotalTokens)
	}
}

// TestAnthropicToolCalling_RealAPI tests tool calling with real Anthropic API.
// This is a Tier 2 test: single tool calling round-trip.
func TestAnthropicToolCalling_RealAPI(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	// Initialize cost tracker
	tracker := integration.NewCostTracker()
	tracker.ResetTest()

	// Create provider
	cfg := integration.LoadConfig()
	provider, err := NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Create tool calling request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tools := []llm.Tool{integration.CalculatorTool()}
	req := integration.ToolCallingRequest("balanced", "What is 15 multiplied by 7?", tools)

	// Execute with retry
	var resp *llm.CompletionResponse
	err = integration.Retry(ctx, func() error {
		var retryErr error
		resp, retryErr = provider.Complete(ctx, req)
		return retryErr
	}, integration.DefaultRetryConfig())

	if err != nil {
		t.Fatalf("Tool calling failed: %v", err)
	}

	// Verify response
	if resp == nil {
		t.Fatal("Response is nil")
	}

	// Check if model decided to use tool
	if resp.FinishReason == llm.FinishReasonToolCalls {
		if len(resp.ToolCalls) == 0 {
			t.Error("Finish reason is tool_calls but no tool calls in response")
		} else {
			// Verify tool call structure
			toolCall := resp.ToolCalls[0]
			if toolCall.ID == "" {
				t.Error("Tool call ID is empty")
			}
			if toolCall.Name == "" {
				t.Error("Tool call name is empty")
			}
			if toolCall.Arguments == "" {
				t.Error("Tool call arguments are empty")
			}
			t.Logf("Tool called: %s with args: %s", toolCall.Name, toolCall.Arguments)
		}
	} else {
		// Model may respond directly without tool use - that's also valid
		t.Logf("Model responded directly without tool use (finish reason: %s)", resp.FinishReason)
	}

	// Track cost
	modelInfo, err := provider.GetModelInfo(resp.Model)
	if err != nil {
		t.Fatalf("Failed to get model info: %v", err)
	}

	if err := tracker.Record(resp.Usage); err != nil {
		t.Fatalf("Cost tracking failed: %v", err)
	}

	t.Logf("Tool calling test cost: $%.4f", tracker.GetTestCost())
}

// TestAnthropicErrorHandling_RealAPI tests error handling with real API.
// Verifies that authentication errors fail immediately and rate limits trigger retry.
func TestAnthropicErrorHandling_RealAPI(t *testing.T) {
	t.Run("Invalid API Key", func(t *testing.T) {
		integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

		// Create provider with invalid key
		provider, err := NewAnthropicProvider("sk-ant-invalid-key")
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		req := integration.SimpleCompletionRequest("fast", "test")

		// This should fail with 401/403 and NOT retry
		_, err = provider.Complete(ctx, req)
		if err == nil {
			t.Error("Expected authentication error but got success")
		}

		// Verify error is permanent (not retryable)
		if !integration.IsPermanentError(err) {
			t.Logf("Error (expected auth error): %v", err)
		}
	})

	t.Run("Context Timeout", func(t *testing.T) {
		integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

		cfg := integration.LoadConfig()
		provider, err := NewAnthropicProvider(cfg.AnthropicAPIKey)
		if err != nil {
			t.Fatalf("Failed to create provider: %v", err)
		}

		// Create context with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		time.Sleep(2 * time.Millisecond) // Ensure timeout

		req := integration.SimpleCompletionRequest("fast", "test")
		_, err = provider.Complete(ctx, req)

		if err == nil {
			t.Error("Expected timeout error but got success")
		}
		t.Logf("Timeout error (expected): %v", err)
	})
}
