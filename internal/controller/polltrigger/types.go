// Package polltrigger implements poll-based triggers for external service events.
//
// Poll triggers periodically query external service APIs (PagerDuty, Slack, Jira, Datadog)
// for events relevant to the user and fire workflows for new events. This enables personal
// automation use cases without requiring webhooks or public endpoints.
package polltrigger

import (
	"context"
	"time"
)

// PollState tracks the state of a poll trigger across executions.
// This enables timestamp-first deduplication and recovery from controller restarts.
type PollState struct {
	// TriggerID is the unique identifier for this poll trigger
	TriggerID string `json:"trigger_id"`

	// WorkflowPath is the path to the workflow file
	WorkflowPath string `json:"workflow_path"`

	// Integration is the integration being polled (slack, pagerduty, jira, datadog)
	Integration string `json:"integration"`

	// LastPollTime is the PRIMARY deduplication mechanism - always passed to API as "since" parameter
	LastPollTime time.Time `json:"last_poll_time"`

	// HighWaterMark tracks the newest event timestamp we've processed
	// May differ from LastPollTime if events arrive out of order
	HighWaterMark time.Time `json:"high_water_mark"`

	// SeenEvents is the SECONDARY deduplication for edge cases within poll windows
	// Maps event ID to first seen timestamp
	SeenEvents map[string]int64 `json:"seen_events"`

	// Cursor is the API pagination cursor, if applicable
	Cursor string `json:"cursor,omitempty"`

	// LastError is the last error message encountered
	LastError string `json:"last_error,omitempty"`

	// ErrorCount tracks consecutive errors for circuit breaker
	ErrorCount int `json:"error_count"`

	// CreatedAt is when this poll trigger was first registered
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when this state was last updated
	UpdatedAt time.Time `json:"updated_at"`
}

// PollTriggerContext contains the data passed to workflows when a poll trigger fires.
type PollTriggerContext struct {
	// Integration is the integration that sourced this event (slack, pagerduty, jira, datadog)
	Integration string `json:"integration"`

	// TriggerTime is when the trigger fired (when we decided to invoke the workflow)
	TriggerTime time.Time `json:"trigger_time"`

	// PollTime is when the poll executed (may be slightly before TriggerTime)
	PollTime time.Time `json:"poll_time"`

	// Event is the integration-specific event data
	// Structure varies by integration but always includes common fields like id, timestamp
	Event map[string]interface{} `json:"event"`

	// Query contains the query parameters that matched this event (for debugging)
	Query map[string]interface{} `json:"query"`
}

// IntegrationPoller defines the interface for integration-specific polling implementations.
// Each integration (PagerDuty, Slack, Jira, Datadog) implements this interface to provide
// polling logic for its API.
type IntegrationPoller interface {
	// Poll queries the integration API for new events since the given timestamp.
	// Returns a list of events and an optional cursor for pagination.
	// The poller should respect the state.LastPollTime as the "since" parameter.
	Poll(ctx context.Context, state *PollState, query map[string]interface{}) ([]map[string]interface{}, string, error)

	// Name returns the integration name (e.g., "pagerduty", "slack")
	Name() string
}

// PollTriggerRegistration contains the configuration for a registered poll trigger.
type PollTriggerRegistration struct {
	// TriggerID is the unique identifier for this trigger
	TriggerID string

	// WorkflowPath is the path to the workflow file
	WorkflowPath string

	// Integration is the integration to poll
	Integration string

	// Query contains integration-specific query parameters
	Query map[string]interface{}

	// Interval is the polling interval in seconds (minimum 10)
	Interval int

	// Startup defines behavior on controller start
	Startup string

	// Backfill duration in seconds (only used when Startup is "backfill")
	Backfill int

	// InputMapping maps trigger event fields to workflow inputs
	InputMapping map[string]string
}
