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

package workflow

import (
	"fmt"
	"regexp"

	"github.com/tombee/conductor/pkg/errors"
)

type TriggerConfig struct {
	// Webhook configures webhook listeners
	Webhook *WebhookTrigger `yaml:"webhook,omitempty" json:"webhook,omitempty"`

	// API configures API endpoint listeners (Bearer token auth)
	API *APITriggerConfig `yaml:"api,omitempty" json:"api,omitempty"`

	// Schedule configures scheduled execution
	Schedule *ScheduleTrigger `yaml:"schedule,omitempty" json:"schedule,omitempty"`

	// File configures file watcher listeners
	File *FileTriggerConfig `yaml:"file,omitempty" json:"file,omitempty"`

	// Poll configures poll-based triggers for external service events
	Poll *PollTriggerConfig `yaml:"poll,omitempty" json:"poll,omitempty"`
}

// APIListenerConfig defines API endpoint authentication configuration.
type APITriggerConfig struct {
	// Secret is the Bearer token required to trigger this workflow via API.
	// Callers must provide this as: Authorization: Bearer <secret>
	// Should be a strong, cryptographically random value (recommended: >=32 bytes).
	// Can be an environment variable reference like ${API_SECRET}
	Secret string `yaml:"secret" json:"secret"`
}

// TriggerType represents the type of trigger.
type TriggerType string

const (
	TriggerTypeWebhook  TriggerType = "webhook"
	TriggerTypeSchedule TriggerType = "schedule"
	TriggerTypeFile     TriggerType = "file"
	TriggerTypeManual   TriggerType = "manual"
)

// WebhookTrigger defines webhook trigger configuration.
type WebhookTrigger struct {
	// Path is the URL path for the webhook (e.g., "/webhooks/my-workflow")
	Path string `yaml:"path" json:"path"`

	// Source is the webhook source type (github, slack, generic)
	Source string `yaml:"source,omitempty" json:"source,omitempty"`

	// Events limits which events trigger the workflow
	Events []string `yaml:"events,omitempty" json:"events,omitempty"`

	// Secret for signature verification (can be env var reference like ${SECRET_NAME})
	Secret string `yaml:"secret,omitempty" json:"secret,omitempty"`

	// InputMapping maps webhook payload fields to workflow inputs
	InputMapping map[string]string `yaml:"input_mapping,omitempty" json:"input_mapping,omitempty"`
}

// ScheduleTrigger defines schedule trigger configuration.
type ScheduleTrigger struct {
	// Cron is the cron expression
	Cron string `yaml:"cron" json:"cron"`

	// Timezone for cron evaluation (e.g., "America/New_York")
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`

	// Enabled controls if this schedule is active
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// Inputs are the static inputs to pass when scheduled
	Inputs map[string]any `yaml:"inputs,omitempty" json:"inputs,omitempty"`
}

// FileTriggerConfig defines file watcher trigger configuration.
type FileTriggerConfig struct {
	// Paths are the filesystem paths to watch
	Paths []string `yaml:"paths" json:"paths"`

	// Events are the event types to watch (created, modified, deleted, renamed)
	// If empty, defaults to all event types
	Events []string `yaml:"events,omitempty" json:"events,omitempty"`

	// IncludePatterns are glob patterns for files to include
	// If empty, all files are included
	IncludePatterns []string `yaml:"include_patterns,omitempty" json:"include_patterns,omitempty"`

	// ExcludePatterns are glob patterns for files to exclude
	// Applied after include patterns
	ExcludePatterns []string `yaml:"exclude_patterns,omitempty" json:"exclude_patterns,omitempty"`

	// Debounce is the duration string to wait for additional events before triggering (e.g., "500ms", "1s")
	// Zero or empty disables debouncing
	Debounce string `yaml:"debounce,omitempty" json:"debounce,omitempty"`

	// BatchMode determines if events during debounce window are batched together
	// If false, only the last event is delivered
	BatchMode bool `yaml:"batch_mode,omitempty" json:"batch_mode,omitempty"`

	// MaxTriggersPerMinute limits the rate of workflow triggers
	// Zero means no limit
	MaxTriggersPerMinute int `yaml:"max_triggers_per_minute,omitempty" json:"max_triggers_per_minute,omitempty"`

	// Recursive enables watching subdirectories
	Recursive bool `yaml:"recursive,omitempty" json:"recursive,omitempty"`

	// MaxDepth limits recursive watching depth (0 = unlimited)
	MaxDepth int `yaml:"max_depth,omitempty" json:"max_depth,omitempty"`

	// Inputs are the static inputs to pass when triggered
	Inputs map[string]any `yaml:"inputs,omitempty" json:"inputs,omitempty"`
}

// PollTriggerConfig defines poll-based trigger configuration for external service events.
// Poll triggers periodically query external APIs (PagerDuty, Slack, Jira, Datadog) for
// events relevant to the user and fire workflows for new events.
type PollTriggerConfig struct {
	// Integration specifies which integration to poll (slack, pagerduty, jira, datadog)
	Integration string `yaml:"integration" json:"integration"`

	// Query contains integration-specific query parameters for filtering events
	Query map[string]interface{} `yaml:"query" json:"query"`

	// Interval is the polling interval (e.g., "30s", "1m")
	// Minimum: 10s, Default: 30s
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"`

	// Startup defines behavior on controller start
	// - "since_last" (default): Process events since last poll time
	// - "ignore_historical": Only process events from now forward
	// - "backfill": Process events from specified duration ago
	Startup string `yaml:"startup,omitempty" json:"startup,omitempty"`

	// Backfill duration for startup backfill mode (e.g., "1h", "4h")
	// Only used when Startup is "backfill". Maximum: 24h
	Backfill string `yaml:"backfill,omitempty" json:"backfill,omitempty"`

	// InputMapping maps trigger event fields to workflow inputs
	// Example: incident_id: "{{.trigger.event.id}}"
	InputMapping map[string]string `yaml:"input_mapping,omitempty" json:"input_mapping,omitempty"`
}

// Validate checks the trigger configuration for errors.
func (t *TriggerConfig) Validate() error {
	// Check that only one trigger type is configured
	triggerCount := 0
	if t.Webhook != nil {
		triggerCount++
	}
	if t.API != nil {
		triggerCount++
	}
	if t.Schedule != nil {
		triggerCount++
	}
	if t.Poll != nil {
		triggerCount++
	}
	if t.File != nil {
		triggerCount++
	}

	if triggerCount == 0 {
		return &errors.ValidationError{
			Field:      "listen",
			Message:    "at least one trigger type must be configured",
			Suggestion: "add one of: webhook, api, schedule, poll, or file",
		}
	}

	if triggerCount > 1 {
		return &errors.ValidationError{
			Field:      "listen",
			Message:    "only one trigger type can be configured per workflow",
			Suggestion: "remove all but one trigger type (webhook, api, schedule, poll, or file)",
		}
	}

	// Validate poll trigger if present
	if t.Poll != nil {
		if err := t.Poll.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks the poll trigger configuration for errors.
func (p *PollTriggerConfig) Validate() error {
	// Validate integration is specified
	if p.Integration == "" {
		return &errors.ValidationError{
			Field:      "integration",
			Message:    "integration is required for poll triggers",
			Suggestion: "specify one of: slack, pagerduty, jira, datadog",
		}
	}

	// Validate integration is a supported type
	validIntegrations := map[string]bool{
		"slack":     true,
		"pagerduty": true,
		"jira":      true,
		"datadog":   true,
	}
	if !validIntegrations[p.Integration] {
		return &errors.ValidationError{
			Field:      "integration",
			Message:    fmt.Sprintf("unsupported integration: %s", p.Integration),
			Suggestion: "use one of: slack, pagerduty, jira, datadog",
		}
	}

	// Validate query is provided
	if len(p.Query) == 0 {
		return &errors.ValidationError{
			Field:      "query",
			Message:    "query parameters are required for poll triggers",
			Suggestion: "add query parameters specific to the integration (e.g., user_id, mentions, assignee, tags)",
		}
	}

	// Validate interval if specified
	if p.Interval != "" {
		duration, err := parseDuration(p.Interval)
		if err != nil {
			return &errors.ValidationError{
				Field:      "interval",
				Message:    fmt.Sprintf("invalid interval format: %s", p.Interval),
				Suggestion: "use duration format like '30s', '1m', '5m'",
			}
		}
		if duration < 10 {
			return &errors.ValidationError{
				Field:      "interval",
				Message:    fmt.Sprintf("interval must be at least 10s, got: %s", p.Interval),
				Suggestion: "increase interval to at least 10s to avoid excessive API calls",
			}
		}
	}

	// Validate startup if specified
	if p.Startup != "" {
		validStartup := map[string]bool{
			"since_last":        true,
			"ignore_historical": true,
			"backfill":          true,
		}
		if !validStartup[p.Startup] {
			return &errors.ValidationError{
				Field:      "startup",
				Message:    fmt.Sprintf("invalid startup mode: %s", p.Startup),
				Suggestion: "use one of: since_last, ignore_historical, backfill",
			}
		}

		// If startup is backfill, validate backfill duration
		if p.Startup == "backfill" {
			if p.Backfill == "" {
				return &errors.ValidationError{
					Field:      "backfill",
					Message:    "backfill duration is required when startup is 'backfill'",
					Suggestion: "specify backfill duration like '1h', '4h', '24h'",
				}
			}
			duration, err := parseDuration(p.Backfill)
			if err != nil {
				return &errors.ValidationError{
					Field:      "backfill",
					Message:    fmt.Sprintf("invalid backfill duration format: %s", p.Backfill),
					Suggestion: "use duration format like '1h', '4h', '24h'",
				}
			}
			// Maximum 24 hours
			if duration > 24*3600 {
				return &errors.ValidationError{
					Field:      "backfill",
					Message:    fmt.Sprintf("backfill duration cannot exceed 24h, got: %s", p.Backfill),
					Suggestion: "reduce backfill duration to at most 24h",
				}
			}
		}
	}

	// Validate query parameters match expected pattern (alphanumeric, underscore, hyphen)
	validPattern := regexp.MustCompile(`^[a-zA-Z0-9_@.-]+$`)
	for key, value := range p.Query {
		// Skip validation for array/object values
		if strValue, ok := value.(string); ok {
			if !validPattern.MatchString(strValue) {
				return &errors.ValidationError{
					Field:      fmt.Sprintf("query.%s", key),
					Message:    fmt.Sprintf("invalid query parameter value: %s", strValue),
					Suggestion: "query values must contain only alphanumeric characters, underscores, hyphens, @ and dots",
				}
			}
		}
	}

	return nil
}

// parseDuration parses a duration string like "30s", "1m", "1h" and returns seconds.
func parseDuration(s string) (int, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format")
	}

	var multiplier int
	unit := s[len(s)-1]
	switch unit {
	case 's':
		multiplier = 1
	case 'm':
		multiplier = 60
	case 'h':
		multiplier = 3600
	default:
		return 0, fmt.Errorf("invalid duration unit: %c (must be s, m, or h)", unit)
	}

	valueStr := s[:len(s)-1]
	var value int
	_, err := fmt.Sscanf(valueStr, "%d", &value)
	if err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", valueStr)
	}

	return value * multiplier, nil
}
