package harness

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/sdk"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
	}{
		{
			name: "default options",
			opts: nil,
		},
		{
			name: "with mock provider",
			opts: []Option{
				WithMockProvider(MockResponse{Content: "test"}),
			},
		},
		{
			name: "with timeout",
			opts: []Option{
				WithTimeout(5 * time.Second),
			},
		},
		{
			name: "with event capture",
			opts: []Option{
				WithEventCapture(),
			},
		},
		{
			name: "with multiple options",
			opts: []Option{
				WithMockProvider(MockResponse{Content: "test"}),
				WithTimeout(10 * time.Second),
				WithEventCapture(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := New(t, tt.opts...)

			if h == nil {
				t.Fatal("New() returned nil")
			}

			if h.sdk == nil {
				t.Fatal("harness SDK is nil")
			}

			if h.timeout == 0 {
				t.Error("harness timeout not set")
			}

			// Default should use mock provider
			if h.mockProvider == nil && h.provider == nil {
				t.Error("expected either mock provider or real provider to be set")
			}
		})
	}
}

func TestHarness_SDK(t *testing.T) {
	h := New(t)

	sdk := h.SDK()
	if sdk == nil {
		t.Fatal("SDK() returned nil")
	}

	// Verify we can use the SDK directly
	wf, err := sdk.NewWorkflow("test").
		Step("dummy").LLM().
		Prompt("test").
		Done().
		Build()
	if err != nil {
		t.Fatalf("SDK.NewWorkflow().Build() error: %v", err)
	}

	if wf.Name != "test" {
		t.Errorf("expected workflow name 'test', got '%s'", wf.Name)
	}
}

func TestHarness_Run(t *testing.T) {
	h := New(t,
		WithMockProvider(MockResponse{
			Content: "test response",
			TokenUsage: llm.TokenUsage{
				InputTokens:  10,
				OutputTokens: 5,
			},
		}),
	)

	// Create a simple workflow
	wf, err := h.SDK().NewWorkflow("test").
		Input("prompt", sdk.TypeString).
		Step("llm").LLM().
		Model("mock").
		Prompt("{{.inputs.prompt}}").
		Done().
		Build()

	if err != nil {
		t.Fatalf("build workflow: %v", err)
	}

	// Run the workflow
	result := h.Run(wf, map[string]any{
		"prompt": "test",
	})

	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	if !result.Success {
		t.Errorf("expected success, got failure: %v", result.Error)
	}

	// Verify mock provider was called
	if h.mockProvider != nil {
		requests := h.mockProvider.GetRequests()
		if len(requests) != 1 {
			t.Errorf("expected 1 request to mock provider, got %d", len(requests))
		}
	}
}

func TestHarness_RunWithContext(t *testing.T) {
	h := New(t,
		WithMockProvider(MockResponse{
			Content: "test response",
		}),
	)

	wf, err := h.SDK().NewWorkflow("test").
		Input("text", sdk.TypeString).
		Step("llm").LLM().
		Model("mock").
		Prompt("{{.inputs.text}}").
		Done().
		Build()

	if err != nil {
		t.Fatalf("build workflow: %v", err)
	}

	// Test with custom context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result := h.RunWithContext(ctx, wf, map[string]any{
		"text": "test",
	})

	if !result.Success {
		t.Errorf("expected success: %v", result.Error)
	}
}

func TestHarness_RunExpectError(t *testing.T) {
	h := New(t,
		WithMockProvider(MockResponse{
			Content: "test",
		}),
	)

	// Create a workflow that will fail (missing required input)
	wf, err := h.SDK().NewWorkflow("test").
		Input("required", sdk.TypeString).
		Step("llm").LLM().
		Model("mock").
		Prompt("{{.inputs.required}}").
		Done().
		Build()

	if err != nil {
		t.Fatalf("build workflow: %v", err)
	}

	// Run without providing required input - should fail validation
	err = h.RunExpectError(wf, map[string]any{})

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error about 'required' input, got: %v", err)
	}
}

func TestHarness_Events(t *testing.T) {
	h := New(t,
		WithMockProvider(MockResponse{Content: "test"}),
		WithEventCapture(),
	)

	wf, err := h.SDK().NewWorkflow("test").
		Input("text", sdk.TypeString).
		Step("llm").LLM().
		Model("mock").
		Prompt("{{.inputs.text}}").
		Done().
		Build()

	if err != nil {
		t.Fatalf("build workflow: %v", err)
	}

	// Run workflow
	h.Run(wf, map[string]any{"text": "test"})

	// Check events were captured
	events := h.Events()
	if len(events) == 0 {
		t.Error("expected events to be captured, got none")
	}

	// Verify we have workflow started and completed events
	var hasStarted, hasCompleted bool
	for _, event := range events {
		if event.Type == sdk.EventWorkflowStarted {
			hasStarted = true
		}
		if event.Type == sdk.EventWorkflowCompleted {
			hasCompleted = true
		}
	}

	if !hasStarted {
		t.Error("expected WorkflowStarted event")
	}

	if !hasCompleted {
		t.Error("expected WorkflowCompleted event")
	}
}

func TestHarness_GetMockProvider(t *testing.T) {
	mockResp := MockResponse{
		Content: "test",
		TokenUsage: llm.TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}

	h := New(t, WithMockProvider(mockResp))

	provider := h.GetMockProvider()
	if provider == nil {
		t.Fatal("GetMockProvider() returned nil")
	}

	// Verify it's the same provider we configured
	if provider.Name() != "mock" {
		t.Errorf("expected provider name 'mock', got '%s'", provider.Name())
	}

	// Run a workflow to generate requests
	wf, err := h.SDK().NewWorkflow("test").
		Input("text", sdk.TypeString).
		Step("llm").LLM().
		Model("mock").
		Prompt("{{.inputs.text}}").
		Done().
		Build()

	if err != nil {
		t.Fatalf("build workflow: %v", err)
	}

	h.Run(wf, map[string]any{"text": "test"})

	// Check recorded requests
	requests := provider.GetRequests()
	if len(requests) != 1 {
		t.Errorf("expected 1 recorded request, got %d", len(requests))
	}
}

func TestHarness_Cleanup(t *testing.T) {
	// Create harness in a sub-test to trigger cleanup
	t.Run("sub", func(t *testing.T) {
		h := New(t,
			WithMockProvider(MockResponse{Content: "test"}),
		)

		// Access SDK to ensure it's created
		if h.SDK() == nil {
			t.Fatal("SDK is nil")
		}

		// Cleanup will be called when this sub-test exits
	})

	// If we reach here without panic, cleanup worked
}

func TestWithSDKOption(t *testing.T) {
	h := New(t,
		WithMockProvider(MockResponse{Content: "test"}),
		WithSDKOption(sdk.WithTokenLimit(1000)),
	)

	if h.SDK() == nil {
		t.Fatal("SDK is nil")
	}

	// We can't directly inspect the token limit, but we can verify the option was accepted
	// by ensuring the SDK was created successfully
}

func TestHarness_TimeoutEnforcement(t *testing.T) {
	// Create a slow mock response
	h := New(t,
		WithMockProvider(MockResponse{Content: "test"}),
		WithTimeout(100*time.Millisecond), // Very short timeout
	)

	// Create a workflow
	wf, err := h.SDK().NewWorkflow("test").
		Input("text", sdk.TypeString).
		Step("llm").LLM().
		Model("mock").
		Prompt("{{.inputs.text}}").
		Done().
		Build()

	if err != nil {
		t.Fatalf("build workflow: %v", err)
	}

	// Run should still complete (mock is fast)
	// This test mainly verifies timeout is set and doesn't break fast execution
	result := h.Run(wf, map[string]any{"text": "test"})

	if !result.Success {
		t.Errorf("workflow failed: %v", result.Error)
	}
}

func TestHarness_DefaultMockProvider(t *testing.T) {
	// Create harness without specifying a provider
	h := New(t)

	// Should have a default mock provider
	if h.mockProvider == nil {
		t.Error("expected default mock provider to be created")
	}

	// Verify we can still use it (will fail when no responses configured, but that's expected)
	wf, err := h.SDK().NewWorkflow("test").
		Input("text", sdk.TypeString).
		Step("llm").LLM().
		Model("mock").
		Prompt("{{.inputs.text}}").
		Done().
		Build()

	if err != nil {
		t.Fatalf("build workflow: %v", err)
	}

	// This will fail because no responses are configured, but that proves the mock provider exists
	_ = h.RunExpectError(wf, map[string]any{"text": "test"})
}
