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

package debug_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tombee/conductor/internal/debug"
	"github.com/tombee/conductor/pkg/workflow"
)

// TestBreakpointWorkflow tests that a workflow can be executed with breakpoints
// and that execution pauses at the specified steps.
func TestBreakpointWorkflow(t *testing.T) {
	// Create a simple workflow definition
	wf := &workflow.Definition{
		Name:        "test-debug-workflow",
		Description: "Test workflow for debugging",
		Steps: []workflow.StepDefinition{
			{ID: "step1", Type: "llm"},
			{ID: "step2", Type: "llm"},
			{ID: "step3", Type: "llm"},
		},
	}

	// Create a debug configuration with breakpoints on step2
	config := &debug.Config{
		Enabled:     true,
		Breakpoints: []string{"step2"},
	}

	// Validate config
	err := config.Validate(wf)
	require.NoError(t, err, "Config should be valid")

	// Create an adapter for debugging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := debug.NewAdapter(config, logger)
	require.NotNil(t, adapter, "Failed to create debug adapter")
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Monitor events in a goroutine
	var pausedAtStep2 bool
	var resumedFromStep2 bool

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-adapter.EventChan():
				if event == nil {
					return
				}

				t.Logf("Event received: Type=%s, StepID=%s, StepIndex=%d",
					event.Type, event.StepID, event.StepIndex)

				switch event.Type {
				case debug.EventPaused:
					if event.StepID == "step2" {
						pausedAtStep2 = true
						// Send continue command after a short delay
						go func() {
							time.Sleep(100 * time.Millisecond)
							adapter.CommandChan() <- &debug.Command{Type: debug.CommandContinue}
						}()
					}

				case debug.EventResumed:
					if event.StepID == "step2" {
						resumedFromStep2 = true
					}
				}
			}
		}
	}()

	// Simulate workflow execution - step2 should trigger breakpoint
	err = adapter.OnStepStart(ctx, "step2", 1, map[string]any{
		"step1": "Step 1 output",
	})
	require.NoError(t, err, "OnStepStart should not error")

	// Wait for the breakpoint to be hit and resumed
	time.Sleep(500 * time.Millisecond)

	// Verify expectations
	assert.True(t, pausedAtStep2, "Should have paused at step2 breakpoint")
	assert.True(t, resumedFromStep2, "Should have resumed from step2 after continue command")
}

// TestBreakpointSkipStep tests that a step can be skipped during debugging.
func TestBreakpointSkipStep(t *testing.T) {
	config := &debug.Config{
		Enabled:     true,
		Breakpoints: []string{"step2"},
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := debug.NewAdapter(config, logger)
	require.NotNil(t, adapter)
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var skipped bool
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-adapter.EventChan():
				if event == nil {
					return
				}

				if event.Type == debug.EventPaused && event.StepID == "step2" {
					// Send skip command
					go func() {
						time.Sleep(100 * time.Millisecond)
						adapter.CommandChan() <- &debug.Command{Type: debug.CommandSkip}
					}()
				}

				if event.Type == debug.EventSkipped && event.StepID == "step2" {
					skipped = true
				}
			}
		}
	}()

	// Trigger the breakpoint
	stepErr := adapter.OnStepStart(ctx, "step2", 1, map[string]any{})

	// OnStepStart returns an error when skip is requested
	if stepErr != nil && stepErr.Error() != "debug: skip step" {
		t.Fatalf("Unexpected error: %v", stepErr)
	}

	time.Sleep(500 * time.Millisecond)

	assert.True(t, skipped, "Step should have been skipped")
}

// TestMultipleBreakpoints tests pausing at multiple steps.
func TestMultipleBreakpoints(t *testing.T) {
	config := &debug.Config{
		Enabled:     true,
		Breakpoints: []string{"step1", "step3"}, // Pause at step1 and step3
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	adapter := debug.NewAdapter(config, logger)
	require.NotNil(t, adapter)
	defer adapter.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pausedSteps := make(map[string]bool)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-adapter.EventChan():
				if event == nil {
					return
				}

				if event.Type == debug.EventPaused {
					pausedSteps[event.StepID] = true
					// Auto-continue
					go func() {
						time.Sleep(50 * time.Millisecond)
						adapter.CommandChan() <- &debug.Command{Type: debug.CommandContinue}
					}()
				}
			}
		}
	}()

	// Simulate execution of each step
	steps := []string{"step1", "step2", "step3"}
	for i, stepID := range steps {
		err := adapter.OnStepStart(ctx, stepID, i, map[string]any{})
		if err != nil {
			t.Logf("OnStepStart error for %s: %v", stepID, err)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Verify that only step1 and step3 were paused
	assert.True(t, pausedSteps["step1"], "Should pause at step1")
	assert.False(t, pausedSteps["step2"], "Should NOT pause at step2")
	assert.True(t, pausedSteps["step3"], "Should pause at step3")
}
