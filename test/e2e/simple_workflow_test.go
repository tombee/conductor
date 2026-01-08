package e2e

import (
	"testing"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/sdk"
	"github.com/tombee/conductor/test/e2e/harness"
)

func TestSimpleLLMWorkflow(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(harness.MockResponse{
			Content: "This is a test response from the mock LLM.",
			TokenUsage: llm.TokenUsage{
				InputTokens:  15,
				OutputTokens: 10,
			},
		}),
	)

	wf := h.LoadWorkflow("testdata/simple_llm.yaml")

	result := h.Run(wf, map[string]any{
		"prompt": "Test prompt",
	})

	// Assert workflow succeeded
	h.AssertSuccess(t, result)

	// Assert step output contains expected content
	h.AssertStepOutput(t, result, "generate", "test response")

	// Assert token usage
	h.AssertTokenUsage(t, result, 20, 30) // Should be 25 total (15 in + 10 out)

	// Verify mock provider recorded the request
	provider := h.GetMockProvider()
	requests := provider.GetRequests()
	if len(requests) != 1 {
		t.Errorf("expected 1 request, got %d", len(requests))
	}
}

func TestSimpleLLMWorkflow_WithEvents(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(harness.MockResponse{
			Content: "Event test response",
		}),
		harness.WithEventCapture(),
	)

	wf := h.LoadWorkflow("testdata/simple_llm.yaml")

	result := h.Run(wf, map[string]any{
		"prompt": "Event test",
	})

	h.AssertSuccess(t, result)

	// Verify events were captured
	events := h.Events()
	if len(events) == 0 {
		t.Error("expected events to be captured")
	}

	// Check for workflow started and completed events
	h.AssertEventCount(t, sdk.EventWorkflowStarted, 1)
	h.AssertEventCount(t, sdk.EventWorkflowCompleted, 1)
}

func TestSimpleLLMWorkflow_MissingInput(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(harness.MockResponse{
			Content: "Should not be used",
		}),
	)

	wf := h.LoadWorkflow("testdata/simple_llm.yaml")

	// Run without required input - should fail validation
	err := h.RunExpectError(wf, map[string]any{})

	if err == nil {
		t.Fatal("expected error for missing input")
	}

	t.Logf("Got expected error: %v", err)
}

func TestSimpleLLMWorkflow_OutputMapping(t *testing.T) {
	expectedResponse := "Unique test output 12345"

	h := harness.New(t,
		harness.WithMockProvider(harness.MockResponse{
			Content: expectedResponse,
			TokenUsage: llm.TokenUsage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}),
	)

	wf := h.LoadWorkflow("testdata/simple_llm.yaml")

	result := h.Run(wf, map[string]any{
		"prompt": "Test",
	})

	h.AssertSuccess(t, result)

	// Verify output mapping
	if result.Output == nil {
		t.Fatal("result.Output is nil")
	}

	t.Logf("Available outputs: %+v", result.Output)

	// SDK uses "response" as the default output key
	resultValue, ok := result.Output["response"]
	if !ok {
		t.Fatal("output 'response' not found")
	}

	if resultValue != expectedResponse {
		t.Errorf("expected output %q, got %q", expectedResponse, resultValue)
	}
}

func TestSimpleLLMWorkflow_EmptyResponse(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(harness.MockResponse{
			Content: "", // Empty response
			TokenUsage: llm.TokenUsage{
				InputTokens:  10,
				OutputTokens: 0,
			},
		}),
	)

	wf := h.LoadWorkflow("testdata/simple_llm.yaml")

	result := h.Run(wf, map[string]any{
		"prompt": "Test",
	})

	// Should still succeed even with empty response
	h.AssertSuccess(t, result)

	// Verify step was completed
	h.AssertStepStatus(t, result, "generate", sdk.StepStatusSuccess)
}
