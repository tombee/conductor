// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package runner

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/tombee/conductor/internal/daemon/backend"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	daemonremote "github.com/tombee/conductor/internal/daemon/remote"
	"github.com/tombee/conductor/internal/mcp"
	"github.com/tombee/conductor/internal/remote"
	"github.com/tombee/conductor/internal/tracing"
	"github.com/tombee/conductor/pkg/tools"
	"github.com/tombee/conductor/pkg/workflow"
)

// RunStatus represents the status of a workflow run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// MetricsCollector defines the interface for recording workflow metrics.
type MetricsCollector interface {
	RecordRunStart(ctx context.Context, runID, workflowID string)
	RecordRunComplete(ctx context.Context, runID, workflowID, status, trigger string, duration time.Duration)
	RecordStepComplete(ctx context.Context, workflowID, stepName, status string, duration time.Duration)
	IncrementQueueDepth()
	DecrementQueueDepth()
}

// Run represents a workflow execution.
type Run struct {
	ID            string         `json:"id"`
	WorkflowID    string         `json:"workflow_id"`
	Workflow      string         `json:"workflow"` // Workflow name
	Status        RunStatus      `json:"status"`
	CorrelationID string         `json:"correlation_id,omitempty"` // Correlation ID for request tracing
	Inputs        map[string]any `json:"inputs,omitempty"`
	Output        map[string]any `json:"output,omitempty"`
	Error         string         `json:"error,omitempty"`
	Progress      *Progress      `json:"progress,omitempty"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	Logs          []LogEntry     `json:"logs,omitempty"`
	SourceURL     string         `json:"source_url,omitempty"` // Remote workflow source (for provenance)

	// Internal
	ctx        context.Context
	cancel     context.CancelFunc
	definition *workflow.Definition
	cancelOnce sync.Once
	stopped    chan struct{}
}

// RunSnapshot is an immutable deep copy of Run state for external access.
// Contains NO aliasing to internal mutable state.
type RunSnapshot struct {
	ID            string         `json:"id"`
	WorkflowID    string         `json:"workflow_id"`
	Workflow      string         `json:"workflow"`
	Status        RunStatus      `json:"status"`
	CorrelationID string         `json:"correlation_id,omitempty"` // Correlation ID for request tracing
	Inputs        map[string]any `json:"inputs,omitempty"`
	Output        map[string]any `json:"output,omitempty"`
	Error         string         `json:"error,omitempty"`
	Progress      *Progress      `json:"progress,omitempty"`
	StartedAt     *time.Time     `json:"started_at,omitempty"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	Logs          []LogEntry     `json:"logs,omitempty"`
	SourceURL     string         `json:"source_url,omitempty"`
}

// Progress tracks workflow execution progress.
type Progress struct {
	CurrentStep string `json:"current_step"`
	Completed   int    `json:"completed"`
	Total       int    `json:"total"`
}

// LogEntry represents a log message from a run.
type LogEntry struct {
	Timestamp     time.Time `json:"timestamp"`
	Level         string    `json:"level"`
	Message       string    `json:"message"`
	StepID        string    `json:"step_id,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"` // Correlation ID for distributed tracing
}

// Config contains runner configuration.
type Config struct {
	MaxParallel    int
	DefaultTimeout time.Duration
}

// Runner manages workflow executions.
type Runner struct {
	mu          sync.RWMutex
	runs        map[string]*Run
	maxParallel int
	semaphore   chan struct{}
	defTimeout  time.Duration

	// Backend for persistence
	backend backend.Backend

	// Checkpoint manager
	checkpoints *checkpoint.Manager

	// MCP manager for MCP server lifecycle
	mcpManager mcp.MCPManagerProvider

	// Tool registry for tool resolution
	toolRegistry *tools.Registry

	// Remote workflow fetcher (optional)
	fetcher *daemonremote.Fetcher

	// Subscribers for log streaming
	subscribers map[string][]chan LogEntry
	subMu       sync.RWMutex

	// Execution adapter for step execution (required for workflow execution)
	adapter ExecutionAdapter

	// Metrics collector for observability (optional)
	metrics MetricsCollector

	// draining indicates the runner is in graceful shutdown mode
	draining atomic.Bool
}

// New creates a new Runner with the given configuration.
func New(cfg Config, be backend.Backend, cm *checkpoint.Manager, opts ...Option) *Runner {
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 10
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 30 * time.Minute
	}
	r := &Runner{
		runs:         make(map[string]*Run),
		maxParallel:  cfg.MaxParallel,
		semaphore:    make(chan struct{}, cfg.MaxParallel),
		defTimeout:   cfg.DefaultTimeout,
		backend:      be,
		checkpoints:  cm,
		toolRegistry: tools.NewRegistry(),
		subscribers:  make(map[string][]chan LogEntry),
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	// Set default MCP manager if not provided
	if r.mcpManager == nil {
		r.mcpManager = mcp.NewManager(mcp.ManagerConfig{})
	}

	return r
}

// SetFetcher sets the remote workflow fetcher.
// This is optional - if not set, remote workflows will not be supported.
func (r *Runner) SetFetcher(f *daemonremote.Fetcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetcher = f
}

// SetAdapter sets the execution adapter for step execution.
// This must be called before submitting workflows, otherwise execution will fail.
// The daemon initialization in internal/daemon/daemon.go automatically sets up
// the adapter with the configured LLM provider.
func (r *Runner) SetAdapter(adapter ExecutionAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapter = adapter
}

// ToolRegistry returns the runner's tool registry.
// This is used to register tools and pass to the step executor.
func (r *Runner) ToolRegistry() *tools.Registry {
	return r.toolRegistry
}

// SetMetrics sets the metrics collector for observability.
// This is optional - if not set, metrics will not be recorded.
func (r *Runner) SetMetrics(metrics MetricsCollector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = metrics
}

// SubmitRequest contains the parameters for submitting a workflow run.
type SubmitRequest struct {
	WorkflowYAML []byte
	Inputs       map[string]any
	// RemoteRef is an optional remote reference (e.g., github:user/repo)
	// If provided, WorkflowYAML should be empty (daemon will fetch it)
	RemoteRef string
	// NoCache forces a fresh fetch of remote workflows, bypassing cache
	NoCache bool
}

// Submit submits a workflow for execution and returns an immutable snapshot of the run.
func (r *Runner) Submit(ctx context.Context, req SubmitRequest) (*RunSnapshot, error) {
	var workflowYAML []byte
	var sourceURL string

	// Check if this is a remote workflow
	if req.RemoteRef != "" {
		// Validate that it's a remote reference
		if !remote.IsRemote(req.RemoteRef) {
			return nil, fmt.Errorf("invalid remote reference: %s", req.RemoteRef)
		}

		// Check if fetcher is available
		r.mu.RLock()
		fetcher := r.fetcher
		r.mu.RUnlock()

		if fetcher == nil {
			return nil, fmt.Errorf("remote workflows not supported (fetcher not configured)")
		}

		// Fetch the remote workflow
		result, err := fetcher.Fetch(ctx, req.RemoteRef, req.NoCache)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch remote workflow: %w", err)
		}

		workflowYAML = result.Content
		sourceURL = result.SourceURL
	} else {
		// Local workflow
		workflowYAML = req.WorkflowYAML
	}

	// Parse workflow
	def, err := workflow.ParseDefinition(workflowYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Create run
	runID := uuid.New().String()[:8]
	runCtx, cancel := context.WithCancel(ctx)

	// Extract correlation ID from context (set by HTTP middleware)
	correlationID := string(tracing.FromContextOrEmpty(ctx))

	run := &Run{
		ID:            runID,
		WorkflowID:    def.Name,
		Workflow:      def.Name,
		Status:        RunStatusPending,
		CorrelationID: correlationID,
		Inputs:        req.Inputs,
		CreatedAt:     time.Now(),
		SourceURL:     sourceURL,
		Progress: &Progress{
			Total: len(def.Steps),
		},
		ctx:        runCtx,
		cancel:     cancel,
		definition: def,
		stopped:    make(chan struct{}),
	}

	r.mu.Lock()
	r.runs[runID] = run
	metrics := r.metrics
	r.mu.Unlock()

	// Increment queue depth for metrics (run is pending)
	if metrics != nil {
		metrics.IncrementQueueDepth()
	}

	// Persist to backend
	if r.backend != nil {
		beRun := r.toBackendRun(run)
		if err := r.backend.CreateRun(ctx, beRun); err != nil {
			// Log error but continue - in-memory state is the source of truth
			r.addLog(run, "warn", fmt.Sprintf("Failed to persist run: %v", err), "")
		}
	}

	// Create initial snapshot before background execution starts
	r.mu.Lock()
	snapshot := r.snapshotRun(run)
	r.mu.Unlock()

	// Start execution in background
	go r.execute(run)

	return snapshot, nil
}

// Get returns an immutable snapshot of a run by ID.
func (r *Runner) Get(id string) (*RunSnapshot, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	run, exists := r.runs[id]
	if !exists {
		return nil, fmt.Errorf("run not found: %s", id)
	}
	return r.snapshotRun(run), nil
}

// List returns immutable snapshots of all runs, optionally filtered.
func (r *Runner) List(filter ListFilter) []*RunSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*RunSnapshot
	for _, run := range r.runs {
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		if filter.Workflow != "" && run.Workflow != filter.Workflow {
			continue
		}
		result = append(result, r.snapshotRun(run))
	}
	return result
}

// ListFilter contains filtering options for listing runs.
type ListFilter struct {
	Status   RunStatus
	Workflow string
	Limit    int
}

// Cancel cancels a running workflow.
// Cancel only signals cancellation via the stopped channel.
// The execute() goroutine is responsible for updating the final state.
func (r *Runner) Cancel(id string) error {
	r.mu.RLock()
	run, exists := r.runs[id]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("run not found: %s", id)
	}

	// Signal cancellation via stopped channel (idempotent with sync.Once)
	run.cancelOnce.Do(func() {
		close(run.stopped)
	})

	// Also cancel the context for immediate effect
	run.cancel()

	return nil
}

// Subscribe returns a channel that receives log entries for a run.
func (r *Runner) Subscribe(runID string) (<-chan LogEntry, func()) {
	ch := make(chan LogEntry, 100)

	r.subMu.Lock()
	r.subscribers[runID] = append(r.subscribers[runID], ch)
	r.subMu.Unlock()

	// Unsubscribe function
	unsub := func() {
		r.subMu.Lock()
		defer r.subMu.Unlock()

		subs := r.subscribers[runID]
		for i, sub := range subs {
			if sub == ch {
				r.subscribers[runID] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		close(ch)
	}

	return ch, unsub
}

// StartDraining puts the runner into draining mode, preventing new workflow submissions.
func (r *Runner) StartDraining() {
	r.draining.Store(true)
}

// IsDraining returns true if the runner is in draining mode.
func (r *Runner) IsDraining() bool {
	return r.draining.Load()
}

// ActiveRunCount returns the number of currently active (running or pending) workflow runs.
func (r *Runner) ActiveRunCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, run := range r.runs {
		if run.Status == RunStatusRunning || run.Status == RunStatusPending {
			count++
		}
	}
	return count
}

// WaitForDrain waits for all active runs to complete or until the timeout is reached.
// Returns nil if all runs complete, or an error if the timeout expires with runs still active.
func (r *Runner) WaitForDrain(ctx context.Context, timeout time.Duration) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeoutCh:
			remaining := r.ActiveRunCount()
			if remaining > 0 {
				return fmt.Errorf("drain timeout: %d workflow(s) still running", remaining)
			}
			return nil
		case <-ticker.C:
			if r.ActiveRunCount() == 0 {
				return nil
			}
		}
	}
}
