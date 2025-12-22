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
	"fmt"
	"sync"
	"time"
)

// Metrics tracks security-related metrics for observability.
// This is a snapshot type returned by MetricsCollector.GetMetrics().
type Metrics struct {
	// Security event counters
	AccessGranted     int64
	AccessDenied      int64
	PermissionPrompts int64

	// Sandbox metrics
	SandboxCreated       int64
	SandboxFailed        int64
	SandboxFallbackUsed  int64
	SandboxAvailable     bool
	SandboxType          string
	SandboxLatencyMs     int64

	// Rate limiting metrics
	RateLimitHits    int64
	ThrottledRequests int64

	// Audit metrics
	AuditEventsLogged   int64
	AuditEventsDropped  int64
	AuditBufferCapacity int
	AuditBufferUsed     int

	// Profile metrics
	ActiveProfile        string
	ProfileSwitches      int64
	ProfileLoadFailures  int64

	// Resource metrics by type
	FileAccessRequests    int64
	NetworkAccessRequests int64
	CommandAccessRequests int64

	// Last event timestamp
	LastEventTime time.Time
}

// MetricsCollector collects and exports security metrics.
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics *Metrics
}

// NewMetricsCollector creates a new metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: &Metrics{
			LastEventTime: time.Now(),
		},
	}
}

// RecordAccessDecision records an access control decision.
func (m *MetricsCollector) RecordAccessDecision(decision AccessDecision, resourceType ResourceType) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.metrics.LastEventTime = time.Now()

	if decision.Allowed {
		m.metrics.AccessGranted++
	} else {
		m.metrics.AccessDenied++
	}

	switch resourceType {
	case ResourceTypeFile:
		m.metrics.FileAccessRequests++
	case ResourceTypeNetwork:
		m.metrics.NetworkAccessRequests++
	case ResourceTypeCommand:
		m.metrics.CommandAccessRequests++
	}
}

// RecordPermissionPrompt records a permission prompt shown to user.
func (m *MetricsCollector) RecordPermissionPrompt() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics.PermissionPrompts++
	m.metrics.LastEventTime = time.Now()
}

// RecordSandboxCreated records successful sandbox creation.
func (m *MetricsCollector) RecordSandboxCreated(sandboxType string, latencyMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics.SandboxCreated++
	m.metrics.SandboxAvailable = true
	m.metrics.SandboxType = sandboxType
	m.metrics.SandboxLatencyMs = latencyMs
	m.metrics.LastEventTime = time.Now()
}

// RecordSandboxFailed records sandbox creation failure.
func (m *MetricsCollector) RecordSandboxFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics.SandboxFailed++
	m.metrics.SandboxAvailable = false
	m.metrics.LastEventTime = time.Now()
}

// RecordSandboxFallback records fallback to non-sandboxed execution.
func (m *MetricsCollector) RecordSandboxFallback() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics.SandboxFallbackUsed++
	m.metrics.SandboxType = "fallback"
	m.metrics.LastEventTime = time.Now()
}

// RecordRateLimitHit records a rate limit being hit.
func (m *MetricsCollector) RecordRateLimitHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics.RateLimitHits++
	m.metrics.ThrottledRequests++
	m.metrics.LastEventTime = time.Now()
}

// RecordAuditEvent records audit logging activity.
func (m *MetricsCollector) RecordAuditEvent(logged bool, bufferUsed, bufferCapacity int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if logged {
		m.metrics.AuditEventsLogged++
	} else {
		m.metrics.AuditEventsDropped++
	}

	m.metrics.AuditBufferUsed = bufferUsed
	m.metrics.AuditBufferCapacity = bufferCapacity
	m.metrics.LastEventTime = time.Now()
}

// RecordProfileSwitch records a security profile switch.
func (m *MetricsCollector) RecordProfileSwitch(profileName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics.ActiveProfile = profileName
	m.metrics.ProfileSwitches++
	m.metrics.LastEventTime = time.Now()
}

// RecordProfileLoadFailure records a profile load failure.
func (m *MetricsCollector) RecordProfileLoadFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics.ProfileLoadFailures++
	m.metrics.LastEventTime = time.Now()
}

// GetMetrics returns a snapshot of current metrics.
func (m *MetricsCollector) GetMetrics() Metrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.metrics
}

// Reset resets all metrics (useful for testing).
func (m *MetricsCollector) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = &Metrics{
		LastEventTime: time.Now(),
	}
}

// PrometheusExporter exports metrics in Prometheus format.
type PrometheusExporter struct {
	collector *MetricsCollector
}

// NewPrometheusExporter creates a Prometheus metrics exporter.
func NewPrometheusExporter(collector *MetricsCollector) *PrometheusExporter {
	return &PrometheusExporter{
		collector: collector,
	}
}

// Export returns metrics in Prometheus text format.
func (e *PrometheusExporter) Export() string {
	metrics := e.collector.GetMetrics()

	output := `# HELP conductor_security_access_granted_total Total number of granted access requests
# TYPE conductor_security_access_granted_total counter
conductor_security_access_granted_total ` + formatInt64(metrics.AccessGranted) + `

# HELP conductor_security_access_denied_total Total number of denied access requests
# TYPE conductor_security_access_denied_total counter
conductor_security_access_denied_total ` + formatInt64(metrics.AccessDenied) + `

# HELP conductor_security_permission_prompts_total Total number of permission prompts shown
# TYPE conductor_security_permission_prompts_total counter
conductor_security_permission_prompts_total ` + formatInt64(metrics.PermissionPrompts) + `

# HELP conductor_security_sandbox_created_total Total number of sandboxes created
# TYPE conductor_security_sandbox_created_total counter
conductor_security_sandbox_created_total ` + formatInt64(metrics.SandboxCreated) + `

# HELP conductor_security_sandbox_failed_total Total number of failed sandbox creations
# TYPE conductor_security_sandbox_failed_total counter
conductor_security_sandbox_failed_total ` + formatInt64(metrics.SandboxFailed) + `

# HELP conductor_security_sandbox_fallback_total Total number of fallback sandbox uses
# TYPE conductor_security_sandbox_fallback_total counter
conductor_security_sandbox_fallback_total ` + formatInt64(metrics.SandboxFallbackUsed) + `

# HELP conductor_security_sandbox_available Is sandbox available (1=yes, 0=no)
# TYPE conductor_security_sandbox_available gauge
conductor_security_sandbox_available ` + formatBool(metrics.SandboxAvailable) + `

# HELP conductor_security_sandbox_latency_ms Sandbox creation latency in milliseconds
# TYPE conductor_security_sandbox_latency_ms gauge
conductor_security_sandbox_latency_ms ` + formatInt64(metrics.SandboxLatencyMs) + `

# HELP conductor_security_rate_limit_hits_total Total number of rate limit hits
# TYPE conductor_security_rate_limit_hits_total counter
conductor_security_rate_limit_hits_total ` + formatInt64(metrics.RateLimitHits) + `

# HELP conductor_security_throttled_requests_total Total number of throttled requests
# TYPE conductor_security_throttled_requests_total counter
conductor_security_throttled_requests_total ` + formatInt64(metrics.ThrottledRequests) + `

# HELP conductor_security_audit_events_logged_total Total number of audit events logged
# TYPE conductor_security_audit_events_logged_total counter
conductor_security_audit_events_logged_total ` + formatInt64(metrics.AuditEventsLogged) + `

# HELP conductor_security_audit_events_dropped_total Total number of dropped audit events
# TYPE conductor_security_audit_events_dropped_total counter
conductor_security_audit_events_dropped_total ` + formatInt64(metrics.AuditEventsDropped) + `

# HELP conductor_security_audit_buffer_used Current audit buffer usage
# TYPE conductor_security_audit_buffer_used gauge
conductor_security_audit_buffer_used ` + formatInt(metrics.AuditBufferUsed) + `

# HELP conductor_security_audit_buffer_capacity Audit buffer capacity
# TYPE conductor_security_audit_buffer_capacity gauge
conductor_security_audit_buffer_capacity ` + formatInt(metrics.AuditBufferCapacity) + `

# HELP conductor_security_profile_switches_total Total number of profile switches
# TYPE conductor_security_profile_switches_total counter
conductor_security_profile_switches_total ` + formatInt64(metrics.ProfileSwitches) + `

# HELP conductor_security_profile_load_failures_total Total number of profile load failures
# TYPE conductor_security_profile_load_failures_total counter
conductor_security_profile_load_failures_total ` + formatInt64(metrics.ProfileLoadFailures) + `

# HELP conductor_security_file_access_requests_total Total number of file access requests
# TYPE conductor_security_file_access_requests_total counter
conductor_security_file_access_requests_total ` + formatInt64(metrics.FileAccessRequests) + `

# HELP conductor_security_network_access_requests_total Total number of network access requests
# TYPE conductor_security_network_access_requests_total counter
conductor_security_network_access_requests_total ` + formatInt64(metrics.NetworkAccessRequests) + `

# HELP conductor_security_command_access_requests_total Total number of command access requests
# TYPE conductor_security_command_access_requests_total counter
conductor_security_command_access_requests_total ` + formatInt64(metrics.CommandAccessRequests) + `

# HELP conductor_security_last_event_timestamp_seconds Timestamp of last security event
# TYPE conductor_security_last_event_timestamp_seconds gauge
conductor_security_last_event_timestamp_seconds ` + formatTimestamp(metrics.LastEventTime) + `
`

	return output
}

func formatInt64(v int64) string {
	return fmt.Sprintf("%d", v)
}

func formatInt(v int) string {
	return fmt.Sprintf("%d", v)
}

func formatBool(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func formatTimestamp(t time.Time) string {
	return fmt.Sprintf("%d", t.Unix())
}
