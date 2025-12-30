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
	"fmt"
	"log/slog"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

// Adapter wraps a workflow executor to provide debugging capabilities.
// It intercepts step execution to implement breakpoints and interactive debugging.
type Adapter struct {
	config *Config
	logger *slog.Logger

	// eventChan sends debug events to the debugger shell.
	eventChan chan *Event

	// cmdChan receives debug commands from the shell.
	cmdChan chan *Command

	// currentStep tracks the current step being executed.
	currentStep string

	// stepIndex tracks the current step index.
	stepIndex int

	// context snapshot of the workflow context.
	contextSnapshot map[string]interface{}

	// nextBreakpoint is set when "next" command is used.
	nextBreakpoint bool

	// aborted indicates execution was aborted.
	aborted bool
}

// NewAdapter creates a new debug adapter with the given configuration.
func NewAdapter(config *Config, logger *slog.Logger) *Adapter {
	return &Adapter{
		config:          config,
		logger:          logger,
		eventChan:       make(chan *Event, 10),
		cmdChan:         make(chan *Command, 1),
		contextSnapshot: make(map[string]interface{}),
	}
}

// EventChan returns the channel for debug events.
func (a *Adapter) EventChan() <-chan *Event {
	return a.eventChan
}

// CommandChan returns the channel for debug commands.
func (a *Adapter) CommandChan() chan<- *Command {
	return a.cmdChan
}

// OnStepStart is called before each step executes.
// It checks for breakpoints and pauses execution if needed.
func (a *Adapter) OnStepStart(ctx context.Context, stepID string, stepIndex int, inputs map[string]interface{}) error {
	a.currentStep = stepID
	a.stepIndex = stepIndex

	// Send step_start event
	a.sendEvent(&Event{
		Type:      EventStepStart,
		StepID:    stepID,
		StepIndex: stepIndex,
		Snapshot:  a.captureSnapshot(inputs),
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Starting step: %s", stepID),
	})

	// Check if we should pause at this step
	shouldPause := a.config.ShouldPauseAt(stepID) || a.nextBreakpoint

	if shouldPause {
		a.nextBreakpoint = false // Clear temporary breakpoint
		return a.pause(ctx, stepID, stepIndex, inputs)
	}

	return nil
}

// OnStepEnd is called after each step completes.
func (a *Adapter) OnStepEnd(ctx context.Context, stepID string, result *workflow.StepResult, err error) error {
	// Update context snapshot with step output
	if result != nil && result.Output != nil {
		a.updateSnapshot(stepID, result.Output)
	}

	return nil
}

// pause pauses execution at the current step and waits for a command.
func (a *Adapter) pause(ctx context.Context, stepID string, stepIndex int, inputs map[string]interface{}) error {
	a.logger.Info("Paused at breakpoint", slog.String("step_id", stepID))

	// Send paused event
	a.sendEvent(&Event{
		Type:      EventPaused,
		StepID:    stepID,
		StepIndex: stepIndex,
		Snapshot:  a.captureSnapshot(inputs),
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Paused at step: %s", stepID),
	})

	// Wait for command
	select {
	case <-ctx.Done():
		return ctx.Err()
	case cmd := <-a.cmdChan:
		return a.handleCommand(ctx, cmd)
	}
}

// handleCommand processes a debug command.
func (a *Adapter) handleCommand(ctx context.Context, cmd *Command) error {
	switch cmd.Type {
	case CommandContinue:
		a.logger.Debug("Resuming execution")
		a.sendEvent(&Event{
			Type:      EventResumed,
			StepID:    a.currentStep,
			Timestamp: time.Now(),
			Message:   "Resuming execution",
		})
		return nil

	case CommandNext:
		a.logger.Debug("Stepping to next step")
		a.nextBreakpoint = true
		a.sendEvent(&Event{
			Type:      EventResumed,
			StepID:    a.currentStep,
			Timestamp: time.Now(),
			Message:   "Stepping to next step",
		})
		return nil

	case CommandSkip:
		a.logger.Debug("Skipping current step", slog.String("step_id", a.currentStep))
		a.sendEvent(&Event{
			Type:      EventSkipped,
			StepID:    a.currentStep,
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Skipped step: %s", a.currentStep),
		})
		// Return a special error that the executor can recognize
		return fmt.Errorf("debug: skip step")

	case CommandAbort:
		a.logger.Info("Aborting execution")
		a.aborted = true
		a.sendEvent(&Event{
			Type:      EventAborted,
			StepID:    a.currentStep,
			Timestamp: time.Now(),
			Message:   "Execution aborted by user",
		})
		return context.Canceled

	case CommandInspect:
		// Inspection is handled by the shell, just acknowledge
		return nil

	case CommandContext:
		// Context dump is handled by the shell, just acknowledge
		return nil

	default:
		return fmt.Errorf("unknown command: %s", cmd.Type)
	}
}

// captureSnapshot creates a copy of the current workflow context.
func (a *Adapter) captureSnapshot(inputs map[string]interface{}) map[string]interface{} {
	snapshot := make(map[string]interface{})

	// Copy existing context
	for k, v := range a.contextSnapshot {
		snapshot[k] = v
	}

	// Add current step inputs
	if inputs != nil {
		for k, v := range inputs {
			snapshot[k] = v
		}
	}

	return snapshot
}

// updateSnapshot updates the context snapshot with new values.
func (a *Adapter) updateSnapshot(stepID string, output interface{}) {
	if a.contextSnapshot == nil {
		a.contextSnapshot = make(map[string]interface{})
	}
	a.contextSnapshot[stepID] = output
}

// sendEvent sends a debug event to the event channel.
func (a *Adapter) sendEvent(event *Event) {
	select {
	case a.eventChan <- event:
	default:
		a.logger.Warn("Debug event channel full, dropping event", slog.String("event_type", string(event.Type)))
	}
}

// Close closes the adapter's channels.
func (a *Adapter) Close() {
	close(a.eventChan)
	close(a.cmdChan)
}

// IsAborted returns true if execution was aborted.
func (a *Adapter) IsAborted() bool {
	return a.aborted
}
