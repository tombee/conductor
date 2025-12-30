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
	"time"
)

// EventType represents the type of debug event.
type EventType string

const (
	// EventStepStart indicates a step is about to start.
	EventStepStart EventType = "step_start"

	// EventPaused indicates execution has paused at a breakpoint.
	EventPaused EventType = "paused"

	// EventResumed indicates execution has resumed.
	EventResumed EventType = "resumed"

	// EventCompleted indicates the workflow has completed.
	EventCompleted EventType = "completed"

	// EventSkipped indicates a step was skipped.
	EventSkipped EventType = "skipped"

	// EventAborted indicates execution was aborted by user.
	EventAborted EventType = "aborted"
)

// Event represents a debug event emitted during workflow execution.
type Event struct {
	// Type is the type of event.
	Type EventType

	// StepID is the identifier of the step (if applicable).
	StepID string

	// StepIndex is the zero-based index of the step in the workflow.
	StepIndex int

	// Snapshot is a copy of the workflow context at this point.
	Snapshot map[string]interface{}

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// Message is an optional human-readable message.
	Message string
}

// CommandType represents the type of debug command.
type CommandType string

const (
	// CommandContinue resumes execution until the next breakpoint or completion.
	CommandContinue CommandType = "continue"

	// CommandNext steps to the next step (single-step execution).
	CommandNext CommandType = "next"

	// CommandSkip skips the current step and proceeds to the next.
	CommandSkip CommandType = "skip"

	// CommandAbort cancels execution immediately.
	CommandAbort CommandType = "abort"

	// CommandInspect evaluates an expression against the current context.
	CommandInspect CommandType = "inspect"

	// CommandContext dumps the full workflow context.
	CommandContext CommandType = "context"
)

// Command represents a debug command issued by the user.
type Command struct {
	// Type is the type of command.
	Type CommandType

	// Args are optional arguments for the command (e.g., expression for inspect).
	Args []string
}
