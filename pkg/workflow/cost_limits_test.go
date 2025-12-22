package workflow

import (
	"context"
	"testing"

	"github.com/tombee/conductor/pkg/llm"
)

func TestCostLimitEnforcer_CheckAfterStep(t *testing.T) {
	tests := []struct {
		name          string
		workflowLimit *CostLimits
		stepLimit     *CostLimits
		stepCost      float64
		stepTokens    int
		wantError     bool
		errorContains string
	}{
		{
			name:          "no limits configured",
			workflowLimit: nil,
			stepLimit:     nil,
			stepCost:      1.0,
			stepTokens:    10000,
			wantError:     false,
		},
		{
			name: "under workflow cost limit",
			workflowLimit: &CostLimits{
				MaxCost: floatPtr(2.0),
			},
			stepLimit:  nil,
			stepCost:   1.0,
			stepTokens: 10000,
			wantError:  false,
		},
		{
			name: "exceeds workflow cost limit",
			workflowLimit: &CostLimits{
				MaxCost:  floatPtr(0.5),
				OnLimit:  LimitBehaviorAbort,
			},
			stepLimit:     nil,
			stepCost:      1.0,
			stepTokens:    10000,
			wantError:     true,
			errorContains: "cost limit exceeded",
		},
		{
			name: "under workflow token limit",
			workflowLimit: &CostLimits{
				MaxTokens: intPtr(20000),
			},
			stepLimit:  nil,
			stepCost:   1.0,
			stepTokens: 10000,
			wantError:  false,
		},
		{
			name: "exceeds workflow token limit",
			workflowLimit: &CostLimits{
				MaxTokens: intPtr(5000),
				OnLimit:   LimitBehaviorAbort,
			},
			stepLimit:     nil,
			stepCost:      1.0,
			stepTokens:    10000,
			wantError:     true,
			errorContains: "tokens",
		},
		{
			name:          "exceeds step cost limit",
			workflowLimit: nil,
			stepLimit: &CostLimits{
				MaxCost: floatPtr(0.5),
				OnLimit: LimitBehaviorAbort,
			},
			stepCost:      1.0,
			stepTokens:    10000,
			wantError:     true,
			errorContains: "step",
		},
		{
			name: "warn behavior does not error",
			workflowLimit: &CostLimits{
				MaxCost: floatPtr(0.5),
				OnLimit: LimitBehaviorWarn,
			},
			stepLimit:  nil,
			stepCost:   1.0,
			stepTokens: 10000,
			wantError:  false,
		},
		{
			name: "continue behavior does not error",
			workflowLimit: &CostLimits{
				MaxCost: floatPtr(0.5),
				OnLimit: LimitBehaviorContinue,
			},
			stepLimit:  nil,
			stepCost:   1.0,
			stepTokens: 10000,
			wantError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := llm.NewCostTracker()
			enforcer := NewCostLimitEnforcer(tt.workflowLimit, tracker, "test-run-123")

			step := &StepDefinition{
				ID: "test-step",
			}
			// Convert stepLimit to individual step fields
			if tt.stepLimit != nil {
				step.MaxCost = tt.stepLimit.MaxCost
				step.MaxTokens = tt.stepLimit.MaxTokens
				step.OnLimit = tt.stepLimit.OnLimit
			}

			costInfo := &llm.CostInfo{
				Amount:   tt.stepCost,
				Currency: "USD",
				Accuracy: llm.CostMeasured,
			}

			err := enforcer.CheckAfterStep(context.Background(), step, costInfo, tt.stepTokens)

			if tt.wantError && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantError && err != nil && tt.errorContains != "" {
				if errStr := err.Error(); !containsStr(errStr, tt.errorContains) {
					t.Errorf("error %q does not contain %q", errStr, tt.errorContains)
				}
			}
		})
	}
}

func TestCostLimitEnforcer_AccumulatedCost(t *testing.T) {
	tracker := llm.NewCostTracker()
	runID := "test-run-456"

	// Add some cost records to the tracker
	tracker.Track(llm.CostRecord{
		ID:        "rec1",
		RunID:     runID,
		Provider:  "anthropic",
		Model:     "claude-3-opus",
		Usage:     llm.TokenUsage{TotalTokens: 1000},
		Cost:      &llm.CostInfo{Amount: 0.5, Currency: "USD", Accuracy: llm.CostMeasured},
	})

	tracker.Track(llm.CostRecord{
		ID:        "rec2",
		RunID:     runID,
		Provider:  "anthropic",
		Model:     "claude-3-opus",
		Usage:     llm.TokenUsage{TotalTokens: 2000},
		Cost:      &llm.CostInfo{Amount: 1.0, Currency: "USD", Accuracy: llm.CostMeasured},
	})

	// Add a record from a different run (should not be counted)
	tracker.Track(llm.CostRecord{
		ID:        "rec3",
		RunID:     "other-run",
		Provider:  "anthropic",
		Model:     "claude-3-opus",
		Usage:     llm.TokenUsage{TotalTokens: 5000},
		Cost:      &llm.CostInfo{Amount: 2.0, Currency: "USD", Accuracy: llm.CostMeasured},
	})

	// Create enforcer with a limit
	workflowLimit := &CostLimits{
		MaxCost:  floatPtr(2.0), // Total so far is 1.5, so next step should exceed
		OnLimit:  LimitBehaviorAbort,
	}
	enforcer := NewCostLimitEnforcer(workflowLimit, tracker, runID)

	// Try to add another step that would exceed the limit
	step := &StepDefinition{
		ID: "test-step-3",
	}

	costInfo := &llm.CostInfo{
		Amount:   0.6, // 1.5 + 0.6 = 2.1, exceeds limit of 2.0
		Currency: "USD",
		Accuracy: llm.CostMeasured,
	}

	err := enforcer.CheckAfterStep(context.Background(), step, costInfo, 1000)
	if err == nil {
		t.Error("expected error for exceeding accumulated limit, got none")
	}

	// Verify error is cost limit exceeded error
	if _, ok := err.(*CostLimitExceededError); !ok {
		t.Errorf("expected CostLimitExceededError, got %T", err)
	}
}

func TestCostLimitEnforcer_GetCurrentUsage(t *testing.T) {
	tracker := llm.NewCostTracker()
	runID := "test-run-789"

	// Add cost records
	tracker.Track(llm.CostRecord{
		ID:        "rec1",
		RunID:     runID,
		Usage:     llm.TokenUsage{TotalTokens: 1000},
		Cost:      &llm.CostInfo{Amount: 0.5, Currency: "USD"},
	})

	tracker.Track(llm.CostRecord{
		ID:        "rec2",
		RunID:     runID,
		Usage:     llm.TokenUsage{TotalTokens: 2000},
		Cost:      &llm.CostInfo{Amount: 1.0, Currency: "USD"},
	})

	enforcer := NewCostLimitEnforcer(nil, tracker, runID)

	usage := enforcer.GetCurrentUsage()

	if usage.TotalCost != 1.5 {
		t.Errorf("expected total cost 1.5, got %.2f", usage.TotalCost)
	}

	if usage.TotalTokens != 3000 {
		t.Errorf("expected total tokens 3000, got %d", usage.TotalTokens)
	}

	if usage.RequestCount != 2 {
		t.Errorf("expected request count 2, got %d", usage.RequestCount)
	}
}

func TestStreamingCostMonitor_CheckDuringStream(t *testing.T) {
	tracker := llm.NewCostTracker()
	runID := "test-run-streaming"

	// Pre-populate with some cost
	tracker.Track(llm.CostRecord{
		ID:        "rec1",
		RunID:     runID,
		Usage:     llm.TokenUsage{TotalTokens: 1000},
		Cost:      &llm.CostInfo{Amount: 0.5, Currency: "USD"},
	})

	workflowLimit := &CostLimits{
		MaxTokens: intPtr(6000), // Allow 6000 total tokens
		OnLimit:   LimitBehaviorAbort,
	}

	enforcer := NewCostLimitEnforcer(workflowLimit, tracker, runID)

	step := &StepDefinition{
		ID: "streaming-step",
	}

	monitor := NewStreamingCostMonitor(enforcer, step)

	// Simulate receiving tokens in chunks
	// First chunk: 2000 tokens (total now 3000, under limit)
	err := monitor.CheckDuringStream(context.Background(), 2000)
	if err != nil {
		t.Errorf("unexpected error on first chunk: %v", err)
	}

	// Second chunk: 3000 more tokens (total now 6000, at limit)
	err = monitor.CheckDuringStream(context.Background(), 3000)
	if err != nil {
		t.Errorf("unexpected error at limit: %v", err)
	}

	// Manually trigger check by resetting last check time and adding tokens
	// to simulate reaching threshold
	monitor.tokensProcessed = 5000 // Over token threshold

	// Add one more record to tracker to exceed limit
	tracker.Track(llm.CostRecord{
		ID:        "rec2",
		RunID:     runID,
		Usage:     llm.TokenUsage{TotalTokens: 5500},
		Cost:      &llm.CostInfo{Amount: 2.0, Currency: "USD"},
	})

	// Third chunk should trigger check and error (total now > 6000)
	err = monitor.CheckDuringStream(context.Background(), 100)
	if err == nil {
		t.Error("expected error when exceeding limit during streaming, got none")
	}
}

// Helper functions

func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}

func containsStr(s, substr string) bool {
	// Simple substring check
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
