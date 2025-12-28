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
	"sync"
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

	// eventLogger logs security events
	eventLogger EventLogger

	// metricsCollector records security metrics
	metricsCollector *MetricsCollector
}

// NewWorkflowSecurityContext creates a new security context for a workflow.
func NewWorkflowSecurityContext(workflowID string, profile *SecurityProfile, eventLogger EventLogger) *WorkflowSecurityContext {
	return &WorkflowSecurityContext{
		WorkflowID:  workflowID,
		Profile:     profile,
		eventLogger: eventLogger,
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

// Cleanup releases any resources held by the security context.
// Must be called when workflow completes.
func (sc *WorkflowSecurityContext) Cleanup() error {
	// No resources to clean up currently
	return nil
}

// LogEvent logs a security event for this workflow.
func (sc *WorkflowSecurityContext) LogEvent(event SecurityEvent) {
	// Ensure workflow ID is set
	event.WorkflowID = sc.WorkflowID
	sc.eventLogger.Log(event)
}
