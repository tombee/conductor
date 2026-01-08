//go:build smoke

package e2e

import (
	"testing"

	"github.com/tombee/conductor/internal/testing/integration"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/sdk"
	"github.com/tombee/conductor/test/e2e/harness"
)

// Global cost tracker shared across all smoke tests
var costTracker = integration.NewCostTracker()

// smokeTestRunner encapsulates common smoke test setup and tracking.
type smokeTestRunner struct {
	t        *testing.T
	provider llm.Provider
	tracker  *integration.CostTracker
}

// newSmokeTest creates a new smoke test runner with the given provider.
func newSmokeTest(t *testing.T, provider llm.Provider) *smokeTestRunner {
	t.Helper()

	// Reset test token counter
	costTracker.ResetTest()

	return &smokeTestRunner{
		t:        t,
		provider: provider,
		tracker:  costTracker,
	}
}

// run executes a workflow and tracks token usage.
func (s *smokeTestRunner) run(wf *sdk.Workflow, inputs map[string]any) *sdk.Result {
	s.t.Helper()

	// Create harness with the provider
	h := harness.New(s.t,
		harness.WithProvider(s.provider),
		harness.WithEventCapture(),
	)

	// Run workflow
	result := h.Run(wf, inputs)

	// Convert SDK UsageStats to llm.TokenUsage for cost tracker
	usage := llm.TokenUsage{
		InputTokens:  result.Usage.InputTokens,
		OutputTokens: result.Usage.OutputTokens,
		TotalTokens:  result.Usage.TotalTokens,
	}

	// Record token usage
	if err := s.tracker.Record(usage); err != nil {
		s.t.Fatalf("Token budget exceeded: %v", err)
	}

	// Log token usage for visibility
	s.t.Logf("Tokens used: %d input + %d output = %d total (test total: %d, suite total: %d)",
		result.Usage.InputTokens,
		result.Usage.OutputTokens,
		result.Usage.TotalTokens,
		s.tracker.GetTestTokens(),
		s.tracker.GetSuiteTokens(),
	)

	return result
}

// assertSuccess verifies the workflow completed successfully.
func (s *smokeTestRunner) assertSuccess(result *sdk.Result) {
	s.t.Helper()

	if !result.Success {
		s.t.Fatalf("workflow failed: %v", result.Error)
	}

	if result.Output == nil || len(result.Output) == 0 {
		s.t.Error("workflow output is empty")
	}
}

// assertStepCount verifies the expected number of steps executed.
func (s *smokeTestRunner) assertStepCount(result *sdk.Result, expected int) {
	s.t.Helper()

	if len(result.Steps) != expected {
		s.t.Errorf("expected %d steps, got %d", expected, len(result.Steps))
	}
}

// assertOutputContains verifies the workflow output contains expected content.
func (s *smokeTestRunner) assertOutputContains(result *sdk.Result, expected string) {
	s.t.Helper()

	// Check default "response" output key
	if response, ok := result.Output["response"].(string); ok {
		if len(response) == 0 {
			s.t.Error("response output is empty")
		}
		s.t.Logf("Response preview: %.100s...", response)
	} else {
		s.t.Error("expected 'response' key in output")
	}
}
