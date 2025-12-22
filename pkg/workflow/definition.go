// Package workflow provides workflow orchestration primitives.
package workflow

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Definition represents a YAML-based workflow definition.
// It defines the structure, steps, conditions, and outputs of a workflow
// that can be loaded from a YAML file and executed by the workflow engine.
type Definition struct {
	// Name is the workflow identifier
	Name string `yaml:"name" json:"name"`

	// Description provides human-readable context about the workflow
	Description string `yaml:"description" json:"description"`

	// Version tracks the workflow definition schema version
	Version string `yaml:"version" json:"version"`

	// Inputs defines the expected input parameters for the workflow
	Inputs []InputDefinition `yaml:"inputs" json:"inputs"`

	// Steps are the executable units of the workflow
	Steps []StepDefinition `yaml:"steps" json:"steps"`

	// Outputs define what data is returned when the workflow completes
	Outputs []OutputDefinition `yaml:"outputs" json:"outputs"`
}

// InputDefinition describes a workflow input parameter.
type InputDefinition struct {
	// Name is the input parameter identifier
	Name string `yaml:"name" json:"name"`

	// Type specifies the data type (string, number, boolean, object, array)
	Type string `yaml:"type" json:"type"`

	// Required indicates if this input must be provided
	Required bool `yaml:"required" json:"required"`

	// Default provides a fallback value if input is not provided
	Default interface{} `yaml:"default,omitempty" json:"default,omitempty"`

	// Description explains what this input is for
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

// StepDefinition represents a single step in a workflow.
type StepDefinition struct {
	// ID is the unique step identifier within this workflow
	ID string `yaml:"id" json:"id"`

	// Name is a human-readable step name
	Name string `yaml:"name" json:"name"`

	// Type specifies the step type (action, condition, parallel, etc.)
	Type StepType `yaml:"type" json:"type"`

	// Action specifies what to execute (tool name, LLM call, etc.)
	Action string `yaml:"action,omitempty" json:"action,omitempty"`

	// Inputs maps input names to values (can reference previous step outputs)
	Inputs map[string]interface{} `yaml:"inputs,omitempty" json:"inputs,omitempty"`

	// Condition defines when this step should execute
	Condition *ConditionDefinition `yaml:"condition,omitempty" json:"condition,omitempty"`

	// OnError specifies error handling behavior
	OnError *ErrorHandlingDefinition `yaml:"on_error,omitempty" json:"on_error,omitempty"`

	// Timeout sets the maximum execution time for this step (in seconds)
	Timeout int `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Retry configures retry behavior for this step
	Retry *RetryDefinition `yaml:"retry,omitempty" json:"retry,omitempty"`
}

// StepType represents the type of workflow step.
type StepType string

const (
	// StepTypeAction executes a tool or action
	StepTypeAction StepType = "action"

	// StepTypeCondition evaluates a condition and branches
	StepTypeCondition StepType = "condition"

	// StepTypeLLM makes an LLM API call
	StepTypeLLM StepType = "llm"

	// StepTypeParallel executes multiple steps concurrently
	StepTypeParallel StepType = "parallel"
)

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

// ParseDefinition parses a workflow definition from YAML bytes.
func ParseDefinition(data []byte) (*Definition, error) {
	var def Definition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("failed to parse workflow definition: %w", err)
	}

	if err := def.Validate(); err != nil {
		return nil, fmt.Errorf("invalid workflow definition: %w", err)
	}

	return &def, nil
}

// Validate checks if the workflow definition is valid.
func (d *Definition) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("workflow name is required")
	}

	if d.Version == "" {
		return fmt.Errorf("workflow version is required")
	}

	if len(d.Steps) == 0 {
		return fmt.Errorf("workflow must have at least one step")
	}

	// Validate step IDs are unique
	stepIDs := make(map[string]bool)
	for _, step := range d.Steps {
		if step.ID == "" {
			return fmt.Errorf("step ID is required")
		}
		if stepIDs[step.ID] {
			return fmt.Errorf("duplicate step ID: %s", step.ID)
		}
		stepIDs[step.ID] = true

		// Validate step
		if err := step.Validate(); err != nil {
			return fmt.Errorf("invalid step %s: %w", step.ID, err)
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

	return nil
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

	if s.Name == "" {
		return fmt.Errorf("step name is required")
	}

	if s.Type == "" {
		return fmt.Errorf("step type is required")
	}

	// Validate step type
	validTypes := map[StepType]bool{
		StepTypeAction:    true,
		StepTypeCondition: true,
		StepTypeLLM:       true,
		StepTypeParallel:  true,
	}
	if !validTypes[s.Type] {
		return fmt.Errorf("invalid step type: %s", s.Type)
	}

	// Validate action is present for action and LLM steps
	if (s.Type == StepTypeAction || s.Type == StepTypeLLM) && s.Action == "" {
		return fmt.Errorf("action is required for %s step type", s.Type)
	}

	// Validate condition is present for condition steps
	if s.Type == StepTypeCondition && s.Condition == nil {
		return fmt.Errorf("condition is required for condition step type")
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
