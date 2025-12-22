package workflow

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/pkg/llm"
)

// TokenLimitEnforcer checks and enforces token limits during workflow execution.
type TokenLimitEnforcer struct {
	// workflowLimit is the token limit defined at the workflow level
	workflowLimit int

	// currentUsage tracks accumulated token usage for the current workflow run
	currentUsage LimitUsage

	// tracker is the usage tracker to pull records from
	tracker *llm.UsageTracker

	// runID is the current run ID for filtering records
	runID string
}

// LimitUsage tracks current accumulated token usage for limit enforcement.
type LimitUsage struct {
	TotalTokens  int
	RequestCount int
}

// NewTokenLimitEnforcer creates a new token limit enforcer for a workflow run.
func NewTokenLimitEnforcer(workflowLimit int, tracker *llm.UsageTracker, runID string) *TokenLimitEnforcer {
	return &TokenLimitEnforcer{
		workflowLimit: workflowLimit,
		tracker:       tracker,
		runID:         runID,
	}
}

// CheckBeforeStep checks if executing this step would exceed limits.
// Returns an error if limits would be exceeded.
func (e *TokenLimitEnforcer) CheckBeforeStep(ctx context.Context, step *StepDefinition) error {
	if e.workflowLimit == 0 && (step.MaxTokens == nil || *step.MaxTokens == 0) {
		// No limits configured
		return nil
	}

	// Get current usage
	e.updateCurrentUsage()

	// Check workflow-level limits
	if e.workflowLimit > 0 && e.currentUsage.TotalTokens > e.workflowLimit {
		return &TokenLimitExceededError{
			Scope:  "workflow",
			Limit:  e.workflowLimit,
			Actual: e.currentUsage.TotalTokens,
		}
	}

	return nil
}

// CheckAfterStep checks if the step execution exceeded limits.
// This is the definitive check after actual token usage is known.
func (e *TokenLimitEnforcer) CheckAfterStep(ctx context.Context, step *StepDefinition, stepTokens int) error {
	if e.workflowLimit == 0 && (step.MaxTokens == nil || *step.MaxTokens == 0) {
		// No limits configured
		return nil
	}

	// Get current accumulated usage from tracker
	e.updateCurrentUsage()

	// Calculate usage including this step
	totalTokens := e.currentUsage.TotalTokens + stepTokens

	// Check step-level limits
	if step.MaxTokens != nil && *step.MaxTokens > 0 && stepTokens > *step.MaxTokens {
		return &TokenLimitExceededError{
			Scope:  fmt.Sprintf("step '%s'", step.ID),
			Limit:  *step.MaxTokens,
			Actual: stepTokens,
		}
	}

	// Check workflow-level limits
	if e.workflowLimit > 0 && totalTokens > e.workflowLimit {
		return &TokenLimitExceededError{
			Scope:  "workflow",
			Limit:  e.workflowLimit,
			Actual: totalTokens,
		}
	}

	return nil
}

// updateCurrentUsage refreshes current usage from the cost tracker.
func (e *TokenLimitEnforcer) updateCurrentUsage() {
	if e.tracker == nil {
		return
	}

	// Get all records for this run
	records := e.tracker.GetRecords()

	// Reset usage
	e.currentUsage = LimitUsage{}

	// Sum up usage from records matching this run
	for _, record := range records {
		if record.RunID == e.runID {
			e.currentUsage.TotalTokens += record.Usage.TotalTokens
			e.currentUsage.RequestCount++
		}
	}
}

// GetCurrentUsage returns the current accumulated usage.
func (e *TokenLimitEnforcer) GetCurrentUsage() LimitUsage {
	e.updateCurrentUsage()
	return e.currentUsage
}

// TokenLimitExceededError is returned when token limits are exceeded.
type TokenLimitExceededError struct {
	Scope  string
	Limit  int
	Actual int
}

func (e *TokenLimitExceededError) Error() string {
	return fmt.Sprintf("token limit exceeded for %s: %d > %d tokens", e.Scope, e.Actual, e.Limit)
}

// PartialResultsHandler saves partial results when a workflow is aborted due to limits.
type PartialResultsHandler struct {
	// basePath is the directory where partial results are saved
	basePath string
}

// NewPartialResultsHandler creates a new partial results handler.
func NewPartialResultsHandler(basePath string) *PartialResultsHandler {
	return &PartialResultsHandler{
		basePath: basePath,
	}
}

// SavePartialResults saves the workflow state when execution is aborted.
// Currently a no-op; future implementation will write to ~/.conductor/partial-results/<runID>.json.
func (h *PartialResultsHandler) SavePartialResults(ctx context.Context, runID string, completedSteps []StepResult, abortReason error) error {
	return nil
}
