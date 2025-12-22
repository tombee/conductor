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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/security"
	"github.com/tombee/conductor/pkg/security/sandbox"
)

func newSecurityStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show current security profile and permissions",
		Long: `Display the active security profile and its permissions.

Shows:
- Active security profile name
- Filesystem access permissions (read/write/deny)
- Network access permissions (allowed hosts, restrictions)
- Command execution permissions (allowed/denied commands)
- Isolation level
- Resource limits

Example:
  conductor security status
  conductor security status --json`,
		RunE: runSecurityStatus,
	}

	return cmd
}

type statusOutput struct {
	ActiveProfile string                     `json:"active_profile"`
	Filesystem    filesystemPermissions      `json:"filesystem"`
	Network       networkPermissions         `json:"network"`
	Execution     executionPermissions       `json:"execution"`
	Isolation     string                     `json:"isolation"`
	Limits        resourceLimits             `json:"limits"`
	SandboxInfo   *sandboxInfo               `json:"sandbox_info,omitempty"`
}

type filesystemPermissions struct {
	Read  []string `json:"read"`
	Write []string `json:"write"`
	Deny  []string `json:"deny"`
}

type networkPermissions struct {
	Allow       []string `json:"allow"`
	DenyPrivate bool     `json:"deny_private"`
	DenyAll     bool     `json:"deny_all"`
}

type executionPermissions struct {
	AllowedCommands []string `json:"allowed_commands"`
	DeniedCommands  []string `json:"denied_commands"`
}

type resourceLimits struct {
	TimeoutPerTool string `json:"timeout_per_tool,omitempty"`
	TotalRuntime   string `json:"total_runtime,omitempty"`
	MaxMemory      string `json:"max_memory,omitempty"`
	MaxProcesses   int    `json:"max_processes,omitempty"`
	MaxFileSize    string `json:"max_file_size,omitempty"`
}

type sandboxInfo struct {
	Available bool   `json:"available"`
	Type      string `json:"type,omitempty"`
	Status    string `json:"status,omitempty"`
}

func runSecurityStatus(cmd *cobra.Command, args []string) error {
	// Load configuration to get security settings
	configPath, err := config.ConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create security manager
	mgr, err := security.NewManager(&cfg.Security)
	if err != nil {
		return fmt.Errorf("failed to create security manager: %w", err)
	}

	// Get active profile
	profile := mgr.GetActiveProfile()

	// Check sandbox availability
	sandboxAvailable, sandboxType := checkSandboxAvailability(profile)

	// Build output
	output := buildStatusOutput(profile, sandboxAvailable, sandboxType)

	// Output in requested format
	if shared.GetJSON() {
		return outputSecurityStatusJSON(output)
	}

	return outputSecurityStatusHuman(output)
}

func buildStatusOutput(profile *security.SecurityProfile, sandboxAvailable bool, sandboxType string) statusOutput {
	output := statusOutput{
		ActiveProfile: profile.Name,
		Filesystem: filesystemPermissions{
			Read:  profile.Filesystem.Read,
			Write: profile.Filesystem.Write,
			Deny:  profile.Filesystem.Deny,
		},
		Network: networkPermissions{
			Allow:       profile.Network.Allow,
			DenyPrivate: profile.Network.DenyPrivate,
			DenyAll:     profile.Network.DenyAll,
		},
		Execution: executionPermissions{
			AllowedCommands: profile.Execution.AllowedCommands,
			DeniedCommands:  profile.Execution.DeniedCommands,
		},
		Isolation: string(profile.Isolation),
		Limits: resourceLimits{
			MaxProcesses: profile.Limits.MaxProcesses,
		},
	}

	if profile.Limits.TimeoutPerTool > 0 {
		output.Limits.TimeoutPerTool = profile.Limits.TimeoutPerTool.String()
	}
	if profile.Limits.TotalRuntime > 0 {
		output.Limits.TotalRuntime = profile.Limits.TotalRuntime.String()
	}
	if profile.Limits.MaxMemory > 0 {
		output.Limits.MaxMemory = formatBytes(profile.Limits.MaxMemory)
	}
	if profile.Limits.MaxFileSize > 0 {
		output.Limits.MaxFileSize = formatBytes(profile.Limits.MaxFileSize)
	}

	// Add sandbox info if isolation is required
	if profile.Isolation == security.IsolationSandbox {
		status := "healthy"
		if !sandboxAvailable {
			status = "degraded"
		}
		output.SandboxInfo = &sandboxInfo{
			Available: sandboxAvailable,
			Type:      sandboxType,
			Status:    status,
		}
	}

	return output
}

func outputSecurityStatusJSON(output statusOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func outputSecurityStatusHuman(output statusOutput) error {
	fmt.Printf("Active Profile: %s\n", output.ActiveProfile)
	fmt.Println()

	fmt.Println("Permissions:")

	// Filesystem
	fmt.Println("  Filesystem:")
	if len(output.Filesystem.Read) > 0 {
		fmt.Printf("    Read:  %s\n", strings.Join(output.Filesystem.Read, ", "))
	} else {
		fmt.Println("    Read:  (unrestricted)")
	}
	if len(output.Filesystem.Write) > 0 {
		fmt.Printf("    Write: %s\n", strings.Join(output.Filesystem.Write, ", "))
	} else {
		fmt.Println("    Write: (unrestricted)")
	}
	if len(output.Filesystem.Deny) > 0 {
		fmt.Printf("    Deny:  %s\n", strings.Join(output.Filesystem.Deny, ", "))
	}
	fmt.Println()

	// Network
	fmt.Println("  Network:")
	if output.Network.DenyAll {
		fmt.Println("    Allow: (none - network disabled)")
	} else if len(output.Network.Allow) > 0 {
		fmt.Printf("    Allow: %s\n", strings.Join(output.Network.Allow, ", "))
	} else {
		fmt.Println("    Allow: (unrestricted)")
	}
	if output.Network.DenyPrivate {
		fmt.Println("    Deny:  Private IPs, Metadata endpoints")
	}
	fmt.Println()

	// Execution
	fmt.Println("  Execution:")
	if len(output.Execution.AllowedCommands) > 0 {
		fmt.Printf("    Allow: %s\n", strings.Join(output.Execution.AllowedCommands, ", "))
	} else {
		fmt.Println("    Allow: (unrestricted)")
	}
	if len(output.Execution.DeniedCommands) > 0 {
		fmt.Printf("    Deny:  %s\n", strings.Join(output.Execution.DeniedCommands, ", "))
	}
	fmt.Println()

	// Isolation
	fmt.Printf("  Isolation: %s\n", output.Isolation)
	if output.SandboxInfo != nil {
		if output.SandboxInfo.Available {
			fmt.Printf("    Sandbox: Available (%s)\n", output.SandboxInfo.Type)
		} else {
			fmt.Println("    Sandbox: Unavailable (degraded mode)")
		}
	}
	fmt.Println()

	// Resource Limits
	if hasLimits(output.Limits) {
		fmt.Println("Resource Limits:")
		if output.Limits.TimeoutPerTool != "" {
			fmt.Printf("  Timeout per tool: %s\n", output.Limits.TimeoutPerTool)
		}
		if output.Limits.TotalRuntime != "" {
			fmt.Printf("  Total runtime:    %s\n", output.Limits.TotalRuntime)
		}
		if output.Limits.MaxMemory != "" {
			fmt.Printf("  Max memory:       %s\n", output.Limits.MaxMemory)
		}
		if output.Limits.MaxProcesses > 0 {
			fmt.Printf("  Max processes:    %d\n", output.Limits.MaxProcesses)
		}
		if output.Limits.MaxFileSize != "" {
			fmt.Printf("  Max file size:    %s\n", output.Limits.MaxFileSize)
		}
	}

	return nil
}

func checkSandboxAvailability(profile *security.SecurityProfile) (bool, string) {
	if profile.Isolation != security.IsolationSandbox {
		return false, ""
	}

	// Try to detect Docker/Podman availability
	ctx := context.Background()
	selector := sandbox.NewFactorySelector()
	factory, degraded, err := selector.SelectFactory(ctx)
	if err != nil {
		return false, "none"
	}

	return !degraded, string(factory.Type())
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

func hasLimits(limits resourceLimits) bool {
	return limits.TimeoutPerTool != "" ||
		limits.TotalRuntime != "" ||
		limits.MaxMemory != "" ||
		limits.MaxProcesses > 0 ||
		limits.MaxFileSize != ""
}
