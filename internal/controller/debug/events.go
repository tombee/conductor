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
	"encoding/json"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/tracing/redact"
)

// EventType represents the type of debug event.
type EventType string

const (
	// EventTypeHeartbeat is a periodic ping to keep the SSE connection alive.
	EventTypeHeartbeat EventType = "heartbeat"
	// EventTypeStepStart indicates a workflow step has started.
	EventTypeStepStart EventType = "step_start"
	// EventTypePaused indicates the workflow is paused at a breakpoint.
	EventTypePaused EventType = "paused"
	// EventTypeResumed indicates the workflow has resumed after a pause.
	EventTypeResumed EventType = "resumed"
	// EventTypeCompleted indicates a step has completed.
	EventTypeCompleted EventType = "completed"
	// EventTypeCommandError indicates a command resulted in an error.
	EventTypeCommandError EventType = "command_error"
)

// Event represents a debug event that can be sent over SSE.
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewEvent creates a new event with a unique ID.
func NewEvent(eventType EventType, data map[string]interface{}) Event {
	return Event{
		ID:        fmt.Sprintf("%s-%d", eventType, time.Now().UnixNano()),
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// NewHeartbeatEvent creates a heartbeat event.
func NewHeartbeatEvent() Event {
	return NewEvent(EventTypeHeartbeat, map[string]interface{}{
		"timestamp": time.Now().Unix(),
	})
}

// NewStepStartEvent creates a step start event.
func NewStepStartEvent(stepID string, stepIndex int, inputs map[string]interface{}) Event {
	return NewEvent(EventTypeStepStart, map[string]interface{}{
		"step_id":    stepID,
		"step_index": stepIndex,
		"inputs":     inputs,
	})
}

// NewPausedEvent creates a paused event with redacted context snapshot.
func NewPausedEvent(stepID string, contextSnapshot map[string]interface{}, redactor *redact.Redactor) Event {
	// Redact sensitive data from context snapshot
	redactedContext := contextSnapshot
	if redactor != nil {
		redactedContext = redactContextSnapshot(contextSnapshot, redactor)
	}

	return NewEvent(EventTypePaused, map[string]interface{}{
		"step_id":          stepID,
		"context_snapshot": redactedContext,
	})
}

// NewResumedEvent creates a resumed event.
func NewResumedEvent(stepID string) Event {
	return NewEvent(EventTypeResumed, map[string]interface{}{
		"step_id": stepID,
	})
}

// NewCompletedEvent creates a completed event.
func NewCompletedEvent(stepID string, output interface{}, duration time.Duration) Event {
	return NewEvent(EventTypeCompleted, map[string]interface{}{
		"step_id":      stepID,
		"output":       output,
		"duration_ms":  duration.Milliseconds(),
		"duration_str": duration.String(),
	})
}

// NewCommandErrorEvent creates a command error event.
func NewCommandErrorEvent(command string, errorMessage string) Event {
	return NewEvent(EventTypeCommandError, map[string]interface{}{
		"command": command,
		"error":   errorMessage,
	})
}

// ToSSE formats the event as a Server-Sent Events message.
func (e Event) ToSSE() (string, error) {
	dataJSON, err := json.Marshal(e)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event data: %w", err)
	}

	// SSE format:
	// id: <event-id>
	// event: <event-type>
	// data: <json-data>
	// (blank line)
	return fmt.Sprintf("id: %s\nevent: %s\ndata: %s\n\n", e.ID, e.Type, string(dataJSON)), nil
}

// redactContextSnapshot redacts sensitive data from a context snapshot.
func redactContextSnapshot(context map[string]interface{}, redactor *redact.Redactor) map[string]interface{} {
	redacted := make(map[string]interface{})

	for key, value := range context {
		switch v := value.(type) {
		case string:
			// Redact string values that might contain secrets
			redacted[key] = redactor.RedactString(v)
		case map[string]interface{}:
			// Recursively redact nested maps
			redacted[key] = redactContextSnapshot(v, redactor)
		case []interface{}:
			// Redact arrays
			redacted[key] = redactArray(v, redactor)
		default:
			// Keep other types as-is
			redacted[key] = value
		}
	}

	return redacted
}

// redactArray redacts sensitive data from an array.
func redactArray(arr []interface{}, redactor *redact.Redactor) []interface{} {
	redacted := make([]interface{}, len(arr))

	for i, value := range arr {
		switch v := value.(type) {
		case string:
			redacted[i] = redactor.RedactString(v)
		case map[string]interface{}:
			redacted[i] = redactContextSnapshot(v, redactor)
		case []interface{}:
			redacted[i] = redactArray(v, redactor)
		default:
			redacted[i] = value
		}
	}

	return redacted
}
