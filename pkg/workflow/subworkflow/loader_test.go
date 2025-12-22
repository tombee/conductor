package subworkflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoader_LoadSimple tests loading a simple sub-workflow.
func TestLoader_LoadSimple(t *testing.T) {
	// Create a temporary directory with a workflow file
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "test.yaml")

	workflowContent := `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("failed to write test workflow: %v", err)
	}

	// Load the workflow
	loader := NewLoader()
	def, err := loader.Load(tmpDir, "test.yaml", nil)
	if err != nil {
		t.Fatalf("failed to load workflow: %v", err)
	}

	// Verify the workflow was loaded correctly
	if def.Name != "test-workflow" {
		t.Errorf("expected workflow name 'test-workflow', got %q", def.Name)
	}
	if len(def.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(def.Steps))
	}
}

// TestLoader_LoadWithRelativePath tests loading with a relative path.
func TestLoader_LoadWithRelativePath(t *testing.T) {
	// Create a temporary directory with subdirectories
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "helpers")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	workflowPath := filepath.Join(subDir, "helper.yaml")
	workflowContent := `
name: helper-workflow
steps:
  - id: step1
    type: llm
    prompt: "helper"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("failed to write test workflow: %v", err)
	}

	// Load the workflow with relative path
	loader := NewLoader()
	def, err := loader.Load(tmpDir, "helpers/helper.yaml", nil)
	if err != nil {
		t.Fatalf("failed to load workflow: %v", err)
	}

	if def.Name != "helper-workflow" {
		t.Errorf("expected workflow name 'helper-workflow', got %q", def.Name)
	}
}

// TestLoader_RejectAbsolutePath tests that absolute paths are rejected.
func TestLoader_RejectAbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader()

	_, err := loader.Load(tmpDir, "/etc/passwd", nil)
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("expected error message to mention 'absolute', got: %v", err)
	}
}

// TestLoader_RejectPathTraversal tests that path traversal attempts are rejected.
func TestLoader_RejectPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader()

	testCases := []string{
		"../../../etc/passwd",
		"helpers/../../etc/passwd",
		"./../../etc/passwd",
	}

	for _, path := range testCases {
		t.Run(path, func(t *testing.T) {
			_, err := loader.Load(tmpDir, path, nil)
			if err == nil {
				t.Fatal("expected error for path traversal, got nil")
			}
		})
	}
}

// TestLoader_RejectNonexistent tests that nonexistent files return an error.
func TestLoader_RejectNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader()

	_, err := loader.Load(tmpDir, "nonexistent.yaml", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("expected error message to mention 'failed to read', got: %v", err)
	}
}

// TestLoader_DetectRecursion tests that direct recursion is detected.
func TestLoader_DetectRecursion(t *testing.T) {
	// Create two workflows that reference each other
	tmpDir := t.TempDir()

	workflow1 := `
name: workflow1
steps:
  - id: call2
    type: workflow
    workflow: ./workflow2.yaml
`
	workflow2 := `
name: workflow2
steps:
  - id: call1
    type: workflow
    workflow: ./workflow1.yaml
`

	if err := os.WriteFile(filepath.Join(tmpDir, "workflow1.yaml"), []byte(workflow1), 0644); err != nil {
		t.Fatalf("failed to write workflow1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow2.yaml"), []byte(workflow2), 0644); err != nil {
		t.Fatalf("failed to write workflow2: %v", err)
	}

	// Try to load workflow1 (which will try to load workflow2, which tries to load workflow1)
	loader := NewLoader()
	_, err := loader.Load(tmpDir, "workflow1.yaml", nil)
	if err == nil {
		t.Fatal("expected error for recursion, got nil")
	}
	if !strings.Contains(err.Error(), "recursion detected") {
		t.Errorf("expected error message to mention 'recursion detected', got: %v", err)
	}
}

// TestLoader_DetectSelfRecursion tests that a workflow calling itself is detected.
func TestLoader_DetectSelfRecursion(t *testing.T) {
	tmpDir := t.TempDir()

	workflow := `
name: recursive
steps:
  - id: call_self
    type: workflow
    workflow: ./recursive.yaml
`

	if err := os.WriteFile(filepath.Join(tmpDir, "recursive.yaml"), []byte(workflow), 0644); err != nil {
		t.Fatalf("failed to write workflow: %v", err)
	}

	loader := NewLoader()
	_, err := loader.Load(tmpDir, "recursive.yaml", nil)
	if err == nil {
		t.Fatal("expected error for self-recursion, got nil")
	}
	if !strings.Contains(err.Error(), "recursion detected") {
		t.Errorf("expected error message to mention 'recursion detected', got: %v", err)
	}
}

// TestLoader_EnforceDepthLimit tests that the maximum nesting depth is enforced.
func TestLoader_EnforceDepthLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a chain of workflows exceeding the depth limit
	// workflow1 -> workflow2 -> ... -> workflow6 (depth 6, exceeds limit of 5)
	for i := 1; i <= 6; i++ {
		var content string
		if i == 6 {
			// Final workflow with no sub-workflow call
			content = `
name: workflow6
steps:
  - id: final
    type: llm
    prompt: "done"
`
		} else {
			// Workflow that calls the next one
			content = `
name: workflow` + string(rune('0'+i)) + `
steps:
  - id: call_next
    type: workflow
    workflow: ./workflow` + string(rune('0'+i+1)) + `.yaml
`
		}
		filename := filepath.Join(tmpDir, "workflow"+string(rune('0'+i))+".yaml")
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write workflow%d: %v", i, err)
		}
	}

	// Try to load the first workflow
	loader := NewLoader()
	_, err := loader.Load(tmpDir, "workflow1.yaml", nil)
	if err == nil {
		t.Fatal("expected error for depth limit exceeded, got nil")
	}
	if !strings.Contains(err.Error(), "maximum nesting depth") {
		t.Errorf("expected error message to mention 'maximum nesting depth', got: %v", err)
	}
}

// TestLoader_AllowMaxDepth tests that the maximum allowed depth works correctly.
func TestLoader_AllowMaxDepth(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a chain of workflows at exactly the max depth (5)
	for i := 1; i <= 5; i++ {
		var content string
		if i == 5 {
			// Final workflow
			content = `
name: workflow5
steps:
  - id: final
    type: llm
    prompt: "done"
`
		} else {
			// Workflow that calls the next one
			content = `
name: workflow` + string(rune('0'+i)) + `
steps:
  - id: call_next
    type: workflow
    workflow: ./workflow` + string(rune('0'+i+1)) + `.yaml
`
		}
		filename := filepath.Join(tmpDir, "workflow"+string(rune('0'+i))+".yaml")
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write workflow%d: %v", i, err)
		}
	}

	// Try to load the first workflow (should succeed)
	loader := NewLoader()
	def, err := loader.Load(tmpDir, "workflow1.yaml", nil)
	if err != nil {
		t.Fatalf("unexpected error at max depth: %v", err)
	}
	if def.Name != "workflow1" {
		t.Errorf("expected workflow name 'workflow1', got %q", def.Name)
	}
}

// TestLoader_Caching tests that definitions are cached and reused.
func TestLoader_Caching(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "test.yaml")

	workflowContent := `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent), 0644); err != nil {
		t.Fatalf("failed to write test workflow: %v", err)
	}

	loader := NewLoader()

	// Load the workflow first time
	def1, err := loader.Load(tmpDir, "test.yaml", nil)
	if err != nil {
		t.Fatalf("failed to load workflow first time: %v", err)
	}

	// Load the workflow second time (should be cached)
	def2, err := loader.Load(tmpDir, "test.yaml", nil)
	if err != nil {
		t.Fatalf("failed to load workflow second time: %v", err)
	}

	// Verify both definitions are the same object (cached)
	if def1 != def2 {
		t.Error("expected cached definition to be reused")
	}
}

// TestLoader_CacheInvalidation tests that cache is invalidated when file is modified.
func TestLoader_CacheInvalidation(t *testing.T) {
	tmpDir := t.TempDir()
	workflowPath := filepath.Join(tmpDir, "test.yaml")

	workflowContent1 := `
name: test-workflow-v1
steps:
  - id: step1
    type: llm
    prompt: "test"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent1), 0644); err != nil {
		t.Fatalf("failed to write test workflow: %v", err)
	}

	loader := NewLoader()

	// Load the workflow first time
	def1, err := loader.Load(tmpDir, "test.yaml", nil)
	if err != nil {
		t.Fatalf("failed to load workflow first time: %v", err)
	}
	if def1.Name != "test-workflow-v1" {
		t.Errorf("expected workflow name 'test-workflow-v1', got %q", def1.Name)
	}

	// Wait a bit to ensure modification time changes
	time.Sleep(10 * time.Millisecond)

	// Modify the file
	workflowContent2 := `
name: test-workflow-v2
steps:
  - id: step1
    type: llm
    prompt: "test"
`
	if err := os.WriteFile(workflowPath, []byte(workflowContent2), 0644); err != nil {
		t.Fatalf("failed to modify test workflow: %v", err)
	}

	// Load the workflow again (should get new version)
	def2, err := loader.Load(tmpDir, "test.yaml", nil)
	if err != nil {
		t.Fatalf("failed to load workflow second time: %v", err)
	}
	if def2.Name != "test-workflow-v2" {
		t.Errorf("expected workflow name 'test-workflow-v2', got %q", def2.Name)
	}
}

// TestLoader_NestedSubworkflows tests loading workflows with nested sub-workflows.
func TestLoader_NestedSubworkflows(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "helpers")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}

	// Create main workflow
	mainWorkflow := `
name: main
steps:
  - id: call_helper
    type: workflow
    workflow: ./helpers/helper.yaml
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.yaml"), []byte(mainWorkflow), 0644); err != nil {
		t.Fatalf("failed to write main workflow: %v", err)
	}

	// Create helper workflow
	helperWorkflow := `
name: helper
steps:
  - id: call_util
    type: workflow
    workflow: ./util.yaml
`
	if err := os.WriteFile(filepath.Join(subDir, "helper.yaml"), []byte(helperWorkflow), 0644); err != nil {
		t.Fatalf("failed to write helper workflow: %v", err)
	}

	// Create util workflow
	utilWorkflow := `
name: util
steps:
  - id: final
    type: llm
    prompt: "done"
`
	if err := os.WriteFile(filepath.Join(subDir, "util.yaml"), []byte(utilWorkflow), 0644); err != nil {
		t.Fatalf("failed to write util workflow: %v", err)
	}

	// Load the main workflow (should recursively load sub-workflows)
	loader := NewLoader()
	def, err := loader.Load(tmpDir, "main.yaml", nil)
	if err != nil {
		t.Fatalf("failed to load main workflow: %v", err)
	}
	if def.Name != "main" {
		t.Errorf("expected workflow name 'main', got %q", def.Name)
	}
}
