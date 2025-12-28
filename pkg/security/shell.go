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
	"strings"
)

// ShellSecurityConfig defines security controls for shell command execution.
type ShellSecurityConfig struct {
	// AllowedCommands lists base commands allowed (empty = all allowed)
	AllowedCommands []string `yaml:"allowed_commands,omitempty" json:"allowed_commands,omitempty"`

	// DeniedCommands lists explicit denials (higher priority than AllowedCommands)
	DeniedCommands []string `yaml:"denied_commands,omitempty" json:"denied_commands,omitempty"`

	// AllowedArgs defines per-command argument allowlists
	AllowedArgs map[string][]string `yaml:"allowed_args,omitempty" json:"allowed_args,omitempty"`

	// SanitizeEnv removes sensitive environment variables
	SanitizeEnv bool `yaml:"sanitize_env" json:"sanitize_env"`

	// AllowShellExpand allows shell expansions ($(), backticks, etc.)
	// DANGEROUS: Enables command injection - only use in unrestricted profile
	AllowShellExpand bool `yaml:"allow_shell_expand" json:"allow_shell_expand"`

	// MaxOutputSize truncates output beyond this limit (bytes)
	MaxOutputSize int64 `yaml:"max_output_size,omitempty" json:"max_output_size,omitempty"`

	// WorkingDir enforces a specific working directory
	WorkingDir string `yaml:"working_dir,omitempty" json:"working_dir,omitempty"`

	// BlockedMetachars are characters rejected in command arguments
	// Default: [$, `, |, &, ;, <, >, \n, \r]
	BlockedMetachars []string `yaml:"blocked_metachars,omitempty" json:"blocked_metachars,omitempty"`

	// ParseArguments parses command into argv[] before execution
	// When true, uses exec.Command(cmd, args...) instead of shell invocation
	ParseArguments bool `yaml:"parse_arguments" json:"parse_arguments"`
}

// DefaultShellSecurityConfig returns a secure default configuration.
func DefaultShellSecurityConfig() *ShellSecurityConfig {
	return &ShellSecurityConfig{
		AllowedCommands:  []string{},
		DeniedCommands:   []string{},
		SanitizeEnv:      true,
		AllowShellExpand: false,
		MaxOutputSize:    1024 * 1024, // 1 MB
		BlockedMetachars: []string{"$", "`", "|", "&", ";", "<", ">", "\n", "\r"},
		ParseArguments:   true,
	}
}

// ValidateCommand validates a command against the security configuration.
func (c *ShellSecurityConfig) ValidateCommand(command string, args []string) error {
	// Extract base command
	baseCmd := extractBaseCommand(command)
	if baseCmd == "" {
		return fmt.Errorf("empty command")
	}

	// Check denied commands first (highest priority)
	for _, deniedCmd := range c.DeniedCommands {
		if matchesCommand(command, deniedCmd) || matchesCommand(baseCmd, deniedCmd) {
			return fmt.Errorf("command explicitly denied: %s", deniedCmd)
		}
	}

	// Check allowed commands
	if len(c.AllowedCommands) > 0 {
		allowed := false
		for _, allowedCmd := range c.AllowedCommands {
			if matchesCommand(command, allowedCmd) || matchesCommand(baseCmd, allowedCmd) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command not in allowlist: %s", baseCmd)
		}
	}

	// Validate arguments if not allowing shell expansion
	if !c.AllowShellExpand {
		if err := c.validateArguments(args); err != nil {
			return err
		}
	}

	// Check allowed args for specific commands
	if c.AllowedArgs != nil {
		if allowedArgs, ok := c.AllowedArgs[baseCmd]; ok {
			if err := c.validateAllowedArgs(args, allowedArgs); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateArguments checks arguments for blocked metacharacters.
func (c *ShellSecurityConfig) validateArguments(args []string) error {
	for _, arg := range args {
		for _, metachar := range c.BlockedMetachars {
			if strings.Contains(arg, metachar) {
				return fmt.Errorf("argument contains blocked metacharacter %q: %s", metachar, arg)
			}
		}
	}
	return nil
}

// validateAllowedArgs checks if arguments match the allowlist for a command.
func (c *ShellSecurityConfig) validateAllowedArgs(args, allowedArgs []string) error {
	if len(allowedArgs) == 0 {
		return nil // No restrictions
	}

	for _, arg := range args {
		allowed := false
		for _, allowedArg := range allowedArgs {
			if arg == allowedArg || matchesPattern(arg, allowedArg) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("argument not in allowlist: %s", arg)
		}
	}
	return nil
}

// SanitizeEnvironment removes sensitive environment variables.
func (c *ShellSecurityConfig) SanitizeEnvironment(env []string) []string {
	if !c.SanitizeEnv {
		return env
	}

	// List of sensitive environment variable prefixes
	sensitiveVars := []string{
		"AWS_",
		"ANTHROPIC_",
		"OPENAI_",
		"GOOGLE_",
		"AZURE_",
		"GITHUB_TOKEN",
		"GITLAB_TOKEN",
		"DOCKER_PASSWORD",
		"NPM_TOKEN",
		"SSH_",
		"API_KEY",
		"SECRET",
		"PASSWORD",
		"TOKEN",
	}

	sanitized := []string{}
	for _, envVar := range env {
		key := strings.Split(envVar, "=")[0]
		isSensitive := false

		for _, prefix := range sensitiveVars {
			if strings.HasPrefix(key, prefix) || strings.Contains(strings.ToUpper(key), prefix) {
				isSensitive = true
				break
			}
		}

		if !isSensitive {
			sanitized = append(sanitized, envVar)
		}
	}

	return sanitized
}

// ParseCommandLine parses a command line into base command and arguments.
// Returns (baseCommand, args, error).
func ParseCommandLine(commandLine string) (string, []string, error) {
	if commandLine == "" {
		return "", nil, fmt.Errorf("empty command line")
	}

	// Simple whitespace-based parsing
	// More sophisticated parsing (handling quotes, escapes) would use shlex
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("no command found")
	}

	return parts[0], parts[1:], nil
}

// matchesPattern checks if a string matches a pattern (supports wildcards).
func matchesPattern(s, pattern string) bool {
	// Simple wildcard matching (* = any sequence)
	if pattern == "*" {
		return true
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(s, prefix)
	}

	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(s, suffix)
	}

	return s == pattern
}
