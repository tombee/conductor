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

	// Validate no circular dependencies using DFS
	if err := b.validateNoCycles(); err != nil {
		return nil, err
	}

	// Validate template references exist
	if err := b.validateTemplateReferences(); err != nil {
		return nil, err
	}

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

// validateNoCycles checks for circular dependencies using depth-first search
func (b *WorkflowBuilder) validateNoCycles() error {
	// Build adjacency list for dependency graph
	graph := make(map[string][]string)
	for _, step := range b.steps {
		graph[step.id] = step.dependencies
	}

	// Track visited nodes and nodes in current path
	visited := make(map[string]bool)
	inPath := make(map[string]bool)

	// DFS function to detect cycles
	var dfs func(string, []string) error
	dfs = func(stepID string, path []string) error {
		if inPath[stepID] {
			// Found a cycle - build cycle path for error message
			cycleStart := -1
			for i, id := range path {
				if id == stepID {
					cycleStart = i
					break
				}
			}
			cyclePath := append(path[cycleStart:], stepID)
			return &ValidationError{
				Field:   "dependencies",
				Message: fmt.Sprintf("circular dependency detected: %v", cyclePath),
			}
		}

		if visited[stepID] {
			return nil // Already checked this path
		}

		visited[stepID] = true
		inPath[stepID] = true
		path = append(path, stepID)

		// Visit all dependencies
		for _, depID := range graph[stepID] {
			if err := dfs(depID, path); err != nil {
				return err
			}
		}

		inPath[stepID] = false
		return nil
	}

	// Check all steps for cycles
	for _, step := range b.steps {
		if !visited[step.id] {
			if err := dfs(step.id, []string{}); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateTemplateReferences validates that all template references exist
func (b *WorkflowBuilder) validateTemplateReferences() error {
	// Build a map of valid step IDs
	stepIDs := make(map[string]bool)
	for _, step := range b.steps {
		stepIDs[step.id] = true
	}

	// Check each step's template fields for references
	for _, step := range b.steps {
		// Collect all template strings from this step
		var templates []string

		switch step.stepType {
		case "llm":
			if step.system != "" {
				templates = append(templates, step.system)
			}
			if step.prompt != "" {
				templates = append(templates, step.prompt)
			}
		case "agent":
			if step.agentPrompt != "" {
				templates = append(templates, step.agentPrompt)
			}
		case "action":
			// Check action inputs for template strings
			for _, input := range step.actionInputs {
				if str, ok := input.(string); ok {
					templates = append(templates, str)
				}
			}
		}

		// Validate each template
		for _, tmpl := range templates {
			if err := validateTemplateString(tmpl, stepIDs, step.id); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateTemplateString checks if a template string references valid steps
func validateTemplateString(tmpl string, stepIDs map[string]bool, currentStepID string) error {
	// Simple regex-based validation for {{.steps.STEPID}} references
	// This is a basic implementation - a full template parser would be more robust
	const stepRefPrefix = "{{.steps."
	const stepRefSuffix = "}}"

	pos := 0
	for {
		// Find next step reference
		start := -1
		for i := pos; i < len(tmpl)-len(stepRefPrefix); i++ {
			if tmpl[i:i+len(stepRefPrefix)] == stepRefPrefix {
				start = i
				break
			}
		}

		if start == -1 {
			break // No more references
		}

		// Find end of reference
		end := -1
		for i := start + len(stepRefPrefix); i < len(tmpl)-1; i++ {
			if tmpl[i:i+2] == stepRefSuffix {
				end = i
				break
			}
		}

		if end == -1 {
			// Unclosed template reference - this is a template syntax error
			// We'll let the template engine handle this at runtime
			break
		}

		// Extract step ID (between "{{.steps." and the next ".")
		refStart := start + len(stepRefPrefix)
		refContent := tmpl[refStart:end]

		// Find the step ID (first part before any ".")
		stepID := refContent
		for dotPos := 0; dotPos < len(refContent); dotPos++ {
			if refContent[dotPos] == '.' {
				stepID = refContent[:dotPos]
				break
			}
		}

		// Validate step exists
		if !stepIDs[stepID] {
			return &ValidationError{
				Field:   fmt.Sprintf("step[%s].template", currentStepID),
				Message: fmt.Sprintf("template references non-existent step: %s", stepID),
			}
		}

		pos = end + 2
	}

	return nil
}
