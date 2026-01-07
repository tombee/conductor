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

	// Trigger defines how this workflow can be invoked (webhooks, API, schedules)
	Trigger *TriggerConfig `yaml:"trigger,omitempty" json:"trigger,omitempty"`

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


// InputDefinition describes a workflow input parameter.
// Inputs without a default value are required.
type InputDefinition struct {
	// Name is the input parameter identifier
	Name string `yaml:"name" json:"name"`

	// Type specifies the data type (string, number, boolean, object, array, enum)
	Type string `yaml:"type" json:"type"`

	// Default provides a fallback value if input is not provided.
	// Inputs without a default are required.
	Default interface{} `yaml:"default,omitempty" json:"default,omitempty"`

	// Description explains what this input is for
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Enum defines the allowed values for enum-type inputs
	Enum []string `yaml:"enum,omitempty" json:"enum,omitempty"`

	// Pattern is a regex pattern for validating string inputs
	Pattern string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
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

	// SystemPrompt is the system prompt for agent steps (optional)
	// Guides agent behavior and provides context
	SystemPrompt string `yaml:"system_prompt,omitempty" json:"system_prompt,omitempty"`

	// UserPrompt is the user prompt for agent steps (required for type=agent)
	// Defines the agent's task objective
	UserPrompt string `yaml:"user_prompt,omitempty" json:"user_prompt,omitempty"`

	// AgentConfig configures agent execution limits (max_iterations, token_limit, stop_on_error)
	AgentConfig *AgentConfigDefinition `yaml:"config,omitempty" json:"config,omitempty"`

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

	// StepTypeAgent executes a ReAct loop with tool use
	StepTypeAgent StepType = "agent"
)

// Default step timeouts in seconds.
// These are applied when a step does not specify an explicit timeout.
const (
	// DefaultLLMStepTimeout is the default timeout for LLM steps (10 minutes).
	// LLM calls can take significant time, especially with local models like Ollama
	// which may need 2-5+ minutes for complex prompts. Cloud APIs are faster but
	// we use a generous default to support all providers.
	DefaultLLMStepTimeout = 600

	// DefaultActionStepTimeout is the default timeout for action and other steps (2 minutes).
	// This covers HTTP calls, shell commands, file operations, etc.
	DefaultActionStepTimeout = 120
)

// Default retry configuration values.
const (
	// DefaultRetryMaxAttempts is the default number of retry attempts.
	DefaultRetryMaxAttempts = 2

	// DefaultRetryBackoffBase is the base backoff duration in seconds.
	DefaultRetryBackoffBase = 1

	// DefaultRetryBackoffMultiplier is the exponential backoff multiplier.
	DefaultRetryBackoffMultiplier = 2.0
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

	// Type specifies the output data type (defaults to "string" if not specified)
	Type string `yaml:"type,omitempty" json:"type,omitempty"`

	// Value is an expression that computes the output value
	// (e.g., "$.final_step.result")
	Value string `yaml:"value" json:"value"`

	// Description explains what this output represents
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Format specifies how the output should be formatted and validated.
	// Supported formats: string (default), number, markdown, json, code, code:<language>
	Format string `yaml:"format,omitempty" json:"format,omitempty"`
}

// AgentConfigDefinition configures agent step execution limits.
type AgentConfigDefinition struct {
	// MaxIterations limits the number of ReAct loop iterations
	MaxIterations int `yaml:"max_iterations,omitempty" json:"max_iterations,omitempty"`

	// TokenLimit sets cumulative token threshold across all iterations
	TokenLimit int `yaml:"token_limit,omitempty" json:"token_limit,omitempty"`

	// StopOnError determines agent behavior on tool failures
	// When true: stop immediately on first tool error
	// When false: report error to agent, allow recovery attempts (default)
	StopOnError bool `yaml:"stop_on_error,omitempty" json:"stop_on_error,omitempty"`
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

		// Default timeout based on step type:
		// - Loop/parallel steps: no default timeout (max_iterations provides safety limit)
		// - LLM steps: DefaultLLMStepTimeout (LLM calls can take 30+ seconds each)
		// - Other steps: DefaultActionStepTimeout
		if step.Timeout == 0 {
			switch step.Type {
			case StepTypeLoop, StepTypeParallel:
				// No default timeout for loop/parallel - they have max_iterations
				// and will inherit the workflow-level timeout
			case StepTypeLLM:
				step.Timeout = DefaultLLMStepTimeout
			default:
				step.Timeout = DefaultActionStepTimeout
			}
		}

		// Default retry configuration
		if step.Retry == nil {
			step.Retry = &RetryDefinition{
				MaxAttempts:       DefaultRetryMaxAttempts,
				BackoffBase:       DefaultRetryBackoffBase,
				BackoffMultiplier: DefaultRetryBackoffMultiplier,
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

		// Recursively apply defaults to nested steps (loop, parallel)
		for j := range step.Steps {
			nested := &step.Steps[j]
			applyStepDefaults(nested)
		}
	}
	return nil
}

// applyStepDefaults applies default values to a single step.
func applyStepDefaults(step *StepDefinition) {
	// Default timeout based on step type
	if step.Timeout == 0 {
		switch step.Type {
		case StepTypeLoop, StepTypeParallel:
			// No default timeout for loop/parallel
		case StepTypeLLM:
			step.Timeout = DefaultLLMStepTimeout
		default:
			step.Timeout = DefaultActionStepTimeout
		}
	}

	// Default retry configuration
	if step.Retry == nil {
		step.Retry = &RetryDefinition{
			MaxAttempts:       DefaultRetryMaxAttempts,
			BackoffBase:       DefaultRetryBackoffBase,
			BackoffMultiplier: DefaultRetryBackoffMultiplier,
		}
	}

	// Default model tier for LLM steps
	if step.Type == StepTypeLLM && step.Model == "" {
		step.Model = string(ModelTierBalanced)
	}

	// Recursively apply to nested steps
	for i := range step.Steps {
		applyStepDefaults(&step.Steps[i])
	}
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

		// Validate parallel nesting depth (security limit)
		if err := ValidateParallelNestingDepth(&step, 0); err != nil {
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
	// Only validate if functions are defined in the workflow.
	// If no functions are defined, tools are assumed to come from a runtime registry.
	if len(functionNames) > 0 {
		for _, step := range d.Steps {
			for _, functionName := range step.Tools {
				if !functionNames[functionName] {
					return fmt.Errorf("step %s references undefined function: %s", step.ID, functionName)
				}
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

			// Check integration exists in workflow definition
			// If not defined here, it may be a workspace integration (configured via CLI)
			// which will be resolved at runtime
			if integrationNames[integrationName] {
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
			// If integration not in workflow, assume it's a workspace integration
			// Runtime will error if it doesn't exist
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

	// Validate pattern is a valid regex (only for string type)
	if i.Pattern != "" {
		if i.Type != "string" {
			return fmt.Errorf("pattern can only be used with string type inputs")
		}
		if _, err := regexp.Compile(i.Pattern); err != nil {
			return fmt.Errorf("invalid pattern regex: %w", err)
		}
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
		StepTypeAgent:       true,
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

	// Validate agent steps
	if s.Type == StepTypeAgent {
		// user_prompt is required
		if s.UserPrompt == "" {
			return fmt.Errorf("user_prompt is required for agent step type")
		}

		// tools array must not be empty
		if len(s.Tools) == 0 {
			return fmt.Errorf("tools array cannot be empty for agent step type")
		}

		// Validate agent config if present
		if s.AgentConfig != nil {
			// max_iterations must be positive
			if s.AgentConfig.MaxIterations < 0 {
				return fmt.Errorf("max_iterations must be non-negative")
			}

			// token_limit must be positive
			if s.AgentConfig.TokenLimit < 0 {
				return fmt.Errorf("token_limit must be non-negative")
			}
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
		// Validate max_concurrency bounds
		if err := ValidateMaxConcurrency(s); err != nil {
			return err
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

	// Default type to "string" if not specified
	if o.Type == "" {
		o.Type = "string"
	}

	if o.Value == "" {
		return fmt.Errorf("output value expression is required")
	}

	// Validate format if specified
	if o.Format != "" {
		if err := o.validateFormat(); err != nil {
			return err
		}
	}

	return nil
}

// validateFormat checks if the format value is valid.
func (o *OutputDefinition) validateFormat() error {
	format := strings.ToLower(o.Format)

	// Check for code with language (e.g., "code:python")
	if strings.HasPrefix(format, "code:") {
		// Language is valid as long as it's not empty after the colon
		lang := strings.TrimPrefix(format, "code:")
		if lang == "" {
			return fmt.Errorf("invalid format: code language cannot be empty (use 'code' for no highlighting or 'code:<language>')")
		}
		return nil
	}

	// Check for basic formats
	validFormats := map[string]bool{
		"string":   true,
		"number":   true,
		"markdown": true,
		"json":     true,
		"code":     true,
	}

	if !validFormats[format] {
		return fmt.Errorf("invalid format: %s (must be one of: string, number, markdown, json, code, code:<language>)", o.Format)
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

	// Check for parallel shorthand syntax (parallel: [...])
	// This must be checked before the action shorthand pattern matching
	if parallelSteps, ok := raw["parallel"]; ok {
		nestedSteps, err := parseParallelShorthand(parallelSteps)
		if err != nil {
			stepID := ""
			if id, ok := raw["id"].(string); ok {
				stepID = id
			}
			return fmt.Errorf("Invalid parallel block: %s. parallel: must be a list of steps: %w", stepID, err)
		}

		s.Type = StepTypeParallel
		s.Steps = nestedSteps
		extractParallelFields(raw, s)
		return nil
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

// parseParallelShorthand parses the value of a parallel: shorthand key into nested steps
func parseParallelShorthand(value interface{}) ([]StepDefinition, error) {
	// Value must be an array of step definitions
	rawSteps, ok := value.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array of steps, got %T", value)
	}

	if len(rawSteps) == 0 {
		return nil, fmt.Errorf("parallel block requires at least one nested step")
	}

	steps := make([]StepDefinition, 0, len(rawSteps))
	for i, rawStep := range rawSteps {
		// Each step should be a map
		stepMap, ok := rawStep.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("step %d: expected map, got %T", i, rawStep)
		}

		// Re-marshal and unmarshal to get proper StepDefinition
		// This allows nested steps to use their own shorthand syntax
		yamlBytes, err := yaml.Marshal(stepMap)
		if err != nil {
			return nil, fmt.Errorf("step %d: failed to marshal: %w", i, err)
		}

		var step StepDefinition
		if err := yaml.Unmarshal(yamlBytes, &step); err != nil {
			return nil, fmt.Errorf("step %d: failed to unmarshal: %w", i, err)
		}

		steps = append(steps, step)
	}

	return steps, nil
}

// extractParallelFields copies parallel-specific fields from raw map to step
func extractParallelFields(raw map[string]interface{}, s *StepDefinition) {
	// Extract standard fields
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

	// Extract parallel-specific fields
	if maxConcurrency, ok := raw["max_concurrency"].(int); ok {
		s.MaxConcurrency = maxConcurrency
	}

	// Extract foreach if present
	if foreach, ok := raw["foreach"].(string); ok {
		s.Foreach = foreach
	}

	// Note: on_error is a complex type that would require additional
	// unmarshaling support. For now, parallel shorthand doesn't support it.
	// Users needing on_error should use the full type: parallel syntax.
}
