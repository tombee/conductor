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

// Package security provides the security framework for Conductor agents.
//
// This package implements a multi-layered security architecture that constrains
// agent access to filesystem, network, and process execution through configurable
// security profiles and layered isolation mechanisms.
package security

import (
	"time"
)

// SecurityProfile represents a named security configuration.
type SecurityProfile struct {
	// Name is the unique identifier for this profile
	Name string `yaml:"name" json:"name"`

	// Filesystem defines file access restrictions
	Filesystem FilesystemConfig `yaml:"filesystem" json:"filesystem"`

	// Network defines network access restrictions
	Network NetworkConfig `yaml:"network" json:"network"`

	// Execution defines process execution restrictions
	Execution ExecutionConfig `yaml:"execution" json:"execution"`

	// Isolation defines the isolation level
	Isolation IsolationLevel `yaml:"isolation" json:"isolation"`

	// Limits defines resource limits
	Limits ResourceLimits `yaml:"limits" json:"limits"`
}

// FilesystemConfig defines file access restrictions.
type FilesystemConfig struct {
	// Read lists paths that can be read
	// Empty list means no restrictions
	Read []string `yaml:"read,omitempty" json:"read,omitempty"`

	// Write lists paths that can be written
	// Empty list means no restrictions
	Write []string `yaml:"write,omitempty" json:"write,omitempty"`

	// Deny lists paths that are always denied (higher priority than Read/Write)
	Deny []string `yaml:"deny,omitempty" json:"deny,omitempty"`
}

// NetworkConfig defines network access restrictions.
type NetworkConfig struct {
	// Allow lists hosts that can be contacted
	// Empty list means no network access allowed
	Allow []string `yaml:"allow,omitempty" json:"allow,omitempty"`

	// DenyPrivate blocks RFC1918, link-local, localhost
	DenyPrivate bool `yaml:"deny_private" json:"deny_private"`

	// DenyAll blocks all network access
	DenyAll bool `yaml:"deny_all" json:"deny_all"`
}

// ExecutionConfig defines process execution restrictions.
type ExecutionConfig struct {
	// AllowedCommands lists commands that can be executed
	// Empty list means all commands allowed
	AllowedCommands []string `yaml:"allowed_commands,omitempty" json:"allowed_commands,omitempty"`

	// DeniedCommands lists commands that are explicitly denied (higher priority)
	DeniedCommands []string `yaml:"denied_commands,omitempty" json:"denied_commands,omitempty"`

	// Sandbox indicates whether sandboxed execution is required
	Sandbox bool `yaml:"sandbox" json:"sandbox"`
}

// IsolationLevel represents the degree of process isolation.
type IsolationLevel string

const (
	// IsolationNone means no process isolation
	IsolationNone IsolationLevel = "none"

	// IsolationSandbox means tools run in sandboxed subprocess
	IsolationSandbox IsolationLevel = "sandbox"
)

// ResourceLimits defines resource constraints for tool execution.
type ResourceLimits struct {
	// TimeoutPerTool is the maximum duration for a single tool execution
	TimeoutPerTool time.Duration `yaml:"timeout_per_tool" json:"timeout_per_tool"`

	// TotalRuntime is the maximum total runtime for all tools in a workflow
	TotalRuntime time.Duration `yaml:"total_runtime,omitempty" json:"total_runtime,omitempty"`

	// MaxMemory is the maximum memory usage in bytes
	MaxMemory int64 `yaml:"max_memory,omitempty" json:"max_memory,omitempty"`

	// MaxProcesses is the maximum number of processes
	MaxProcesses int `yaml:"max_processes,omitempty" json:"max_processes,omitempty"`

	// MaxFileSize is the maximum file size that can be read or written
	MaxFileSize int64 `yaml:"max_file_size,omitempty" json:"max_file_size,omitempty"`
}

// AccessRequest represents a request to access a protected resource.
type AccessRequest struct {
	// WorkflowID identifies the workflow making the request
	WorkflowID string

	// StepID identifies the step within the workflow
	StepID string

	// ToolName is the name of the tool being executed
	ToolName string

	// ResourceType is the type of resource being accessed (file, network, command)
	ResourceType ResourceType

	// Resource is the specific resource being accessed (path, URL, command)
	Resource string

	// Action is the action being performed (read, write, execute, connect)
	Action AccessAction
}

// ResourceType represents the type of resource being accessed.
type ResourceType string

const (
	// ResourceTypeFile represents filesystem access
	ResourceTypeFile ResourceType = "file"

	// ResourceTypeNetwork represents network access
	ResourceTypeNetwork ResourceType = "network"

	// ResourceTypeCommand represents command execution
	ResourceTypeCommand ResourceType = "command"
)

// AccessAction represents the action being performed on a resource.
type AccessAction string

const (
	// ActionRead represents read access
	ActionRead AccessAction = "read"

	// ActionWrite represents write access
	ActionWrite AccessAction = "write"

	// ActionExecute represents execution access
	ActionExecute AccessAction = "execute"

	// ActionConnect represents network connection
	ActionConnect AccessAction = "connect"
)

// AccessDecision represents the result of an access control check.
type AccessDecision struct {
	// Allowed indicates whether the access is allowed
	Allowed bool

	// Reason explains why the access was allowed or denied
	Reason string

	// Profile is the security profile that made the decision
	Profile string
}

// SecurityConfig is the top-level configuration for the security system.
// This gets embedded in internal/config/config.go.
type SecurityConfig struct {
	// DefaultProfile is the default security profile to use
	DefaultProfile string `yaml:"default_profile" json:"default_profile"`

	// Policy defines organization-level security policy
	Policy PolicyConfig `yaml:"policy,omitempty" json:"policy,omitempty"`

	// Profiles defines custom security profiles
	Profiles map[string]*SecurityProfile `yaml:"profiles,omitempty" json:"profiles,omitempty"`

	// Audit configures audit logging
	Audit AuditConfig `yaml:"audit,omitempty" json:"audit,omitempty"`

	// PrewarmSandbox enables sandbox pre-warming for latency-sensitive workflows
	PrewarmSandbox bool `yaml:"prewarm_sandbox,omitempty" json:"prewarm_sandbox,omitempty"`

	// DNS configures DNS monitoring and exfiltration prevention
	DNS DNSSecurityConfig `yaml:"dns,omitempty" json:"dns,omitempty"`

	// Metrics configures security metrics collection
	Metrics MetricsConfig `yaml:"metrics,omitempty" json:"metrics,omitempty"`

	// Override configures emergency security override system
	Override OverrideConfig `yaml:"override,omitempty" json:"override,omitempty"`
}

// PolicyConfig defines organization-level security policy.
type PolicyConfig struct {
	// MinimumProfile is the minimum security profile that can be used
	// Users cannot downgrade below this level
	MinimumProfile string `yaml:"minimum_profile,omitempty" json:"minimum_profile,omitempty"`

	// RequireAuditLog requires audit logging to be enabled
	RequireAuditLog bool `yaml:"require_audit_log,omitempty" json:"require_audit_log,omitempty"`
}

// AuditConfig configures security audit logging.
type AuditConfig struct {
	// Enabled controls whether audit logging is active
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Destinations defines where audit events are sent
	Destinations []AuditDestination `yaml:"destinations,omitempty" json:"destinations,omitempty"`

	// Rotation configures audit log rotation
	Rotation AuditRotationConfig `yaml:"rotation,omitempty" json:"rotation,omitempty"`
}

// AuditDestination represents a destination for audit logs.
type AuditDestination struct {
	// Type is the destination type (file, syslog, webhook)
	Type string `yaml:"type" json:"type"`

	// Path is the file path (for type=file)
	Path string `yaml:"path,omitempty" json:"path,omitempty"`

	// Format is the output format (json, text)
	Format string `yaml:"format,omitempty" json:"format,omitempty"`

	// Facility is the syslog facility (for type=syslog)
	Facility string `yaml:"facility,omitempty" json:"facility,omitempty"`

	// Severity is the syslog severity (for type=syslog)
	Severity string `yaml:"severity,omitempty" json:"severity,omitempty"`

	// URL is the webhook URL (for type=webhook)
	URL string `yaml:"url,omitempty" json:"url,omitempty"`

	// Headers are HTTP headers to send with webhook requests
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
}

// MetricsConfig configures security metrics collection.
type MetricsConfig struct {
	// Enabled controls whether metrics collection is active
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Namespace is the Prometheus namespace for metrics (default: conductor)
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

// OverrideConfig configures the security override system.
type OverrideConfig struct {
	// Enabled controls whether security overrides are allowed
	Enabled bool `yaml:"enabled" json:"enabled"`

	// DefaultTTL is the default duration for overrides
	DefaultTTL time.Duration `yaml:"default_ttl,omitempty" json:"default_ttl,omitempty"`

	// MaxTTL is the maximum allowed override duration
	MaxTTL time.Duration `yaml:"max_ttl,omitempty" json:"max_ttl,omitempty"`

	// RequireReason requires a reason when creating overrides
	RequireReason bool `yaml:"require_reason" json:"require_reason"`
}

// AuditRotationConfig configures audit log rotation.
type AuditRotationConfig struct {
	// Enabled controls whether log rotation is active
	Enabled bool `yaml:"enabled" json:"enabled"`

	// MaxSizeMB is the maximum file size before rotation in megabytes
	MaxSizeMB int64 `yaml:"max_size_mb,omitempty" json:"max_size_mb,omitempty"`

	// MaxAgeDays is the number of days to retain rotated logs
	MaxAgeDays int `yaml:"max_age_days,omitempty" json:"max_age_days,omitempty"`

	// MaxBackups is the maximum number of rotated logs to keep
	MaxBackups int `yaml:"max_backups,omitempty" json:"max_backups,omitempty"`

	// Compress enables gzip compression of rotated logs
	Compress bool `yaml:"compress" json:"compress"`
}
