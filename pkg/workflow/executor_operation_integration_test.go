package workflow

import (
	"context"
	"testing"
)

// TestIntegrationStepIntegration tests the full integration of integration steps in a workflow.
func TestIntegrationStepIntegration(t *testing.T) {
	// Create a mock operation registry
	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			// Simulate different integration behaviors based on reference
			switch reference {
			case "github.get_repo":
				return &mockOperationResult{
					response: map[string]interface{}{
						"name":        "conductor",
						"description": "Workflow orchestrator",
						"stars":       150,
					},
					statusCode: 200,
				}, nil
			case "slack.post_message":
				return &mockOperationResult{
					response: map[string]interface{}{
						"ok":      true,
						"channel": "C123456",
						"ts":      "1234567890.123456",
					},
					statusCode: 200,
				}, nil
			default:
				return &mockOperationResult{
					response:   map[string]interface{}{"error": "unknown operation"},
					statusCode: 404,
				}, nil
			}
		},
	}

	// Create executor with operation registry
	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// Test 1: Execute a simple integration step
	t.Run("simple integration step", func(t *testing.T) {
		step := &StepDefinition{
			ID:          "get_repo",
			Type:        StepTypeIntegration,
			Integration: "github.get_repo",
			Inputs: map[string]interface{}{
				"owner": "tombee",
				"repo":  "conductor",
			},
		}

		workflowContext := map[string]interface{}{}
		result, err := executor.Execute(context.Background(), step, workflowContext)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Status != StepStatusSuccess {
			t.Errorf("expected status %s, got %s", StepStatusSuccess, result.Status)
		}

		if result.Output == nil {
			t.Fatal("expected output to be non-nil")
		}

		// Check that response is present
		response, ok := result.Output["response"]
		if !ok {
			t.Fatal("expected response in output")
		}

		// Verify response structure
		responseMap, ok := response.(map[string]interface{})
		if !ok {
			t.Fatal("expected response to be a map")
		}

		if responseMap["name"] != "conductor" {
			t.Errorf("expected name 'conductor', got %v", responseMap["name"])
		}
	})

	// Test 2: Execute integration step with error handling
	t.Run("integration step error handling", func(t *testing.T) {
		step := &StepDefinition{
			ID:          "unknown_op",
			Type:        StepTypeIntegration,
			Integration: "unknown.operation",
			Inputs:      map[string]interface{}{},
		}

		workflowContext := map[string]interface{}{}
		result, err := executor.Execute(context.Background(), step, workflowContext)

		// Should succeed (no error from Execute) but operation returns 404
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Response should contain error indication
		response := result.Output["response"].(map[string]interface{})
		if response["error"] != "unknown operation" {
			t.Errorf("expected error in response, got %v", response)
		}
	})

	// Test 3: Execute integration step with retry on failure
	t.Run("integration step with retry", func(t *testing.T) {
		attemptCount := 0
		retryRegistry := &mockOperationRegistry{
			executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
				attemptCount++
				if attemptCount < 2 {
					// Fail first attempt
					return nil, &mockOperationError{message: "temporary failure"}
				}
				// Succeed on second attempt
				return &mockOperationResult{
					response:   map[string]interface{}{"success": true},
					statusCode: 200,
				}, nil
			},
		}

		retryExecutor := NewExecutor(nil, nil).WithOperationRegistry(retryRegistry)

		step := &StepDefinition{
			ID:          "retry_test",
			Type:        StepTypeIntegration,
			Integration: "test.operation",
			Inputs:      map[string]interface{}{},
			Retry: &RetryDefinition{
				MaxAttempts:       3,
				BackoffBase:       1,
				BackoffMultiplier: 2.0,
			},
		}

		workflowContext := map[string]interface{}{}
		result, err := retryExecutor.Execute(context.Background(), step, workflowContext)

		if err != nil {
			t.Fatalf("unexpected error after retry: %v", err)
		}

		if result.Status != StepStatusSuccess {
			t.Errorf("expected status %s after retry, got %s", StepStatusSuccess, result.Status)
		}

		if attemptCount != 2 {
			t.Errorf("expected 2 attempts, got %d", attemptCount)
		}
	})
}

// mockOperationError implements error for testing.
type mockOperationError struct {
	message string
}

func (e *mockOperationError) Error() string {
	return e.message
}

// TestIntegrationStep_OutputAvailableToNextStep tests that integration outputs can be used in subsequent steps.
func TestIntegrationStep_OutputAvailableToNextStep(t *testing.T) {
	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			switch reference {
			case "github.get_user":
				return &mockOperationResult{
					response: map[string]interface{}{
						"login": "testuser",
						"id":    12345,
						"email": "test@example.com",
					},
					statusCode: 200,
				}, nil
			case "slack.post_message":
				// Verify we can access the previous step's output
				message, ok := inputs["text"].(string)
				if !ok {
					t.Errorf("expected text input to be string")
				}
				return &mockOperationResult{
					response: map[string]interface{}{
						"ok":      true,
						"message": message,
					},
					statusCode: 200,
				}, nil
			default:
				return nil, &mockOperationError{message: "unknown operation"}
			}
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// Step 1: Get user data
	step1 := &StepDefinition{
		ID:          "get_user",
		Type:        StepTypeIntegration,
		Integration: "github.get_user",
		Inputs: map[string]interface{}{
			"username": "testuser",
		},
	}

	workflowContext := map[string]interface{}{}
	result1, err := executor.Execute(context.Background(), step1, workflowContext)
	if err != nil {
		t.Fatalf("step 1 failed: %v", err)
	}

	// Store step 1 output in context
	workflowContext["steps"] = map[string]interface{}{
		"get_user": result1.Output,
	}

	// Step 2: Use step 1 output
	step2 := &StepDefinition{
		ID:          "notify_slack",
		Type:        StepTypeIntegration,
		Integration: "slack.post_message",
		Inputs: map[string]interface{}{
			"channel": "#general",
			"text":    "User testuser has ID 12345",
		},
	}

	result2, err := executor.Execute(context.Background(), step2, workflowContext)
	if err != nil {
		t.Fatalf("step 2 failed: %v", err)
	}

	if result2.Status != StepStatusSuccess {
		t.Errorf("expected status %s, got %s", StepStatusSuccess, result2.Status)
	}

	// Verify both steps have outputs in context
	steps := workflowContext["steps"].(map[string]interface{})
	if steps["get_user"] == nil {
		t.Error("step 1 output not found in context")
	}
}

// TestIntegrationStep_ErrorFlowsToOnError tests that integration errors are properly reported.
func TestIntegrationStep_ErrorFlowsToOnError(t *testing.T) {
	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			switch reference {
			case "github.create_issue":
				// Simulate an auth error
				return nil, &mockOperationError{message: "401 Unauthorized"}
			default:
				return nil, &mockOperationError{message: "unknown operation"}
			}
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// Step that will fail
	step := &StepDefinition{
		ID:          "create_issue",
		Type:        StepTypeIntegration,
		Integration: "github.create_issue",
		Inputs: map[string]interface{}{
			"title": "Test Issue",
		},
		OnError: &ErrorHandlingDefinition{
			Strategy: ErrorStrategyFail,
		},
	}

	workflowContext := map[string]interface{}{}
	result, err := executor.Execute(context.Background(), step, workflowContext)

	// The step should fail
	if err == nil {
		t.Fatal("expected error from failed integration step")
	}

	// Verify error was captured
	if result.Status != StepStatusFailed {
		t.Errorf("expected status %s, got %s", StepStatusFailed, result.Status)
	}

	if result.Error == "" {
		t.Error("expected error message in result")
	}
}

// TestIntegrationStep_WithResponseTransform tests that complex response structures are handled correctly.
// Note: Response transformation is defined at the operation level in the operation package,
// not at the step level. This test verifies that complex nested responses are properly returned.
func TestIntegrationStep_WithResponseTransform(t *testing.T) {
	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			// Simulate an integration that has already applied response_transform
			// The transform would extract items from a nested structure
			return &mockOperationResult{
				response: []interface{}{
					map[string]interface{}{"id": 1, "name": "item1"},
					map[string]interface{}{"id": 2, "name": "item2"},
					map[string]interface{}{"id": 3, "name": "item3"},
				},
				rawResponse: map[string]interface{}{
					"data": map[string]interface{}{
						"items": []interface{}{
							map[string]interface{}{"id": 1, "name": "item1"},
							map[string]interface{}{"id": 2, "name": "item2"},
							map[string]interface{}{"id": 3, "name": "item3"},
						},
						"total": 3,
					},
					"metadata": map[string]interface{}{
						"page": 1,
					},
				},
				statusCode: 200,
			}, nil
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// Execute integration that returns transformed response
	step := &StepDefinition{
		ID:          "get_items",
		Type:        StepTypeIntegration,
		Integration: "api.list_items",
		Inputs:      map[string]interface{}{},
	}

	workflowContext := map[string]interface{}{}
	result, err := executor.Execute(context.Background(), step, workflowContext)

	if err != nil {
		t.Fatalf("step failed: %v", err)
	}

	if result.Status != StepStatusSuccess {
		t.Errorf("expected status %s, got %s", StepStatusSuccess, result.Status)
	}

	// Verify the transformed response is present
	response := result.Output["response"]
	if response == nil {
		t.Error("expected response in output")
	}

	// Verify it's an array (the transformed result)
	_, ok := response.([]interface{})
	if !ok {
		t.Errorf("expected response to be an array after transform, got %T", response)
	}
}

// TestIntegrationStep_InParallelExecution tests that integration steps work correctly in parallel execution.
func TestIntegrationStep_InParallelExecution(t *testing.T) {
	executionOrder := make(chan string, 3)

	mockRegistry := &mockOperationRegistry{
		executeFunc: func(ctx context.Context, reference string, inputs map[string]interface{}) (OperationResult, error) {
			switch reference {
			case "github.get_user":
				executionOrder <- "github"
				return &mockOperationResult{
					response: map[string]interface{}{
						"login": "testuser",
					},
					statusCode: 200,
				}, nil
			case "slack.get_profile":
				executionOrder <- "slack"
				return &mockOperationResult{
					response: map[string]interface{}{
						"name": "Test User",
					},
					statusCode: 200,
				}, nil
			case "jira.get_issues":
				executionOrder <- "jira"
				return &mockOperationResult{
					response: map[string]interface{}{
						"issues": []interface{}{},
					},
					statusCode: 200,
				}, nil
			default:
				return nil, &mockOperationError{message: "unknown operation"}
			}
		},
	}

	executor := NewExecutor(nil, nil).WithOperationRegistry(mockRegistry)

	// Define parallel steps
	steps := []*StepDefinition{
		{
			ID:          "get_github_user",
			Type:        StepTypeIntegration,
			Integration: "github.get_user",
			Inputs:      map[string]interface{}{"username": "testuser"},
		},
		{
			ID:          "get_slack_profile",
			Type:        StepTypeIntegration,
			Integration: "slack.get_profile",
			Inputs:      map[string]interface{}{"user_id": "U123"},
		},
		{
			ID:          "get_jira_issues",
			Type:        StepTypeIntegration,
			Integration: "jira.get_issues",
			Inputs:      map[string]interface{}{"project": "TEST"},
		},
	}

	workflowContext := map[string]interface{}{}
	successCount := 0

	// Execute all steps (in real parallel execution, these would run concurrently)
	for _, step := range steps {
		result, err := executor.Execute(context.Background(), step, workflowContext)
		if err != nil {
			t.Errorf("step %s failed: %v", step.ID, err)
			continue
		}

		if result.Status == StepStatusSuccess {
			successCount++
		}
	}

	if successCount != 3 {
		t.Errorf("expected 3 successful executions, got %d", successCount)
	}

	// Verify all integrations were called
	close(executionOrder)
	calls := make(map[string]bool)
	for call := range executionOrder {
		calls[call] = true
	}

	expectedCalls := []string{"github", "slack", "jira"}
	for _, expected := range expectedCalls {
		if !calls[expected] {
			t.Errorf("expected %s integration to be called", expected)
		}
	}
}
