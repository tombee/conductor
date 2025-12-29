package sdk

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/pkg/workflow"
)

// InputType represents the type of a workflow input parameter.
type InputType string

const (
	TypeString  InputType = "string"
	TypeNumber  InputType = "number"
	TypeBoolean InputType = "boolean"
	TypeArray   InputType = "array"
	TypeObject  InputType = "object"
)

// Workflow represents an executable workflow definition.
// Created by WorkflowBuilder.Build() or loaded from YAML.
type Workflow struct {
	// Name is the workflow identifier (exported for SDK consumers)
	Name string

	// Internal fields
	name       string
	inputs     map[string]*InputDef
	steps      []*stepDef
	definition *workflow.Definition // Set when loaded from YAML
}

// InputDef defines a workflow input parameter.
type InputDef struct {
	name         string
	typ          InputType
	defaultValue any
	hasDefault   bool
}

// WorkflowBuilder provides fluent workflow definition.
type WorkflowBuilder struct {
	sdk    *SDK
	name   string
	inputs map[string]*InputDef
	steps  []*stepDef
}

// Input declares a workflow input parameter.
//
// Example:
//
//	wf := s.NewWorkflow("example").
//		Input("code", sdk.TypeString).
//		Input("language", sdk.TypeString)
func (b *WorkflowBuilder) Input(name string, typ InputType) *WorkflowBuilder {
	b.inputs[name] = &InputDef{
		name:       name,
		typ:        typ,
		hasDefault: false,
	}
	return b
}

// InputWithDefault declares an input with a default value.
//
// Example:
//
//	wf := s.NewWorkflow("example").
//		InputWithDefault("temperature", sdk.TypeNumber, 0.7)
func (b *WorkflowBuilder) InputWithDefault(name string, typ InputType, defaultVal any) *WorkflowBuilder {
	b.inputs[name] = &InputDef{
		name:         name,
		typ:          typ,
		defaultValue: defaultVal,
		hasDefault:   true,
	}
	return b
}

// Step starts defining a new step.
//
// Example:
//
//	wf := s.NewWorkflow("example").
//		Step("greet").LLM().
//			Model("claude-sonnet-4-20250514").
//			Prompt("Say hello!").
//			Done()
func (b *WorkflowBuilder) Step(id string) *StepBuilder {
	return &StepBuilder{
		workflow: b,
		id:       id,
	}
}

// Build finalizes the workflow definition and validates it.
// Returns an error if the workflow is invalid (missing steps, circular dependencies, etc).
//
// Validation includes:
//   - At least one step defined
//   - All step IDs are unique
//   - All DependsOn references exist
//   - No circular dependencies
//   - All template references ({{.steps.X}}) exist (Decision D12)
func (b *WorkflowBuilder) Build() (*Workflow, error) {
	// Validate at least one step
	if len(b.steps) == 0 {
		return nil, &ValidationError{
			Field:   "steps",
			Message: "workflow must have at least one step",
		}
	}

	// Validate step IDs are unique
	stepIDs := make(map[string]bool)
	for _, step := range b.steps {
		if stepIDs[step.id] {
			return nil, &ValidationError{
				Field:   "steps",
				Message: fmt.Sprintf("duplicate step ID: %s", step.id),
			}
		}
		stepIDs[step.id] = true
	}

	// Validate dependencies exist
	for _, step := range b.steps {
		for _, depID := range step.dependencies {
			if !stepIDs[depID] {
				return nil, &ValidationError{
					Field:   fmt.Sprintf("step[%s].depends_on", step.id),
					Message: fmt.Sprintf("dependency not found: %s", depID),
				}
			}
		}
	}

	// TODO: Validate no circular dependencies
	// TODO: Validate template references (D12)

	return &Workflow{
		Name:   b.name,
		name:   b.name,
		inputs: b.inputs,
		steps:  b.steps,
	}, nil
}

// stepDef is an internal representation of a workflow step.
type stepDef struct {
	id           string
	stepType     string // "llm", "action", "agent", "parallel", "condition"
	dependencies []string

	// LLM step fields
	model        string
	system       string
	prompt       string
	temperature  *float64
	maxTokens    *int
	outputSchema map[string]any
	tools        []string

	// Action step fields
	actionName   string
	actionInputs map[string]any

	// Agent step fields
	agentPrompt string

	// Parallel step fields
	parallelSteps  []*stepDef
	maxConcurrency int

	// Condition step fields
	condition     string
	thenSteps     []*stepDef
	elseSteps     []*stepDef
}

// StepCount returns the number of steps in the workflow.
// Useful for testing and workflow inspection.
func (wf *Workflow) StepCount() int {
	return len(wf.steps)
}

// ValidateInputs validates workflow inputs against expected types.
// Call before Run() to catch validation errors early.
//
// Example:
//
//	inputs := map[string]any{
//		"code": "func main() {}",
//		"language": "Go",
//	}
//	if err := s.ValidateInputs(ctx, wf, inputs); err != nil {
//		return err
//	}
func (s *SDK) ValidateInputs(ctx context.Context, wf *Workflow, inputs map[string]any) error {
	// Check required inputs
	for name, inputDef := range wf.inputs {
		if !inputDef.hasDefault {
			if _, ok := inputs[name]; !ok {
				return &ValidationError{
					Field:   name,
					Message: "required input missing",
				}
			}
		}
	}

	// TODO: Validate input types match expected types
	// This requires type checking logic

	return nil
}
