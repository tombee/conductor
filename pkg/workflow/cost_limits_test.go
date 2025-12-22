package workflow

import (
	"context"
	"testing"

	"github.com/tombee/conductor/pkg/llm"
)

func TestTokenLimitEnforcer_CheckAfterStep(t *testing.T) {
	tests := []struct {
		name          string
		workflowLimit int
		stepLimit     *int
		stepTokens    int
		wantError     bool
		errorContains string
	}{
		{
			name:          "no limits configured",
			workflowLimit: 0,
			stepLimit:     nil,
			stepTokens:    10000,
			wantError:     false,
		},
		{
			name:          "under workflow token limit",
			workflowLimit: 20000,
			stepLimit:     nil,
			stepTokens:    10000,
			wantError:     false,
		},
		{
			name:          "exceeds workflow token limit",
			workflowLimit: 5000,
			stepLimit:     nil,
			stepTokens:    10000,
			wantError:     true,
			errorContains: "token limit exceeded",
		},
		{
			name:          "exceeds step token limit",
			workflowLimit: 0,
			stepLimit:     intPtr(5000),
			stepTokens:    10000,
			wantError:     true,
			errorContains: "step",
		},
		{
			name:          "under step token limit",
			workflowLimit: 0,
			stepLimit:     intPtr(15000),
			stepTokens:    10000,
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := llm.NewUsageTracker()
			enforcer := NewTokenLimitEnforcer(tt.workflowLimit, tracker, "test-run-123")

			step := &StepDefinition{
				ID:        "test-step",
				MaxTokens: tt.stepLimit,
			}

			err := enforcer.CheckAfterStep(context.Background(), step, tt.stepTokens)

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

func TestTokenLimitEnforcer_AccumulatedTokens(t *testing.T) {
	tracker := llm.NewUsageTracker()
	runID := "test-run-456"

	// Add some token usage records to the tracker
	tracker.Track(llm.UsageRecord{
		RunID:    runID,
		Provider: "anthropic",
		Model:    "claude-3-opus",
		Usage:    llm.TokenUsage{TotalTokens: 1000},
	})

	tracker.Track(llm.UsageRecord{
		RunID:    runID,
		Provider: "anthropic",
		Model:    "claude-3-opus",
		Usage:    llm.TokenUsage{TotalTokens: 2000},
	})

	// Add a record from a different run (should not be counted)
	tracker.Track(llm.UsageRecord{
		RunID:    "other-run",
		Provider: "anthropic",
		Model:    "claude-3-opus",
		Usage:    llm.TokenUsage{TotalTokens: 5000},
	})

	// Create enforcer with a limit (3000 accumulated + 500 step = 3500, exceeds 3400)
	workflowLimit := 3400
	enforcer := NewTokenLimitEnforcer(workflowLimit, tracker, runID)

	// Try to add another step that would exceed the limit
	step := &StepDefinition{
		ID: "test-step-3",
	}

	err := enforcer.CheckAfterStep(context.Background(), step, 500)
	if err == nil {
		t.Error("expected error for exceeding accumulated limit, got none")
	}

	// Verify error is token limit exceeded error
	if _, ok := err.(*TokenLimitExceededError); !ok {
		t.Errorf("expected TokenLimitExceededError, got %T", err)
	}
}

func TestTokenLimitEnforcer_GetCurrentUsage(t *testing.T) {
	tracker := llm.NewUsageTracker()
	runID := "test-run-789"

	// Add token records
	tracker.Track(llm.UsageRecord{
		RunID: runID,
		Usage: llm.TokenUsage{TotalTokens: 1000},
	})

	tracker.Track(llm.UsageRecord{
		RunID: runID,
		Usage: llm.TokenUsage{TotalTokens: 2000},
	})

	enforcer := NewTokenLimitEnforcer(0, tracker, runID)

	usage := enforcer.GetCurrentUsage()

	if usage.TotalTokens != 3000 {
		t.Errorf("expected total tokens 3000, got %d", usage.TotalTokens)
	}

	if usage.RequestCount != 2 {
		t.Errorf("expected request count 2, got %d", usage.RequestCount)
	}
}

// Helper functions

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
