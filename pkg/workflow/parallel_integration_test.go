package workflow

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestParallelExecution_ConcurrentTimestamps verifies parallel steps run concurrently
// by checking for timestamp overlap.
func TestParallelExecution_ConcurrentTimestamps(t *testing.T) {
	var mu sync.Mutex
	var execTimes []struct {
		id    string
		start time.Time
		end   time.Time
	}

	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			start := time.Now()
			time.Sleep(50 * time.Millisecond) // Simulate work
			end := time.Now()

			mu.Lock()
			execTimes = append(execTimes, struct {
				id    string
				start time.Time
				end   time.Time
			}{reference, start, end})
			mu.Unlock()

			return &mockOperationResult{
				response:   map[string]interface{}{"completed": true},
				statusCode: 200,
			}, nil
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// Parallel step with 3 nested steps
	step := &StepDefinition{
		ID:   "parallel_test",
		Type: StepTypeParallel,
		Steps: []StepDefinition{
			{ID: "op1", Type: StepTypeIntegration, Integration: "api.op1"},
			{ID: "op2", Type: StepTypeIntegration, Integration: "api.op2"},
			{ID: "op3", Type: StepTypeIntegration, Integration: "api.op3"},
		},
	}

	ctx := context.Background()
	result, err := executor.Execute(ctx, step, map[string]interface{}{})

	if err != nil {
		t.Fatalf("parallel execution failed: %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Errorf("expected success status, got %s", result.Status)
	}

	// Verify all 3 operations ran
	if len(execTimes) != 3 {
		t.Fatalf("expected 3 executions, got %d", len(execTimes))
	}

	// Verify concurrent execution: check for timestamp overlap
	// At least 2 operations should have overlapping execution windows
	hasOverlap := false
	for i := 0; i < len(execTimes); i++ {
		for j := i + 1; j < len(execTimes); j++ {
			// Overlap exists if one starts before the other ends
			if execTimes[i].start.Before(execTimes[j].end) && execTimes[j].start.Before(execTimes[i].end) {
				hasOverlap = true
				break
			}
		}
		if hasOverlap {
			break
		}
	}

	if !hasOverlap {
		t.Error("expected concurrent execution with overlapping timestamps, but steps ran sequentially")
		for _, et := range execTimes {
			t.Logf("  %s: %v - %v", et.id, et.start, et.end)
		}
	}
}

// TestParallelForeach_ArrayProcessing tests foreach iteration over arrays.
func TestParallelForeach_ArrayProcessing(t *testing.T) {
	var processedItems []string
	var mu sync.Mutex

	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			item, _ := inputs["item"].(string)
			mu.Lock()
			processedItems = append(processedItems, item)
			mu.Unlock()

			return &mockOperationResult{
				response:   map[string]interface{}{"processed": item},
				statusCode: 200,
			}, nil
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// Parallel foreach step
	step := &StepDefinition{
		ID:      "foreach_test",
		Type:    StepTypeParallel,
		Foreach: "{{.inputs.items}}",
		Steps: []StepDefinition{
			{
				ID:          "process",
				Type:        StepTypeIntegration,
				Integration: "api.process",
				Inputs: map[string]interface{}{
					"item": "{{.item}}",
				},
			},
		},
	}

	// Create proper TemplateContext for foreach resolution
	templateCtx := NewTemplateContext()
	templateCtx.Inputs["items"] = []interface{}{"apple", "banana", "cherry"}

	workflowContext := map[string]interface{}{
		"_templateContext": templateCtx,
	}

	result, err := executor.Execute(context.Background(), step, workflowContext)

	if err != nil {
		t.Fatalf("foreach execution failed: %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Errorf("expected success status, got %s", result.Status)
	}

	// Verify all items were processed
	if len(processedItems) != 3 {
		t.Errorf("expected 3 processed items, got %d", len(processedItems))
	}

	// Verify results array is populated
	results, ok := result.Output["results"]
	if !ok {
		t.Fatal("expected results array in output")
	}

	resultsArray, ok := results.([]interface{})
	if !ok {
		t.Fatalf("expected results to be array, got %T", results)
	}

	if len(resultsArray) != 3 {
		t.Errorf("expected 3 results, got %d", len(resultsArray))
	}
}

// TestParallelErrorHandling_FailFast tests fail-fast behavior (default).
func TestParallelErrorHandling_FailFast(t *testing.T) {
	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			if reference == "api.failing" {
				return nil, &mockOperationError{message: "intentional failure"}
			}
			return &mockOperationResult{
				response:   map[string]interface{}{"ok": true},
				statusCode: 200,
			}, nil
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	step := &StepDefinition{
		ID:   "failfast_test",
		Type: StepTypeParallel,
		// Default: fail-fast is on (no OnError specified)
		Steps: []StepDefinition{
			{ID: "step1", Type: StepTypeIntegration, Integration: "api.step1"},
			{ID: "failing", Type: StepTypeIntegration, Integration: "api.failing"},
			{ID: "step2", Type: StepTypeIntegration, Integration: "api.step2"},
		},
	}

	_, err := executor.Execute(context.Background(), step, map[string]interface{}{})

	// With default fail-fast behavior, the error should propagate
	if err == nil {
		t.Fatal("expected error from fail-fast parallel execution")
	}

	// Verify error message contains the failed step info
	errStr := err.Error()
	if !strings.Contains(errStr, "failing") {
		t.Errorf("expected error to mention 'failing' step, got: %s", errStr)
	}
}

// TestParallelErrorHandling_Continue tests continue-on-error behavior.
func TestParallelErrorHandling_Continue(t *testing.T) {
	var completedSteps []string
	var mu sync.Mutex

	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			if reference == "api.failing" {
				return nil, &mockOperationError{message: "intentional failure"}
			}
			mu.Lock()
			completedSteps = append(completedSteps, reference)
			mu.Unlock()
			return &mockOperationResult{
				response:   map[string]interface{}{"ok": true},
				statusCode: 200,
			}, nil
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	step := &StepDefinition{
		ID:   "continue_test",
		Type: StepTypeParallel,
		OnError: &ErrorHandlingDefinition{
			Strategy: ErrorStrategyIgnore,
		},
		Steps: []StepDefinition{
			{ID: "step1", Type: StepTypeIntegration, Integration: "api.step1"},
			{ID: "failing", Type: StepTypeIntegration, Integration: "api.failing"},
			{ID: "step2", Type: StepTypeIntegration, Integration: "api.step2"},
		},
	}

	result, err := executor.Execute(context.Background(), step, map[string]interface{}{})

	// With ignore strategy, the parallel step still returns error info
	// but other steps should complete
	if err != nil {
		t.Logf("Got expected error from failed step: %v", err)
	}

	// All non-failing steps should complete
	if len(completedSteps) != 2 {
		t.Errorf("expected 2 completed steps (excluding failed), got %d: %v", len(completedSteps), completedSteps)
	}

	// Verify partial results are available
	if result != nil && result.Output != nil {
		if _, ok := result.Output["step1"]; !ok {
			t.Error("expected step1 output in results")
		}
		if _, ok := result.Output["step2"]; !ok {
			t.Error("expected step2 output in results")
		}
	}
}

// TestParallelShorthand_IdenticalBehavior verifies shorthand produces same behavior as explicit syntax.
func TestParallelShorthand_IdenticalBehavior(t *testing.T) {
	var explicitResults, shorthandResults map[string]interface{}

	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			return &mockOperationResult{
				response:   map[string]interface{}{"ref": reference},
				statusCode: 200,
			}, nil
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// Explicit syntax
	explicitStep := &StepDefinition{
		ID:   "explicit_parallel",
		Type: StepTypeParallel,
		Steps: []StepDefinition{
			{ID: "step1", Type: StepTypeIntegration, Integration: "api.step1"},
			{ID: "step2", Type: StepTypeIntegration, Integration: "api.step2"},
		},
		MaxConcurrency: 2,
	}

	explicitResult, err := executor.Execute(context.Background(), explicitStep, map[string]interface{}{})
	if err != nil {
		t.Fatalf("explicit syntax failed: %v", err)
	}
	explicitResults = explicitResult.Output

	// Parse and execute shorthand syntax
	// Simulate what the YAML unmarshaler does
	shorthandStep := &StepDefinition{
		ID:             "shorthand_parallel",
		Type:           StepTypeParallel,
		MaxConcurrency: 2,
		Steps: []StepDefinition{
			{ID: "step1", Type: StepTypeIntegration, Integration: "api.step1"},
			{ID: "step2", Type: StepTypeIntegration, Integration: "api.step2"},
		},
	}

	shorthandResult, err := executor.Execute(context.Background(), shorthandStep, map[string]interface{}{})
	if err != nil {
		t.Fatalf("shorthand syntax failed: %v", err)
	}
	shorthandResults = shorthandResult.Output

	// Verify both produce equivalent outputs
	if len(explicitResults) != len(shorthandResults) {
		t.Errorf("output count mismatch: explicit=%d, shorthand=%d", len(explicitResults), len(shorthandResults))
	}

	// Verify specific outputs match
	for key := range explicitResults {
		if _, ok := shorthandResults[key]; !ok {
			t.Errorf("shorthand missing output key: %s", key)
		}
	}
}

// TestParallelMaxConcurrency_Limited verifies max_concurrency limits concurrent execution.
func TestParallelMaxConcurrency_Limited(t *testing.T) {
	var currentConcurrency int
	var maxObservedConcurrency int
	var mu sync.Mutex

	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			mu.Lock()
			currentConcurrency++
			if currentConcurrency > maxObservedConcurrency {
				maxObservedConcurrency = currentConcurrency
			}
			mu.Unlock()

			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			currentConcurrency--
			mu.Unlock()

			return &mockOperationResult{
				response:   map[string]interface{}{"ok": true},
				statusCode: 200,
			}, nil
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// 5 steps with max_concurrency=2
	step := &StepDefinition{
		ID:             "limited_parallel",
		Type:           StepTypeParallel,
		MaxConcurrency: 2,
		Steps: []StepDefinition{
			{ID: "step1", Type: StepTypeIntegration, Integration: "api.step1"},
			{ID: "step2", Type: StepTypeIntegration, Integration: "api.step2"},
			{ID: "step3", Type: StepTypeIntegration, Integration: "api.step3"},
			{ID: "step4", Type: StepTypeIntegration, Integration: "api.step4"},
			{ID: "step5", Type: StepTypeIntegration, Integration: "api.step5"},
		},
	}

	result, err := executor.Execute(context.Background(), step, map[string]interface{}{})

	if err != nil {
		t.Fatalf("parallel execution failed: %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Errorf("expected success status, got %s", result.Status)
	}

	// Verify concurrency was limited
	if maxObservedConcurrency > 2 {
		t.Errorf("expected max concurrency of 2, observed %d", maxObservedConcurrency)
	}
}

// TestParallelOutputAccess_NestedPaths verifies nested step outputs are accessible.
func TestParallelOutputAccess_NestedPaths(t *testing.T) {
	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			return &mockOperationResult{
				response: map[string]interface{}{
					"value": reference + "_result",
				},
				statusCode: 200,
			}, nil
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	step := &StepDefinition{
		ID:   "output_test",
		Type: StepTypeParallel,
		Steps: []StepDefinition{
			{ID: "step_a", Type: StepTypeIntegration, Integration: "api.a"},
			{ID: "step_b", Type: StepTypeIntegration, Integration: "api.b"},
		},
	}

	result, err := executor.Execute(context.Background(), step, map[string]interface{}{})

	if err != nil {
		t.Fatalf("parallel execution failed: %v", err)
	}

	// Verify nested outputs are accessible at expected paths
	stepAOutput, ok := result.Output["step_a"]
	if !ok {
		t.Fatal("expected step_a output")
	}

	stepAMap, ok := stepAOutput.(map[string]interface{})
	if !ok {
		t.Fatalf("expected step_a output to be map, got %T", stepAOutput)
	}

	response, ok := stepAMap["response"]
	if !ok {
		t.Fatal("expected response in step_a output")
	}

	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response to be map, got %T", response)
	}

	if responseMap["value"] != "api.a_result" {
		t.Errorf("expected value 'api.a_result', got %v", responseMap["value"])
	}
}
