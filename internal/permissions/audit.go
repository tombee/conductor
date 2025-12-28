package permissions

import (
	"context"
	"sync"
	"time"
)

// AuditEventType represents the type of permission audit event.
type AuditEventType string

const (
	// EventPermissionDenied is logged when a permission check denies access
	EventPermissionDenied AuditEventType = "permission.denied"

	// EventPermissionWouldBlock is logged when a permission would block but enforcement is disabled
	EventPermissionWouldBlock AuditEventType = "permission.would_block"

	// EventBaselineBlocked is logged when a baseline security control blocks access
	// (e.g., metadata endpoint, private IP, dangerous env var)
	EventBaselineBlocked AuditEventType = "permission.baseline_blocked"
)

// AuditEvent represents a permission audit event.
type AuditEvent struct {
	// Timestamp when the event occurred
	Timestamp time.Time `json:"timestamp"`

	// Type of audit event
	Type AuditEventType `json:"type"`

	// WorkflowID identifies the workflow
	WorkflowID string `json:"workflow_id"`

	// StepID identifies the step within the workflow
	StepID string `json:"step_id"`

	// PermissionType is the type of permission that was checked (paths.read, network.request, etc.)
	PermissionType string `json:"permission_type"`

	// Resource is the resource that was denied
	Resource string `json:"resource"`

	// Allowed patterns that were configured
	Allowed []string `json:"allowed,omitempty"`

	// Blocked patterns that were configured
	Blocked []string `json:"blocked,omitempty"`

	// Message provides additional context
	Message string `json:"message"`

	// Enforced indicates whether the denial was enforced (true) or just logged (false)
	Enforced bool `json:"enforced"`
}

// AuditLogger handles permission audit logging with rate limiting.
type AuditLogger struct {
	mu sync.Mutex

	// events stores recent audit events per workflow
	events map[string][]time.Time

	// maxEventsPerMinute is the rate limit per workflow (default: 100)
	maxEventsPerMinute int

	// handler is called for each audit event
	handler func(AuditEvent)
}

// NewAuditLogger creates a new audit logger with the specified rate limit.
func NewAuditLogger(maxEventsPerMinute int, handler func(AuditEvent)) *AuditLogger {
	if maxEventsPerMinute <= 0 {
		maxEventsPerMinute = 100
	}

	return &AuditLogger{
		events:             make(map[string][]time.Time),
		maxEventsPerMinute: maxEventsPerMinute,
		handler:            handler,
	}
}

// Log logs an audit event with rate limiting.
// Returns true if the event was logged, false if rate limited.
func (l *AuditLogger) Log(event AuditEvent) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Set timestamp if not provided
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Check rate limit
	if !l.checkRateLimit(event.WorkflowID, event.Timestamp) {
		// Rate limited - don't log
		return false
	}

	// Record event timestamp for rate limiting
	l.recordEvent(event.WorkflowID, event.Timestamp)

	// Call handler if provided
	if l.handler != nil {
		l.handler(event)
	}

	return true
}

// checkRateLimit checks if logging an event would exceed the rate limit.
func (l *AuditLogger) checkRateLimit(workflowID string, timestamp time.Time) bool {
	events := l.events[workflowID]

	// Clean up old events (older than 1 minute)
	cutoff := timestamp.Add(-1 * time.Minute)
	validEvents := make([]time.Time, 0, len(events))
	for _, t := range events {
		if t.After(cutoff) {
			validEvents = append(validEvents, t)
		}
	}
	l.events[workflowID] = validEvents

	// Check if we're at the limit
	return len(validEvents) < l.maxEventsPerMinute
}

// recordEvent records an event timestamp for rate limiting.
func (l *AuditLogger) recordEvent(workflowID string, timestamp time.Time) {
	l.events[workflowID] = append(l.events[workflowID], timestamp)
}

// LogPermissionDenied logs a permission denial event from a PermissionError.
func (l *AuditLogger) LogPermissionDenied(ctx context.Context, workflowID, stepID string, err *PermissionError, enforced bool) bool {
	if err == nil {
		return false
	}

	event := AuditEvent{
		Type:           EventPermissionDenied,
		WorkflowID:     workflowID,
		StepID:         stepID,
		PermissionType: err.Type,
		Resource:       err.Resource,
		Allowed:        err.Allowed,
		Blocked:        err.Blocked,
		Message:        err.Message,
		Enforced:       enforced,
	}

	return l.Log(event)
}

// LogBaselineBlocked logs a baseline security control blocking access.
func (l *AuditLogger) LogBaselineBlocked(ctx context.Context, workflowID, stepID, permissionType, resource, message string) bool {
	event := AuditEvent{
		Type:           EventBaselineBlocked,
		WorkflowID:     workflowID,
		StepID:         stepID,
		PermissionType: permissionType,
		Resource:       resource,
		Message:        message,
		Enforced:       true, // Baseline controls are always enforced
	}

	return l.Log(event)
}

// LogWouldBlock logs when a permission would block but enforcement is disabled.
func (l *AuditLogger) LogWouldBlock(ctx context.Context, workflowID, stepID string, err *PermissionError) bool {
	if err == nil {
		return false
	}

	event := AuditEvent{
		Type:           EventPermissionWouldBlock,
		WorkflowID:     workflowID,
		StepID:         stepID,
		PermissionType: err.Type,
		Resource:       err.Resource,
		Allowed:        err.Allowed,
		Blocked:        err.Blocked,
		Message:        err.Message,
		Enforced:       false,
	}

	return l.Log(event)
}

// GetEventCount returns the number of events logged for a workflow in the last minute.
func (l *AuditLogger) GetEventCount(workflowID string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-1 * time.Minute)

	events := l.events[workflowID]
	count := 0
	for _, t := range events {
		if t.After(cutoff) {
			count++
		}
	}

	return count
}

// Reset clears all recorded events for a workflow.
func (l *AuditLogger) Reset(workflowID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.events, workflowID)
}
