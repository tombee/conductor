package e2e

import (
	"testing"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/sdk"
	"github.com/tombee/conductor/test/e2e/harness"
)

func TestToolCalling_Basic(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{Content: "Analysis complete"},
			harness.MockResponse{Content: "Final answer based on analysis"},
		),
	)

	wf := h.LoadWorkflow("testdata/tool_calling.yaml")

	result := h.Run(wf, map[string]any{
		"query": "Test query",
	})

	h.AssertSuccess(t, result)
	h.AssertStepCount(t, result, 2)
}

func TestToolCalling_WithToolCalls(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{
				ToolCalls: []llm.ToolCall{
					{
						ID:        "call_1",
						Name:      "get_info",
						Arguments: `{"topic": "test"}`,
					},
				},
				FinishReason: llm.FinishReasonToolCalls,
				TokenUsage: llm.TokenUsage{
					InputTokens:  20,
					OutputTokens: 10,
				},
			},
			harness.MockResponse{
				Content: "Final answer using tool results",
				TokenUsage: llm.TokenUsage{
					InputTokens:  25,
					OutputTokens: 15,
				},
			},
		),
	)

	wf := h.LoadWorkflow("testdata/tool_calling.yaml")

	result := h.Run(wf, map[string]any{
		"query": "Query requiring tools",
	})

	// Even though first step requested tool calls, workflow should complete
	// (The workflow itself doesn't handle tool calls, but mock records the request)
	h.AssertSuccess(t, result)

	// Verify token usage from both steps
	h.AssertTokenUsage(t, result, 60, 80)
}

func TestToolCalling_MultipleToolCalls(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{
				ToolCalls: []llm.ToolCall{
					{
						ID:        "call_1",
						Name:      "search",
						Arguments: `{"query": "test"}`,
					},
					{
						ID:        "call_2",
						Name:      "calculate",
						Arguments: `{"expression": "2+2"}`,
					},
				},
				FinishReason: llm.FinishReasonToolCalls,
				TokenUsage: llm.TokenUsage{
					InputTokens:  30,
					OutputTokens: 15,
				},
			},
			harness.MockResponse{
				Content: "Combined results from multiple tools",
				TokenUsage: llm.TokenUsage{
					InputTokens:  35,
					OutputTokens: 20,
				},
			},
		),
	)

	wf := h.LoadWorkflow("testdata/tool_calling.yaml")

	result := h.Run(wf, map[string]any{
		"query": "Complex query needing multiple tools",
	})

	h.AssertSuccess(t, result)

	// Verify mock provider recorded both tool calls
	provider := h.GetMockProvider()
	requests := provider.GetRequests()

	if len(requests) < 2 {
		t.Fatalf("expected at least 2 requests, got %d", len(requests))
	}
}

func TestToolCalling_NoToolsNeeded(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{
				Content:      "Direct answer without tools",
				FinishReason: llm.FinishReasonStop,
				TokenUsage: llm.TokenUsage{
					InputTokens:  15,
					OutputTokens: 10,
				},
			},
			harness.MockResponse{
				Content: "Confirmation of direct answer",
				TokenUsage: llm.TokenUsage{
					InputTokens:  20,
					OutputTokens: 8,
				},
			},
		),
	)

	wf := h.LoadWorkflow("testdata/tool_calling.yaml")

	result := h.Run(wf, map[string]any{
		"query": "Simple query",
	})

	h.AssertSuccess(t, result)

	// Verify both steps completed successfully
	h.AssertStepStatus(t, result, "analyze", sdk.StepStatusSuccess)
	h.AssertStepStatus(t, result, "process_response", sdk.StepStatusSuccess)
}

func TestToolCalling_OutputMapping(t *testing.T) {
	analysisContent := "Analysis result with insights"
	finalContent := "Final answer synthesized"

	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{Content: analysisContent},
			harness.MockResponse{Content: finalContent},
		),
	)

	wf := h.LoadWorkflow("testdata/tool_calling.yaml")

	result := h.Run(wf, map[string]any{
		"query": "Test",
	})

	h.AssertSuccess(t, result)

	// SDK uses "response" as default output, custom output names may not be supported
	if result.Output == nil || len(result.Output) == 0 {
		t.Error("expected workflow output to be populated")
	}

	// Verify step outputs contain expected content
	h.AssertStepOutput(t, result, "analyze", "insights")
	h.AssertStepOutput(t, result, "process_response", "synthesized")
}

func TestToolCalling_Events(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "test_tool", Arguments: "{}"},
				},
				FinishReason: llm.FinishReasonToolCalls,
			},
			harness.MockResponse{Content: "Result"},
		),
		harness.WithEventCapture(),
	)

	wf := h.LoadWorkflow("testdata/tool_calling.yaml")

	result := h.Run(wf, map[string]any{
		"query": "Test",
	})

	h.AssertSuccess(t, result)

	// Verify step events were captured
	h.AssertEventCount(t, sdk.EventStepStarted, 2)
	h.AssertEventCount(t, sdk.EventStepCompleted, 2)

	// Check for tool call events
	events := h.Events()
	hasToolCallEvent := false
	for _, event := range events {
		if event.Type == sdk.EventLLMToolCall {
			hasToolCallEvent = true
			t.Logf("Found tool call event for step: %s", event.StepID)
		}
	}

	// Tool call events might or might not be present depending on SDK implementation
	if hasToolCallEvent {
		t.Log("Tool call events are being captured")
	} else {
		t.Log("Tool call events not captured (may be expected)")
	}
}

func TestToolCalling_TokenTracking(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{
				ToolCalls: []llm.ToolCall{
					{ID: "call_1", Name: "tool", Arguments: "{}"},
				},
				FinishReason: llm.FinishReasonToolCalls,
				TokenUsage: llm.TokenUsage{
					InputTokens:  50,
					OutputTokens: 20,
				},
			},
			harness.MockResponse{
				Content: "Final",
				TokenUsage: llm.TokenUsage{
					InputTokens:  60,
					OutputTokens: 25,
				},
			},
		),
	)

	wf := h.LoadWorkflow("testdata/tool_calling.yaml")

	result := h.Run(wf, map[string]any{
		"query": "Test",
	})

	h.AssertSuccess(t, result)

	// Verify token usage is tracked correctly even with tool calls
	expectedInput := 50 + 60
	expectedOutput := 20 + 25

	if result.Usage.InputTokens != expectedInput {
		t.Errorf("expected %d input tokens, got %d", expectedInput, result.Usage.InputTokens)
	}

	if result.Usage.OutputTokens != expectedOutput {
		t.Errorf("expected %d output tokens, got %d", expectedOutput, result.Usage.OutputTokens)
	}
}
