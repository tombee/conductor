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

	"github.com/tombee/conductor/internal/binding"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/daemon/backend"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	daemonremote "github.com/tombee/conductor/internal/daemon/remote"
	"github.com/tombee/conductor/internal/remote"
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
	Workspace     string         `json:"workspace,omitempty"`  // Workspace used for profile resolution (SPEC-130)
	Profile       string         `json:"profile,omitempty"`    // Profile used for binding resolution (SPEC-130)

	// Runtime overrides (SPEC-156)
	Provider   string        `json:"provider,omitempty"`    // Provider override
	Model      string        `json:"model,omitempty"`       // Model tier override
	Timeout    time.Duration `json:"timeout,omitempty"`     // Step timeout override
	Security   string        `json:"security,omitempty"`    // Security profile name
	AllowHosts []string      `json:"allow_hosts,omitempty"` // Extended allowed hosts
	AllowPaths []string      `json:"allow_paths,omitempty"` // Extended allowed paths
	MCPDev     bool          `json:"mcp_dev,omitempty"`     // MCP development mode

	// Internal
	mu         sync.RWMutex // Protects mutable fields (Status, Progress, Output, Error, etc.)
	ctx        context.Context
	cancel     context.CancelFunc
	definition *workflow.Definition
	bindings   *binding.ResolvedBinding // Resolved bindings from profile (SPEC-130)
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
	Workspace     string         `json:"workspace,omitempty"` // Workspace used for profile resolution (SPEC-130)
	Profile       string         `json:"profile,omitempty"`   // Profile used for binding resolution (SPEC-130)

	// Runtime overrides (SPEC-156)
	Provider   string        `json:"provider,omitempty"`    // Provider override
	Model      string        `json:"model,omitempty"`       // Model tier override
	Timeout    time.Duration `json:"timeout,omitempty"`     // Step timeout override
	Security   string        `json:"security,omitempty"`    // Security profile name
	AllowHosts []string      `json:"allow_hosts,omitempty"` // Extended allowed hosts
	AllowPaths []string      `json:"allow_paths,omitempty"` // Extended allowed paths
	MCPDev     bool          `json:"mcp_dev,omitempty"`     // MCP development mode
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

// ListFilter contains filtering options for listing runs.
type ListFilter struct {
	Status   RunStatus
	Workflow string
	Limit    int
}

// RunOverrides contains runtime override parameters for a workflow run (SPEC-156).
type RunOverrides struct {
	Provider   string
	Model      string
	Timeout    time.Duration
	Security   string
	AllowHosts []string
	AllowPaths []string
	MCPDev     bool
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
	// Workspace selects the workspace for profile resolution (SPEC-130)
	// If empty, uses the default workspace
	Workspace string
	// Profile selects the profile within the workspace (SPEC-130)
	// If empty, uses the workspace's default profile
	Profile string
	// Provider overrides the default LLM provider for this run (SPEC-156)
	Provider string
	// Model overrides the model tier for LLM steps (SPEC-156)
	Model string
	// Timeout sets the step timeout duration (SPEC-156)
	Timeout time.Duration
	// DryRun returns execution plan without running the workflow (SPEC-156)
	DryRun bool
	// Security applies a security profile name (SPEC-156)
	Security string
	// AllowHosts extends allowed network hosts (SPEC-156)
	AllowHosts []string
	// AllowPaths extends allowed filesystem paths (SPEC-156)
	AllowPaths []string
	// MCPDev enables MCP development mode (SPEC-156)
	MCPDev bool
}

// Runner manages workflow executions by composing focused components.
type Runner struct {
	// Focused components
	state     *StateManager
	lifecycle *LifecycleManager
	logs      *LogAggregator

	// Concurrency control
	semaphore  chan struct{}
	defTimeout time.Duration

	// Execution adapter for step execution (required for workflow execution)
	mu      sync.RWMutex
	adapter ExecutionAdapter

	// Remote workflow fetcher (optional)
	fetcher *daemonremote.Fetcher

	// Metrics collector for observability (optional)
	metrics MetricsCollector

	// Configuration for profile resolution (SPEC-130)
	config *config.Config

	// Binding resolver for profile-based configuration (SPEC-130)
	resolver *binding.Resolver

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

	// Create default components
	state := NewStateManager(be)
	lifecycle := NewLifecycleManager(nil, cm, nil)
	logs := NewLogAggregator()

	r := &Runner{
		state:      state,
		lifecycle:  lifecycle,
		logs:       logs,
		semaphore:  make(chan struct{}, cfg.MaxParallel),
		defTimeout: cfg.DefaultTimeout,
	}

	// Apply options
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// SetFetcher sets the remote workflow fetcher.
func (r *Runner) SetFetcher(f *daemonremote.Fetcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetcher = f
}

// SetAdapter sets the execution adapter for step execution.
func (r *Runner) SetAdapter(adapter ExecutionAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapter = adapter
}

// ToolRegistry returns the runner's tool registry.
func (r *Runner) ToolRegistry() *tools.Registry {
	return r.lifecycle.ToolRegistry()
}

// SetMetrics sets the metrics collector for observability.
func (r *Runner) SetMetrics(metrics MetricsCollector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = metrics
}

// Submit submits a workflow for execution and returns an immutable snapshot.
func (r *Runner) Submit(ctx context.Context, req SubmitRequest) (*RunSnapshot, error) {
	// Handle dry-run requests separately (SPEC-156 Phase 4)
	if req.DryRun {
		return r.DryRun(ctx, req)
	}

	var workflowYAML []byte
	var sourceURL string

	// Check if this is a remote workflow
	if req.RemoteRef != "" {
		if !remote.IsRemote(req.RemoteRef) {
			return nil, fmt.Errorf("invalid remote reference: %s", req.RemoteRef)
		}

		r.mu.RLock()
		fetcher := r.fetcher
		r.mu.RUnlock()

		if fetcher == nil {
			return nil, fmt.Errorf("remote workflows not supported (fetcher not configured)")
		}

		result, err := fetcher.Fetch(ctx, req.RemoteRef, req.NoCache)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch remote workflow: %w", err)
		}

		workflowYAML = result.Content
		sourceURL = result.SourceURL
	} else {
		workflowYAML = req.WorkflowYAML
	}

	// Parse workflow
	def, err := workflow.ParseDefinition(workflowYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Resolve profile and bindings (SPEC-130)
	workspace, profile, resolvedBindings, err := r.resolveProfile(ctx, req.Workspace, req.Profile, def)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve profile bindings: %w", err)
	}

	// Build runtime overrides from request (SPEC-156)
	var overrides *RunOverrides
	if req.Provider != "" || req.Model != "" || req.Timeout != 0 || req.Security != "" ||
		len(req.AllowHosts) > 0 || len(req.AllowPaths) > 0 || req.MCPDev {
		overrides = &RunOverrides{
			Provider:   req.Provider,
			Model:      req.Model,
			Timeout:    req.Timeout,
			Security:   req.Security,
			AllowHosts: req.AllowHosts,
			AllowPaths: req.AllowPaths,
			MCPDev:     req.MCPDev,
		}
	}

	// Create run via StateManager
	run, err := r.state.CreateRun(ctx, def, req.Inputs, sourceURL, workspace, profile, resolvedBindings, overrides)
	if err != nil {
		return nil, err
	}

	// Increment queue depth for metrics
	r.mu.RLock()
	metrics := r.metrics
	r.mu.RUnlock()
	if metrics != nil {
		metrics.IncrementQueueDepth()
	}

	// Create initial snapshot before background execution starts
	snapshot := r.state.Snapshot(run)

	// Start execution in background
	go r.execute(run)

	return snapshot, nil
}

// resolveProfile determines the workspace and profile to use, then resolves bindings.
// Returns: workspace, profile, resolvedBindings, error
func (r *Runner) resolveProfile(ctx context.Context, requestedWorkspace, requestedProfile string, def *workflow.Definition) (string, string, *binding.ResolvedBinding, error) {
	// If no resolver configured, skip profile resolution (backward compatibility)
	r.mu.RLock()
	resolver := r.resolver
	cfg := r.config
	r.mu.RUnlock()

	if resolver == nil || cfg == nil {
		// No profile support - return empty workspace/profile and nil bindings
		return "", "", nil, nil
	}

	// Determine workspace (default to "default" if not specified)
	workspace := requestedWorkspace
	if workspace == "" {
		workspace = "default"
	}

	// Look up workspace in config
	ws, exists := cfg.Workspaces[workspace]
	if !exists {
		return "", "", nil, fmt.Errorf("workspace not found: %s", workspace)
	}

	// Determine profile within workspace
	profileName := requestedProfile
	if profileName == "" {
		// Use workspace's default profile
		if ws.DefaultProfile != "" {
			profileName = ws.DefaultProfile
		} else {
			profileName = "default"
		}
	}

	// Look up profile in workspace
	prof, exists := ws.Profiles[profileName]
	if !exists {
		return "", "", nil, fmt.Errorf("profile not found: %s/%s", workspace, profileName)
	}

	// Create resolution context
	resCtx := &binding.ResolutionContext{
		Profile:   &prof,
		Workflow:  def,
		RunID:     "", // Will be set by CreateRun
		Workspace: workspace,
	}

	// Resolve bindings using the resolver
	resolvedBindings, err := resolver.Resolve(ctx, resCtx)
	if err != nil {
		return "", "", nil, fmt.Errorf("binding resolution failed for profile %s/%s: %w", workspace, profileName, err)
	}

	return workspace, profileName, resolvedBindings, nil
}

// Get returns an immutable snapshot of a run by ID.
func (r *Runner) Get(id string) (*RunSnapshot, error) {
	return r.state.GetRun(id)
}

// List returns immutable snapshots of all runs, optionally filtered.
func (r *Runner) List(filter ListFilter) []*RunSnapshot {
	return r.state.ListRuns(filter)
}

// Cancel cancels a running workflow.
func (r *Runner) Cancel(id string) error {
	run, exists := r.state.GetRunInternal(id)
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
	return r.logs.Subscribe(runID)
}

// StartDraining puts the runner into draining mode.
func (r *Runner) StartDraining() {
	r.draining.Store(true)
}

// IsDraining returns true if the runner is in draining mode.
func (r *Runner) IsDraining() bool {
	return r.draining.Load()
}

// ActiveRunCount returns the number of currently active runs.
func (r *Runner) ActiveRunCount() int {
	return r.state.ActiveRunCount()
}

// WaitForDrain waits for all active runs to complete or until the timeout is reached.
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

// addLog is a helper that adds a log entry via the LogAggregator.
func (r *Runner) addLog(run *Run, level, message, stepID string) {
	r.logs.AddLog(run, level, message, stepID)
}

// Internal accessors for executor.go

// getBackend returns the backend from StateManager.
func (r *Runner) getBackend() backend.Backend {
	return r.state.backend
}

// toBackendRun converts a Run to backend format.
func (r *Runner) toBackendRun(run *Run) *backend.Run {
	return r.state.toBackendRun(run)
}

// snapshotRun creates an immutable snapshot of a run.
func (r *Runner) snapshotRun(run *Run) *RunSnapshot {
	return r.state.snapshotRun(run)
}
