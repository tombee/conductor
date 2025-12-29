package sdk

import (
	"time"
)

// Result contains the outcome of workflow execution.
type Result struct {
	WorkflowID string
	Success    bool
	Output     map[string]any         // Final workflow output
	Steps      map[string]*StepResult // Per-step results
	Duration   time.Duration
	Cost       CostSummary
	Error      error
}

// StepResult contains the outcome of a single step.
type StepResult struct {
	StepID   string
	Status   StepStatus // pending, running, success, failed, skipped
	Output   map[string]any
	Duration time.Duration
	Tokens   TokenUsage
	Error    string
}

// StepStatus indicates the current state of a workflow step.
type StepStatus string

const (
	StepStatusPending  StepStatus = "pending"
	StepStatusRunning  StepStatus = "running"
	StepStatusSuccess  StepStatus = "success"
	StepStatusFailed   StepStatus = "failed"
	StepStatusSkipped  StepStatus = "skipped"
)

// CostSummary tracks execution costs.
type CostSummary struct {
	TotalTokens      int
	InputTokens      int
	OutputTokens     int
	CacheReadTokens  int
	CacheWriteTokens int
	EstimatedCost    float64 // USD
	ByStep           map[string]float64
}

// AgentResult contains the outcome of agent execution.
type AgentResult struct {
	Success       bool
	FinalResponse string
	ToolCalls     []ToolExecution
	Iterations    int
	Tokens        TokenUsage
	Duration      time.Duration
	Error         error
}

// ToolExecution represents a single tool execution in an agent loop.
type ToolExecution struct {
	ToolName string
	Inputs   map[string]any
	Outputs  map[string]any
	Error    error
}
