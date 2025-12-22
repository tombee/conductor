package workflow_test

import (
	"context"
	"fmt"
	"log"

	"github.com/tombee/conductor/pkg/workflow"
)

// Example demonstrates a complete workflow lifecycle with state machine,
// event emitter, and persistent storage.
func Example() {
	ctx := context.Background()

	// Create a workflow store
	store := workflow.NewMemoryStore()

	// Create an event emitter (async mode)
	emitter := workflow.NewEventEmitter(true)

	// Register event listeners
	emitter.On(workflow.EventStateChanged, func(ctx context.Context, event *workflow.Event) error {
		fmt.Printf("State changed: %v -> %v\n",
			event.Data["from_state"],
			event.Data["to_state"])
		return nil
	})

	// Create a state machine with default transitions
	sm := workflow.NewStateMachine(workflow.DefaultTransitions())

	// Set up hooks to emit events and persist state
	sm.SetHooks(&workflow.Hooks{
		AfterTransition: func(ctx context.Context, w *workflow.Workflow, from workflow.State, to workflow.State) error {
			// Emit state change event
			if err := emitter.EmitStateChanged(ctx, w.ID, from, to, "transition"); err != nil {
				return err
			}
			// Persist the workflow
			return store.Update(ctx, w)
		},
	})

	// Create a new workflow
	wf := &workflow.Workflow{
		ID:   "example-workflow-1",
		Name: "Example Workflow",
		Metadata: map[string]interface{}{
			"project": "demo",
			"priority": "high",
		},
	}

	// Save it to the store
	if err := store.Create(ctx, wf); err != nil {
		log.Fatal(err)
	}

	// Trigger state transitions
	if err := sm.Trigger(ctx, wf, "start"); err != nil {
		log.Fatal(err)
	}

	if err := sm.Trigger(ctx, wf, "complete"); err != nil {
		log.Fatal(err)
	}

	// Retrieve the workflow from storage
	retrieved, err := store.Get(ctx, "example-workflow-1")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Final state: %s\n", retrieved.State)
	fmt.Printf("Is terminal: %v\n", retrieved.State.IsTerminal())

	// Output:
	// State changed: created -> running
	// State changed: running -> completed
	// Final state: completed
	// Is terminal: true
}

// ExampleQuery demonstrates querying workflows by state and metadata.
func Example_query() {
	ctx := context.Background()
	store := workflow.NewMemoryStore()

	// Create multiple workflows
	workflows := []*workflow.Workflow{
		{ID: "wf-1", State: workflow.StateRunning, Metadata: map[string]interface{}{"priority": "high"}},
		{ID: "wf-2", State: workflow.StateCompleted, Metadata: map[string]interface{}{"priority": "low"}},
		{ID: "wf-3", State: workflow.StateRunning, Metadata: map[string]interface{}{"priority": "high"}},
		{ID: "wf-4", State: workflow.StateFailed, Metadata: map[string]interface{}{"priority": "high"}},
	}

	for _, wf := range workflows {
		if err := store.Create(ctx, wf); err != nil {
			log.Fatal(err)
		}
	}

	// Query: Find all running workflows with high priority
	state := workflow.StateRunning
	results, err := store.List(ctx, &workflow.Query{
		State:    &state,
		Metadata: map[string]interface{}{"priority": "high"},
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d running high-priority workflows\n", len(results))

	// Output:
	// Found 2 running high-priority workflows
}

// Example_templateVariables demonstrates using template variables in workflow steps.
// This shows how workflow inputs and step outputs are passed between steps using
// Go template syntax ({{.variable}}).
func Example_templateVariables() {
	ctx := context.Background()

	// Mock LLM provider that returns predictable responses
	llmProvider := &mockLLMProvider{
		responses: map[string]string{
			"Review for security issues:\nfunc main() { ... }": "No security issues found",
			"Summarize this review:\nNo security issues found":  "All security checks passed",
		},
	}

	// Create executor
	executor := workflow.NewStepExecutor(nil, llmProvider)

	// Create template context with workflow inputs
	templateCtx := workflow.NewTemplateContext()
	templateCtx.SetInput("diff", "func main() { ... }")

	// Build workflow context
	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	// Step 1: Security review (uses input variable)
	step1 := &workflow.StepDefinition{
		ID:     "security",
		Type:   workflow.StepTypeLLM,
		Prompt: "Review for security issues:\n{{.diff}}",
	}

	result1, err := executor.Execute(ctx, step1, workflowContext)
	if err != nil {
		log.Fatal(err)
	}

	// Add step output to context for next step
	templateCtx.SetStepOutput("security", result1.Output)

	// Step 2: Summary (uses previous step output)
	step2 := &workflow.StepDefinition{
		ID:     "summary",
		Type:   workflow.StepTypeLLM,
		Prompt: "Summarize this review:\n{{.steps.security.response}}",
	}

	result2, err := executor.Execute(ctx, step2, workflowContext)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Security review: %s\n", result1.Output["response"])
	fmt.Printf("Summary: %s\n", result2.Output["response"])

	// Output:
	// Security review: No security issues found
	// Summary: All security checks passed
}

// Example_simpleWorkflowFormat demonstrates the SPEC-2 simple workflow format.
// This shows how to parse and use minimal workflow definitions with optional fields.
func Example_simpleWorkflowFormat() {
	// Minimal workflow YAML - only name and steps required
	minimalYAML := `
name: summarize
steps:
  - id: summarize
    type: llm
    prompt: "Summarize this text: {{.input}}"
`

	// Parse the workflow definition
	def, err := workflow.ParseDefinition([]byte(minimalYAML))
	if err != nil {
		log.Fatal(err)
	}

	// Version defaults to empty string (no validation required per SPEC-2)
	fmt.Printf("Workflow: %s\n", def.Name)
	fmt.Printf("Steps: %d\n", len(def.Steps))
	fmt.Printf("Step type: %s\n", def.Steps[0].Type)
	fmt.Printf("Model tier: %s (default)\n", def.Steps[0].Model)

	// Output:
	// Workflow: summarize
	// Steps: 1
	// Step type: llm
	// Model tier: balanced (default)
}

// Example_modelTiers demonstrates using model tier selection in LLM steps.
// Model tiers (fast/balanced/strategic) abstract provider-specific model names.
func Example_modelTiers() {
	workflowYAML := `
name: multi-tier-analysis
steps:
  - id: quick-check
    type: llm
    model: fast
    prompt: "Is this text positive or negative? {{.input}}"
  - id: detailed-analysis
    type: llm
    model: balanced
    prompt: "Provide detailed sentiment analysis: {{.input}}"
  - id: deep-reasoning
    type: llm
    model: strategic
    system: "You are an expert analyst."
    prompt: "What are the implications? {{.input}}"
`

	def, err := workflow.ParseDefinition([]byte(workflowYAML))
	if err != nil {
		log.Fatal(err)
	}

	for _, step := range def.Steps {
		fmt.Printf("%s: model=%s\n", step.ID, step.Model)
	}

	// Output:
	// quick-check: model=fast
	// detailed-analysis: model=balanced
	// deep-reasoning: model=strategic
}

// Example_workflowParsing demonstrates parsing a complete workflow with all features.
func Example_workflowParsing() {
	workflowYAML := `
name: code-review
description: Reviews code for security issues and summarizes findings
inputs:
  - name: diff
    type: string
    description: The code diff to review
steps:
  - id: security
    type: llm
    model: balanced
    system: "You are a security expert reviewing code changes."
    prompt: "Review for security issues:\n{{.diff}}"
  - id: summary
    type: llm
    model: fast
    prompt: "Summarize this security review in 2-3 sentences:\n{{.steps.security.response}}"
outputs:
  - name: review
    type: string
    value: "{{.steps.summary.response}}"
`

	def, err := workflow.ParseDefinition([]byte(workflowYAML))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Workflow: %s\n", def.Name)
	fmt.Printf("Description: %s\n", def.Description)
	fmt.Printf("Inputs: %d\n", len(def.Inputs))
	fmt.Printf("Steps: %d\n", len(def.Steps))
	fmt.Printf("Outputs: %d\n", len(def.Outputs))

	// Show step details
	for i, step := range def.Steps {
		fmt.Printf("Step %d: id=%s, type=%s, model=%s\n", i+1, step.ID, step.Type, step.Model)
	}

	// Output:
	// Workflow: code-review
	// Description: Reviews code for security issues and summarizes findings
	// Inputs: 1
	// Steps: 2
	// Outputs: 1
	// Step 1: id=security, type=llm, model=balanced
	// Step 2: id=summary, type=llm, model=fast
}

// mockLLMProvider for example tests
type mockLLMProvider struct {
	responses map[string]string
}

func (m *mockLLMProvider) Complete(ctx context.Context, prompt string, options map[string]interface{}) (string, error) {
	if response, ok := m.responses[prompt]; ok {
		return response, nil
	}
	return "default response", nil
}
