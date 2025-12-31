// Package workflow provides workflow orchestration primitives.
//
// Workflow definitions follow the simple workflow format, which allows
// for concise YAML-based workflow specifications. The version field is optional
// and defaults to "1.0". LLM steps support model tier selection (fast, balanced,
// strategic) and inline prompt/system configuration without requiring separate
// action definitions.
package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tombee/conductor/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Definition represents a YAML-based workflow definition.
// It defines the structure, steps, conditions, and outputs of a workflow
// that can be loaded from a YAML file and executed by the workflow executor.
//
// The Version field is optional and will default to "1.0"
// if not specified. This allows for minimal workflow definitions that only
// require a name and steps array.
type Definition struct {
	// Name is the workflow identifier
	Name string `yaml:"name" json:"name"`

	// Description provides human-readable context about the workflow
	Description string `yaml:"description" json:"description"`

	// Version tracks the workflow definition schema version (optional, defaults to "1.0")
	Version string `yaml:"version" json:"version"`

	// Listen defines how this workflow can be invoked (webhooks, API, schedules)
	// Replaces the deprecated Triggers field
	Trigger *TriggerConfig `yaml:"listen,omitempty" json:"listen,omitempty"`

	// Inputs defines the expected input parameters for the workflow
	Inputs []InputDefinition `yaml:"inputs" json:"inputs"`

	// Steps are the executable units of the workflow
	Steps []StepDefinition `yaml:"steps" json:"steps"`

	// Outputs define what data is returned when the workflow completes
	Outputs []OutputDefinition `yaml:"outputs" json:"outputs"`

	// Agents define named agents with preferences and capabilities
	Agents map[string]AgentDefinition `yaml:"agents,omitempty" json:"agents,omitempty"`

	// Functions define workflow-level LLM-callable functions (HTTP and script functions)
	Functions []FunctionDefinition `yaml:"functions,omitempty" json:"functions,omitempty"`

	// MCPServers define MCP server configurations for tool providers
	MCPServers []MCPServerConfig `yaml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`

	// Integrations define declarative HTTP/SSH integrations for external services
	Integrations map[string]IntegrationDefinition `yaml:"integrations,omitempty" json:"integrations,omitempty"`

	// Permissions define access control at the workflow level (SPEC-141)
	// Step-level permissions are intersected with these (most restrictive wins)
	Permissions *PermissionDefinition `yaml:"permissions,omitempty" json:"permissions,omitempty"`

	// Requires declares abstract service dependencies for this workflow (SPEC-130)
	// This enables portable workflow definitions that don't embed credentials.
	// Runtime bindings are provided by execution profiles.
	Requires *RequirementsDefinition `yaml:"requires,omitempty" json:"requires,omitempty"`

	// Security defines explicit resource access control for this workflow.
	// Declares which filesystem paths, network hosts, and shell commands
	// the workflow can access. Empty or omitted means no access (secure by default).
	Security *SecurityAccessConfig `yaml:"security,omitempty" json:"security,omitempty"`
}

// ListenConfig defines how a workflow can be invoked.
// This replaces the old TriggerDefinition structure.
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

// TriggerDefinition defines how a workflow can be triggered.
// DEPRECATED: Use TriggerConfig instead. This type is kept for backward compatibility
// during migration, but parsing will return an error if triggers: is used.
type TriggerDefinition struct {
	// Type is the trigger type (webhook, schedule, file, manual)
	Type TriggerType `yaml:"type" json:"type"`

	// Webhook configuration (for webhook triggers)
	Webhook *WebhookTrigger `yaml:"webhook,omitempty" json:"webhook,omitempty"`

	// Schedule configuration (for schedule triggers)
	Schedule *ScheduleTrigger `yaml:"schedule,omitempty" json:"schedule,omitempty"`

	// File configuration (for file watcher triggers)
	File *FileTriggerConfig `yaml:"file,omitempty" json:"file,omitempty"`
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
			"since_last":         true,
			"ignore_historical":  true,
			"backfill":           true,
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

// InputDefinition describes a workflow input parameter.
type InputDefinition struct {
	// Name is the input parameter identifier
	Name string `yaml:"name" json:"name"`

	// Type specifies the data type (string, number, boolean, object, array, enum)
	Type string `yaml:"type" json:"type"`

	// Required indicates if this input must be provided
	Required bool `yaml:"required" json:"required"`

	// Default provides a fallback value if input is not provided
	Default interface{} `yaml:"default,omitempty" json:"default,omitempty"`

	// Description explains what this input is for
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Enum defines the allowed values for enum-type inputs
	Enum []string `yaml:"enum,omitempty" json:"enum,omitempty"`
}

// StepDefinition represents a single step in a workflow.
//
// LLM steps have simplified inline configuration:
//   - Model: tier selection (fast/balanced/strategic), defaults to "balanced"
//   - System: optional system prompt for LLM behavior guidance
//   - Prompt: user prompt with template variable support ({{.input}}, {{.steps.id.response}})
//
// Template variables support workflow inputs and step outputs for data flow
// between steps. The Name field is optional for concise definitions.
type StepDefinition struct {
	// ID is the unique step identifier within this workflow
	ID string `yaml:"id" json:"id"`

	// Name is a human-readable step name (optional)
	Name string `yaml:"name" json:"name"`

	// Type specifies the step type (condition, parallel, etc.)
	Type StepType `yaml:"type" json:"type"`

	// hasExplicitID tracks whether the ID was explicitly set in YAML
	// Used for auto-generation to skip steps with explicit IDs
	hasExplicitID bool

	// Agent references an agent definition for provider resolution
	Agent string `yaml:"agent,omitempty" json:"agent,omitempty"`

	// Inputs maps input names to values (can reference previous step outputs)
	Inputs map[string]interface{} `yaml:"inputs,omitempty" json:"inputs,omitempty"`

	// Model specifies the model tier for LLM steps (fast, balanced, strategic)
	// Defaults to "balanced" if not specified
	Model string `yaml:"model,omitempty" json:"model,omitempty"`

	// System is the system prompt for LLM steps, used to guide model behavior
	// Optional - only needed when specific role/behavior is required
	System string `yaml:"system,omitempty" json:"system,omitempty"`

	// Prompt is the user prompt for LLM steps (required for type=llm)
	// Supports template variables: {{.input}}, {{.steps.stepid.response}}
	Prompt string `yaml:"prompt,omitempty" json:"prompt,omitempty"`

	// OutputSchema defines the expected JSON Schema for LLM step outputs
	// Mutually exclusive with OutputType
	OutputSchema map[string]interface{} `yaml:"output_schema,omitempty" json:"output_schema,omitempty"`

	// OutputType specifies a built-in output type (classification, decision, extraction)
	// Mutually exclusive with OutputSchema
	OutputType string `yaml:"output_type,omitempty" json:"output_type,omitempty"`

	// OutputOptions provides configuration for built-in output types
	// Used with OutputType to specify categories, choices, fields, etc.
	OutputOptions map[string]interface{} `yaml:"output_options,omitempty" json:"output_options,omitempty"`

	// Tools lists the custom tools this step can access (references workflow-level tools by name)
	Tools []string `yaml:"tools,omitempty" json:"tools,omitempty"`

	// Integration specifies the integration and operation to invoke (format: "integration_name.operation_name")
	// Only valid for type: integration steps
	Integration string `yaml:"integration,omitempty" json:"integration,omitempty"`

	// Action specifies the action name for builtin operations (file, shell, http, transform)
	// Only valid for type: integration steps when using builtin actions
	Action string `yaml:"action,omitempty" json:"action,omitempty"`

	// Workflow specifies the path to a sub-workflow YAML file to invoke
	// Only valid for type: workflow steps
	// Path must be relative to the parent workflow file (e.g., "./helpers/util.yaml")
	Workflow string `yaml:"workflow,omitempty" json:"workflow,omitempty"`

	// Operation specifies the operation to invoke on the action or integration
	// Only valid for type: integration steps
	Operation string `yaml:"operation,omitempty" json:"operation,omitempty"`

	// Condition defines when this step should execute
	Condition *ConditionDefinition `yaml:"condition,omitempty" json:"condition,omitempty"`

	// OnError specifies error handling behavior
	OnError *ErrorHandlingDefinition `yaml:"on_error,omitempty" json:"on_error,omitempty"`

	// Timeout sets the maximum execution time for this step (in seconds)
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Retry configures retry behavior for this step
	Retry *RetryDefinition `yaml:"retry,omitempty" json:"retry,omitempty"`

	// MaxTokens sets the maximum token count for this step
	MaxTokens *int `yaml:"max_tokens,omitempty" json:"max_tokens,omitempty"`

	// Steps contains nested steps for parallel execution (type: parallel)
	// Each nested step executes concurrently and results are aggregated
	Steps []StepDefinition `yaml:"steps,omitempty" json:"steps,omitempty"`

	// MaxConcurrency limits the number of concurrent nested steps for parallel execution.
	// When set, overrides the executor's default parallelism limit.
	// Useful for resource-intensive steps like agent launches.
	// If 0, uses the executor's default (currently 3).
	MaxConcurrency int `yaml:"max_concurrency,omitempty" json:"max_concurrency,omitempty"`

	// Foreach enables parallel iteration over an array input.
	// The value should be a template expression that resolves to an array.
	// Each nested step in Steps will be executed once per array element with access to:
	//   - .item: the current array element
	//   - .index: zero-based position in the array
	//   - .total: total number of elements in the array
	// Results are collected as an array in the original order.
	// Only valid for type: parallel steps.
	Foreach string `yaml:"foreach,omitempty" json:"foreach,omitempty"`

// MaxIterations limits loop iterations (required for type: loop).
	// Must be between 1 and 100.
	MaxIterations int `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`

	// Until is the termination condition expression (required for type: loop).
	// Evaluated after each iteration (do-while semantics).
	// Loop terminates when expression evaluates to true.
	Until string `yaml:"until,omitempty" json:"until,omitempty"`

	// Permissions define access control at the step level (SPEC-141)
	// Step-level permissions are intersected with workflow permissions (most restrictive wins)
	Permissions *PermissionDefinition `yaml:"permissions,omitempty" json:"permissions,omitempty"`
}

// StepType represents the type of workflow step.
type StepType string

const (
	// StepTypeCondition evaluates a condition and branches
	StepTypeCondition StepType = "condition"

	// StepTypeLLM makes an LLM API call
	StepTypeLLM StepType = "llm"

	// StepTypeParallel executes multiple steps concurrently
	StepTypeParallel StepType = "parallel"

	// StepTypeIntegration executes a declarative integration operation
	StepTypeIntegration StepType = "integration"

// StepTypeLoop executes nested steps repeatedly until a condition is met
	// or a maximum iteration count is reached
	StepTypeLoop StepType = "loop"

	// StepTypeWorkflow invokes another workflow file as a sub-workflow
	StepTypeWorkflow StepType = "workflow"
)

// ModelTier represents the model capability tier for LLM steps.
// This abstraction allows workflow authors to select models based on task
// requirements without coupling to specific provider model names.
type ModelTier string

const (
	// ModelTierFast is for simple, quick tasks requiring low latency and cost
	// Examples: haiku, gpt-4o-mini
	ModelTierFast ModelTier = "fast"

	// ModelTierBalanced is the default tier, suitable for most tasks
	// Provides good quality and reasonable performance/cost tradeoff
	// Examples: sonnet, gpt-4o
	ModelTierBalanced ModelTier = "balanced"

	// ModelTierStrategic is for complex reasoning tasks requiring advanced capabilities
	// Examples: opus, o1
	ModelTierStrategic ModelTier = "strategic"
)

// ValidModelTiers for validation
var ValidModelTiers = map[ModelTier]bool{
	ModelTierFast:      true,
	ModelTierBalanced:  true,
	ModelTierStrategic: true,
}

// ConditionDefinition defines a conditional expression.
type ConditionDefinition struct {
	// Expression is the condition to evaluate (e.g., "$.previous_step.status == 'success'")
	Expression string `yaml:"expression" json:"expression"`

	// ThenSteps are steps to execute if condition is true
	ThenSteps []string `yaml:"then_steps,omitempty" json:"then_steps,omitempty"`

	// ElseSteps are steps to execute if condition is false
	ElseSteps []string `yaml:"else_steps,omitempty" json:"else_steps,omitempty"`
}

// ErrorHandlingDefinition defines how to handle step errors.
type ErrorHandlingDefinition struct {
	// Strategy specifies the error handling approach (fail, ignore, retry, fallback)
	Strategy ErrorStrategy `yaml:"strategy" json:"strategy"`

	// FallbackStep is the step ID to execute on error (when strategy is 'fallback')
	FallbackStep string `yaml:"fallback_step,omitempty" json:"fallback_step,omitempty"`
}

// ErrorStrategy represents an error handling strategy.
type ErrorStrategy string

const (
	// ErrorStrategyFail stops workflow execution on error
	ErrorStrategyFail ErrorStrategy = "fail"

	// ErrorStrategyIgnore continues workflow execution despite error
	ErrorStrategyIgnore ErrorStrategy = "ignore"

	// ErrorStrategyRetry retries the step according to retry configuration
	ErrorStrategyRetry ErrorStrategy = "retry"

	// ErrorStrategyFallback executes a fallback step on error
	ErrorStrategyFallback ErrorStrategy = "fallback"
)

// RetryDefinition configures retry behavior for a step.
type RetryDefinition struct {
	// MaxAttempts is the maximum number of retry attempts
	MaxAttempts int `yaml:"max_attempts" json:"max_attempts"`

	// BackoffBase is the base duration for exponential backoff (in seconds)
	BackoffBase int `yaml:"backoff_base" json:"backoff_base"`

	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64 `yaml:"backoff_multiplier" json:"backoff_multiplier"`
}

// OutputDefinition describes a workflow output value.
type OutputDefinition struct {
	// Name is the output identifier
	Name string `yaml:"name" json:"name"`

	// Type specifies the output data type
	Type string `yaml:"type" json:"type"`

	// Value is an expression that computes the output value
	// (e.g., "$.final_step.result")
	Value string `yaml:"value" json:"value"`

	// Description explains what this output represents
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// AgentDefinition describes an agent with provider preferences and capability requirements.
type AgentDefinition struct {
	// Prefers is a hint about which provider family works best (not enforced)
	Prefers string `yaml:"prefers,omitempty" json:"prefers,omitempty"`

	// Capabilities lists required provider capabilities (vision, long-context, tool-use, streaming, json-mode)
	Capabilities []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

// FunctionDefinition describes a custom function that can be called by LLM steps.
// Functions are defined at the workflow level and can be either HTTP endpoints or shell scripts.
type FunctionDefinition struct {
	// Name is the unique function identifier
	Name string `yaml:"name" json:"name"`

	// Type specifies the function implementation (http or script)
	Type ToolType `yaml:"type" json:"type"`

	// Description provides human-readable context about what the function does
	Description string `yaml:"description" json:"description"`

	// Method is the HTTP method (GET, POST, etc.) for HTTP functions
	Method string `yaml:"method,omitempty" json:"method,omitempty"`

	// URL is the endpoint URL template for HTTP functions (supports {{.param}} interpolation)
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// Headers are HTTP headers for HTTP functions (supports {{.env.VAR}} interpolation)
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Command is the script path for script functions (relative to workflow file directory)
	Command string `yaml:"command,omitempty" json:"command,omitempty"`

	// InputSchema defines the expected input parameters using JSON Schema
	InputSchema map[string]interface{} `yaml:"input_schema,omitempty" json:"input_schema,omitempty"`

	// AutoApprove indicates whether the function can execute without user approval
	// Defaults to false for security
	AutoApprove bool `yaml:"auto_approve,omitempty" json:"auto_approve,omitempty"`

	// Timeout is the maximum execution time in seconds (default: 30s, max: 300s)
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// MaxResponseSize is the maximum response size in bytes (default: 1MB)
	MaxResponseSize int64 `yaml:"max_response_size,omitempty" json:"max_response_size,omitempty"`
}

// ToolType represents the type of custom tool.
type ToolType string

const (
	// ToolTypeHTTP is an HTTP endpoint tool
	ToolTypeHTTP ToolType = "http"

	// ToolTypeScript is a shell script tool
	ToolTypeScript ToolType = "script"
)

// ValidToolTypes for validation
var ValidToolTypes = map[ToolType]bool{
	ToolTypeHTTP:   true,
	ToolTypeScript: true,
}

// MCPServerConfig defines configuration for an MCP (Model Context Protocol) server.
// MCP servers provide tools that can be used in workflow steps via the tool registry.
type MCPServerConfig struct {
	// Name is the unique identifier for this MCP server
	Name string `yaml:"name" json:"name"`

	// Command is the executable to run (e.g., "npx", "python", "/usr/bin/mcp-server")
	Command string `yaml:"command" json:"command"`

	// Args are command-line arguments to pass to the server
	Args []string `yaml:"args,omitempty" json:"args,omitempty"`

	// Env are environment variables to pass to the server (e.g., ["API_KEY=xyz"])
	Env []string `yaml:"env,omitempty" json:"env,omitempty"`

	// Timeout is the default timeout for tool calls in seconds (defaults to 30)
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// IntegrationDefinition defines a declarative integration for external services.
// Integrations provide schema-validated, deterministic operations that execute without LLM involvement.
type IntegrationDefinition struct {
	// Name is inferred from the map key in workflow.integrations
	Name string `yaml:"-" json:"name,omitempty"`

	// From imports an integration package (e.g., "integrations/github", "github.com/org/integration@v1.0")
	// Mutually exclusive with inline definition (BaseURL + Operations)
	From string `yaml:"from,omitempty" json:"from,omitempty"`

	// BaseURL is the base URL for all operations (required for inline integrations)
	BaseURL string `yaml:"base_url,omitempty" json:"base_url,omitempty"`

	// Transport specifies which transport to use ("http", "aws_sigv4", "oauth2")
	// Defaults to "http" if not specified
	Transport string `yaml:"transport,omitempty" json:"transport,omitempty"`

	// Auth defines authentication configuration (for http transport)
	Auth *AuthDefinition `yaml:"auth,omitempty" json:"auth,omitempty"`

	// AWS defines AWS SigV4 transport configuration (for aws_sigv4 transport)
	AWS *AWSConfig `yaml:"aws,omitempty" json:"aws,omitempty"`

	// OAuth2 defines OAuth2 transport configuration (for oauth2 transport)
	OAuth2 *OAuth2Config `yaml:"oauth2,omitempty" json:"oauth2,omitempty"`

	// Headers are default headers applied to all operations
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// RateLimit defines rate limiting configuration
	RateLimit *RateLimitConfig `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`

	// Operations define named operations for inline integrations
	// Not used when From is specified (operations come from package)
	Operations map[string]OperationDefinition `yaml:"operations,omitempty" json:"operations,omitempty"`
}

// OperationDefinition defines a single operation within an integration.
type OperationDefinition struct {
	// Method is the HTTP method (GET, POST, PUT, PATCH, DELETE)
	Method string `yaml:"method" json:"method"`

	// Path is the URL path template with {param} placeholders
	Path string `yaml:"path" json:"path"`

	// RequestSchema is the JSON Schema for operation inputs
	RequestSchema map[string]interface{} `yaml:"request_schema,omitempty" json:"request_schema,omitempty"`

	// ResponseTransform is a jq expression to transform the response
	ResponseTransform string `yaml:"response_transform,omitempty" json:"response_transform,omitempty"`

	// Headers are operation-specific headers (merged with integration headers)
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`

	// Timeout is the operation-specific timeout in seconds
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Retry defines retry configuration for this operation
	Retry *RetryDefinition `yaml:"retry,omitempty" json:"retry,omitempty"`
}

// AuthDefinition defines authentication configuration for an integration.
type AuthDefinition struct {
	// Type is the authentication type (bearer, basic, api_key, oauth2_client)
	// Optional - inferred as "bearer" if only Token is present
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Token is the bearer token (for type: bearer or shorthand)
	// Can reference environment variables: ${GITHUB_TOKEN}
	Token string `yaml:"token,omitempty" json:"token,omitempty"`

	// Username for basic auth (type: basic)
	Username string `yaml:"username,omitempty" json:"username,omitempty"`

	// Password for basic auth (type: basic)
	Password string `yaml:"password,omitempty" json:"password,omitempty"`

	// Header is the header name for API key auth (type: api_key)
	Header string `yaml:"header,omitempty" json:"header,omitempty"`

	// Value is the API key value (type: api_key)
	Value string `yaml:"value,omitempty" json:"value,omitempty"`

	// ClientID for OAuth2 client credentials flow (type: oauth2_client) - future
	ClientID string `yaml:"client_id,omitempty" json:"client_id,omitempty"`

	// ClientSecret for OAuth2 client credentials flow (type: oauth2_client) - future
	ClientSecret string `yaml:"client_secret,omitempty" json:"client_secret,omitempty"`

	// TokenURL for OAuth2 token endpoint (type: oauth2_client) - future
	TokenURL string `yaml:"token_url,omitempty" json:"token_url,omitempty"`

	// Scopes for OAuth2 (type: oauth2_client) - future
	Scopes []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`
}

// RateLimitConfig defines rate limiting configuration for an integration.
type RateLimitConfig struct {
	// RequestsPerSecond limits the number of requests per second
	RequestsPerSecond float64 `yaml:"requests_per_second,omitempty" json:"requests_per_second,omitempty"`

	// RequestsPerMinute limits the number of requests per minute
	RequestsPerMinute int `yaml:"requests_per_minute,omitempty" json:"requests_per_minute,omitempty"`

	// Timeout is the maximum time to wait for rate limit (in seconds, default 30)
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`
}

// AWSConfig defines AWS SigV4 transport configuration.
type AWSConfig struct {
	// Service is the AWS service name (e.g., "s3", "dynamodb", "sqs")
	Service string `yaml:"service" json:"service"`

	// Region is the AWS region (e.g., "us-east-1", "eu-west-1")
	Region string `yaml:"region" json:"region"`
}

// OAuth2Config defines OAuth2 transport configuration.
type OAuth2Config struct {
	// Flow is the OAuth2 flow ("client_credentials" or "authorization_code")
	Flow string `yaml:"flow" json:"flow"`

	// ClientID is the OAuth2 client ID (must use ${ENV_VAR} syntax)
	ClientID string `yaml:"client_id" json:"client_id"`

	// ClientSecret is the OAuth2 client secret (must use ${ENV_VAR} syntax)
	ClientSecret string `yaml:"client_secret" json:"client_secret"`

	// TokenURL is the OAuth2 token endpoint URL
	TokenURL string `yaml:"token_url" json:"token_url"`

	// Scopes are the OAuth2 scopes to request
	Scopes []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`

	// RefreshToken is the refresh token for authorization_code flow (must use ${ENV_VAR} syntax)
	RefreshToken string `yaml:"refresh_token,omitempty" json:"refresh_token,omitempty"`
}

// PermissionDefinition defines access control permissions for a workflow or step.
// Permissions are applied hierarchically: step permissions are intersected with
// workflow permissions (most restrictive wins).
type PermissionDefinition struct {
	// Paths controls file system access
	Paths *PathPermissions `yaml:"paths,omitempty" json:"paths,omitempty"`

	// Network controls network access
	Network *NetworkPermissions `yaml:"network,omitempty" json:"network,omitempty"`

	// Secrets controls which secrets can be accessed
	Secrets *SecretPermissions `yaml:"secrets,omitempty" json:"secrets,omitempty"`

	// Tools controls which tools can be used
	Tools *ToolPermissions `yaml:"tools,omitempty" json:"tools,omitempty"`

	// Shell controls shell command execution
	Shell *ShellPermissions `yaml:"shell,omitempty" json:"shell,omitempty"`

	// Env controls environment variable access
	Env *EnvPermissions `yaml:"env,omitempty" json:"env,omitempty"`

	// AcceptUnenforceable allows running even when some permissions cannot be enforced
	// by the chosen provider. This must be explicitly set to true to acknowledge
	// that some security restrictions may not be enforced.
	AcceptUnenforceable bool `yaml:"accept_unenforceable,omitempty" json:"accept_unenforceable,omitempty"`

	// AcceptUnenforceableFor lists specific providers for which unenforceable
	// permissions are acceptable.
	AcceptUnenforceableFor []string `yaml:"accept_unenforceable_for,omitempty" json:"accept_unenforceable_for,omitempty"`
}

// PathPermissions controls file system access.
// Uses gitignore-style glob patterns with support for **, *, and ! negation.
type PathPermissions struct {
	// Read patterns for allowed read paths (default: ["**/*"] = all)
	Read []string `yaml:"read,omitempty" json:"read,omitempty"`

	// Write patterns for allowed write paths (default: ["$out/**"] = output dir only)
	Write []string `yaml:"write,omitempty" json:"write,omitempty"`
}

// NetworkPermissions controls network access.
type NetworkPermissions struct {
	// AllowedHosts patterns for allowed hosts (empty = all allowed)
	// Supports wildcards like "*.github.com", "api.openai.com"
	AllowedHosts []string `yaml:"allowed_hosts,omitempty" json:"allowed_hosts,omitempty"`

	// BlockedHosts patterns for blocked hosts (always blocked)
	// Default includes cloud metadata endpoints and private IP ranges
	BlockedHosts []string `yaml:"blocked_hosts,omitempty" json:"blocked_hosts,omitempty"`
}

// SecretPermissions controls which secrets can be accessed.
type SecretPermissions struct {
	// Allowed patterns for allowed secret names (default: ["*"] = all)
	// Supports wildcards like "GITHUB_*", "OPENAI_API_KEY"
	Allowed []string `yaml:"allowed,omitempty" json:"allowed,omitempty"`
}

// ToolPermissions controls which tools can be used by LLM steps.
type ToolPermissions struct {
	// Allowed patterns for allowed tool names (default: ["*"] = all)
	// Supports wildcards like "file.*", "transform.*"
	Allowed []string `yaml:"allowed,omitempty" json:"allowed,omitempty"`

	// Blocked patterns for blocked tool names (takes precedence over allowed)
	// Supports wildcards like "shell.*", "!shell.run"
	Blocked []string `yaml:"blocked,omitempty" json:"blocked,omitempty"`
}

// ShellPermissions controls shell command execution.
type ShellPermissions struct {
	// Enabled controls whether shell.run is allowed (default: false)
	Enabled *bool `yaml:"enabled,omitempty" json:"enabled,omitempty"`

	// AllowedCommands restricts to specific command prefixes when enabled
	// Example: ["git", "npm"] allows "git status", "npm install", etc.
	AllowedCommands []string `yaml:"allowed_commands,omitempty" json:"allowed_commands,omitempty"`
}

// EnvPermissions controls environment variable access.
type EnvPermissions struct {
	// Inherit controls whether to inherit the process environment (default: false)
	Inherit bool `yaml:"inherit,omitempty" json:"inherit,omitempty"`

	// Allowed patterns for allowed environment variables when inherit is true
	// Default: ["CI", "PATH", "HOME", "USER", "TERM"]
	Allowed []string `yaml:"allowed,omitempty" json:"allowed,omitempty"`
}

// IsShellEnabled returns whether shell execution is enabled.
// Returns false if Enabled is nil (default is disabled).
func (s *ShellPermissions) IsShellEnabled() bool {
	if s == nil || s.Enabled == nil {
		return false
	}
	return *s.Enabled
}

// Validate checks if the permission definition is valid.
func (p *PermissionDefinition) Validate() error {
	if p == nil {
		return nil
	}

	// Validate path permissions
	if p.Paths != nil {
		if err := p.Paths.Validate(); err != nil {
			return fmt.Errorf("paths: %w", err)
		}
	}

	// Validate network permissions
	if p.Network != nil {
		if err := p.Network.Validate(); err != nil {
			return fmt.Errorf("network: %w", err)
		}
	}

	// Validate secrets permissions
	if p.Secrets != nil {
		if err := p.Secrets.Validate(); err != nil {
			return fmt.Errorf("secrets: %w", err)
		}
	}

	// Validate tools permissions
	if p.Tools != nil {
		if err := p.Tools.Validate(); err != nil {
			return fmt.Errorf("tools: %w", err)
		}
	}

	// Validate shell permissions
	if p.Shell != nil {
		if err := p.Shell.Validate(); err != nil {
			return fmt.Errorf("shell: %w", err)
		}
	}

	// Validate env permissions
	if p.Env != nil {
		if err := p.Env.Validate(); err != nil {
			return fmt.Errorf("env: %w", err)
		}
	}

	return nil
}

// Validate checks if path permissions are valid.
func (p *PathPermissions) Validate() error {
	// Validate glob patterns
	for _, pattern := range p.Read {
		if err := validateGlobPattern(pattern); err != nil {
			return fmt.Errorf("read pattern %q: %w", pattern, err)
		}
	}
	for _, pattern := range p.Write {
		if err := validateGlobPattern(pattern); err != nil {
			return fmt.Errorf("write pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// Validate checks if network permissions are valid.
func (n *NetworkPermissions) Validate() error {
	// Validate host patterns
	for _, pattern := range n.AllowedHosts {
		if err := validateHostPattern(pattern); err != nil {
			return fmt.Errorf("allowed_hosts pattern %q: %w", pattern, err)
		}
	}
	for _, pattern := range n.BlockedHosts {
		if err := validateHostPattern(pattern); err != nil {
			return fmt.Errorf("blocked_hosts pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// Validate checks if secret permissions are valid.
func (s *SecretPermissions) Validate() error {
	// Validate name patterns
	for _, pattern := range s.Allowed {
		if err := validateNamePattern(pattern); err != nil {
			return fmt.Errorf("allowed pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// Validate checks if tool permissions are valid.
func (t *ToolPermissions) Validate() error {
	// Validate tool name patterns
	for _, pattern := range t.Allowed {
		if err := validateToolPattern(pattern); err != nil {
			return fmt.Errorf("allowed pattern %q: %w", pattern, err)
		}
	}
	for _, pattern := range t.Blocked {
		if err := validateToolPattern(pattern); err != nil {
			return fmt.Errorf("blocked pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// Validate checks if shell permissions are valid.
func (s *ShellPermissions) Validate() error {
	// Validate command names (basic check - no path separators)
	for _, cmd := range s.AllowedCommands {
		if cmd == "" {
			return fmt.Errorf("empty command name not allowed")
		}
		// Command should not contain path separators
		for _, ch := range cmd {
			if ch == '/' || ch == '\\' {
				return fmt.Errorf("command %q should not contain path separators", cmd)
			}
		}
	}
	return nil
}

// Validate checks if env permissions are valid.
func (e *EnvPermissions) Validate() error {
	// Validate env var name patterns
	for _, pattern := range e.Allowed {
		if err := validateNamePattern(pattern); err != nil {
			return fmt.Errorf("allowed pattern %q: %w", pattern, err)
		}
	}
	return nil
}

// validateGlobPattern validates a gitignore-style glob pattern.
func validateGlobPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern not allowed")
	}
	// Check for invalid characters
	// Glob patterns can contain most characters, but we check for obvious issues
	// Actual matching will use doublestar library which handles validation
	return nil
}

// validateHostPattern validates a host pattern (e.g., "*.github.com").
func validateHostPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern not allowed")
	}
	// Basic validation - host patterns can have wildcards but not path components
	for _, ch := range pattern {
		if ch == '/' || ch == '\\' {
			return fmt.Errorf("host pattern should not contain path separators")
		}
	}
	return nil
}

// validateNamePattern validates a name pattern (for secrets, env vars).
func validateNamePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern not allowed")
	}
	return nil
}

// validateToolPattern validates a tool name pattern (e.g., "file.*", "!shell.run").
func validateToolPattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("empty pattern not allowed")
	}
	// Remove leading ! for negation patterns
	if len(pattern) > 0 && pattern[0] == '!' {
		pattern = pattern[1:]
	}
	if pattern == "" {
		return fmt.Errorf("empty pattern after negation not allowed")
	}
	return nil
}

// RequirementsDefinition declares abstract service dependencies for a workflow.
// This enables portable workflow definitions that don't embed credentials.
// Runtime bindings are provided by workspaces.
type RequirementsDefinition struct {
	// Integrations lists required integration dependencies.
	// Supports two formats:
	//   - Simple: "github" (requires integration of type github)
	//   - Aliased: "github as source" (requires github, bound to alias "source")
	Integrations []string `yaml:"integrations,omitempty" json:"integrations,omitempty"`

	// MCPServers lists required MCP server dependencies
	MCPServers []MCPServerRequirement `yaml:"mcp_servers,omitempty" json:"mcp_servers,omitempty"`
}

// ParsedIntegrationRequirement represents a parsed integration requirement.
// It is derived from the string format in requires.integrations.
type ParsedIntegrationRequirement struct {
	// Type is the integration type (e.g., "github", "slack")
	Type string

	// Alias is the optional alias for this requirement (e.g., "source", "target")
	// Empty string means no alias (simple requirement)
	Alias string
}

// MCPServerRequirement describes a required MCP server dependency.
type MCPServerRequirement struct {
	// Name is the MCP server identifier (must match profile binding key)
	Name string `yaml:"name" json:"name"`

	// Optional indicates this MCP server is not required for the workflow to function
	Optional bool `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// Validate checks if the requirements definition is valid.
func (r *RequirementsDefinition) Validate() error {
	// Validate integration requirements
	seenTypes := make(map[string]bool)
	seenAliases := make(map[string]bool)

	for i, reqStr := range r.Integrations {
		if reqStr == "" {
			return fmt.Errorf("integration requirement %d: cannot be empty", i)
		}

		// Parse the requirement
		parsed := ParseIntegrationRequirement(reqStr)

		// Check for duplicate types without aliases
		if parsed.Alias == "" {
			if seenTypes[parsed.Type] {
				return fmt.Errorf("duplicate integration requirement: %s", parsed.Type)
			}
			seenTypes[parsed.Type] = true
		}

		// Check for duplicate aliases
		if parsed.Alias != "" {
			if seenAliases[parsed.Alias] {
				return fmt.Errorf("duplicate integration alias: %s", parsed.Alias)
			}
			seenAliases[parsed.Alias] = true
		}
	}

	// Validate MCP server requirements
	mcpNames := make(map[string]bool)
	for i, req := range r.MCPServers {
		if req.Name == "" {
			return fmt.Errorf("mcp_server requirement %d: name is required", i)
		}
		if mcpNames[req.Name] {
			return fmt.Errorf("duplicate mcp_server requirement: %s", req.Name)
		}
		mcpNames[req.Name] = true
	}

	return nil
}

// ParseIntegrationRequirement parses an integration requirement string.
// Supports two formats:
//   - Simple: "github" -> type="github", alias=""
//   - Aliased: "github as source" -> type="github", alias="source"
func ParseIntegrationRequirement(req string) ParsedIntegrationRequirement {
	// Check for "as" keyword (case-insensitive wouldn't make sense here, keep it exact)
	parts := regexp.MustCompile(`\s+as\s+`).Split(req, 2)

	if len(parts) == 2 {
		// Aliased format: "github as source"
		return ParsedIntegrationRequirement{
			Type:  strings.TrimSpace(parts[0]),
			Alias: strings.TrimSpace(parts[1]),
		}
	}

	// Simple format: "github"
	return ParsedIntegrationRequirement{
		Type:  strings.TrimSpace(req),
		Alias: "",
	}
}

// UnmarshalYAML implements custom YAML unmarshaling for Definition
// to detect and reject the deprecated triggers: key.
func (d *Definition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First check for deprecated triggers: key
	var raw map[string]interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	if _, hasTriggersKey := raw["triggers"]; hasTriggersKey {
		return &errors.ConfigError{
			Key: "triggers",
			Reason: `the 'triggers:' key is no longer supported. Use 'listen:' instead.

Migration guide:
  Old syntax (triggers:):
    triggers:
      - type: webhook
        webhook:
          path: /webhooks/my-workflow
          source: github
          secret: ${GITHUB_SECRET}

  New syntax (listen:):
    listen:
      webhook:
        path: /webhooks/my-workflow
        source: github
        secret: ${GITHUB_SECRET}
      api:
        secret: ${API_SECRET}
      schedule:
        cron: "0 9 * * *"

See: https://conductor.sh/docs/workflows/listen`,
		}
	}

	// Standard unmarshaling using a type alias to avoid recursion
	type definitionAlias Definition
	var alias definitionAlias
	if err := unmarshal(&alias); err != nil {
		return err
	}
	*d = Definition(alias)
	return nil
}

// ParseDefinition parses a workflow definition from YAML bytes.
func ParseDefinition(data []byte) (*Definition, error) {
	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse workflow definition: %w", err)
	}

	// Auto-generate step IDs before applying defaults
	def.autoGenerateStepIDs()

	// Apply defaults before validation (may return error from output_type expansion)
	if err := def.ApplyDefaults(); err != nil {
		return nil, fmt.Errorf("failed to apply defaults: %w", err)
	}

	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("invalid workflow definition: %w", err)
	}

	return &def, nil
}

// ApplyDefaults applies default values to workflow and step fields.
// Returns an error if output_type expansion fails.
func (d *Definition) ApplyDefaults() error {
	// Apply defaults to each step
	for i := range d.Steps {
		step := &d.Steps[i]

		// Default timeout: 30 seconds
		if step.Timeout == 0 {
			step.Timeout = 30
		}

		// Default retry configuration: max_attempts=2, backoff_base=1, backoff_multiplier=2.0
		if step.Retry == nil {
			step.Retry = &RetryDefinition{
				MaxAttempts:       2,
				BackoffBase:       1,
				BackoffMultiplier: 2.0,
			}
		}

		// Default model tier for LLM steps: balanced
		if step.Type == StepTypeLLM && step.Model == "" {
			step.Model = string(ModelTierBalanced)
		}

		// Expand output_type to output_schema (T1.3)
		if step.OutputType != "" {
			// This will be validated later, but expansion happens here
			// so that the expanded schema is available for validation
			if err := step.expandOutputType(); err != nil {
				return fmt.Errorf("step %s: %w", step.ID, err)
			}
		}
	}
	return nil
}

// autoGenerateStepIDs generates IDs for steps that don't have explicit IDs.
// Uses a two-pass algorithm:
// 1. First pass: collect all explicit IDs
// 2. Second pass: generate auto-IDs, skipping numbers that would collide
//
// Auto-ID format: {provider}_{operation}_{N}
// Example: file_read_1, github_create_issue_2
func (d *Definition) autoGenerateStepIDs() {
	// First pass: collect all explicit IDs
	explicitIDs := make(map[string]bool)
	for _, step := range d.Steps {
		if step.hasExplicitID {
			explicitIDs[step.ID] = true
		}
	}

	// Track counters for each provider.operation combination
	counters := make(map[string]int)

	// Second pass: generate auto-IDs for steps without explicit IDs
	for i := range d.Steps {
		step := &d.Steps[i]

		// Skip steps that already have explicit IDs
		if step.hasExplicitID {
			continue
		}

		// Determine the base ID based on step type
		var baseID string
		if step.Type == StepTypeIntegration {
			// For integration steps, use action/integration_operation format
			if step.Action != "" {
				baseID = step.Action + "_" + step.Operation
			} else if step.Integration != "" {
				// Integration field is in format "integration.operation", convert to "integration_operation"
				baseID = step.Integration
				// Replace dot with underscore
				for j, c := range baseID {
					if c == '.' {
						baseID = baseID[:j] + "_" + baseID[j+1:]
						break
					}
				}
			} else {
				baseID = "integration"
			}
		} else {
			// For other step types (llm, parallel, condition), generate a generic ID
			// This shouldn't happen in practice since these types should have explicit IDs
			baseID = "step"
		}

		// Find the next available number that doesn't collide
		n := counters[baseID] + 1
		candidate := fmt.Sprintf("%s_%d", baseID, n)

		// Keep incrementing until we find a non-colliding ID
		for explicitIDs[candidate] {
			n++
			candidate = fmt.Sprintf("%s_%d", baseID, n)
		}

		// Assign the generated ID
		step.ID = candidate
		counters[baseID] = n

		// Mark this ID as used to prevent collisions in subsequent steps
		explicitIDs[candidate] = true
	}
}

// Validate checks if the workflow definition is valid.
func (d *Definition) Validate() error {
	if d.Name == "" {
		return &errors.ValidationError{
			Field:      "name",
			Message:    "workflow name is required",
			Suggestion: "add a descriptive name for the workflow",
		}
	}

	// Version is now optional (removed validation check)

	if len(d.Steps) == 0 {
		return &errors.ValidationError{
			Field:      "steps",
			Message:    "workflow must have at least one step",
			Suggestion: "add at least one step to the workflow definition",
		}
	}

	// Validate step IDs are unique
	stepIDs := make(map[string]bool)
	for _, step := range d.Steps {
		if step.ID == "" {
			return &errors.ValidationError{
				Field:      "id",
				Message:    "step ID is required",
				Suggestion: "add an 'id' field to each step",
			}
		}
		if stepIDs[step.ID] {
			return &errors.ValidationError{
				Field:      "id",
				Message:    fmt.Sprintf("duplicate step ID: %s", step.ID),
				Suggestion: "ensure each step has a unique ID",
			}
		}
		stepIDs[step.ID] = true

		// Validate step
		if err := step.Validate(); err != nil {
			return fmt.Errorf("invalid step %s: %w", step.ID, err)
		}

		// Validate expression injection prevention
		if err := ValidateExpressionInjection(&step); err != nil {
			return fmt.Errorf("invalid step %s: %w", step.ID, err)
		}

		// Validate nested foreach prevention
		if err := ValidateNestedForeach(&step, false); err != nil {
			return fmt.Errorf("invalid step %s: %w", step.ID, err)
		}

		// Validate loop expression syntax (compile-time validation)
		if err := ValidateLoopExpression(&step); err != nil {
			return fmt.Errorf("invalid step %s: %w", step.ID, err)
		}

		// Validate agent reference exists if specified
		if step.Agent != "" {
			if _, exists := d.Agents[step.Agent]; !exists {
				return &errors.ValidationError{
					Field:      "agent",
					Message:    fmt.Sprintf("step %s references undefined agent: %s", step.ID, step.Agent),
					Suggestion: "define the agent in the workflow's agents section",
				}
			}
		}
	}

	// Validate inputs
	for _, input := range d.Inputs {
		if err := input.Validate(); err != nil {
			return fmt.Errorf("invalid input %s: %w", input.Name, err)
		}
	}

	// Validate outputs
	for _, output := range d.Outputs {
		if err := output.Validate(); err != nil {
			return fmt.Errorf("invalid output %s: %w", output.Name, err)
		}
	}

	// Validate agents
	for name, agent := range d.Agents {
		if err := agent.Validate(); err != nil {
			return fmt.Errorf("invalid agent %s: %w", name, err)
		}
	}

	// Validate functions and build function name index
	functionNames := make(map[string]bool)
	for i, function := range d.Functions {
		if err := function.Validate(); err != nil {
			return fmt.Errorf("invalid function %s: %w", function.Name, err)
		}
		if functionNames[function.Name] {
			return fmt.Errorf("duplicate function name: %s", function.Name)
		}
		functionNames[function.Name] = true

		// Store index for error messages
		_ = i
	}

	// Validate step function references
	for _, step := range d.Steps {
		for _, functionName := range step.Tools {
			if !functionNames[functionName] {
				return fmt.Errorf("step %s references undefined function: %s", step.ID, functionName)
			}
		}
	}

	// Validate MCP servers
	mcpServerNames := make(map[string]bool)
	for _, server := range d.MCPServers {
		if err := server.Validate(); err != nil {
			return fmt.Errorf("invalid mcp_server %s: %w", server.Name, err)
		}
		// Check for duplicate server names
		if mcpServerNames[server.Name] {
			return fmt.Errorf("duplicate mcp_server name: %s", server.Name)
		}
		mcpServerNames[server.Name] = true
	}

	// Validate integrations and build integration name index
	integrationNames := make(map[string]bool)
	for name, integration := range d.Integrations {
		// Set the name from the map key
		integration.Name = name
		d.Integrations[name] = integration

		if err := integration.Validate(); err != nil {
			return fmt.Errorf("invalid integration %s: %w", name, err)
		}
		integrationNames[name] = true
	}

	// Validate step integration references
	for _, step := range d.Steps {
		if step.Type == StepTypeIntegration && step.Integration != "" {
			// Parse integration.operation format
			parts := splitIntegrationReference(step.Integration)
			if len(parts) != 2 {
				return fmt.Errorf("step %s: integration must be in format 'integration_name.operation_name', got: %s", step.ID, step.Integration)
			}
			integrationName, operationName := parts[0], parts[1]

			// Check integration exists
			if !integrationNames[integrationName] {
				return fmt.Errorf("step %s references undefined integration: %s", step.ID, integrationName)
			}

			// Check operation exists (only for inline integrations, not packages)
			integration := d.Integrations[integrationName]
			if integration.From == "" {
				// Inline integration - validate operation exists
				if _, exists := integration.Operations[operationName]; !exists {
					return fmt.Errorf("step %s references undefined operation %s in integration %s", step.ID, operationName, integrationName)
				}
			}
			// For package integrations, we can't validate operations at definition time
		}
	}

	// Validate requirements section
	if d.Requires != nil {
		if err := d.Requires.Validate(); err != nil {
			return fmt.Errorf("invalid requires section: %w", err)
		}
	}

	// Validate workflow-level permissions (SPEC-141)
	if d.Permissions != nil {
		if err := d.Permissions.Validate(); err != nil {
			return fmt.Errorf("invalid workflow permissions: %w", err)
		}
	}

	// Validate security access configuration
	if d.Security != nil {
		if err := d.Security.Validate(); err != nil {
			return fmt.Errorf("invalid security configuration: %w", err)
		}
	}

	// Validate trigger configuration
	if d.Trigger != nil {
		if err := d.Trigger.Validate(); err != nil {
			return fmt.Errorf("invalid trigger configuration: %w", err)
		}
	}

	return nil
}

// splitIntegrationReference splits an integration reference like "github.create_issue" into ["github", "create_issue"]
func splitIntegrationReference(ref string) []string {
	// Find the first dot
	dotIndex := -1
	for i, ch := range ref {
		if ch == '.' {
			dotIndex = i
			break
		}
	}

	if dotIndex == -1 {
		return []string{ref}
	}

	return []string{ref[:dotIndex], ref[dotIndex+1:]}
}

// Validate checks if the input definition is valid.
func (i *InputDefinition) Validate() error {
	if i.Name == "" {
		return fmt.Errorf("input name is required")
	}

	if i.Type == "" {
		return fmt.Errorf("input type is required")
	}

	// Validate type is one of the allowed types
	validTypes := map[string]bool{
		"string":  true,
		"number":  true,
		"boolean": true,
		"object":  true,
		"array":   true,
	}
	if !validTypes[i.Type] {
		return fmt.Errorf("invalid input type: %s (must be string, number, boolean, object, or array)", i.Type)
	}

	return nil
}

// Validate checks if the step definition is valid.
func (s *StepDefinition) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("step ID is required")
	}

	// Name is now optional (removed validation check)

	if s.Type == "" {
		return fmt.Errorf("step type is required")
	}

	// Validate step type
	validTypes := map[StepType]bool{
		StepTypeCondition:   true,
		StepTypeLLM:         true,
		StepTypeParallel:    true,
		StepTypeIntegration: true,
		StepTypeLoop:        true,
		StepTypeWorkflow:    true,
	}
	if !validTypes[s.Type] {
		return fmt.Errorf("invalid step type: %s", s.Type)
	}

	// Validate prompt is present for LLM steps
	if s.Type == StepTypeLLM && s.Prompt == "" {
		return fmt.Errorf("prompt is required for LLM step type")
	}

	// Validate model tier for LLM steps
	if s.Type == StepTypeLLM && s.Model != "" {
		if !ValidModelTiers[ModelTier(s.Model)] {
			return fmt.Errorf("invalid model tier: %s (must be fast, balanced, or strategic)", s.Model)
		}
	}

	// Validate condition is present for condition steps
	if s.Type == StepTypeCondition && s.Condition == nil {
		return fmt.Errorf("condition is required for condition step type")
	}

	// Validate integration field for integration steps
	if s.Type == StepTypeIntegration {
		// Must have either Integration (for integrations) or Action+Operation (for builtin actions)
		hasIntegration := s.Integration != ""
		hasAction := s.Action != "" && s.Operation != ""

		if !hasIntegration && !hasAction {
			return fmt.Errorf("integration step requires either 'integration' field or 'action'+'operation' fields")
		}

		if hasIntegration && hasAction {
			return fmt.Errorf("integration step cannot have both 'integration' and 'action' fields")
		}

		// Validate builtin action names
		if hasAction {
			validActions := map[string]bool{
				"file":      true,
				"shell":     true,
				"http":      true,
				"transform": true,
				"utility":   true,
			}
			if !validActions[s.Action] {
				return fmt.Errorf("invalid action: %s (must be file, shell, http, transform, or utility)", s.Action)
			}
		}
		// Format validation for integration field happens at workflow level where we can check against defined integrations
	}

	// Validate workflow field for workflow steps
	if s.Type == StepTypeWorkflow {
		if s.Workflow == "" {
			return fmt.Errorf("workflow step requires 'workflow' field with path to sub-workflow file")
		}

		// Workflow steps cannot have prompt field
		if s.Prompt != "" {
			return fmt.Errorf("workflow step cannot have 'prompt' field (use 'inputs' to pass data)")
		}

		// Validate workflow path security at definition time
		if err := ValidateWorkflowPath(s.Workflow); err != nil {
			return fmt.Errorf("invalid workflow path: %w", err)
		}
	}

	// Validate error handling
	if s.OnError != nil {
		if err := s.OnError.Validate(); err != nil {
			return fmt.Errorf("invalid error handling: %w", err)
		}
	}

	// Validate retry configuration
	if s.Retry != nil {
		if err := s.Retry.Validate(); err != nil {
			return fmt.Errorf("invalid retry configuration: %w", err)
		}
	}

	// Validate schema complexity if output_schema is specified (T1.5)
	// This runs after expansion, so we check the final OutputSchema
	if s.OutputSchema != nil {
		if err := validateSchemaComplexity(s.OutputSchema); err != nil {
			return fmt.Errorf("invalid output_schema: %w", err)
		}
	}

	// Validate parallel step nested steps
	if s.Type == StepTypeParallel {
		if len(s.Steps) == 0 {
			return fmt.Errorf("parallel step requires nested steps")
		}
		// Validate each nested step
		nestedIDs := make(map[string]bool)
		for i, nested := range s.Steps {
			if err := nested.Validate(); err != nil {
				return fmt.Errorf("parallel step %s, nested step %d (%s): %w", s.ID, i, nested.ID, err)
			}
			// Check for duplicate IDs within parallel block
			if nestedIDs[nested.ID] {
				return fmt.Errorf("parallel step %s has duplicate nested step ID: %s", s.ID, nested.ID)
			}
			nestedIDs[nested.ID] = true
		}
	}

	// Validate loop step
	if s.Type == StepTypeLoop {
		// max_iterations is required and must be 1-100
		if s.MaxIterations < 1 || s.MaxIterations > 100 {
			return fmt.Errorf("max_iterations must be between 1 and 100, got %d", s.MaxIterations)
		}
		// until expression is required
		if s.Until == "" {
			return fmt.Errorf("until expression is required for loop step")
		}
		// nested steps are required
		if len(s.Steps) == 0 {
			return fmt.Errorf("loop step requires nested steps")
		}
		// Validate timeout if specified (minimum 2 seconds)
		if s.Timeout > 0 && s.Timeout < 2 {
			return fmt.Errorf("loop timeout must be at least 2 seconds")
		}
		// Validate each nested step and check for unique IDs
		nestedIDs := make(map[string]bool)
		for i, nested := range s.Steps {
			// Check for nested loops (not allowed in v1)
			if nested.Type == StepTypeLoop {
				return fmt.Errorf("nested loops are not supported")
			}
			if err := nested.Validate(); err != nil {
				return fmt.Errorf("loop step %s, nested step %d (%s): %w", s.ID, i, nested.ID, err)
			}
			if nestedIDs[nested.ID] {
				return fmt.Errorf("loop step %s has duplicate nested step ID: %s", s.ID, nested.ID)
			}
			nestedIDs[nested.ID] = true
		}
	}

	// Validate step-level permissions (SPEC-141)
	if s.Permissions != nil {
		if err := s.Permissions.Validate(); err != nil {
			return fmt.Errorf("invalid permissions: %w", err)
		}
	}

	return nil
}

// Validate checks if the error handling definition is valid.
func (e *ErrorHandlingDefinition) Validate() error {
	validStrategies := map[ErrorStrategy]bool{
		ErrorStrategyFail:     true,
		ErrorStrategyIgnore:   true,
		ErrorStrategyRetry:    true,
		ErrorStrategyFallback: true,
	}
	if !validStrategies[e.Strategy] {
		return fmt.Errorf("invalid error strategy: %s", e.Strategy)
	}

	if e.Strategy == ErrorStrategyFallback && e.FallbackStep == "" {
		return fmt.Errorf("fallback_step is required when error strategy is 'fallback'")
	}

	return nil
}

// Validate checks if the retry definition is valid.
func (r *RetryDefinition) Validate() error {
	if r.MaxAttempts < 1 {
		return fmt.Errorf("max_attempts must be at least 1")
	}

	if r.BackoffBase < 1 {
		return fmt.Errorf("backoff_base must be at least 1 second")
	}

	if r.BackoffMultiplier < 1.0 {
		return fmt.Errorf("backoff_multiplier must be at least 1.0")
	}

	return nil
}

// Validate checks if the output definition is valid.
func (o *OutputDefinition) Validate() error {
	if o.Name == "" {
		return fmt.Errorf("output name is required")
	}

	if o.Type == "" {
		return fmt.Errorf("output type is required")
	}

	if o.Value == "" {
		return fmt.Errorf("output value expression is required")
	}

	return nil
}

// Validate checks if the agent definition is valid.
func (a *AgentDefinition) Validate() error {
	// Validate capabilities if specified
	if len(a.Capabilities) > 0 {
		validCapabilities := map[string]bool{
			"vision":       true,
			"long-context": true,
			"tool-use":     true,
			"streaming":    true,
			"json-mode":    true,
		}
		for _, cap := range a.Capabilities {
			if !validCapabilities[cap] {
				return fmt.Errorf("invalid capability: %s (must be one of: vision, long-context, tool-use, streaming, json-mode)", cap)
			}
		}
	}

	return nil
}

// Validate checks if the MCP server configuration is valid.
func (m *MCPServerConfig) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("mcp_server name is required")
	}

	if m.Command == "" {
		return fmt.Errorf("mcp_server command is required")
	}

	// Validate timeout if specified
	if m.Timeout < 0 {
		return fmt.Errorf("mcp_server timeout must be non-negative")
	}

	return nil
}

// Validate checks if the integration definition is valid.
func (c *IntegrationDefinition) Validate() error {
	// Validate name (should be set from map key)
	if c.Name == "" {
		return fmt.Errorf("integration name is required")
	}

	// Check for mutually exclusive fields: from vs inline definition
	hasFrom := c.From != ""
	hasInline := c.BaseURL != "" || len(c.Operations) > 0

	if !hasFrom && !hasInline {
		return fmt.Errorf("integration must specify either 'from' (package import) or inline definition (base_url + operations)")
	}

	if hasFrom && hasInline {
		return fmt.Errorf("integration cannot specify both 'from' and inline definition (base_url/operations)")
	}

	// For inline integrations, base_url is required
	if !hasFrom && c.BaseURL == "" {
		return fmt.Errorf("base_url is required for inline integration definition")
	}

	// For inline integrations, must have at least one operation
	if !hasFrom && len(c.Operations) == 0 {
		return fmt.Errorf("inline integration must define at least one operation")
	}

	// Validate auth if specified
	if c.Auth != nil {
		if err := c.Auth.Validate(); err != nil {
			return fmt.Errorf("invalid auth: %w", err)
		}
	}

	// Validate transport field if specified
	if c.Transport != "" {
		validTransports := map[string]bool{
			"http":       true,
			"aws_sigv4":  true,
			"oauth2":     true,
		}
		if !validTransports[c.Transport] {
			return fmt.Errorf("invalid transport %q: must be http, aws_sigv4, or oauth2", c.Transport)
		}

		// Validate AWS config when transport is aws_sigv4
		if c.Transport == "aws_sigv4" {
			if c.AWS == nil {
				return fmt.Errorf("aws configuration is required when transport is aws_sigv4")
			}
			if c.AWS.Service == "" {
				return fmt.Errorf("aws.service is required when transport is aws_sigv4")
			}
			if c.AWS.Region == "" {
				return fmt.Errorf("aws.region is required when transport is aws_sigv4")
			}
		}

		// Validate OAuth2 config when transport is oauth2
		if c.Transport == "oauth2" {
			if c.OAuth2 == nil {
				return fmt.Errorf("oauth2 configuration is required when transport is oauth2")
			}
			if c.OAuth2.ClientID == "" {
				return fmt.Errorf("oauth2.client_id is required when transport is oauth2")
			}
			if c.OAuth2.ClientSecret == "" {
				return fmt.Errorf("oauth2.client_secret is required when transport is oauth2")
			}
			if c.OAuth2.TokenURL == "" {
				return fmt.Errorf("oauth2.token_url is required when transport is oauth2")
			}
			if c.OAuth2.Flow != "" && c.OAuth2.Flow != "client_credentials" && c.OAuth2.Flow != "authorization_code" {
				return fmt.Errorf("oauth2.flow must be client_credentials or authorization_code, got %q", c.OAuth2.Flow)
			}
		}
	}

	// Validate rate limit if specified
	if c.RateLimit != nil {
		if err := c.RateLimit.Validate(); err != nil {
			return fmt.Errorf("invalid rate_limit: %w", err)
		}
	}

	// Validate operations
	for name, op := range c.Operations {
		if err := op.Validate(); err != nil {
			return fmt.Errorf("invalid operation %s: %w", name, err)
		}
	}

	return nil
}

// Validate checks if the operation definition is valid.
func (o *OperationDefinition) Validate() error {
	// Method is required
	if o.Method == "" {
		return fmt.Errorf("method is required")
	}

	// Validate HTTP method
	validMethods := map[string]bool{
		"GET":    true,
		"POST":   true,
		"PUT":    true,
		"PATCH":  true,
		"DELETE": true,
		"HEAD":   true,
	}
	if !validMethods[o.Method] {
		return fmt.Errorf("invalid method: %s (must be GET, POST, PUT, PATCH, DELETE, or HEAD)", o.Method)
	}

	// Path is required
	if o.Path == "" {
		return fmt.Errorf("path is required")
	}

	// Validate timeout if specified
	if o.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}

	// Validate retry if specified
	if o.Retry != nil {
		if err := o.Retry.Validate(); err != nil {
			return fmt.Errorf("invalid retry: %w", err)
		}
	}

	// TODO: Validate request_schema is valid JSON Schema
	// TODO: Validate response_transform is valid jq expression
	// TODO: Validate path template parameters exist in request_schema

	return nil
}

// Validate checks if the auth definition is valid.
func (a *AuthDefinition) Validate() error {
	// Infer type if not specified
	authType := a.Type
	if authType == "" {
		// If only token is present, assume bearer
		if a.Token != "" {
			authType = "bearer"
		}
	}

	switch authType {
	case "bearer", "":
		if a.Token == "" {
			return fmt.Errorf("token is required for bearer auth")
		}

	case "basic":
		if a.Username == "" {
			return fmt.Errorf("username is required for basic auth")
		}
		if a.Password == "" {
			return fmt.Errorf("password is required for basic auth")
		}

	case "api_key":
		if a.Header == "" {
			return fmt.Errorf("header is required for api_key auth")
		}
		if a.Value == "" {
			return fmt.Errorf("value is required for api_key auth")
		}

	case "oauth2_client":
		// OAuth2 is future functionality
		return fmt.Errorf("oauth2_client auth type is not yet implemented")

	default:
		return fmt.Errorf("invalid auth type: %s (must be bearer, basic, api_key, or oauth2_client)", authType)
	}

	return nil
}

// Validate checks if the rate limit config is valid.
func (r *RateLimitConfig) Validate() error {
	// At least one limit must be specified
	if r.RequestsPerSecond == 0 && r.RequestsPerMinute == 0 {
		return fmt.Errorf("at least one of requests_per_second or requests_per_minute must be specified")
	}

	// Values must be positive
	if r.RequestsPerSecond < 0 {
		return fmt.Errorf("requests_per_second must be non-negative")
	}
	if r.RequestsPerMinute < 0 {
		return fmt.Errorf("requests_per_minute must be non-negative")
	}

	// Validate timeout
	if r.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}

	return nil
}

// expandOutputType expands built-in output types to their equivalent output_schema.
// This implements T1.3: schema expansion logic for classification, decision, and extraction types.
// This method should be called from ApplyDefaults before validation.
func (s *StepDefinition) expandOutputType() error {
	// T1.4: Check mutual exclusivity BEFORE expansion
	if s.OutputSchema != nil && s.OutputType != "" {
		return fmt.Errorf("output_schema and output_type are mutually exclusive")
	}

	// If OutputType is not set, nothing to expand
	if s.OutputType == "" {
		return nil
	}

	switch s.OutputType {
	case "classification":
		// Extract categories from output_options
		categories, ok := s.OutputOptions["categories"]
		if !ok {
			return fmt.Errorf("output_type 'classification' requires 'categories' in output_options")
		}
		categoriesSlice, ok := categories.([]interface{})
		if !ok {
			return fmt.Errorf("categories must be an array")
		}
		if len(categoriesSlice) == 0 {
			return fmt.Errorf("categories array cannot be empty")
		}

		// Expand to schema
		s.OutputSchema = map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category": map[string]interface{}{
					"type": "string",
					"enum": categoriesSlice,
				},
			},
			"required": []interface{}{"category"},
		}

	case "decision":
		// Extract choices from output_options
		choices, ok := s.OutputOptions["choices"]
		if !ok {
			return fmt.Errorf("output_type 'decision' requires 'choices' in output_options")
		}
		choicesSlice, ok := choices.([]interface{})
		if !ok {
			return fmt.Errorf("choices must be an array")
		}
		if len(choicesSlice) == 0 {
			return fmt.Errorf("choices array cannot be empty")
		}

		// Build required fields list
		requiredFields := []interface{}{"decision"}

		// Check if reasoning is required
		requireReasoning, _ := s.OutputOptions["require_reasoning"].(bool)
		if requireReasoning {
			requiredFields = append(requiredFields, "reasoning")
		}

		// Expand to schema
		properties := map[string]interface{}{
			"decision": map[string]interface{}{
				"type": "string",
				"enum": choicesSlice,
			},
		}

		// Always include reasoning field, but only require it if specified
		properties["reasoning"] = map[string]interface{}{
			"type": "string",
		}

		s.OutputSchema = map[string]interface{}{
			"type":       "object",
			"properties": properties,
			"required":   requiredFields,
		}

	case "extraction":
		// Extract fields from output_options
		fields, ok := s.OutputOptions["fields"]
		if !ok {
			return fmt.Errorf("output_type 'extraction' requires 'fields' in output_options")
		}
		fieldsSlice, ok := fields.([]interface{})
		if !ok {
			return fmt.Errorf("fields must be an array")
		}
		if len(fieldsSlice) == 0 {
			return fmt.Errorf("fields array cannot be empty")
		}

		// Build properties and required fields
		properties := make(map[string]interface{})
		requiredFields := make([]interface{}, 0, len(fieldsSlice))

		for _, field := range fieldsSlice {
			fieldName, ok := field.(string)
			if !ok {
				return fmt.Errorf("field names must be strings")
			}
			properties[fieldName] = map[string]interface{}{
				"type": "string",
			}
			requiredFields = append(requiredFields, fieldName)
		}

		// Expand to schema
		s.OutputSchema = map[string]interface{}{
			"type":       "object",
			"properties": properties,
			"required":   requiredFields,
		}

	default:
		return fmt.Errorf("unsupported output_type: %s (must be classification, decision, or extraction)", s.OutputType)
	}

	return nil
}

// validateSchemaComplexity validates that a schema doesn't exceed complexity limits.
// This implements T1.5: max depth 10, max properties 100, max size 64KB.
func validateSchemaComplexity(schema map[string]interface{}) error {
	// Check schema size (serialize to JSON and check byte length)
	// Using a simple estimate: each entry is roughly 50 bytes on average
	// This is a rough heuristic to avoid expensive marshaling during validation
	estimatedSize := estimateSchemaSize(schema)
	if estimatedSize > 64*1024 {
		return fmt.Errorf("schema exceeds maximum size of 64KB")
	}

	// Check nesting depth and property count
	return validateSchemaDepthAndProperties(schema, 0, 0)
}

// estimateSchemaSize estimates the JSON size of a schema.
func estimateSchemaSize(v interface{}) int {
	switch val := v.(type) {
	case map[string]interface{}:
		size := 2 // {}
		for k, v := range val {
			size += len(k) + 4 // "key":
			size += estimateSchemaSize(v)
			size += 1 // comma
		}
		return size
	case []interface{}:
		size := 2 // []
		for _, item := range val {
			size += estimateSchemaSize(item)
			size += 1 // comma
		}
		return size
	case string:
		return len(val) + 2 // quotes
	case bool:
		return 5 // true/false
	case float64, int:
		return 10 // rough estimate
	default:
		return 10
	}
}

// validateSchemaDepthAndProperties validates nesting depth and property count recursively.
func validateSchemaDepthAndProperties(schema map[string]interface{}, depth int, propertyCount int) error {
	const maxDepth = 10
	const maxProperties = 100

	if depth > maxDepth {
		return fmt.Errorf("schema exceeds maximum nesting depth of %d", maxDepth)
	}

	// Count properties at this level
	if props, ok := schema["properties"].(map[string]interface{}); ok {
		propertyCount += len(props)
		if propertyCount > maxProperties {
			return fmt.Errorf("schema exceeds maximum of %d properties", maxProperties)
		}

		// Recursively validate nested properties
		for _, propSchema := range props {
			if nestedSchema, ok := propSchema.(map[string]interface{}); ok {
				if err := validateSchemaDepthAndProperties(nestedSchema, depth+1, propertyCount); err != nil {
					return err
				}
			}
		}
	}

	// Check items for arrays
	if items, ok := schema["items"].(map[string]interface{}); ok {
		if err := validateSchemaDepthAndProperties(items, depth+1, propertyCount); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks if the tool definition is valid.
func (t *FunctionDefinition) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("function name is required")
	}

	if t.Type == "" {
		return fmt.Errorf("function type is required")
	}

	// Validate function type
	if !ValidToolTypes[t.Type] {
		return fmt.Errorf("invalid function type: %s (must be http or script)", t.Type)
	}

	if t.Description == "" {
		return fmt.Errorf("function description is required")
	}

	// Type-specific validation
	switch t.Type {
	case ToolTypeHTTP:
		if t.URL == "" {
			return fmt.Errorf("url is required for http function")
		}
		if t.Method == "" {
			return fmt.Errorf("method is required for http function")
		}
		// Validate HTTP method
		validMethods := map[string]bool{
			"GET":     true,
			"POST":    true,
			"PUT":     true,
			"PATCH":   true,
			"DELETE":  true,
			"HEAD":    true,
			"OPTIONS": true,
		}
		if !validMethods[t.Method] {
			return fmt.Errorf("invalid HTTP method: %s", t.Method)
		}

	case ToolTypeScript:
		if t.Command == "" {
			return fmt.Errorf("command is required for script tool")
		}
	}

	// Validate timeout if specified
	if t.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}
	if t.Timeout > 300 {
		return fmt.Errorf("timeout must not exceed 300 seconds")
	}

	// Validate max response size if specified
	if t.MaxResponseSize < 0 {
		return fmt.Errorf("max_response_size must be non-negative")
	}

	return nil
}

// shorthandPattern matches action.operation or integration.operation keys like "file.read" or "github.list_issues"
var shorthandPattern = regexp.MustCompile(`^([a-z][a-z0-9_]*)\.([a-z][a-z0-9_]*)$`)

// builtinActionNames lists builtin actions that don't need integrations: config
var builtinActionNames = map[string]bool{
	"file":      true,
	"shell":     true,
	"http":      true,
	"transform": true,
	"utility":   true,
}

// primaryParameters maps operation names to their primary parameter for inline form
var primaryParameters = map[string]string{
	// File read operations
	"read":       "path",
	"read_text":  "path",
	"read_json":  "path",
	"read_yaml":  "path",
	"read_csv":   "path",
	"read_lines": "path",
	// File write operations (primary is path, content is second)
	"write":      "path",
	"write_text": "path",
	"write_json": "path",
	"write_yaml": "path",
	"append":     "path",
	"render":     "template",
	// Directory operations
	"list":   "path",
	"exists": "path",
	"stat":   "path",
	"mkdir":  "path",
	"copy":   "src",
	"move":   "src",
	"delete": "path",
	// Shell operations
	"run": "command",
	// Transform operations
	"parse_json": "data",
	"parse_xml":  "data",
	"extract":    "data",
	"split":      "data",
	"map":        "data",
	"filter":     "data",
	"flatten":    "data",
	"sort":       "data",
	"group":      "data",
	// Utility operations
	"random_int":      "max",
	"random_choose":   "items",
	"random_weighted": "items",
	"random_sample":   "items",
	"random_shuffle":  "items",
	"id_nanoid":       "length",
	"id_custom":       "length",
	"math_clamp":      "value",
	"math_round":      "value",
	"math_min":        "values",
	"math_max":        "values",
}

// UnmarshalYAML implements custom YAML unmarshaling for StepDefinition
// to support shorthand syntax like "file.read: ./config.json"
func (s *StepDefinition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// First try to unmarshal as a raw map to detect shorthand
	var raw map[string]interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	// Look for shorthand key
	shorthandKey, shorthandValue := findShorthandKey(raw)

	if shorthandKey != "" {
		// Parse shorthand: file.read -> action=file, operation=read
		matches := shorthandPattern.FindStringSubmatch(shorthandKey)
		if matches == nil {
			return fmt.Errorf("invalid shorthand key format: %s", shorthandKey)
		}

		name := matches[1]
		operationName := matches[2]

		// Determine if this is a builtin action or an integration
		isBuiltin := builtinActionNames[name]

		// Extract standard fields (id, condition, etc.)
		extractStandardFields(raw, s, shorthandKey)

		// Parse shorthand value into inputs
		inputs, err := parseShorthandInputs(operationName, shorthandValue)
		if err != nil {
			return fmt.Errorf("invalid shorthand value for %s: %w", shorthandKey, err)
		}

		// All steps use type: integration
		s.Type = StepTypeIntegration
		s.Inputs = inputs

		if isBuiltin {
			// Builtin action: set action and operation fields
			s.Action = name
			s.Operation = operationName
		} else {
			// User-defined integration: set integration field
			s.Integration = name + "." + operationName
		}

		return nil
	}

	// No shorthand found, use standard unmarshaling
	type plainStep StepDefinition
	if err := unmarshal((*plainStep)(s)); err != nil {
		return err
	}

	// Check if ID was explicitly set in the raw map
	if _, ok := raw["id"]; ok {
		s.hasExplicitID = true
	}

	return nil
}

// findShorthandKey looks for a provider.operation key in the map
func findShorthandKey(raw map[string]interface{}) (string, interface{}) {
	for key, value := range raw {
		if shorthandPattern.MatchString(key) {
			return key, value
		}
	}
	return "", nil
}

// extractStandardFields copies standard step fields from raw map to step
func extractStandardFields(raw map[string]interface{}, s *StepDefinition, skipKey string) {
	if id, ok := raw["id"].(string); ok {
		s.ID = id
		s.hasExplicitID = true
	}
	if name, ok := raw["name"].(string); ok {
		s.Name = name
	}
	if timeout, ok := raw["timeout"].(int); ok {
		s.Timeout = timeout
	}
	// Note: condition, on_error, and retry are complex types that require
	// separate YAML unmarshaling if used with shorthand syntax
}

// parseShorthandInputs converts shorthand value to inputs map
func parseShorthandInputs(operation string, value interface{}) (map[string]interface{}, error) {
	inputs := make(map[string]interface{})

	primaryParam := getPrimaryParameter(operation)
	if primaryParam == "" {
		primaryParam = "path" // default fallback
	}

	switch v := value.(type) {
	case string:
		// Simple string value -> primary parameter
		inputs[primaryParam] = v

	case map[string]interface{}:
		// Full object form with explicit parameters
		for k, val := range v {
			inputs[k] = val
		}

	case nil:
		// No value provided, operation may not need inputs
		return inputs, nil

	default:
		return nil, fmt.Errorf("unsupported value type: %T", value)
	}

	return inputs, nil
}

// getPrimaryParameter returns the primary parameter name for an operation
func getPrimaryParameter(operation string) string {
	if param, ok := primaryParameters[operation]; ok {
		return param
	}
	return ""
}
