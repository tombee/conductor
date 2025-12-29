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

	"github.com/bmatcuk/doublestar/v4"
)

// AccessChecker validates resource access against configured permissions.
type AccessChecker interface {
	// CheckFilesystemRead checks if reading the path is allowed.
	CheckFilesystemRead(path string) *AccessCheckResult

	// CheckFilesystemWrite checks if writing the path is allowed.
	CheckFilesystemWrite(path string) *AccessCheckResult

	// CheckNetwork checks if connecting to host:port is allowed.
	CheckNetwork(host string, port int) *AccessCheckResult

	// CheckShell checks if executing the command is allowed.
	CheckShell(command string) *AccessCheckResult
}

// AccessCheckResult contains the result of an access check.
type AccessCheckResult struct {
	Allowed     bool
	Reason      string   // Human-readable explanation
	DeniedBy    string   // Which rule denied (for debugging)
	AllowedList []string // What IS allowed (for error messages)
}

// accessChecker implements the AccessChecker interface.
type accessChecker struct {
	config    *AccessConfig
	cwd       string
	tempDir   string
	// Cached resolved patterns for performance
	fsReadPatterns  []string
	fsWritePatterns []string
	fsDenyPatterns  []string
}

// NewAccessChecker creates a checker from an AccessConfig.
func NewAccessChecker(cfg *AccessConfig) (AccessChecker, error) {
	if cfg == nil {
		cfg = &AccessConfig{}
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	tempDir := os.TempDir()

	checker := &accessChecker{
		config:  cfg,
		cwd:     cwd,
		tempDir: tempDir,
	}

	// Pre-process filesystem patterns
	checker.fsReadPatterns = checker.resolvePatterns(cfg.Filesystem.Read)
	checker.fsWritePatterns = checker.resolvePatterns(cfg.Filesystem.Write)
	checker.fsDenyPatterns = checker.resolvePatterns(cfg.Filesystem.Deny)

	return checker, nil
}

// CheckFilesystemRead checks if reading the path is allowed.
func (c *accessChecker) CheckFilesystemRead(path string) *AccessCheckResult {
	canonPath, err := c.canonicalizePath(path)
	if err != nil {
		return &AccessCheckResult{
			Allowed:  false,
			Reason:   fmt.Sprintf("path canonicalization failed: %v", err),
			DeniedBy: "canonicalization_error",
		}
	}

	// Check deny patterns first
	if c.matchesAnyPattern(canonPath, c.fsDenyPatterns) {
		return &AccessCheckResult{
			Allowed:     false,
			Reason:      "path matches deny pattern",
			DeniedBy:    "filesystem.deny",
			AllowedList: c.config.Filesystem.Read,
		}
	}

	// Check read patterns
	if len(c.fsReadPatterns) == 0 {
		// Empty read list means no filesystem read access
		return &AccessCheckResult{
			Allowed:     false,
			Reason:      "no filesystem read permissions declared",
			DeniedBy:    "no_permissions",
			AllowedList: []string{},
		}
	}

	if c.matchesAnyPattern(canonPath, c.fsReadPatterns) {
		return &AccessCheckResult{
			Allowed: true,
			Reason:  "path matches allowed read pattern",
		}
	}

	return &AccessCheckResult{
		Allowed:     false,
		Reason:      "path does not match any allowed read pattern",
		DeniedBy:    "no_match",
		AllowedList: c.config.Filesystem.Read,
	}
}

// CheckFilesystemWrite checks if writing the path is allowed.
func (c *accessChecker) CheckFilesystemWrite(path string) *AccessCheckResult {
	canonPath, err := c.canonicalizePath(path)
	if err != nil {
		return &AccessCheckResult{
			Allowed:  false,
			Reason:   fmt.Sprintf("path canonicalization failed: %v", err),
			DeniedBy: "canonicalization_error",
		}
	}

	// Check deny patterns first
	if c.matchesAnyPattern(canonPath, c.fsDenyPatterns) {
		return &AccessCheckResult{
			Allowed:     false,
			Reason:      "path matches deny pattern",
			DeniedBy:    "filesystem.deny",
			AllowedList: c.config.Filesystem.Write,
		}
	}

	// Check write patterns
	if len(c.fsWritePatterns) == 0 {
		// Empty write list means no filesystem write access
		return &AccessCheckResult{
			Allowed:     false,
			Reason:      "no filesystem write permissions declared",
			DeniedBy:    "no_permissions",
			AllowedList: []string{},
		}
	}

	if c.matchesAnyPattern(canonPath, c.fsWritePatterns) {
		return &AccessCheckResult{
			Allowed: true,
			Reason:  "path matches allowed write pattern",
		}
	}

	return &AccessCheckResult{
		Allowed:     false,
		Reason:      "path does not match any allowed write pattern",
		DeniedBy:    "no_match",
		AllowedList: c.config.Filesystem.Write,
	}
}

// CheckNetwork checks if connecting to host:port is allowed.
func (c *accessChecker) CheckNetwork(host string, port int) *AccessCheckResult {
	// Normalize host
	host = strings.ToLower(strings.TrimSpace(host))

	target := fmt.Sprintf("%s:%d", host, port)

	// Check deny patterns first
	for _, pattern := range c.config.Network.Deny {
		if c.matchesNetworkPattern(host, port, pattern) {
			return &AccessCheckResult{
				Allowed:     false,
				Reason:      fmt.Sprintf("host matches deny pattern: %s", pattern),
				DeniedBy:    "network.deny",
				AllowedList: c.config.Network.Allow,
			}
		}
	}

	// Check allow patterns
	if len(c.config.Network.Allow) == 0 {
		// Empty allow list means no network access
		return &AccessCheckResult{
			Allowed:     false,
			Reason:      "no network permissions declared",
			DeniedBy:    "no_permissions",
			AllowedList: []string{},
		}
	}

	for _, pattern := range c.config.Network.Allow {
		if c.matchesNetworkPattern(host, port, pattern) {
			return &AccessCheckResult{
				Allowed: true,
				Reason:  fmt.Sprintf("host matches allowed pattern: %s", pattern),
			}
		}
	}

	return &AccessCheckResult{
		Allowed:     false,
		Reason:      fmt.Sprintf("host %s not in allowed list", target),
		DeniedBy:    "no_match",
		AllowedList: c.config.Network.Allow,
	}
}

// CheckShell checks if executing the command is allowed.
func (c *accessChecker) CheckShell(command string) *AccessCheckResult {
	command = strings.TrimSpace(command)
	if command == "" {
		return &AccessCheckResult{
			Allowed:  false,
			Reason:   "empty command",
			DeniedBy: "invalid_command",
		}
	}

	// Check deny patterns first
	for _, pattern := range c.config.Shell.DenyPatterns {
		if c.matchesShellPattern(command, pattern) {
			return &AccessCheckResult{
				Allowed:     false,
				Reason:      fmt.Sprintf("command matches deny pattern: %s", pattern),
				DeniedBy:    "shell.deny_patterns",
				AllowedList: c.config.Shell.Commands,
			}
		}
	}

	// Check allowed commands
	if len(c.config.Shell.Commands) == 0 {
		// Empty commands list means no shell access
		return &AccessCheckResult{
			Allowed:     false,
			Reason:      "no shell permissions declared",
			DeniedBy:    "no_permissions",
			AllowedList: []string{},
		}
	}

	baseCmd := c.extractCommandBase(command)
	for _, allowedCmd := range c.config.Shell.Commands {
		if c.matchesShellPattern(command, allowedCmd) {
			return &AccessCheckResult{
				Allowed: true,
				Reason:  fmt.Sprintf("command matches allowed pattern: %s", allowedCmd),
			}
		}
	}

	return &AccessCheckResult{
		Allowed:     false,
		Reason:      fmt.Sprintf("command '%s' not in allowed list", baseCmd),
		DeniedBy:    "no_match",
		AllowedList: c.config.Shell.Commands,
	}
}

// canonicalizePath resolves a path to its canonical form.
func (c *accessChecker) canonicalizePath(path string) (string, error) {
	// Resolve special variables
	path = c.resolveVariables(path)

	// Expand home directory
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// Convert to absolute path
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.cwd, path)
	}

	// Resolve symlinks and clean path
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If the file doesn't exist yet, just clean the path
		// This allows checking write permissions for non-existent files
		if os.IsNotExist(err) {
			return filepath.Clean(path), nil
		}
		return "", fmt.Errorf("symlink resolution failed: %w", err)
	}

	return filepath.Clean(realPath), nil
}

// resolveVariables resolves special variables in paths.
func (c *accessChecker) resolveVariables(path string) string {
	path = strings.ReplaceAll(path, "$cwd", c.cwd)
	path = strings.ReplaceAll(path, "$temp", c.tempDir)
	return path
}

// resolvePatterns resolves special variables in a list of patterns and cleans them.
func (c *accessChecker) resolvePatterns(patterns []string) []string {
	resolved := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		// Resolve variables
		p := c.resolveVariables(pattern)

		// Convert relative paths to absolute
		if !filepath.IsAbs(p) {
			// For relative paths, we need to make them absolute before cleaning
			// Extract the glob suffix if present
			if idx := strings.Index(p, "*"); idx != -1 {
				lastSep := strings.LastIndex(p[:idx], string(filepath.Separator))
				var basePath, globSuffix string
				if lastSep != -1 {
					basePath = p[:lastSep]
					globSuffix = p[lastSep:]
				} else {
					// Pattern starts with glob (e.g., "*.txt")
					basePath = "."
					globSuffix = string(filepath.Separator) + p
				}
				absBase := filepath.Join(c.cwd, basePath)
				p = absBase + globSuffix
			} else if idx := strings.Index(p, "?"); idx != -1 {
				lastSep := strings.LastIndex(p[:idx], string(filepath.Separator))
				var basePath, globSuffix string
				if lastSep != -1 {
					basePath = p[:lastSep]
					globSuffix = p[lastSep:]
				} else {
					basePath = "."
					globSuffix = string(filepath.Separator) + p
				}
				absBase := filepath.Join(c.cwd, basePath)
				p = absBase + globSuffix
			} else {
				// No glob, make absolute
				p = filepath.Join(c.cwd, p)
			}
		}

		// Clean the path (for absolute paths or already converted relative paths)
		// For patterns with globs, clean only the prefix
		if idx := strings.Index(p, "*"); idx != -1 {
			lastSep := strings.LastIndex(p[:idx], string(filepath.Separator))
			if lastSep != -1 {
				basePath := p[:lastSep]
				globSuffix := p[lastSep:]
				p = filepath.Clean(basePath) + globSuffix
			}
		} else if idx := strings.Index(p, "?"); idx != -1 {
			lastSep := strings.LastIndex(p[:idx], string(filepath.Separator))
			if lastSep != -1 {
				basePath := p[:lastSep]
				globSuffix := p[lastSep:]
				p = filepath.Clean(basePath) + globSuffix
			}
		} else {
			// No glob, just clean
			p = filepath.Clean(p)
		}

		resolved = append(resolved, p)
	}
	return resolved
}

// matchesAnyPattern checks if a path matches any of the given glob patterns.
func (c *accessChecker) matchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, path)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// matchesNetworkPattern checks if a host:port matches a network pattern.
func (c *accessChecker) matchesNetworkPattern(host string, port int, pattern string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))

	// Check for CIDR notation
	if strings.Contains(pattern, "/") {
		_, cidr, err := net.ParseCIDR(pattern)
		if err == nil {
			ip := net.ParseIP(host)
			if ip != nil && cidr.Contains(ip) {
				return true
			}
		}
		return false
	}

	// Extract pattern host and port
	patternHost, patternPort := parseHostPort(pattern)

	// Match port if specified in pattern
	if patternPort != 0 && patternPort != port {
		return false
	}

	// Exact match
	if patternHost == host {
		return true
	}

	// Wildcard subdomain match (*.example.com)
	if strings.HasPrefix(patternHost, "*.") {
		baseDomain := patternHost[2:] // Remove "*."
		// Host must be a subdomain of baseDomain
		if strings.HasSuffix(host, "."+baseDomain) {
			return true
		}
	}

	return false
}

// matchesShellPattern checks if a command matches a shell pattern.
func (c *accessChecker) matchesShellPattern(command, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	command = strings.TrimSpace(command)

	// Extract base command from both
	cmdBase := c.extractCommandBase(command)
	patternBase := c.extractCommandBase(pattern)

	// If pattern is just the base command, match if bases match
	if pattern == patternBase {
		return cmdBase == patternBase
	}

	// If pattern has subcommands, require exact prefix match
	if strings.HasPrefix(command, pattern) {
		// Ensure word boundary (next char is space or end of string)
		if len(command) == len(pattern) || command[len(pattern)] == ' ' {
			return true
		}
	}

	return false
}

// extractCommandBase extracts the base command from a full command string.
// This handles path prefixes (e.g., /usr/bin/git -> git)
func (c *accessChecker) extractCommandBase(command string) string {
	// Trim leading/trailing whitespace
	command = strings.TrimSpace(command)

	// Split on whitespace
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}

	// Get the first part (the command)
	baseCmd := parts[0]

	// Strip path if present (e.g., /usr/bin/git -> git)
	if strings.Contains(baseCmd, "/") {
		baseCmd = filepath.Base(baseCmd)
	}

	return baseCmd
}

// parseHostPort parses a host:port string.
// Returns the host and port. Port is 0 if not specified.
func parseHostPort(hostPort string) (string, int) {
	// Try to split on last colon
	idx := strings.LastIndex(hostPort, ":")
	if idx == -1 {
		// No port specified
		return hostPort, 0
	}

	host := hostPort[:idx]
	portStr := hostPort[idx+1:]

	// Try to parse port
	var port int
	_, err := fmt.Sscanf(portStr, "%d", &port)
	if err != nil {
		// Not a valid port, treat whole thing as host
		return hostPort, 0
	}

	return host, port
}
