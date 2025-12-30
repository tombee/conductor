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

package debug

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestAdapter_BreakpointDetection(t *testing.T) {
	config := New([]string{"step1", "step3"}, "info")
	logger := slog.Default()
	adapter := NewAdapter(config, logger)
	defer adapter.Close()

	tests := []struct {
		name     string
		stepID   string
		expected bool
	}{
		{"step with breakpoint", "step1", true},
		{"step without breakpoint", "step2", false},
		{"another step with breakpoint", "step3", true},
		{"nonexistent step", "step4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.ShouldPauseAt(tt.stepID)
			if result != tt.expected {
				t.Errorf("ShouldPauseAt(%s) = %v, want %v", tt.stepID, result, tt.expected)
			}
		})
	}
}

func TestAdapter_OnStepStart(t *testing.T) {
	config := New([]string{"step2"}, "info")
	logger := slog.Default()
	adapter := NewAdapter(config, logger)
	defer adapter.Close()

	ctx := context.Background()
	inputs := map[string]interface{}{"test": "value"}

	// Test step without breakpoint - should not pause
	err := adapter.OnStepStart(ctx, "step1", 0, inputs)
	if err != nil {
		t.Fatalf("OnStepStart() error = %v, want nil", err)
	}

	// Verify event was sent
	select {
	case event := <-adapter.EventChan():
		if event.Type != EventStepStart {
			t.Errorf("Expected EventStepStart, got %v", event.Type)
		}
		if event.StepID != "step1" {
			t.Errorf("Expected StepID step1, got %v", event.StepID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected event but none received")
	}
}

func TestAdapter_OnStepEnd(t *testing.T) {
	config := New([]string{"step1"}, "info")
	logger := slog.Default()
	adapter := NewAdapter(config, logger)
	defer adapter.Close()

	ctx := context.Background()
	result := &workflow.StepResult{
		StepID:   "step1",
		Status:   workflow.StepStatusSuccess,
		Output:   map[string]interface{}{"result": "success"},
		Duration: 100 * time.Millisecond,
	}

	err := adapter.OnStepEnd(ctx, "step1", result, nil)
	if err != nil {
		t.Fatalf("OnStepEnd() error = %v, want nil", err)
	}

	// Verify snapshot was updated
	if adapter.contextSnapshot["step1"] == nil {
		t.Error("Expected step1 output in context snapshot")
	}
}

func TestAdapter_CommandHandling(t *testing.T) {
	config := New([]string{"step1"}, "info")
	logger := slog.Default()
	adapter := NewAdapter(config, logger)
	defer adapter.Close()

	tests := []struct {
		name    string
		cmd     *Command
		wantErr bool
	}{
		{
			name:    "continue command",
			cmd:     &Command{Type: CommandContinue},
			wantErr: false,
		},
		{
			name:    "next command",
			cmd:     &Command{Type: CommandNext},
			wantErr: false,
		},
		{
			name:    "skip command",
			cmd:     &Command{Type: CommandSkip},
			wantErr: true, // Skip returns a special error
		},
		{
			name:    "abort command",
			cmd:     &Command{Type: CommandAbort},
			wantErr: true, // Abort returns context.Canceled
		},
		{
			name:    "inspect command",
			cmd:     &Command{Type: CommandInspect, Args: []string{"key"}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter.currentStep = "step1"
			err := adapter.handleCommand(context.Background(), tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("handleCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAdapter_NextBreakpoint(t *testing.T) {
	config := New([]string{"step2"}, "info")
	logger := slog.Default()
	adapter := NewAdapter(config, logger)
	defer adapter.Close()

	// Initially, nextBreakpoint should be false
	if adapter.nextBreakpoint {
		t.Error("Expected nextBreakpoint to be false initially")
	}

	// Send a "next" command
	adapter.currentStep = "step1"
	err := adapter.handleCommand(context.Background(), &Command{Type: CommandNext})
	if err != nil {
		t.Fatalf("handleCommand() error = %v", err)
	}

	// nextBreakpoint should now be true
	if !adapter.nextBreakpoint {
		t.Error("Expected nextBreakpoint to be true after 'next' command")
	}

	// OnStepStart should pause at the next step
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start a goroutine to send continue command
	go func() {
		time.Sleep(10 * time.Millisecond)
		adapter.CommandChan() <- &Command{Type: CommandContinue}
	}()

	err = adapter.OnStepStart(ctx, "step3", 2, nil)
	if err != nil {
		t.Fatalf("OnStepStart() error = %v", err)
	}

	// nextBreakpoint should be cleared after pause
	if adapter.nextBreakpoint {
		t.Error("Expected nextBreakpoint to be false after pause")
	}
}

func TestAdapter_AbortFlag(t *testing.T) {
	config := New([]string{"step1"}, "info")
	logger := slog.Default()
	adapter := NewAdapter(config, logger)
	defer adapter.Close()

	// Initially, aborted should be false
	if adapter.IsAborted() {
		t.Error("Expected IsAborted() to be false initially")
	}

	// Send an abort command
	adapter.currentStep = "step1"
	_ = adapter.handleCommand(context.Background(), &Command{Type: CommandAbort})

	// Now aborted should be true
	if !adapter.IsAborted() {
		t.Error("Expected IsAborted() to be true after abort command")
	}
}

func TestAdapter_EventFlow(t *testing.T) {
	config := New([]string{}, "info")
	logger := slog.Default()
	adapter := NewAdapter(config, logger)
	defer adapter.Close()

	ctx := context.Background()
	inputs := map[string]interface{}{"test": "value"}

	// Start a step
	err := adapter.OnStepStart(ctx, "step1", 0, inputs)
	if err != nil {
		t.Fatalf("OnStepStart() error = %v", err)
	}

	// Check for step start event
	select {
	case event := <-adapter.EventChan():
		if event.Type != EventStepStart {
			t.Errorf("Expected EventStepStart, got %v", event.Type)
		}
		if event.StepID != "step1" {
			t.Errorf("Expected StepID step1, got %v", event.StepID)
		}
		if event.StepIndex != 0 {
			t.Errorf("Expected StepIndex 0, got %v", event.StepIndex)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected step start event but none received")
	}

	// End the step
	result := &workflow.StepResult{
		StepID:   "step1",
		Status:   workflow.StepStatusSuccess,
		Output:   map[string]interface{}{"result": "success"},
		Duration: 50 * time.Millisecond,
	}
	err = adapter.OnStepEnd(ctx, "step1", result, nil)
	if err != nil {
		t.Fatalf("OnStepEnd() error = %v", err)
	}

	// Verify no events were sent by OnStepEnd
	select {
	case <-adapter.EventChan():
		t.Error("Did not expect any event from OnStepEnd")
	case <-time.After(10 * time.Millisecond):
		// Expected - no event
	}
}
