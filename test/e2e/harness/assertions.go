package harness

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tombee/conductor/sdk"
)

// AssertSuccess asserts that the workflow completed successfully.
// Fails the test if the result indicates an error or failure status.
func (h *Harness) AssertSuccess(t *testing.T, result *sdk.Result) {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}

	if !result.Success {
		t.Errorf("expected workflow to succeed, but it failed")
	}

	if result.Error != nil {
		t.Errorf("expected no error, got: %v", result.Error)
	}
}

// AssertError asserts that the workflow failed with an error containing the expected string.
// Fails the test if the workflow succeeded or the error doesn't match.
func (h *Harness) AssertError(t *testing.T, result *sdk.Result, errContains string) {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Success {
		t.Error("expected workflow to fail, but it succeeded")
	}

	if result.Error == nil {
		t.Errorf("expected error containing %q, got no error", errContains)
		return
	}

	if !strings.Contains(result.Error.Error(), errContains) {
		t.Errorf("expected error to contain %q, got: %v", errContains, result.Error)
	}
}

// AssertStepOutput asserts that a specific step produced output containing the expected string.
// Fails the test if the step doesn't exist or the output doesn't match.
func (h *Harness) AssertStepOutput(t *testing.T, result *sdk.Result, stepID string, contains string) {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}

	stepResult, exists := result.Steps[stepID]
	if !exists {
		t.Fatalf("step %q not found in results (available steps: %v)", stepID, getStepIDs(result))
	}

	// Check for content in the output map
	found := false
	var outputStr string

	// Try common output keys
	for _, key := range []string{"content", "result", "output", "text", "response"} {
		if val, ok := stepResult.Output[key]; ok {
			outputStr = fmt.Sprintf("%v", val)
			if strings.Contains(outputStr, contains) {
				found = true
				break
			}
		}
	}

	// If not found in known keys, check all values
	if !found {
		for _, val := range stepResult.Output {
			valStr := fmt.Sprintf("%v", val)
			if strings.Contains(valStr, contains) {
				found = true
				outputStr = valStr
				break
			}
		}
	}

	if !found {
		t.Errorf("step %q output does not contain %q\nGot output: %v", stepID, contains, stepResult.Output)
	}
}

// AssertStepStatus asserts that a step has the expected status.
// Fails the test if the step doesn't exist or has a different status.
func (h *Harness) AssertStepStatus(t *testing.T, result *sdk.Result, stepID string, expectedStatus sdk.StepStatus) {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}

	stepResult, exists := result.Steps[stepID]
	if !exists {
		t.Fatalf("step %q not found in results (available steps: %v)", stepID, getStepIDs(result))
	}

	if stepResult.Status != expectedStatus {
		t.Errorf("step %q expected status %q, got %q", stepID, expectedStatus, stepResult.Status)
	}
}

// AssertTokenUsage asserts that the total token usage falls within the expected range.
// Fails the test if token usage is outside [min, max].
// Use -1 for min to skip minimum check, -1 for max to skip maximum check.
func (h *Harness) AssertTokenUsage(t *testing.T, result *sdk.Result, min, max int) {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}

	total := result.Usage.TotalTokens

	if min >= 0 && total < min {
		t.Errorf("expected at least %d tokens, got %d", min, total)
	}

	if max >= 0 && total > max {
		t.Errorf("expected at most %d tokens, got %d", max, total)
	}
}

// AssertStepCount asserts that the workflow executed the expected number of steps.
// Fails the test if the step count doesn't match.
func (h *Harness) AssertStepCount(t *testing.T, result *sdk.Result, expectedCount int) {
	t.Helper()

	if result == nil {
		t.Fatal("result is nil")
	}

	actualCount := len(result.Steps)
	if actualCount != expectedCount {
		t.Errorf("expected %d steps, got %d (steps: %v)", expectedCount, actualCount, getStepIDs(result))
	}
}

// AssertEventCount asserts that the expected number of events were captured.
// Requires WithEventCapture() option to be set on the harness.
func (h *Harness) AssertEventCount(t *testing.T, eventType sdk.EventType, expectedCount int) {
	t.Helper()

	if !h.captureEvents {
		t.Fatal("event capture not enabled (use WithEventCapture option)")
	}

	actualCount := 0
	for _, event := range h.events {
		if event.Type == eventType {
			actualCount++
		}
	}

	if actualCount != expectedCount {
		t.Errorf("expected %d %q events, got %d", expectedCount, eventType, actualCount)
	}
}

// getStepIDs returns a slice of step IDs from the result for better error messages.
func getStepIDs(result *sdk.Result) []string {
	if result == nil || result.Steps == nil {
		return nil
	}

	ids := make([]string, 0, len(result.Steps))
	for id := range result.Steps {
		ids = append(ids, id)
	}
	return ids
}
