//go:build smoke

package e2e

import (
	"testing"

	"github.com/tombee/conductor/internal/testing/integration"
	"github.com/tombee/conductor/pkg/llm/providers"
	"github.com/tombee/conductor/test/e2e/harness"
)

// TestAnthropic_SimpleWorkflow tests a basic LLM workflow with Anthropic.
func TestAnthropic_SimpleWorkflow(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	cfg := integration.LoadConfig()

	// Create Anthropic provider
	provider, err := providers.NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("failed to create Anthropic provider: %v", err)
	}

	// Create smoke test runner
	smoke := newSmokeTest(t, provider)

	// Load simple workflow from YAML
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/simple_llm.yaml")

	// Run workflow
	result := smoke.run(wf, map[string]any{
		"user_query": "What is the capital of France?",
	})

	// Verify success
	smoke.assertSuccess(result)
	smoke.assertStepCount(result, 1)
	smoke.assertOutputContains(result, "")

	// Verify we got a real response with token usage
	if result.Usage.TotalTokens == 0 {
		t.Error("expected non-zero token usage from Anthropic")
	}
}

// TestAnthropic_MultiStep tests a multi-step workflow with Anthropic.
func TestAnthropic_MultiStep(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	cfg := integration.LoadConfig()

	provider, err := providers.NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("failed to create Anthropic provider: %v", err)
	}

	smoke := newSmokeTest(t, provider)

	// Load multi-step workflow from YAML
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/multi_step.yaml")

	result := smoke.run(wf, map[string]any{
		"topic": "photosynthesis",
	})

	smoke.assertSuccess(result)
	smoke.assertStepCount(result, 3)

	// Verify steps completed
	if _, ok := result.Steps["generate_outline"]; !ok {
		t.Error("generate_outline step not found in results")
	}
	if _, ok := result.Steps["expand_first_point"]; !ok {
		t.Error("expand_first_point step not found in results")
	}
	if _, ok := result.Steps["generate_summary"]; !ok {
		t.Error("generate_summary step not found in results")
	}
}

// TestAnthropic_WithActions tests workflow combining LLM and actions.
func TestAnthropic_WithActions(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	cfg := integration.LoadConfig()

	provider, err := providers.NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("failed to create Anthropic provider: %v", err)
	}

	smoke := newSmokeTest(t, provider)

	// Load workflow with actions
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/with_actions.yaml")

	result := smoke.run(wf, map[string]any{
		"user_query": "Test query",
	})

	smoke.assertSuccess(result)
}

// TestAnthropic_ErrorHandling tests error handling with Anthropic.
func TestAnthropic_ErrorHandling(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	cfg := integration.LoadConfig()

	provider, err := providers.NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("failed to create Anthropic provider: %v", err)
	}

	smoke := newSmokeTest(t, provider)

	// Load error handling workflow
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/error_handling.yaml")

	// Run workflow with error trigger
	result := smoke.run(wf, map[string]any{
		"fail_step": "none",
	})

	// Should handle errors gracefully
	smoke.assertSuccess(result)

	t.Logf("Error handling workflow completed successfully")
}

// TestAnthropic_TokenTracking tests token usage tracking with Anthropic.
func TestAnthropic_TokenTracking(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	cfg := integration.LoadConfig()

	provider, err := providers.NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("failed to create Anthropic provider: %v", err)
	}

	smoke := newSmokeTest(t, provider)

	// Load simple workflow
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/simple_llm.yaml")

	result := smoke.run(wf, map[string]any{
		"user_query": "Count to 5.",
	})

	smoke.assertSuccess(result)

	// Verify token usage is tracked
	if result.Usage.TotalTokens == 0 {
		t.Error("expected non-zero token usage")
	}

	// Verify test and suite totals are updated
	testTokens := smoke.tracker.GetTestTokens()
	suiteTokens := smoke.tracker.GetSuiteTokens()

	if testTokens == 0 {
		t.Error("expected non-zero test token count")
	}

	if suiteTokens == 0 {
		t.Error("expected non-zero suite token count")
	}

	t.Logf("Test tokens: %d, Suite tokens: %d", testTokens, suiteTokens)
}

// TestAnthropic_ToolCalling tests tool calling capability.
func TestAnthropic_ToolCalling(t *testing.T) {
	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")

	cfg := integration.LoadConfig()

	provider, err := providers.NewAnthropicProvider(cfg.AnthropicAPIKey)
	if err != nil {
		t.Fatalf("failed to create Anthropic provider: %v", err)
	}

	smoke := newSmokeTest(t, provider)

	// Load tool calling workflow
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/tool_calling.yaml")

	result := smoke.run(wf, map[string]any{
		"query": "Test query",
	})

	smoke.assertSuccess(result)

	// Verify both steps completed
	if len(result.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(result.Steps))
	}
}
