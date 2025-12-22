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

package mcp

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/tombee/conductor/internal/config"
)

// ServerNameRegex validates MCP server names.
// Names must start with a letter and contain only letters, numbers, hyphens, and underscores.
// Maximum length is 64 characters.
var ServerNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`)

// RestartPolicy defines when a server should be restarted after failure.
type RestartPolicy string

const (
	// RestartAlways always restarts the server on failure.
	RestartAlways RestartPolicy = "always"
	// RestartOnFailure only restarts on non-zero exit codes.
	RestartOnFailure RestartPolicy = "on-failure"
	// RestartNever never automatically restarts.
	RestartNever RestartPolicy = "never"
)

// MCPGlobalConfig represents the global MCP server configuration file.
// Stored at ~/.config/conductor/mcp.yaml
type MCPGlobalConfig struct {
	// Servers is a map of server name to configuration.
	Servers map[string]*MCPServerEntry `yaml:"servers,omitempty"`

	// Defaults provides default values for server configuration.
	Defaults MCPDefaults `yaml:"defaults,omitempty"`
}

// MCPServerEntry represents a single MCP server configuration entry.
type MCPServerEntry struct {
	// Command is the executable to run (e.g., "npx", "python").
	Command string `yaml:"command,omitempty"`

	// Args are command-line arguments.
	Args []string `yaml:"args,omitempty"`

	// Env are environment variables in KEY=VALUE format.
	// Supports ${VAR} syntax for runtime variable substitution.
	Env []string `yaml:"env,omitempty"`

	// Timeout is the default timeout for tool calls in seconds.
	// Defaults to 30 seconds if not specified.
	Timeout int `yaml:"timeout,omitempty"`

	// AutoStart indicates whether to start this server when the controller starts.
	AutoStart bool `yaml:"auto_start,omitempty"`

	// RestartPolicy defines the restart behavior on failure.
	// Valid values: "always", "on-failure", "never"
	RestartPolicy RestartPolicy `yaml:"restart_policy,omitempty"`

	// MaxRestartAttempts limits the number of restart attempts.
	// Only applies when RestartPolicy is not "never".
	// 0 means unlimited (default).
	MaxRestartAttempts int `yaml:"max_restart_attempts,omitempty"`

	// Source is the package source for version management.
	// Format: "npm:<package>", "pypi:<package>", "local:<path>"
	Source string `yaml:"source,omitempty"`

	// Version is the semver constraint for the server.
	// Only used when Source is set.
	Version string `yaml:"version,omitempty"`
}

// MCPDefaults provides default values for MCP server configuration.
type MCPDefaults struct {
	// Timeout is the default timeout in seconds (default: 30).
	Timeout int `yaml:"timeout,omitempty"`

	// AutoStart is the default auto_start value (default: false).
	AutoStart bool `yaml:"auto_start,omitempty"`

	// RestartPolicy is the default restart policy (default: "always").
	RestartPolicy RestartPolicy `yaml:"restart_policy,omitempty"`

	// MaxRestartAttempts is the default max restart attempts (default: 5).
	MaxRestartAttempts int `yaml:"max_restart_attempts,omitempty"`
}

// DefaultMCPDefaults returns the default values for MCP configuration.
func DefaultMCPDefaults() MCPDefaults {
	return MCPDefaults{
		Timeout:            30,
		AutoStart:          false,
		RestartPolicy:      RestartAlways,
		MaxRestartAttempts: 5,
	}
}

// MCPConfigPath returns the path to the global MCP configuration file.
func MCPConfigPath() (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mcp.yaml"), nil
}

// LoadMCPConfig loads the global MCP configuration from disk.
// Returns an empty config if the file doesn't exist.
func LoadMCPConfig() (*MCPGlobalConfig, error) {
	path, err := MCPConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %w", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty config with defaults
			return &MCPGlobalConfig{
				Servers:  make(map[string]*MCPServerEntry),
				Defaults: DefaultMCPDefaults(),
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg MCPGlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Initialize nil maps
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]*MCPServerEntry)
	}

	// Apply defaults
	cfg.applyDefaults()

	return &cfg, nil
}

// SaveMCPConfig saves the global MCP configuration to disk.
func SaveMCPConfig(cfg *MCPGlobalConfig) error {
	path, err := MCPConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to temp file first, then rename (atomic operation)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return fmt.Errorf("failed to save config file: %w", err)
	}

	return nil
}

// applyDefaults applies default values to server entries.
func (c *MCPGlobalConfig) applyDefaults() {
	defaults := c.Defaults
	if defaults.Timeout == 0 {
		defaults.Timeout = 30
	}
	if defaults.RestartPolicy == "" {
		defaults.RestartPolicy = RestartAlways
	}
	if defaults.MaxRestartAttempts == 0 {
		defaults.MaxRestartAttempts = 5
	}

	for _, entry := range c.Servers {
		if entry.Timeout == 0 {
			entry.Timeout = defaults.Timeout
		}
		if entry.RestartPolicy == "" {
			entry.RestartPolicy = defaults.RestartPolicy
		}
		if entry.MaxRestartAttempts == 0 {
			entry.MaxRestartAttempts = defaults.MaxRestartAttempts
		}
	}
}

// Validate validates the entire configuration.
func (c *MCPGlobalConfig) Validate() error {
	for name, entry := range c.Servers {
		if err := ValidateServerName(name); err != nil {
			return fmt.Errorf("server %q: %w", name, err)
		}
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("server %q: %w", name, err)
		}
	}
	return nil
}

// Validate validates a single server entry.
func (e *MCPServerEntry) Validate() error {
	if e.Command == "" && e.Source == "" {
		return fmt.Errorf("either command or source is required")
	}

	if e.Command != "" {
		if err := ValidateCommand(e.Command); err != nil {
			return err
		}
	}

	if e.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}

	if e.RestartPolicy != "" {
		switch e.RestartPolicy {
		case RestartAlways, RestartOnFailure, RestartNever:
			// Valid
		default:
			return fmt.Errorf("invalid restart_policy: %s (must be 'always', 'on-failure', or 'never')", e.RestartPolicy)
		}
	}

	if e.MaxRestartAttempts < 0 {
		return fmt.Errorf("max_restart_attempts must be non-negative")
	}

	// Validate args for shell injection
	for i, arg := range e.Args {
		if err := ValidateArg(arg); err != nil {
			return fmt.Errorf("args[%d]: %w", i, err)
		}
	}

	// Validate env vars
	for i, env := range e.Env {
		if err := ValidateEnv(env); err != nil {
			return fmt.Errorf("env[%d]: %w", i, err)
		}
	}

	return nil
}

// ToServerConfig converts an MCPServerEntry to a ServerConfig for the manager.
func (e *MCPServerEntry) ToServerConfig(name string) ServerConfig {
	return ServerConfig{
		Name:               name,
		Command:            e.Command,
		Args:               e.Args,
		Env:                e.Env,
		Timeout:            time.Duration(e.Timeout) * time.Second,
		RestartPolicy:      string(e.RestartPolicy),
		MaxRestartAttempts: e.MaxRestartAttempts,
		Source:             e.Source,
		Version:            e.Version,
	}
}

// ValidateServerName validates an MCP server name.
func ValidateServerName(name string) error {
	if name == "" {
		return fmt.Errorf("server name is required")
	}
	if len(name) > 64 {
		return fmt.Errorf("server name exceeds 64 character limit")
	}
	if !ServerNameRegex.MatchString(name) {
		return fmt.Errorf("invalid server name: must start with a letter and contain only letters, numbers, hyphens, and underscores")
	}
	return nil
}

// ValidateCommand validates a command is safe to execute.
func ValidateCommand(cmd string) error {
	if cmd == "" {
		return fmt.Errorf("command is required")
	}

	// Check if it's an absolute path
	if filepath.IsAbs(cmd) {
		// Warn if the path is outside standard directories
		if !strings.HasPrefix(cmd, "/usr/bin/") && !strings.HasPrefix(cmd, "/usr/local/bin/") {
			slog.Warn("MCP server command path is outside standard directories",
				"command", cmd,
				"recommendation", "Consider using commands from /usr/bin or /usr/local/bin for better security")
		}

		// Verify the file exists and is executable
		info, err := os.Stat(cmd)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("command not found: %s", cmd)
			}
			return fmt.Errorf("cannot access command: %w", err)
		}
		if info.IsDir() {
			return fmt.Errorf("command is a directory: %s", cmd)
		}
		// Check if executable (Unix only, but Windows will still work)
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("command is not executable: %s", cmd)
		}
		return nil
	}

	// Check if command is in PATH
	if _, err := exec.LookPath(cmd); err != nil {
		return fmt.Errorf("command not found in PATH: %s", cmd)
	}

	return nil
}

// shellInjectionPatterns are patterns that could indicate shell injection attempts.
var shellInjectionPatterns = []string{
	";", "&&", "||", "|", "`", "$(", "${", "\n", "\r",
}

// ValidateArg validates a command argument for shell injection.
func ValidateArg(arg string) error {
	for _, pattern := range shellInjectionPatterns {
		if strings.Contains(arg, pattern) {
			return fmt.Errorf("argument contains potentially unsafe pattern %q", pattern)
		}
	}
	return nil
}

// ValidateEnv validates an environment variable.
func ValidateEnv(env string) error {
	// Must be in KEY=VALUE format
	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("environment variable must be in KEY=VALUE format")
	}

	key := parts[0]
	if key == "" {
		return fmt.Errorf("environment variable key is required")
	}

	// Key must be valid identifier
	if !regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`).MatchString(key) {
		return fmt.Errorf("invalid environment variable key: %s", key)
	}

	// Value is allowed to contain ${VAR} for variable substitution
	// but not shell injection patterns (except ${)
	value := parts[1]
	for _, pattern := range shellInjectionPatterns {
		// Allow ${VAR} syntax for variable substitution
		if pattern == "${" {
			continue
		}
		if strings.Contains(value, pattern) {
			return fmt.Errorf("environment value contains potentially unsafe pattern %q", pattern)
		}
	}

	return nil
}

// sensitiveKeyPatterns are patterns that indicate a sensitive value.
var sensitiveKeyPatterns = []string{
	"SECRET", "TOKEN", "KEY", "PASSWORD", "CREDENTIAL", "AUTH", "API_KEY",
}

// IsSensitiveEnvKey returns true if the key appears to contain sensitive data.
func IsSensitiveEnvKey(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, pattern := range sensitiveKeyPatterns {
		if strings.Contains(upperKey, pattern) {
			return true
		}
	}
	return false
}

// RedactEnv redacts sensitive values from an environment variable list.
func RedactEnv(envs []string) []string {
	result := make([]string, len(envs))
	for i, env := range envs {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 && IsSensitiveEnvKey(parts[0]) {
			result[i] = parts[0] + "=***REDACTED***"
		} else {
			result[i] = env
		}
	}
	return result
}
