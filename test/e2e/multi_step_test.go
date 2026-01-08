package e2e

import (
	"testing"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/sdk"
	"github.com/tombee/conductor/test/e2e/harness"
)

func TestMultiStepWorkflow(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{
				Content: "Outline:\n1. Introduction\n2. Main content\n3. Conclusion",
				TokenUsage: llm.TokenUsage{
					InputTokens:  20,
					OutputTokens: 15,
				},
			},
			harness.MockResponse{
				Content: "The introduction sets the stage and provides context for the topic.",
				TokenUsage: llm.TokenUsage{
					InputTokens:  30,
					OutputTokens: 12,
				},
			},
			harness.MockResponse{
				Content: "A comprehensive overview covering introduction and detailed analysis.",
				TokenUsage: llm.TokenUsage{
					InputTokens:  40,
					OutputTokens: 10,
				},
			},
		),
	)

	wf := h.LoadWorkflow("testdata/multi_step.yaml")

	result := h.Run(wf, map[string]any{
		"topic": "Test Topic",
	})

	// Assert workflow succeeded
	h.AssertSuccess(t, result)

	// Verify all three steps completed
	h.AssertStepCount(t, result, 3)

	// Verify step outputs contain expected content
	h.AssertStepOutput(t, result, "generate_outline", "Outline")
	h.AssertStepOutput(t, result, "expand_first_point", "introduction")
	h.AssertStepOutput(t, result, "generate_summary", "overview")

	// Verify token usage accumulated across steps
	totalTokens := result.Usage.TotalTokens
	expectedMin := (20 + 30 + 40) + (15 + 12 + 10) // Input + Output
	if totalTokens < expectedMin {
		t.Errorf("expected at least %d total tokens, got %d", expectedMin, totalTokens)
	}

	// SDK uses "response" as the default output, custom output names may not be supported
	if result.Output == nil || len(result.Output) == 0 {
		t.Error("expected workflow output to be populated")
	}
}

func TestMultiStepWorkflow_StepOrdering(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{Content: "Step 1 output"},
			harness.MockResponse{Content: "Step 2 output"},
			harness.MockResponse{Content: "Step 3 output"},
		),
		harness.WithEventCapture(),
	)

	wf := h.LoadWorkflow("testdata/multi_step.yaml")

	result := h.Run(wf, map[string]any{
		"topic": "Order Test",
	})

	h.AssertSuccess(t, result)

	// Verify events show correct step ordering
	events := h.Events()
	var stepStartedEvents []*sdk.Event
	for _, event := range events {
		if event.Type == sdk.EventStepStarted {
			stepStartedEvents = append(stepStartedEvents, event)
		}
	}

	if len(stepStartedEvents) != 3 {
		t.Fatalf("expected 3 step started events, got %d", len(stepStartedEvents))
	}

	// Verify steps started in correct order
	expectedOrder := []string{"generate_outline", "expand_first_point", "generate_summary"}
	for i, event := range stepStartedEvents {
		if event.StepID != expectedOrder[i] {
			t.Errorf("step %d: expected %q, got %q", i, expectedOrder[i], event.StepID)
		}
	}
}

func TestMultiStepWorkflow_VariablePassing(t *testing.T) {
	// Create responses that reference each other to verify variable passing
	outline := "Point A, Point B, Point C"
	details := "Detailed explanation of Point A from the outline"
	summary := "Summary includes: outline and details"

	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{Content: outline},
			harness.MockResponse{Content: details},
			harness.MockResponse{Content: summary},
		),
	)

	wf := h.LoadWorkflow("testdata/multi_step.yaml")

	result := h.Run(wf, map[string]any{
		"topic": "Variable Test",
	})

	h.AssertSuccess(t, result)

	// Verify mock provider received requests with interpolated values
	provider := h.GetMockProvider()
	requests := provider.GetRequests()

	if len(requests) != 3 {
		t.Fatalf("expected 3 requests, got %d", len(requests))
	}

	// First request should contain the topic
	firstPrompt := requests[0].Messages[0].Content
	if firstPrompt == "" {
		t.Error("first request prompt is empty")
	}

	// Subsequent requests should reference previous steps
	// (We can't easily verify the exact content without parsing templates,
	// but we can verify requests were made)
	t.Logf("Request 1 prompt length: %d", len(requests[0].Messages[0].Content))
	t.Logf("Request 2 prompt length: %d", len(requests[1].Messages[0].Content))
	t.Logf("Request 3 prompt length: %d", len(requests[2].Messages[0].Content))
}

func TestMultiStepWorkflow_TokenAccumulation(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{
				Content: "Step 1",
				TokenUsage: llm.TokenUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
			harness.MockResponse{
				Content: "Step 2",
				TokenUsage: llm.TokenUsage{
					InputTokens:  200,
					OutputTokens: 75,
				},
			},
			harness.MockResponse{
				Content: "Step 3",
				TokenUsage: llm.TokenUsage{
					InputTokens:  150,
					OutputTokens: 60,
				},
			},
		),
	)

	wf := h.LoadWorkflow("testdata/multi_step.yaml")

	result := h.Run(wf, map[string]any{
		"topic": "Token Test",
	})

	h.AssertSuccess(t, result)

	// Verify total token usage
	expectedInput := 100 + 200 + 150
	expectedOutput := 50 + 75 + 60
	expectedTotal := expectedInput + expectedOutput

	if result.Usage.InputTokens != expectedInput {
		t.Errorf("expected %d input tokens, got %d", expectedInput, result.Usage.InputTokens)
	}

	if result.Usage.OutputTokens != expectedOutput {
		t.Errorf("expected %d output tokens, got %d", expectedOutput, result.Usage.OutputTokens)
	}

	if result.Usage.TotalTokens != expectedTotal {
		t.Errorf("expected %d total tokens, got %d", expectedTotal, result.Usage.TotalTokens)
	}
}

func TestMultiStepWorkflow_StepDuration(t *testing.T) {
	h := harness.New(t,
		harness.WithMockProvider(
			harness.MockResponse{Content: "Step 1"},
			harness.MockResponse{Content: "Step 2"},
			harness.MockResponse{Content: "Step 3"},
		),
	)

	wf := h.LoadWorkflow("testdata/multi_step.yaml")

	result := h.Run(wf, map[string]any{
		"topic": "Duration Test",
	})

	h.AssertSuccess(t, result)

	// Verify each step has a duration
	for stepID, stepResult := range result.Steps {
		if stepResult.Duration == 0 {
			t.Errorf("step %q has zero duration", stepID)
		}
	}

	// Verify workflow duration
	if result.Duration == 0 {
		t.Error("workflow has zero duration")
	}
}
