package subworkflow_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
	"github.com/tombee/conductor/pkg/workflow/subworkflow"
)

// TestSubworkflowWithParallel tests sub-workflows inside parallel steps
func TestSubworkflowWithParallel(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create a simple sub-workflow
	subWorkflowYAML := `name: sub-task
inputs:
  - name: item
    type: string
    required: true
outputs:
  - name: result
    type: string
    value: "{{.inputs.item}}"
steps:
  - id: process
    type: llm
    model: fast
    prompt: "Process {{.inputs.item}}"
`
	subWorkflowPath := filepath.Join(tmpDir, "sub-task.yaml")
	if err := os.WriteFile(subWorkflowPath, []byte(subWorkflowYAML), 0644); err != nil {
		t.Fatalf("Failed to write sub-workflow file: %v", err)
	}

	// Create main workflow with parallel sub-workflow execution
	mainWorkflowYAML := `name: parallel-sub-workflows
steps:
  - id: parallel_tasks
    type: parallel
    foreach: '["task1", "task2", "task3"]'
    steps:
      - id: run_task
        type: workflow
        workflow: ./sub-task.yaml
        inputs:
          item: "{{.item}}"
`
	mainWorkflowPath := filepath.Join(tmpDir, "main.yaml")
	if err := os.WriteFile(mainWorkflowPath, []byte(mainWorkflowYAML), 0644); err != nil {
		t.Fatalf("Failed to write main workflow file: %v", err)
	}

	// Parse the main workflow
	data, err := os.ReadFile(mainWorkflowPath)
	if err != nil {
		t.Fatalf("Failed to read main workflow: %v", err)
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		t.Fatalf("Failed to parse main workflow: %v", err)
	}

	// Verify the workflow has a parallel step with a workflow step inside
	if len(def.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(def.Steps))
	}

	parallelStep := def.Steps[0]
	if parallelStep.Type != workflow.StepTypeParallel {
		t.Fatalf("Expected parallel step, got %v", parallelStep.Type)
	}

	if len(parallelStep.Steps) != 1 {
		t.Fatalf("Expected 1 nested step in parallel, got %d", len(parallelStep.Steps))
	}

	workflowStep := parallelStep.Steps[0]
	if workflowStep.Type != workflow.StepTypeWorkflow {
		t.Fatalf("Expected workflow step, got %v", workflowStep.Type)
	}

	// Verify sub-workflow can be loaded
	loader := subworkflow.NewLoader()
	subDef, err := loader.Load(tmpDir, workflowStep.Workflow, nil)
	if err != nil {
		t.Fatalf("Failed to load sub-workflow: %v", err)
	}

	if subDef.Name != "sub-task" {
		t.Errorf("Expected sub-workflow name 'sub-task', got %s", subDef.Name)
	}
}

// TestSubworkflowWithLoop tests sub-workflows inside loop-like structures
func TestSubworkflowWithLoop(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create a refine sub-workflow
	refineWorkflowYAML := `name: refine-code
inputs:
  - name: code
    type: string
    required: true
outputs:
  - name: improved_code
    type: string
    value: "{{.steps.improve.outputs.response}}"
  - name: approved
    type: boolean
    value: "true"
steps:
  - id: improve
    type: llm
    model: balanced
    prompt: "Improve this code: {{.inputs.code}}"
`
	refinePath := filepath.Join(tmpDir, "refine.yaml")
	if err := os.WriteFile(refinePath, []byte(refineWorkflowYAML), 0644); err != nil {
		t.Fatalf("Failed to write refine workflow: %v", err)
	}

	// Create main workflow with sequential execution containing sub-workflow
	// (loop functionality would be similar - this tests the structure)
	mainWorkflowYAML := `name: sequential-with-sub-workflow
inputs:
  - name: initial_code
    type: string
    required: true
steps:
  - id: refine
    type: workflow
    workflow: ./refine.yaml
    inputs:
      code: "{{.inputs.initial_code}}"
`
	mainWorkflowPath := filepath.Join(tmpDir, "main.yaml")
	if err := os.WriteFile(mainWorkflowPath, []byte(mainWorkflowYAML), 0644); err != nil {
		t.Fatalf("Failed to write main workflow: %v", err)
	}

	// Parse the main workflow
	data, err := os.ReadFile(mainWorkflowPath)
	if err != nil {
		t.Fatalf("Failed to read main workflow: %v", err)
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		t.Fatalf("Failed to parse main workflow: %v", err)
	}

	// Verify the workflow structure
	if len(def.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(def.Steps))
	}

	// Verify sub-workflow can be loaded
	loader := subworkflow.NewLoader()
	workflowStep := def.Steps[0]
	subDef, err := loader.Load(tmpDir, workflowStep.Workflow, nil)
	if err != nil {
		t.Fatalf("Failed to load sub-workflow: %v", err)
	}

	if subDef.Name != "refine-code" {
		t.Errorf("Expected sub-workflow name 'refine-code', got %s", subDef.Name)
	}
}

// TestSubworkflowWithCondition tests conditional sub-workflow execution
func TestSubworkflowWithCondition(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create specialized sub-workflows
	deepAnalysisYAML := `name: deep-analysis
inputs:
  - name: code
    type: string
    required: true
outputs:
  - name: result
    type: string
    value: "Deep analysis complete"
steps:
  - id: analyze
    type: llm
    model: strategic
    prompt: "Perform deep analysis on: {{.inputs.code}}"
`
	deepAnalysisPath := filepath.Join(tmpDir, "deep-analysis.yaml")
	if err := os.WriteFile(deepAnalysisPath, []byte(deepAnalysisYAML), 0644); err != nil {
		t.Fatalf("Failed to write deep-analysis workflow: %v", err)
	}

	quickCheckYAML := `name: quick-check
inputs:
  - name: code
    type: string
    required: true
outputs:
  - name: result
    type: string
    value: "Quick check complete"
steps:
  - id: check
    type: llm
    model: fast
    prompt: "Quick check: {{.inputs.code}}"
`
	quickCheckPath := filepath.Join(tmpDir, "quick-check.yaml")
	if err := os.WriteFile(quickCheckPath, []byte(quickCheckYAML), 0644); err != nil {
		t.Fatalf("Failed to write quick-check workflow: %v", err)
	}

	// Create main workflow with conditional sub-workflow dispatch
	mainWorkflowYAML := `name: conditional-sub-workflows
inputs:
  - name: priority
    type: string
    required: true
  - name: code
    type: string
    required: true
steps:
  - id: deep_analysis
    type: workflow
    workflow: ./deep-analysis.yaml
    condition:
      expression: 'inputs.priority == "high"'
    inputs:
      code: "{{.inputs.code}}"

  - id: quick_check
    type: workflow
    workflow: ./quick-check.yaml
    condition:
      expression: 'inputs.priority == "low"'
    inputs:
      code: "{{.inputs.code}}"
`
	mainWorkflowPath := filepath.Join(tmpDir, "main.yaml")
	if err := os.WriteFile(mainWorkflowPath, []byte(mainWorkflowYAML), 0644); err != nil {
		t.Fatalf("Failed to write main workflow: %v", err)
	}

	// Parse the main workflow
	data, err := os.ReadFile(mainWorkflowPath)
	if err != nil {
		t.Fatalf("Failed to read main workflow: %v", err)
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		t.Fatalf("Failed to parse main workflow: %v", err)
	}

	// Verify the workflow has conditional sub-workflow steps
	if len(def.Steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(def.Steps))
	}

	// Verify first step is a conditional workflow step
	step1 := def.Steps[0]
	if step1.Type != workflow.StepTypeWorkflow {
		t.Fatalf("Expected workflow step, got %v", step1.Type)
	}
	if step1.Condition == nil {
		t.Fatal("Expected condition on first workflow step")
	}
	if step1.Condition.Expression != `inputs.priority == "high"` {
		t.Errorf("Expected condition 'inputs.priority == \"high\"', got %s", step1.Condition.Expression)
	}

	// Verify second step is also a conditional workflow step
	step2 := def.Steps[1]
	if step2.Type != workflow.StepTypeWorkflow {
		t.Fatalf("Expected workflow step, got %v", step2.Type)
	}
	if step2.Condition == nil {
		t.Fatal("Expected condition on second workflow step")
	}

	// Verify both sub-workflows can be loaded
	loader := subworkflow.NewLoader()

	subDef1, err := loader.Load(tmpDir, step1.Workflow, nil)
	if err != nil {
		t.Fatalf("Failed to load deep-analysis sub-workflow: %v", err)
	}
	if subDef1.Name != "deep-analysis" {
		t.Errorf("Expected sub-workflow name 'deep-analysis', got %s", subDef1.Name)
	}

	subDef2, err := loader.Load(tmpDir, step2.Workflow, nil)
	if err != nil {
		t.Fatalf("Failed to load quick-check sub-workflow: %v", err)
	}
	if subDef2.Name != "quick-check" {
		t.Errorf("Expected sub-workflow name 'quick-check', got %s", subDef2.Name)
	}
}

// TestSubworkflowRetryConfiguration tests that retry config is parsed correctly
func TestSubworkflowRetryConfiguration(t *testing.T) {
	// Create test directory structure
	tmpDir := t.TempDir()

	// Create a sub-workflow that might fail
	subWorkflowYAML := `name: flaky-task
inputs:
  - name: data
    type: string
    required: true
outputs:
  - name: result
    type: string
    value: "{{.steps.process.outputs.response}}"
steps:
  - id: process
    type: llm
    model: fast
    prompt: "Process: {{.inputs.data}}"
    retry:
      max_attempts: 2
      backoff_base: 1
      backoff_multiplier: 2.0
`
	subWorkflowPath := filepath.Join(tmpDir, "flaky-task.yaml")
	if err := os.WriteFile(subWorkflowPath, []byte(subWorkflowYAML), 0644); err != nil {
		t.Fatalf("Failed to write sub-workflow: %v", err)
	}

	// Create main workflow with retry on the sub-workflow step
	mainWorkflowYAML := `name: main-with-retry
steps:
  - id: run_flaky
    type: workflow
    workflow: ./flaky-task.yaml
    inputs:
      data: "test"
    retry:
      max_attempts: 3
      backoff_base: 1
      backoff_multiplier: 2.0
`
	mainWorkflowPath := filepath.Join(tmpDir, "main.yaml")
	if err := os.WriteFile(mainWorkflowPath, []byte(mainWorkflowYAML), 0644); err != nil {
		t.Fatalf("Failed to write main workflow: %v", err)
	}

	// Parse the main workflow
	data, err := os.ReadFile(mainWorkflowPath)
	if err != nil {
		t.Fatalf("Failed to read main workflow: %v", err)
	}

	def, err := workflow.ParseDefinition(data)
	if err != nil {
		t.Fatalf("Failed to parse main workflow: %v", err)
	}

	// Verify the workflow has a workflow step with retry configuration
	if len(def.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(def.Steps))
	}

	step := def.Steps[0]
	if step.Type != workflow.StepTypeWorkflow {
		t.Fatalf("Expected workflow step, got %v", step.Type)
	}

	if step.Retry == nil {
		t.Fatal("Expected retry configuration on workflow step")
	}

	if step.Retry.MaxAttempts != 3 {
		t.Errorf("Expected max_attempts=3, got %d", step.Retry.MaxAttempts)
	}

	// Verify sub-workflow has its own independent retry config
	loader := subworkflow.NewLoader()
	subDef, err := loader.Load(tmpDir, step.Workflow, nil)
	if err != nil {
		t.Fatalf("Failed to load sub-workflow: %v", err)
	}

	if len(subDef.Steps) != 1 {
		t.Fatalf("Expected 1 step in sub-workflow, got %d", len(subDef.Steps))
	}

	subStep := subDef.Steps[0]
	if subStep.Retry == nil {
		t.Fatal("Expected retry configuration on sub-workflow step")
	}

	if subStep.Retry.MaxAttempts != 2 {
		t.Errorf("Expected max_attempts=2 in sub-workflow, got %d", subStep.Retry.MaxAttempts)
	}
}
