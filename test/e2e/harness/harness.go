// Package harness provides testing utilities for E2E workflow tests.
package harness

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/sdk"
)

// Harness provides a test harness for E2E workflow testing.
// It wraps SDK creation, workflow loading, and execution with test-friendly defaults.
type Harness struct {
	t             *testing.T
	sdk           *sdk.SDK
	timeout       time.Duration
	mockProvider  *MockLLMProvider
	provider      llm.Provider
	sdkOptions    []sdk.Option
	events        []*sdk.Event
	captureEvents bool
}

// New creates a new test harness with the given options.
// The harness automatically registers cleanup via t.Cleanup() to ensure resources are released.
//
// Example:
//
//	h := harness.New(t,
//		harness.WithMockProvider(
//			harness.MockResponse{Content: "test"},
//		),
//	)
func New(t *testing.T, opts ...Option) *Harness {
	t.Helper()

	h := &Harness{
		t:          t,
		timeout:    30 * time.Second, // Default timeout for mock tests
		sdkOptions: make([]sdk.Option, 0),
		events:     make([]*sdk.Event, 0),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(h); err != nil {
			t.Fatalf("apply harness option: %v", err)
		}
	}

	// If no provider configured, default to mock provider
	if h.mockProvider == nil && h.provider == nil {
		h.mockProvider = NewMockProvider()
	}

	// Build SDK options
	sdkOpts := h.sdkOptions

	// Add provider option
	if h.mockProvider != nil {
		sdkOpts = append(sdkOpts, sdk.WithProvider("mock", h.mockProvider))
	} else if h.provider != nil {
		providerName := h.provider.Name()
		sdkOpts = append(sdkOpts, sdk.WithProvider(providerName, h.provider))
	}

	// Setup event capture if enabled
	if h.captureEvents {
		// We'll register event handlers after SDK creation
	}

	// Create SDK instance
	s, err := sdk.New(sdkOpts...)
	if err != nil {
		t.Fatalf("create SDK: %v", err)
	}

	h.sdk = s

	// Register event handlers if event capture is enabled
	if h.captureEvents {
		// Capture all event types
		eventTypes := []sdk.EventType{
			sdk.EventWorkflowStarted,
			sdk.EventWorkflowCompleted,
			sdk.EventWorkflowFailed,
			sdk.EventStepStarted,
			sdk.EventStepCompleted,
			sdk.EventStepFailed,
			sdk.EventLLMToken,
			sdk.EventLLMToolCall,
			sdk.EventLLMToolResult,
			sdk.EventTokenUpdate,
		}

		for _, et := range eventTypes {
			eventType := et // Capture for closure
			h.sdk.OnEvent(eventType, func(ctx context.Context, event *sdk.Event) {
				// Store a copy of the event
				eventCopy := *event
				h.events = append(h.events, &eventCopy)
			})
		}
	}

	// Register cleanup
	t.Cleanup(func() {
		if err := h.sdk.Close(); err != nil {
			t.Logf("cleanup SDK: %v", err)
		}
	})

	return h
}

// SDK returns the underlying SDK instance.
// This allows tests to access SDK methods not wrapped by the harness.
func (h *Harness) SDK() *sdk.SDK {
	return h.sdk
}

// LoadWorkflow loads a workflow from a YAML file path.
// The path is relative to the test file location.
//
// Example:
//
//	wf := h.LoadWorkflow("testdata/simple_llm.yaml")
func (h *Harness) LoadWorkflow(path string) *sdk.Workflow {
	h.t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("read workflow file %q: %v", path, err)
	}

	wf, err := h.sdk.LoadWorkflow(content)
	if err != nil {
		h.t.Fatalf("load workflow from %q: %v", path, err)
	}

	return wf
}

// Run executes a workflow with the given inputs and returns the result.
// The execution is subject to the harness timeout (default 30s).
//
// Example:
//
//	result := h.Run(wf, map[string]any{"prompt": "test"})
func (h *Harness) Run(wf *sdk.Workflow, inputs map[string]any, opts ...sdk.RunOption) *sdk.Result {
	h.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	result, err := h.sdk.Run(ctx, wf, inputs, opts...)
	if err != nil {
		h.t.Fatalf("run workflow: %v", err)
	}

	return result
}

// RunWithContext executes a workflow with a custom context.
// This allows tests to control cancellation and deadlines.
//
// Example:
//
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	result := h.RunWithContext(ctx, wf, inputs)
func (h *Harness) RunWithContext(ctx context.Context, wf *sdk.Workflow, inputs map[string]any, opts ...sdk.RunOption) *sdk.Result {
	h.t.Helper()

	result, err := h.sdk.Run(ctx, wf, inputs, opts...)
	if err != nil {
		h.t.Fatalf("run workflow: %v", err)
	}

	return result
}

// RunExpectError executes a workflow expecting it to fail.
// Returns the error if one occurred, fails the test if workflow succeeded.
//
// Example:
//
//	err := h.RunExpectError(wf, inputs)
//	if !strings.Contains(err.Error(), "expected error") {
//		t.Errorf("wrong error: %v", err)
//	}
func (h *Harness) RunExpectError(wf *sdk.Workflow, inputs map[string]any, opts ...sdk.RunOption) error {
	h.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	result, err := h.sdk.Run(ctx, wf, inputs, opts...)
	if err != nil {
		return err
	}

	if result.Error != nil {
		return result.Error
	}

	if !result.Success {
		h.t.Fatal("workflow failed but no error was returned")
	}

	h.t.Fatal("expected workflow to fail, but it succeeded")
	return nil
}

// Events returns all captured events.
// Requires WithEventCapture() option to be set on the harness.
//
// Example:
//
//	h := harness.New(t, harness.WithEventCapture())
//	result := h.Run(wf, inputs)
//	events := h.Events()
//	for _, event := range events {
//		t.Logf("Event: %s at %v", event.Type, event.Timestamp)
//	}
func (h *Harness) Events() []*sdk.Event {
	return h.events
}

// GetMockProvider returns the mock provider if one is configured.
// This allows tests to inspect recorded requests.
//
// Example:
//
//	h := harness.New(t, harness.WithMockProvider(resp))
//	result := h.Run(wf, inputs)
//	requests := h.GetMockProvider().GetRequests()
//	if len(requests) != 1 {
//		t.Errorf("expected 1 request, got %d", len(requests))
//	}
func (h *Harness) GetMockProvider() *MockLLMProvider {
	return h.mockProvider
}
