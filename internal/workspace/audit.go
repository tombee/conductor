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

package workspace

import (
	"encoding/json"
	"log/slog"
	"time"
)

// AuditEvent represents a workspace or integration operation that should be logged.
// All credential access and configuration changes are audited for security and compliance.
type AuditEvent struct {
	// Timestamp is when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// EventType identifies the kind of operation
	EventType AuditEventType `json:"event_type"`

	// WorkspaceName identifies the workspace context
	WorkspaceName string `json:"workspace_name"`

	// IntegrationName identifies the integration (if applicable)
	IntegrationName string `json:"integration_name,omitempty"`

	// IntegrationType identifies the integration type (if applicable)
	IntegrationType string `json:"integration_type,omitempty"`

	// RunID identifies the workflow run (for access events)
	RunID string `json:"run_id,omitempty"`

	// StepID identifies the workflow step (for access events)
	StepID string `json:"step_id,omitempty"`

	// BindingMethod indicates how binding was resolved (auto/explicit)
	BindingMethod string `json:"binding_method,omitempty"`

	// Success indicates if the operation succeeded
	Success bool `json:"success"`

	// ErrorCategory categorizes failures (NOT_FOUND, AUTH_FAILED, etc.)
	ErrorCategory string `json:"error_category,omitempty"`

	// FieldsChanged lists which fields were modified (for update events)
	// Does NOT include the actual values, only field names
	FieldsChanged []string `json:"fields_changed,omitempty"`
}

// AuditEventType identifies the kind of audit event.
type AuditEventType string

const (
	// EventIntegrationCreated is logged when an integration is created
	EventIntegrationCreated AuditEventType = "integration.created"

	// EventIntegrationUpdated is logged when an integration is modified
	EventIntegrationUpdated AuditEventType = "integration.updated"

	// EventIntegrationDeleted is logged when an integration is removed
	EventIntegrationDeleted AuditEventType = "integration.deleted"

	// EventIntegrationAccessed is logged when an integration is used in a workflow
	EventIntegrationAccessed AuditEventType = "integration.accessed"

	// EventIntegrationTested is logged when an integration connectivity test runs
	EventIntegrationTested AuditEventType = "integration.tested"

	// EventBindingResolved is logged when a workflow requirement is bound to an integration
	EventBindingResolved AuditEventType = "binding.resolved"

	// EventBindingFailed is logged when binding resolution fails
	EventBindingFailed AuditEventType = "binding.failed"
)

// AuditLogger handles structured logging of workspace audit events.
// All events are logged as JSON for SIEM integration and compliance tracking.
type AuditLogger struct {
	logger *slog.Logger
}

// NewAuditLogger creates a new audit logger.
// The logger should be configured to write to a dedicated audit log file.
func NewAuditLogger(logger *slog.Logger) *AuditLogger {
	return &AuditLogger{
		logger: logger,
	}
}

// Log writes an audit event to the structured log.
// Events are logged at INFO level for successful operations and ERROR level for failures.
func (a *AuditLogger) Log(event AuditEvent) {
	// Set timestamp if not already set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Convert event to JSON for structured logging
	eventJSON, err := json.Marshal(event)
	if err != nil {
		// Fallback if JSON marshaling fails - log basic info
		a.logger.Error("failed to marshal audit event",
			"error", err,
			"event_type", event.EventType,
		)
		return
	}

	// Choose log level based on success
	level := slog.LevelInfo
	if !event.Success {
		level = slog.LevelError
	}

	// Log the event
	a.logger.Log(nil, level, "audit event",
		"event_type", event.EventType,
		"workspace", event.WorkspaceName,
		"integration", event.IntegrationName,
		"success", event.Success,
		"audit_json", string(eventJSON),
	)
}

// LogIntegrationCreated logs when an integration is created.
func (a *AuditLogger) LogIntegrationCreated(workspace, integrationName, integrationType string) {
	a.Log(AuditEvent{
		EventType:       EventIntegrationCreated,
		WorkspaceName:   workspace,
		IntegrationName: integrationName,
		IntegrationType: integrationType,
		Success:         true,
	})
}

// LogIntegrationUpdated logs when an integration is updated.
func (a *AuditLogger) LogIntegrationUpdated(workspace, integrationName string, fieldsChanged []string) {
	a.Log(AuditEvent{
		EventType:       EventIntegrationUpdated,
		WorkspaceName:   workspace,
		IntegrationName: integrationName,
		FieldsChanged:   fieldsChanged,
		Success:         true,
	})
}

// LogIntegrationDeleted logs when an integration is deleted.
func (a *AuditLogger) LogIntegrationDeleted(workspace, integrationName string) {
	a.Log(AuditEvent{
		EventType:       EventIntegrationDeleted,
		WorkspaceName:   workspace,
		IntegrationName: integrationName,
		Success:         true,
	})
}

// LogIntegrationAccessed logs when an integration is used in a workflow.
func (a *AuditLogger) LogIntegrationAccessed(workspace, integrationName, runID, stepID string) {
	a.Log(AuditEvent{
		EventType:       EventIntegrationAccessed,
		WorkspaceName:   workspace,
		IntegrationName: integrationName,
		RunID:           runID,
		StepID:          stepID,
		Success:         true,
	})
}

// LogBindingResolved logs when a workflow requirement is successfully bound.
func (a *AuditLogger) LogBindingResolved(workspace, integrationName, runID, bindingMethod string) {
	a.Log(AuditEvent{
		EventType:       EventBindingResolved,
		WorkspaceName:   workspace,
		IntegrationName: integrationName,
		RunID:           runID,
		BindingMethod:   bindingMethod,
		Success:         true,
	})
}

// LogBindingFailed logs when binding resolution fails.
func (a *AuditLogger) LogBindingFailed(workspace, requirement, runID, errorCategory string) {
	a.Log(AuditEvent{
		EventType:     EventBindingFailed,
		WorkspaceName: workspace,
		RunID:         runID,
		ErrorCategory: errorCategory,
		Success:       false,
	})
}
