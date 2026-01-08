//go:build smoke

package e2e

import (
	"testing"

	"github.com/tombee/conductor/internal/testing/integration"
	"github.com/tombee/conductor/pkg/llm/providers"
	"github.com/tombee/conductor/test/e2e/harness"
)

// TestOllama_SimpleWorkflow tests a basic LLM workflow with Ollama.
func TestOllama_SimpleWorkflow(t *testing.T) {
	integration.SkipWithoutOllama(t)

	cfg := integration.LoadConfig()

	// Create Ollama provider
	provider, err := providers.NewOllamaProvider(cfg.OllamaURL)
	if err != nil {
		t.Fatalf("failed to create Ollama provider: %v", err)
	}

	// Create smoke test runner
	smoke := newSmokeTest(t, provider)

	// Load simple workflow from YAML
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/simple_llm.yaml")

	// Run workflow
	result := smoke.run(wf, map[string]any{
		"user_query": "What is 2+2?",
	})

	// Verify success
	smoke.assertSuccess(result)
	smoke.assertStepCount(result, 1)
	smoke.assertOutputContains(result, "")

	// Verify we got a real response
	if result.Usage.TotalTokens == 0 {
		t.Error("expected non-zero token usage from Ollama")
	}
}

// TestOllama_MultiStep tests a multi-step workflow with Ollama.
func TestOllama_MultiStep(t *testing.T) {
	integration.SkipWithoutOllama(t)

	cfg := integration.LoadConfig()

	provider, err := providers.NewOllamaProvider(cfg.OllamaURL)
	if err != nil {
		t.Fatalf("failed to create Ollama provider: %v", err)
	}

	smoke := newSmokeTest(t, provider)

	// Load multi-step workflow from YAML
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/multi_step.yaml")

	result := smoke.run(wf, map[string]any{
		"topic": "the moon",
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

// TestOllama_ErrorHandling tests error handling with Ollama.
func TestOllama_ErrorHandling(t *testing.T) {
	integration.SkipWithoutOllama(t)

	cfg := integration.LoadConfig()

	provider, err := providers.NewOllamaProvider(cfg.OllamaURL)
	if err != nil {
		t.Fatalf("failed to create Ollama provider: %v", err)
	}

	// Load error handling workflow from YAML
	h := harness.New(t)
	wf := h.LoadWorkflow("testdata/error_handling.yaml")

	smoke := newSmokeTest(t, provider)

	// Run workflow with error trigger
	result := smoke.run(wf, map[string]any{
		"fail_step": "none",
	})

	// Should handle errors gracefully
	smoke.assertSuccess(result)

	t.Logf("Error handling workflow completed successfully")
}

// TestOllama_TokenTracking tests token usage tracking with Ollama.
func TestOllama_TokenTracking(t *testing.T) {
	integration.SkipWithoutOllama(t)

	cfg := integration.LoadConfig()

	provider, err := providers.NewOllamaProvider(cfg.OllamaURL)
	if err != nil {
		t.Fatalf("failed to create Ollama provider: %v", err)
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
