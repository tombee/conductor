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
	"time"
)

// Built-in profile names
const (
	ProfileUnrestricted = "unrestricted"
	ProfileStandard     = "standard"
	ProfileStrict       = "strict"
	ProfileAirGapped    = "air-gapped"
)

// builtinProfiles contains the built-in security profiles
var builtinProfiles = map[string]*SecurityProfile{
	ProfileUnrestricted: {
		Name: ProfileUnrestricted,
		Filesystem: FilesystemConfig{
			Read:  []string{}, // Empty = all allowed
			Write: []string{}, // Empty = all allowed
			Deny:  []string{}, // No restrictions
		},
		Network: NetworkConfig{
			Allow:       []string{}, // Empty = all allowed
			DenyPrivate: false,
			DenyAll:     false,
		},
		Execution: ExecutionConfig{
			AllowedCommands: []string{}, // Empty = all allowed
			DeniedCommands:  []string{},
			Sandbox:         false,
		},
		Isolation: IsolationNone,
		Limits: ResourceLimits{
			TimeoutPerTool: 60 * time.Second,
			TotalRuntime:   0, // No limit
			MaxFileSize:    10 * 1024 * 1024, // 10 MB
		},
	},
	ProfileStandard: {
		Name: ProfileStandard,
		Filesystem: FilesystemConfig{
			Read: []string{
				".",              // Current workspace
				"/tmp/conductor-*", // Temp files
			},
			Write: []string{
				".",              // Current workspace
				"/tmp/conductor-*", // Temp files
			},
			Deny: []string{
				"~/.ssh",
				"~/.aws",
				"~/.gnupg",
				"~/.config/conductor/credentials",
			},
		},
		Network: NetworkConfig{
			Allow: []string{
				"api.anthropic.com",
				"api.openai.com",
			},
			DenyPrivate: true,
			DenyAll:     false,
		},
		Execution: ExecutionConfig{
			AllowedCommands: []string{
				"git",
				"ls",
				"cat",
				"grep",
				"find",
				"jq",
				"curl",
			},
			DeniedCommands: []string{
				"rm -rf",
				"sudo",
				"chmod",
				"chown",
			},
			Sandbox: false,
		},
		Isolation: IsolationNone,
		Limits: ResourceLimits{
			TimeoutPerTool: 60 * time.Second,
			TotalRuntime:   5 * time.Minute,
			MaxFileSize:    50 * 1024 * 1024, // 50 MB
		},
	},
	ProfileStrict: {
		Name: ProfileStrict,
		Filesystem: FilesystemConfig{
			Read: []string{
				".", // Current workspace only
			},
			Write: []string{
				"./.conductor-output", // Designated output directory
			},
			Deny: []string{
				"~",             // Home directory
				"/**/.git/config", // Git configs
				"/**/.env*",     // Environment files
			},
		},
		Network: NetworkConfig{
			Allow:       []string{}, // Must be explicitly configured
			DenyPrivate: true,
			DenyAll:     false,
		},
		Execution: ExecutionConfig{
			AllowedCommands: []string{
				"git status",
				"git diff",
				"git log",
			},
			DeniedCommands: []string{
				"git push",
				"git remote",
			},
			Sandbox: true,
		},
		Isolation: IsolationSandbox,
		Limits: ResourceLimits{
			TimeoutPerTool: 30 * time.Second,
			TotalRuntime:   2 * time.Minute,
			MaxMemory:      512 * 1024 * 1024, // 512 MB
			MaxProcesses:   10,
			MaxFileSize:    10 * 1024 * 1024, // 10 MB
		},
	},
	ProfileAirGapped: {
		Name: ProfileAirGapped,
		Filesystem: FilesystemConfig{
			Read: []string{
				// Must be explicitly configured with input files
			},
			Write: []string{
				// Must be explicitly configured with output directory
			},
			Deny: []string{},
		},
		Network: NetworkConfig{
			Allow:       []string{},
			DenyPrivate: true,
			DenyAll:     true, // No network access at all
		},
		Execution: ExecutionConfig{
			AllowedCommands: []string{}, // No shell access
			DeniedCommands:  []string{},
			Sandbox:         true,
		},
		Isolation: IsolationSandbox,
		Limits: ResourceLimits{
			TimeoutPerTool: 60 * time.Second,
			TotalRuntime:   1 * time.Minute,
			MaxMemory:      256 * 1024 * 1024, // 256 MB
			MaxProcesses:   1,
			MaxFileSize:    10 * 1024 * 1024, // 10 MB
		},
	},
}

// LoadProfile loads a security profile by name.
// It first checks built-in profiles, then custom profiles from config.
func LoadProfile(name string, customProfiles map[string]*SecurityProfile) (*SecurityProfile, error) {
	// Check built-in profiles first
	if profile, ok := builtinProfiles[name]; ok {
		// Return a copy to prevent modification
		return copyProfile(profile), nil
	}

	// Check custom profiles
	if customProfiles != nil {
		if profile, ok := customProfiles[name]; ok {
			// Validate custom profile
			if err := ValidateProfile(profile); err != nil {
				return nil, fmt.Errorf("invalid custom profile %q: %w", name, err)
			}
			return copyProfile(profile), nil
		}
	}

	return nil, fmt.Errorf("profile not found: %s", name)
}

// GetBuiltinProfiles returns all built-in profile names.
func GetBuiltinProfiles() []string {
	return []string{
		ProfileUnrestricted,
		ProfileStandard,
		ProfileStrict,
		ProfileAirGapped,
	}
}

// ValidateProfile validates a security profile for correctness.
func ValidateProfile(profile *SecurityProfile) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	if profile.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	// Validate isolation level
	switch profile.Isolation {
	case IsolationNone, IsolationSandbox:
		// Valid
	default:
		return fmt.Errorf("invalid isolation level: %s (must be 'none' or 'sandbox')", profile.Isolation)
	}

	// Validate resource limits
	if profile.Limits.TimeoutPerTool <= 0 {
		return fmt.Errorf("timeout_per_tool must be positive")
	}

	if profile.Limits.MaxMemory < 0 {
		return fmt.Errorf("max_memory cannot be negative")
	}

	if profile.Limits.MaxProcesses < 0 {
		return fmt.Errorf("max_processes cannot be negative")
	}

	if profile.Limits.MaxFileSize < 0 {
		return fmt.Errorf("max_file_size cannot be negative")
	}

	// Validate that sandbox isolation is enabled when required
	if profile.Isolation == IsolationSandbox && !profile.Execution.Sandbox {
		return fmt.Errorf("sandbox execution must be enabled when isolation is 'sandbox'")
	}

	return nil
}

// copyProfile creates a deep copy of a security profile.
func copyProfile(p *SecurityProfile) *SecurityProfile {
	if p == nil {
		return nil
	}

	return &SecurityProfile{
		Name: p.Name,
		Filesystem: FilesystemConfig{
			Read:  copyStringSlice(p.Filesystem.Read),
			Write: copyStringSlice(p.Filesystem.Write),
			Deny:  copyStringSlice(p.Filesystem.Deny),
		},
		Network: NetworkConfig{
			Allow:       copyStringSlice(p.Network.Allow),
			DenyPrivate: p.Network.DenyPrivate,
			DenyAll:     p.Network.DenyAll,
		},
		Execution: ExecutionConfig{
			AllowedCommands: copyStringSlice(p.Execution.AllowedCommands),
			DeniedCommands:  copyStringSlice(p.Execution.DeniedCommands),
			Sandbox:         p.Execution.Sandbox,
		},
		Isolation: p.Isolation,
		Limits:    p.Limits,
	}
}

// copyStringSlice creates a copy of a string slice.
func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	result := make([]string, len(s))
	copy(result, s)
	return result
}
