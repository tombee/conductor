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
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tombee/conductor/pkg/errors"
)

// Manager orchestrates security across the system.
type Manager interface {
	// LoadProfile loads and validates a security profile
	LoadProfile(name string) (*SecurityProfile, error)

	// GetActiveProfile returns the current profile
	GetActiveProfile() *SecurityProfile

	// CheckAccess validates an access request against policy
	CheckAccess(req AccessRequest) AccessDecision

	// CreateContext creates a security context for a workflow
	CreateContext(workflowID string, prewarm bool) *WorkflowSecurityContext

	// LogEvent records a security event
	LogEvent(event SecurityEvent)
}

// manager implements the Manager interface.
type manager struct {
	mu               sync.RWMutex
	activeProfile    *SecurityProfile
	customProfiles   map[string]*SecurityProfile
	eventLogger      EventLogger
	prewarmSandbox   bool
	metricsCollector *MetricsCollector
}

// NewManager creates a new security manager with the given configuration.
func NewManager(config *SecurityConfig) (Manager, error) {
	if config == nil {
		config = &SecurityConfig{
			DefaultProfile: ProfileStandard,
		}
	}

	m := &manager{
		customProfiles: config.Profiles,
		eventLogger:    NewEventLogger(config.Audit),
		prewarmSandbox: config.PrewarmSandbox,
	}

	// Load the default profile
	profile, err := LoadProfile(config.DefaultProfile, config.Profiles)
	if err != nil {
		return nil, fmt.Errorf("failed to load default profile: %w", err)
	}

	m.activeProfile = profile
	return m, nil
}

// SetMetricsCollector sets the metrics collector for recording security metrics.
func (m *manager) SetMetricsCollector(collector *MetricsCollector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metricsCollector = collector
}

// LoadProfile loads and validates a security profile.
func (m *manager) LoadProfile(name string) (*SecurityProfile, error) {
	profile, err := LoadProfile(name, m.customProfiles)
	if err != nil {
		return nil, &errors.NotFoundError{
			Resource: "security profile",
			ID:       name,
		}
	}

	m.mu.Lock()
	m.activeProfile = profile
	if m.metricsCollector != nil {
		m.metricsCollector.RecordProfileSwitch(profile.Name)
	}
	m.mu.Unlock()

	return profile, nil
}

// GetActiveProfile returns the current profile.
func (m *manager) GetActiveProfile() *SecurityProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return copyProfile(m.activeProfile)
}

// CheckAccess validates an access request against policy.
func (m *manager) CheckAccess(req AccessRequest) AccessDecision {
	m.mu.RLock()
	profile := m.activeProfile
	collector := m.metricsCollector
	m.mu.RUnlock()

	var decision AccessDecision
	switch req.ResourceType {
	case ResourceTypeFile:
		decision = m.checkFileAccess(profile, req)
	case ResourceTypeNetwork:
		decision = m.checkNetworkAccess(profile, req)
	case ResourceTypeCommand:
		decision = m.checkCommandAccess(profile, req)
	default:
		decision = AccessDecision{
			Allowed: false,
			Reason:  fmt.Sprintf("unknown resource type: %s", req.ResourceType),
			Profile: profile.Name,
		}
	}

	// Record access decision metrics
	if collector != nil {
		collector.RecordAccessDecision(decision, req.ResourceType)
	}

	return decision
}

// CreateContext creates a security context for a workflow.
func (m *manager) CreateContext(workflowID string, prewarm bool) *WorkflowSecurityContext {
	m.mu.RLock()
	profile := m.activeProfile
	m.mu.RUnlock()

	// Use manager's prewarm setting if not overridden
	if !prewarm {
		prewarm = m.prewarmSandbox
	}

	return NewWorkflowSecurityContext(workflowID, profile, m.eventLogger, prewarm)
}

// LogEvent records a security event.
func (m *manager) LogEvent(event SecurityEvent) {
	if m.eventLogger != nil {
		m.eventLogger.Log(event)
	}
}

// checkFileAccess checks if file access is allowed.
func (m *manager) checkFileAccess(profile *SecurityProfile, req AccessRequest) AccessDecision {
	path := req.Resource

	// Expand home directory if present
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		}
	}

	// Clean and resolve the path
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return AccessDecision{
			Allowed: false,
			Reason:  "failed to resolve path",
			Profile: profile.Name,
		}
	}

	// Expand home directory in deny patterns
	expandedDenyPaths := expandHomePaths(profile.Filesystem.Deny)

	// Check deny list first (highest priority)
	for _, denyPattern := range expandedDenyPaths {
		if matchesPath(absPath, denyPattern) {
			return AccessDecision{
				Allowed: false,
				Reason:  "path is explicitly denied by security policy",
				Profile: profile.Name,
			}
		}
	}

	// Determine which allowlist to check based on action
	var allowlist []string
	switch req.Action {
	case ActionRead:
		allowlist = profile.Filesystem.Read
	case ActionWrite:
		allowlist = profile.Filesystem.Write
	default:
		return AccessDecision{
			Allowed: false,
			Reason:  fmt.Sprintf("unknown action for file access: %s", req.Action),
			Profile: profile.Name,
		}
	}

	// If allowlist is empty, allow all (unless profile is air-gapped)
	if len(allowlist) == 0 {
		if profile.Name == ProfileAirGapped {
			return AccessDecision{
				Allowed: false,
				Reason:  "air-gapped profile requires explicit file permissions",
				Profile: profile.Name,
			}
		}
		return AccessDecision{
			Allowed: true,
			Reason:  "no restrictions configured",
			Profile: profile.Name,
		}
	}

	// Expand home directory and relative paths in allowlist
	expandedAllowPaths := expandHomePaths(allowlist)

	// Check if path is within allowlist
	for _, allowedPattern := range expandedAllowPaths {
		if matchesPath(absPath, allowedPattern) {
			return AccessDecision{
				Allowed: true,
				Reason:  fmt.Sprintf("path allowed by profile: %s", allowedPattern),
				Profile: profile.Name,
			}
		}
	}

	return AccessDecision{
		Allowed: false,
		Reason:  "path not allowed by security policy",
		Profile: profile.Name,
	}
}

// checkNetworkAccess checks if network access is allowed.
func (m *manager) checkNetworkAccess(profile *SecurityProfile, req AccessRequest) AccessDecision {
	host := req.Resource

	// If deny_all is set, block all network access
	if profile.Network.DenyAll {
		return AccessDecision{
			Allowed: false,
			Reason:  "all network access denied by profile",
			Profile: profile.Name,
		}
	}

	// Check if host is a private IP (if deny_private is enabled)
	if profile.Network.DenyPrivate {
		if isPrivateIP(host) {
			return AccessDecision{
				Allowed: false,
				Reason:  "private IP addresses are denied by profile",
				Profile: profile.Name,
			}
		}
	}

	// If allowlist is empty, allow all
	if len(profile.Network.Allow) == 0 {
		return AccessDecision{
			Allowed: true,
			Reason:  "no network restrictions configured",
			Profile: profile.Name,
		}
	}

	// Check if host is in allowlist
	for _, allowedHost := range profile.Network.Allow {
		if matchesHost(host, allowedHost) {
			return AccessDecision{
				Allowed: true,
				Reason:  fmt.Sprintf("host allowed by profile: %s", allowedHost),
				Profile: profile.Name,
			}
		}
	}

	return AccessDecision{
		Allowed: false,
		Reason:  "host not allowed by security policy",
		Profile: profile.Name,
	}
}

// checkCommandAccess checks if command execution is allowed.
func (m *manager) checkCommandAccess(profile *SecurityProfile, req AccessRequest) AccessDecision {
	command := req.Resource

	// Check deny list first (highest priority)
	for _, deniedCmd := range profile.Execution.DeniedCommands {
		if matchesCommand(command, deniedCmd) {
			return AccessDecision{
				Allowed: false,
				Reason:  "command is explicitly denied by security policy",
				Profile: profile.Name,
			}
		}
	}

	// If allowlist is empty, allow all
	if len(profile.Execution.AllowedCommands) == 0 {
		return AccessDecision{
			Allowed: true,
			Reason:  "no command restrictions configured",
			Profile: profile.Name,
		}
	}

	// Check if command is in allowlist
	for _, allowedCmd := range profile.Execution.AllowedCommands {
		if matchesCommand(command, allowedCmd) {
			return AccessDecision{
				Allowed: true,
				Reason:  fmt.Sprintf("command allowed by profile: %s", allowedCmd),
				Profile: profile.Name,
			}
		}
	}

	return AccessDecision{
		Allowed: false,
		Reason:  "command not allowed by security policy",
		Profile: profile.Name,
	}
}

// expandHomePaths expands ~ to home directory in paths.
func expandHomePaths(paths []string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return paths
	}

	expanded := make([]string, len(paths))
	for i, p := range paths {
		if strings.HasPrefix(p, "~/") {
			expanded[i] = filepath.Join(home, p[2:])
		} else if p == "~" {
			expanded[i] = home
		} else {
			// Also resolve relative paths to absolute
			if abs, err := filepath.Abs(p); err == nil {
				expanded[i] = abs
			} else {
				expanded[i] = p
			}
		}
	}
	return expanded
}

// matchesPath checks if a path matches a pattern.
// Supports wildcards like /**/*.env
func matchesPath(path, pattern string) bool {
	// Expand pattern if it contains ~
	if strings.HasPrefix(pattern, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			pattern = filepath.Join(home, pattern[2:])
		}
	} else if pattern == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			pattern = home
		}
	}

	// Convert pattern to absolute path if needed
	if !filepath.IsAbs(pattern) {
		if abs, err := filepath.Abs(pattern); err == nil {
			pattern = abs
		}
	}

	// Check for wildcard patterns
	if strings.Contains(pattern, "*") {
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return true
		}

		// Handle /**/ pattern for recursive matching
		if strings.Contains(pattern, "/**/") {
			parts := strings.Split(pattern, "/**/")
			if len(parts) == 2 {
				prefix := parts[0]
				suffix := parts[1]
				if strings.HasPrefix(path, prefix) {
					if suffix == "" {
						return true
					}
					if matched, err := filepath.Match(suffix, filepath.Base(path)); err == nil && matched {
						return true
					}
				}
			}
		}
	}

	// Check if path is within the pattern directory
	rel, err := filepath.Rel(pattern, path)
	if err == nil && !strings.HasPrefix(rel, "..") && !filepath.IsAbs(rel) {
		return true
	}

	return false
}

// matchesHost checks if a host matches a pattern.
func matchesHost(host, pattern string) bool {
	// Exact match
	if host == pattern {
		return true
	}

	// Remove port if present
	hostWithoutPort := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostWithoutPort = h
	}

	if hostWithoutPort == pattern {
		return true
	}

	// Check if pattern is a suffix match (e.g., *.example.com)
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // Remove the *
		if strings.HasSuffix(hostWithoutPort, suffix) {
			return true
		}
	}

	return false
}

// matchesCommand checks if a command matches a pattern.
func matchesCommand(command, pattern string) bool {
	// Exact match
	if command == pattern {
		return true
	}

	// Check if command starts with pattern (for "git status" matching "git")
	if strings.HasPrefix(command, pattern+" ") {
		return true
	}

	// Extract base command and check
	baseCmd := extractBaseCommand(command)
	if baseCmd == pattern {
		return true
	}

	return false
}

// extractBaseCommand extracts the base command from a full command string.
func extractBaseCommand(command string) string {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// isPrivateIP checks if a host is a private IP address.
// For performance, this only handles IP addresses directly, not hostnames.
// DNS resolution should be done by the caller using DNSCache if needed.
func isPrivateIP(host string) bool {
	// Remove port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	// Parse as IP address
	ip := net.ParseIP(host)
	if ip == nil {
		// Not an IP address - hostname
		// Don't do DNS lookup here to avoid blocking (violates <10ms NFR)
		// The caller should use DNSCache for pre-resolved lookups if needed
		return false
	}

	// Check if IP is private
	// RFC1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
	// Link-local: 169.254.0.0/16
	// Loopback: 127.0.0.0/8
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return true
	}

	return false
}
