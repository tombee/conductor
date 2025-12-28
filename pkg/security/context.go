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

package security

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tombee/conductor/pkg/security/sandbox"
)

// workflowContextKey is the type for workflow context keys.
type workflowContextKey int

const (
	// workflowSecurityContextKey is the context key for WorkflowSecurityContext
	workflowSecurityContextKey workflowContextKey = iota
)

// WorkflowSecurityContext holds isolated security state for a workflow execution.
//
// Each workflow gets its own WorkflowSecurityContext containing:
//   - Security profile and policy
//   - Sandbox instance (lazily initialized)
//   - Audit logger
//   - Resource tracking
//
// WorkflowSecurityContext is thread-safe and can be used concurrently.
type WorkflowSecurityContext struct {
	mu sync.Mutex

	// WorkflowID identifies the workflow
	WorkflowID string

	// Profile is the active security profile
	Profile *SecurityProfile

	// sandbox is the isolated execution environment (lazily initialized)
	sandbox sandbox.Sandbox

	// sandboxFactory creates sandboxes
	sandboxFactory sandbox.Factory

	// sandboxConfig is the configuration for sandbox creation
	sandboxConfig sandbox.Config

	// eventLogger logs security events
	eventLogger EventLogger

	// metricsCollector records security metrics
	metricsCollector *MetricsCollector

	// degraded indicates if running in degraded mode (no sandbox available)
	degraded bool

	// prewarm enables sandbox pre-warming
	prewarm bool
}

// NewWorkflowSecurityContext creates a new security context for a workflow.
func NewWorkflowSecurityContext(workflowID string, profile *SecurityProfile, eventLogger EventLogger, prewarm bool) *WorkflowSecurityContext {
	return &WorkflowSecurityContext{
		WorkflowID:  workflowID,
		Profile:     profile,
		eventLogger: eventLogger,
		prewarm:     prewarm,
	}
}

// SetMetricsCollector sets the metrics collector for this context.
func (sc *WorkflowSecurityContext) SetMetricsCollector(collector *MetricsCollector) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.metricsCollector = collector
}

// WithWorkflowSecurityContext adds a WorkflowSecurityContext to a context.
func WithWorkflowSecurityContext(ctx context.Context, secCtx *WorkflowSecurityContext) context.Context {
	return context.WithValue(ctx, workflowSecurityContextKey, secCtx)
}

// FromWorkflowContext extracts a WorkflowSecurityContext from a context.
// Returns nil if no WorkflowSecurityContext is present.
func FromWorkflowContext(ctx context.Context) *WorkflowSecurityContext {
	secCtx, _ := ctx.Value(workflowSecurityContextKey).(*WorkflowSecurityContext)
	return secCtx
}

// GetSandbox returns the sandbox, creating it lazily if needed.
//
// Returns (nil, false) if sandbox is not required or unavailable.
// Returns (sandbox, true) if sandbox is available.
func (sc *WorkflowSecurityContext) GetSandbox(ctx context.Context) (sandbox.Sandbox, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Check if sandbox is required by profile
	if sc.Profile.Isolation != IsolationSandbox {
		return nil, false
	}

	// Return existing sandbox if already created
	if sc.sandbox != nil {
		return sc.sandbox, true
	}

	// Try to create sandbox
	if err := sc.initializeSandbox(ctx); err != nil {
		// Log warning about degraded mode
		sc.eventLogger.Log(SecurityEvent{
			EventType:  EventViolation,
			WorkflowID: sc.WorkflowID,
			Reason:     fmt.Sprintf("sandbox unavailable: %v", err),
			Profile:    sc.Profile.Name,
			Decision:   "degraded",
		})
		sc.degraded = true
		return nil, false
	}

	return sc.sandbox, true
}

// PrewarmSandbox creates the sandbox immediately if pre-warming is enabled.
// Should be called during workflow initialization to reduce latency.
func (sc *WorkflowSecurityContext) PrewarmSandbox(ctx context.Context) error {
	if !sc.prewarm {
		return nil
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.Profile.Isolation != IsolationSandbox {
		return nil
	}

	if sc.sandbox != nil {
		return nil // Already initialized
	}

	return sc.initializeSandbox(ctx)
}

// initializeSandbox creates and configures a sandbox instance.
// Must be called with sc.mu held.
func (sc *WorkflowSecurityContext) initializeSandbox(ctx context.Context) error {
	// Determine sandbox factory to use
	factory := sc.determineSandboxFactory(ctx)
	if factory == nil {
		// Record sandbox creation failure
		if sc.metricsCollector != nil {
			sc.metricsCollector.RecordSandboxFailed()
		}
		return fmt.Errorf("no sandbox factory available")
	}

	// Build sandbox configuration
	cfg := sc.buildSandboxConfig()

	// Measure sandbox creation time
	startTime := currentTimeMillis()

	// Create sandbox
	sb, err := factory.Create(ctx, cfg)
	latencyMs := currentTimeMillis() - startTime

	if err != nil {
		// Record sandbox creation failure
		if sc.metricsCollector != nil {
			sc.metricsCollector.RecordSandboxFailed()
		}
		return fmt.Errorf("failed to create sandbox: %w", err)
	}

	sc.sandbox = sb
	sc.sandboxFactory = factory

	// Record successful sandbox creation
	if sc.metricsCollector != nil {
		sc.metricsCollector.RecordSandboxCreated(string(factory.Type()), latencyMs)
	}

	// Log sandbox creation
	sc.eventLogger.Log(SecurityEvent{
		EventType:  EventAccessGranted,
		WorkflowID: sc.WorkflowID,
		Profile:    sc.Profile.Name,
		Decision:   "allowed",
		Reason:     fmt.Sprintf("sandbox created (type: %s)", factory.Type()),
	})

	return nil
}

// determineSandboxFactory selects the best available sandbox implementation.
// Returns nil if no sandbox is available.
func (sc *WorkflowSecurityContext) determineSandboxFactory(ctx context.Context) sandbox.Factory {
	// Try Docker/Podman first (preferred)
	dockerFactory := sandbox.NewDockerFactory()
	if dockerFactory.Available(ctx) {
		return dockerFactory
	}

	// Fall back to process-level isolation
	// This provides degraded isolation but better than nothing
	fallbackFactory := sandbox.NewFallbackFactory()
	if fallbackFactory.Available(ctx) {
		sc.degraded = true // Mark as degraded
		// Record fallback usage
		if sc.metricsCollector != nil {
			sc.metricsCollector.RecordSandboxFallback()
		}
		return fallbackFactory
	}

	return nil
}

// buildSandboxConfig constructs sandbox configuration from security profile.
func (sc *WorkflowSecurityContext) buildSandboxConfig() sandbox.Config {
	cfg := sandbox.Config{
		WorkflowID: sc.WorkflowID,
		// WorkDir will be set by caller based on workflow context
		Env: make(map[string]string),
	}

	// Configure network mode
	if sc.Profile.Network.DenyAll {
		cfg.NetworkMode = sandbox.NetworkNone
	} else if len(sc.Profile.Network.Allow) > 0 {
		cfg.NetworkMode = sandbox.NetworkFiltered
		cfg.AllowedHosts = sc.Profile.Network.Allow
	} else {
		cfg.NetworkMode = sandbox.NetworkFull
	}

	// Configure resource limits
	cfg.ResourceLimits = sandbox.ResourceLimits{
		MaxMemory:    sc.Profile.Limits.MaxMemory,
		MaxCPU:       0, // Will be calculated from profile
		MaxProcesses: sc.Profile.Limits.MaxProcesses,
		MaxFileSize:  sc.Profile.Limits.MaxFileSize,
	}

	// Configure filesystem paths
	cfg.ReadOnlyPaths = sc.Profile.Filesystem.Read
	cfg.WritablePaths = sc.Profile.Filesystem.Write

	return cfg
}

// IsDegraded returns true if running in degraded mode.
//
// Degraded mode occurs when:
//   - Sandbox is required but container runtime unavailable
//   - Falling back to process-level isolation
//   - Still enforcing policy via SecurityInterceptor
func (sc *WorkflowSecurityContext) IsDegraded() bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.degraded
}

// Cleanup releases sandbox resources.
// Must be called when workflow completes.
func (sc *WorkflowSecurityContext) Cleanup() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if sc.sandbox != nil {
		if err := sc.sandbox.Cleanup(); err != nil {
			return fmt.Errorf("failed to cleanup sandbox: %w", err)
		}
		sc.sandbox = nil
	}

	return nil
}

// LogEvent logs a security event for this workflow.
func (sc *WorkflowSecurityContext) LogEvent(event SecurityEvent) {
	// Ensure workflow ID is set
	event.WorkflowID = sc.WorkflowID

	// Add degraded marker if applicable
	if sc.degraded && event.Reason != "" {
		event.Reason = event.Reason + " (degraded mode)"
	} else if sc.degraded {
		event.Reason = "degraded mode active"
	}

	sc.eventLogger.Log(event)
}

// currentTimeMillis returns the current time in milliseconds since epoch.
func currentTimeMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
