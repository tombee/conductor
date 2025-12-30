package workflow

import "time"

// LoopContext provides context variables for loop step execution.
// These are injected into TemplateContext during loop execution.
type LoopContext struct {
	// Iteration is the current iteration number (0-indexed)
	Iteration int `json:"iteration"`

	// MaxIterations is the configured maximum iteration count
	MaxIterations int `json:"max_iterations"`

	// History contains records from all previous iterations
	History []IterationRecord `json:"history"`
}

// IterationRecord captures the outputs from a single loop iteration.
type IterationRecord struct {
	// Iteration is the iteration number (0-indexed)
	Iteration int `json:"iteration"`

	// Steps maps step IDs to their outputs from this iteration
	Steps map[string]interface{} `json:"steps"`

	// Timestamp is when the iteration completed
	Timestamp time.Time `json:"timestamp"`

	// DurationMs is the total duration of the iteration in milliseconds
	DurationMs int64 `json:"duration_ms"`
}

// LoopOutput represents the output structure of a completed loop step.
type LoopOutput struct {
	// StepOutputs contains the final iteration's nested step outputs
	StepOutputs map[string]interface{} `json:"step_outputs"`

	// IterationCount is the number of iterations executed
	IterationCount int `json:"iteration_count"`

	// TerminatedBy indicates how the loop ended: "condition" or "max_iterations"
	TerminatedBy string `json:"terminated_by"`

	// History is the full iteration history (may be truncated if > 1MB)
	History []IterationRecord `json:"history,omitempty"`

	// HistoryTruncated indicates if history was truncated due to size limits
	HistoryTruncated bool `json:"history_truncated,omitempty"`

	// RetainedIterations is the number of iterations kept when truncated
	RetainedIterations int `json:"retained_iterations,omitempty"`

	// TotalIterations is the total iterations when history is truncated
	TotalIterations int `json:"total_iterations,omitempty"`
}

// LoopTermination constants for TerminatedBy field
const (
	LoopTerminatedByCondition     = "condition"
	LoopTerminatedByMaxIterations = "max_iterations"
	LoopTerminatedByTimeout       = "timeout"
	LoopTerminatedByError         = "error"
)

// MaxHistorySizeBytes is the maximum size of loop history before truncation
const MaxHistorySizeBytes = 1024 * 1024 // 1MB
