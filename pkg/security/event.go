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
	"log/slog"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/security/audit"
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
	Close() error
}

// eventLogger implements EventLogger.
type eventLogger struct {
	enabled          bool
	auditLogger      *audit.Logger
	logger           *slog.Logger
	metricsCollector *MetricsCollector
}

// NewEventLogger creates a new event logger from audit configuration.
func NewEventLogger(config AuditConfig) EventLogger {
	if !config.Enabled {
		return &eventLogger{enabled: false}
	}

	// Convert security audit config to audit.Config
	auditCfg := audit.Config{
		Destinations: make([]audit.DestinationConfig, 0, len(config.Destinations)+1),
		BufferSize:   1000, // Default buffer size
	}

	// If rotation is enabled, replace file destinations with rotating-file
	if config.Rotation.Enabled {
		// Find a file destination to use as the base path for rotation
		var basePath string
		var format string = "json"

		for _, dest := range config.Destinations {
			if dest.Type == "file" {
				basePath = dest.Path
				if dest.Format != "" {
					format = dest.Format
				}
				break
			}
		}

		// If no file destination, use a default path
		if basePath == "" {
			basePath = "~/.conductor/logs/audit.log"
		}

		// Add rotating file destination
		auditCfg.Destinations = append(auditCfg.Destinations, audit.DestinationConfig{
			Type:        "rotating-file",
			Path:        basePath,
			Format:      format,
			MaxSize:     config.Rotation.MaxSizeMB * 1024 * 1024, // Convert MB to bytes
			MaxAge:      time.Duration(config.Rotation.MaxAgeDays) * 24 * time.Hour,
			RotateDaily: true,
			Compress:    config.Rotation.Compress,
		})

		// Add non-file destinations (syslog, webhook) as-is
		for _, dest := range config.Destinations {
			if dest.Type != "file" {
				auditCfg.Destinations = append(auditCfg.Destinations, audit.DestinationConfig{
					Type:     dest.Type,
					Path:     dest.Path,
					Format:   dest.Format,
					Facility: dest.Facility,
					Severity: dest.Severity,
					URL:      dest.URL,
					Headers:  dest.Headers,
				})
			}
		}
	} else {
		// No rotation - use destinations as-is
		for _, dest := range config.Destinations {
			auditCfg.Destinations = append(auditCfg.Destinations, audit.DestinationConfig{
				Type:     dest.Type,
				Path:     dest.Path,
				Format:   dest.Format,
				Facility: dest.Facility,
				Severity: dest.Severity,
				URL:      dest.URL,
				Headers:  dest.Headers,
			})
		}
	}

	// Create audit logger
	auditLogger, err := audit.NewLogger(auditCfg)
	if err != nil {
		slog.Default().Error("failed to create audit logger",
			"error", err)
		return &eventLogger{enabled: false}
	}

	return &eventLogger{
		enabled:     true,
		auditLogger: auditLogger,
		logger:      slog.Default(),
	}
}

// SetMetricsCollector sets the metrics collector for the event logger.
func (l *eventLogger) SetMetricsCollector(collector *MetricsCollector) {
	l.metricsCollector = collector
}

// Log records a security event.
func (l *eventLogger) Log(event SecurityEvent) {
	if !l.enabled {
		return
	}

	// Convert SecurityEvent to audit.Event
	auditEvent := audit.Event{
		Timestamp:    event.Timestamp,
		EventType:    string(event.EventType),
		WorkflowID:   event.WorkflowID,
		StepID:       event.StepID,
		ToolName:     event.ToolName,
		Resource:     event.Resource,
		ResourceType: "", // Not used in SecurityEvent
		Action:       string(event.Action),
		Decision:     event.Decision,
		Reason:       event.Reason,
		Profile:      event.Profile,
		UserID:       event.UserID,
	}

	// Log to audit logger if available
	if l.auditLogger != nil {
		l.auditLogger.Log(auditEvent)

		// Record audit event metrics with buffer utilization
		if l.metricsCollector != nil {
			bufferUtil := l.auditLogger.BufferUtilization()
			bufferUsed := int(bufferUtil * 1000) // Approximation based on buffer size
			l.metricsCollector.RecordAuditEvent(true, bufferUsed, 1000)
		}
	}

	// Also log to structured logger for visibility
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
}

// Close closes the event logger and flushes any buffered events.
func (l *eventLogger) Close() error {
	if l.auditLogger != nil {
		return l.auditLogger.Close()
	}
	return nil
}
