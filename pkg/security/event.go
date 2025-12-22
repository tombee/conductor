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

package security

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// EventType represents the type of security event.
type EventType string

const (
	// EventAccessDenied indicates an access request was denied
	EventAccessDenied EventType = "access_denied"

	// EventAccessGranted indicates an access request was granted
	EventAccessGranted EventType = "access_granted"

	// EventViolation indicates a security policy violation
	EventViolation EventType = "violation"

	// EventSandboxEscapeAttempt indicates an attempted sandbox escape
	EventSandboxEscapeAttempt EventType = "sandbox_escape_attempt"
)

// SecurityEvent represents a security-related event.
type SecurityEvent struct {
	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// EventType categorizes the event
	EventType EventType `json:"event_type"`

	// WorkflowID identifies the workflow
	WorkflowID string `json:"workflow_id,omitempty"`

	// StepID identifies the step within the workflow
	StepID string `json:"step_id,omitempty"`

	// ToolName is the name of the tool involved
	ToolName string `json:"tool_name,omitempty"`

	// Resource is the resource being accessed (file path, URL, command)
	// Field is truncated to 1024 characters to prevent log injection
	Resource string `json:"resource,omitempty"`

	// Action is the action being performed (read, write, execute, connect)
	Action AccessAction `json:"action,omitempty"`

	// Decision indicates whether access was allowed
	Decision string `json:"decision"`

	// Reason explains the decision
	// Field is truncated to 512 characters to prevent log injection
	Reason string `json:"reason,omitempty"`

	// Profile is the security profile active during the event
	Profile string `json:"profile"`

	// UserID identifies the user (for multi-tenant systems)
	UserID string `json:"user_id,omitempty"`
}

// sanitizeField truncates and removes control characters from a field.
func sanitizeField(s string, maxLen int) string {
	// Truncate to max length
	if len(s) > maxLen {
		s = s[:maxLen]
	}

	// Remove control characters (except tab and newline which are escaped by JSON)
	s = strings.Map(func(r rune) rune {
		if r < 32 && r != '\t' && r != '\n' && r != '\r' {
			return -1 // Remove character
		}
		return r
	}, s)

	return s
}

// NewSecurityEvent creates a new security event with sanitized fields.
func NewSecurityEvent(eventType EventType, req AccessRequest, decision AccessDecision) SecurityEvent {
	return SecurityEvent{
		Timestamp:  time.Now().UTC(),
		EventType:  eventType,
		WorkflowID: sanitizeField(req.WorkflowID, 128),
		StepID:     sanitizeField(req.StepID, 128),
		ToolName:   sanitizeField(req.ToolName, 128),
		Resource:   sanitizeField(req.Resource, 1024),
		Action:     req.Action,
		Decision: map[bool]string{
			true:  "allowed",
			false: "denied",
		}[decision.Allowed],
		Reason:  sanitizeField(decision.Reason, 512),
		Profile: decision.Profile,
	}
}

// EventLogger logs security events.
type EventLogger interface {
	Log(event SecurityEvent)
}

// eventLogger implements EventLogger.
type eventLogger struct {
	enabled      bool
	destinations []AuditDestination
	logger       *slog.Logger
}

// NewEventLogger creates a new event logger from audit configuration.
func NewEventLogger(config AuditConfig) EventLogger {
	if !config.Enabled {
		return &eventLogger{enabled: false}
	}

	return &eventLogger{
		enabled:      true,
		destinations: config.Destinations,
		logger:       slog.Default(),
	}
}

// Log records a security event.
func (l *eventLogger) Log(event SecurityEvent) {
	if !l.enabled {
		return
	}

	// Log to structured logger
	l.logger.Info("security event",
		"event_type", event.EventType,
		"timestamp", event.Timestamp,
		"workflow_id", event.WorkflowID,
		"step_id", event.StepID,
		"tool_name", event.ToolName,
		"resource", event.Resource,
		"action", event.Action,
		"decision", event.Decision,
		"reason", event.Reason,
		"profile", event.Profile,
	)

	// TODO: Phase 4 - Implement multi-destination logging
	// For now, just log to default logger
	for _, dest := range l.destinations {
		if dest.Type == "file" {
			// Log to file in JSON format
			data, err := json.Marshal(event)
			if err != nil {
				l.logger.Error("failed to marshal security event", "error", err)
				continue
			}
			// Write to file (simplified for Phase 1)
			fmt.Printf("%s\n", string(data))
		}
	}
}
