package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/pkg/llm"
)

// CostLimitEnforcer checks and enforces cost limits during workflow execution.
type CostLimitEnforcer struct {
	// workflowLimits are the limits defined at the workflow level
	workflowLimits *CostLimits

	// currentUsage tracks accumulated cost for the current workflow run
	currentUsage CostUsage

	// tracker is the cost tracker to pull records from
	tracker *llm.CostTracker

	// runID is the current run ID for filtering records
	runID string
}

// CostUsage tracks current accumulated cost and token usage.
type CostUsage struct {
	TotalCost   float64
	TotalTokens int
	RequestCount int
}

// NewCostLimitEnforcer creates a new cost limit enforcer for a workflow run.
func NewCostLimitEnforcer(workflowLimits *CostLimits, tracker *llm.CostTracker, runID string) *CostLimitEnforcer {
	return &CostLimitEnforcer{
		workflowLimits: workflowLimits,
		tracker:        tracker,
		runID:          runID,
	}
}

// getStepLimits converts StepDefinition fields into CostLimits structure.
func (e *CostLimitEnforcer) getStepLimits(step *StepDefinition) *CostLimits {
	if step.MaxCost == nil && step.MaxTokens == nil {
		return nil
	}

	return &CostLimits{
		MaxCost:   step.MaxCost,
		MaxTokens: step.MaxTokens,
		OnLimit:   step.OnLimit,
	}
}

// CheckBeforeStep checks if executing this step would exceed limits.
// Returns an error if limits would be exceeded.
func (e *CostLimitEnforcer) CheckBeforeStep(ctx context.Context, step *StepDefinition) error {
	stepLimits := e.getStepLimits(step)
	if e.workflowLimits == nil && stepLimits == nil {
		// No limits configured
		return nil
	}

	// Get current usage
	e.updateCurrentUsage()

	// Check step-level limits first
	if stepLimits != nil {
		if err := e.checkLimits(stepLimits, e.currentUsage, "step", step.ID); err != nil {
			return err
		}
	}

	// Check workflow-level limits
	if e.workflowLimits != nil {
		if err := e.checkLimits(e.workflowLimits, e.currentUsage, "workflow", ""); err != nil {
			return err
		}
	}

	return nil
}

// CheckAfterStep checks if the step execution exceeded limits.
// This is the definitive check after actual cost is known.
func (e *CostLimitEnforcer) CheckAfterStep(ctx context.Context, step *StepDefinition, stepCost *llm.CostInfo, stepTokens int) error {
	stepLimits := e.getStepLimits(step)
	if e.workflowLimits == nil && stepLimits == nil {
		// No limits configured
		return nil
	}

	// Get current accumulated usage from tracker
	e.updateCurrentUsage()

	// Calculate usage including this step
	totalUsage := e.currentUsage
	if stepCost != nil {
		totalUsage.TotalCost += stepCost.Amount
	}
	totalUsage.TotalTokens += stepTokens
	totalUsage.RequestCount++

	// Check step-level limits (just this step's usage)
	if stepLimits != nil {
		stepUsage := CostUsage{
			TotalCost:   0,
			TotalTokens: stepTokens,
			RequestCount: 1,
		}
		if stepCost != nil {
			stepUsage.TotalCost = stepCost.Amount
		}

		if err := e.checkLimits(stepLimits, stepUsage, "step", step.ID); err != nil {
			return err
		}
	}

	// Check workflow-level limits (accumulated usage including this step)
	if e.workflowLimits != nil {
		if err := e.checkLimits(e.workflowLimits, totalUsage, "workflow", ""); err != nil {
			return err
		}
	}

	return nil
}

// checkLimits checks if usage exceeds configured limits.
func (e *CostLimitEnforcer) checkLimits(limits *CostLimits, usage CostUsage, scope string, scopeName string) error {
	exceeded := false
	var reason string

	// Check cost limit
	if limits.MaxCost != nil && usage.TotalCost > *limits.MaxCost {
		exceeded = true
		reason = fmt.Sprintf("cost $%.4f exceeds limit $%.4f", usage.TotalCost, *limits.MaxCost)
	}

	// Check token limit
	if limits.MaxTokens != nil && usage.TotalTokens > *limits.MaxTokens {
		exceeded = true
		if reason != "" {
			reason += " and "
		}
		reason += fmt.Sprintf("tokens %d exceeds limit %d", usage.TotalTokens, *limits.MaxTokens)
	}

	if !exceeded {
		return nil
	}

	// Handle limit exceeded based on configured behavior
	behavior := limits.OnLimit
	if behavior == "" {
		behavior = LimitBehaviorAbort // Default behavior
	}

	scopeDesc := scope
	if scopeName != "" {
		scopeDesc = fmt.Sprintf("%s '%s'", scope, scopeName)
	}

	switch behavior {
	case LimitBehaviorAbort:
		return &CostLimitExceededError{
			Scope:        scopeDesc,
			Reason:       reason,
			CurrentUsage: usage,
			Limits:       limits,
		}
	case LimitBehaviorWarn:
		// Log warning but continue
		// TODO: Hook into logging system when available
		fmt.Printf("WARNING: %s %s\n", scopeDesc, reason)
		return nil
	case LimitBehaviorContinue:
		// Silently continue
		return nil
	default:
		// Unknown behavior, treat as abort
		return &CostLimitExceededError{
			Scope:        scopeDesc,
			Reason:       reason,
			CurrentUsage: usage,
			Limits:       limits,
		}
	}
}

// updateCurrentUsage refreshes current usage from the cost tracker.
func (e *CostLimitEnforcer) updateCurrentUsage() {
	if e.tracker == nil {
		return
	}

	// Get all records for this run
	records := e.tracker.GetRecords()

	// Reset usage
	e.currentUsage = CostUsage{}

	// Sum up usage from records matching this run
	for _, record := range records {
		if record.RunID == e.runID {
			if record.Cost != nil {
				e.currentUsage.TotalCost += record.Cost.Amount
			}
			e.currentUsage.TotalTokens += record.Usage.TotalTokens
			e.currentUsage.RequestCount++
		}
	}
}

// GetCurrentUsage returns the current accumulated usage.
func (e *CostLimitEnforcer) GetCurrentUsage() CostUsage {
	e.updateCurrentUsage()
	return e.currentUsage
}

// CostLimitExceededError is returned when cost limits are exceeded.
type CostLimitExceededError struct {
	Scope        string
	Reason       string
	CurrentUsage CostUsage
	Limits       *CostLimits
}

func (e *CostLimitExceededError) Error() string {
	return fmt.Sprintf("cost limit exceeded for %s: %s", e.Scope, e.Reason)
}

// PartialResultsHandler saves partial results when a workflow is aborted due to cost limits.
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
func (h *PartialResultsHandler) SavePartialResults(ctx context.Context, runID string, completedSteps []StepResult, abortReason error) error {
	// TODO: Implement actual file saving
	// For now, this is a placeholder
	// Implementation will write to ~/.conductor/partial-results/<runID>.json
	// with completed steps and abort reason
	return nil
}

// StreamingCostMonitor monitors cost during streaming LLM requests.
type StreamingCostMonitor struct {
	enforcer *CostLimitEnforcer
	step     *StepDefinition

	// Monitoring state
	lastCheckTime   time.Time
	tokensProcessed int
	checkInterval   time.Duration
	tokenThreshold  int
}

// NewStreamingCostMonitor creates a monitor for streaming requests.
func NewStreamingCostMonitor(enforcer *CostLimitEnforcer, step *StepDefinition) *StreamingCostMonitor {
	return &StreamingCostMonitor{
		enforcer:       enforcer,
		step:           step,
		lastCheckTime:  time.Now(),
		checkInterval:  10 * time.Second,  // Check every 10 seconds
		tokenThreshold: 5000,               // Or every 5000 tokens
	}
}

// CheckDuringStream checks if limits are exceeded during streaming.
// Should be called periodically as tokens are received.
func (m *StreamingCostMonitor) CheckDuringStream(ctx context.Context, tokensReceived int) error {
	m.tokensProcessed += tokensReceived

	now := time.Now()
	timeSinceLastCheck := now.Sub(m.lastCheckTime)

	// Check if we should verify limits (time or token threshold reached)
	shouldCheck := timeSinceLastCheck >= m.checkInterval ||
	              m.tokensProcessed >= m.tokenThreshold

	if !shouldCheck {
		return nil
	}

	// Update check time
	m.lastCheckTime = now
	m.tokensProcessed = 0

	// Estimate cost so far (this will be approximate)
	// We'll check against limits but this is best-effort
	estimatedUsage := m.enforcer.GetCurrentUsage()

	// Check limits
	stepLimits := m.enforcer.getStepLimits(m.step)
	if stepLimits != nil {
		if err := m.enforcer.checkLimits(stepLimits, estimatedUsage, "step (streaming)", m.step.ID); err != nil {
			return err
		}
	}

	if m.enforcer.workflowLimits != nil {
		if err := m.enforcer.checkLimits(m.enforcer.workflowLimits, estimatedUsage, "workflow (streaming)", ""); err != nil {
			return err
		}
	}

	return nil
}

// PreflightEstimate provides a best-effort estimate of whether a request will exceed limits.
// This is used for non-streaming requests to fail fast before making the API call.
type PreflightEstimate struct {
	EstimatedCost   float64
	EstimatedTokens int
	WouldExceed     bool
	Reason          string
}

// EstimatePreflightCost estimates if a request would exceed limits before execution.
// This is best-effort and may not be accurate for all cases.
func EstimatePreflightCost(ctx context.Context, enforcer *CostLimitEnforcer, step *StepDefinition, promptTokens int) *PreflightEstimate {
	estimate := &PreflightEstimate{}

	// TODO: Implement actual estimation logic
	// This would involve:
	// 1. Estimating prompt tokens from inputs
	// 2. Estimating completion tokens (harder - could use historical averages)
	// 3. Calculating estimated cost using pricing table
	// 4. Checking if current usage + estimate would exceed limits

	// For now, return a basic estimate
	estimate.EstimatedTokens = promptTokens
	estimate.WouldExceed = false

	return estimate
}
