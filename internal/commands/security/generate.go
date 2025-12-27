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
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/pkg/security"
	"gopkg.in/yaml.v3"
)

var (
	generateOutputFile string
	generateExtends    string
)

func newSecurityGenerateProfileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-profile <workflow>",
		Short: "Generate security profile from workflow",
		Long: `Analyze a workflow and generate a custom security profile based on its requirements.

This command examines the workflow and creates a profile that:
- Allows only the specific paths the workflow needs
- Allows only the network hosts the workflow contacts
- Allows only the commands the workflow executes

The generated profile extends a base profile (default: standard) and adds
specific permissions needed for the workflow.

Example:
  conductor security generate-profile workflow.yaml
  conductor security generate-profile workflow.yaml --output my-profile.yaml
  conductor security generate-profile workflow.yaml --extends strict`,
		Args: cobra.ExactArgs(1),
		RunE: runSecurityGenerateProfile,
	}

	cmd.Flags().StringVarP(&generateOutputFile, "output", "o", "", "Output file for generated profile (default: stdout)")
	cmd.Flags().StringVar(&generateExtends, "extends", security.ProfileStandard, "Base profile to extend (unrestricted, standard, strict, air-gapped)")

	return cmd
}

func runSecurityGenerateProfile(cmd *cobra.Command, args []string) error {
	workflowPath := args[0]

	// Load and analyze workflow
	wf, err := loadWorkflowForAnalysis(workflowPath)
	if err != nil {
		return fmt.Errorf("failed to load workflow: %w", err)
	}

	reqs := analyzeWorkflowRequirements(wf, workflowPath)

	// Generate profile
	profile := generateSecurityProfile(wf.Name, workflowPath, reqs, generateExtends)

	// Serialize to YAML
	yamlData, err := yaml.Marshal(profile)
	if err != nil {
		return fmt.Errorf("failed to serialize profile: %w", err)
	}

	// Add header comment
	header := generateProfileHeader(workflowPath)
	output := header + string(yamlData)

	// Write to file or stdout
	if generateOutputFile != "" {
		if err := os.WriteFile(generateOutputFile, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write profile: %w", err)
		}
		fmt.Printf("Generated profile: %s\n", generateOutputFile)
	} else {
		fmt.Print(output)
	}

	return nil
}

func generateSecurityProfile(workflowName, workflowPath string, reqs workflowRequirements, extends string) map[string]interface{} {
	profile := make(map[string]interface{})

	// Profile metadata
	profileName := sanitizeProfileName(workflowName)
	if profileName == "" {
		profileName = "custom-workflow-profile"
	}

	profile["name"] = profileName
	profile["extends"] = extends

	// Permissions section
	permissions := make(map[string]interface{})

	// Filesystem permissions
	if len(reqs.Filesystem.Reads) > 0 || len(reqs.Filesystem.Writes) > 0 {
		fs := make(map[string]interface{})

		if len(reqs.Filesystem.Reads) > 0 {
			reads := make([]string, 0, len(reqs.Filesystem.Reads))
			comments := make([]string, 0)
			for _, r := range reqs.Filesystem.Reads {
				reads = append(reads, r.Path)
				if r.Sensitive {
					comments = append(comments, fmt.Sprintf("      # WARNING: %s contains credentials", r.Path))
				}
			}
			fs["read"] = reads
			if len(comments) > 0 {
				fs["_read_comments"] = comments
			}
		}

		if len(reqs.Filesystem.Writes) > 0 {
			writes := make([]string, 0, len(reqs.Filesystem.Writes))
			for _, w := range reqs.Filesystem.Writes {
				writes = append(writes, w.Path)
			}
			fs["write"] = writes
		}

		permissions["filesystem"] = fs
	}

	// Network permissions
	if len(reqs.Network.Hosts) > 0 {
		net := make(map[string]interface{})
		hosts := make([]string, 0, len(reqs.Network.Hosts))
		comments := make([]string, 0)

		for _, h := range reqs.Network.Hosts {
			if h.Note != "" && strings.Contains(h.Note, "unrecognized") {
				comments = append(comments, fmt.Sprintf("      # WARNING: Unrecognized host - review before allowing: %s", h.Host))
				// Comment out unrecognized hosts by default
				comments = append(comments, fmt.Sprintf("      # - %s", h.Host))
			} else {
				hosts = append(hosts, h.Host)
			}
		}

		if len(hosts) > 0 {
			net["allow"] = hosts
		}
		if len(comments) > 0 {
			net["_allow_comments"] = comments
		}

		permissions["network"] = net
	}

	// Command permissions
	if len(reqs.Commands.Commands) > 0 {
		exec := make(map[string]interface{})
		commands := make([]string, 0, len(reqs.Commands.Commands))
		comments := make([]string, 0)

		for _, c := range reqs.Commands.Commands {
			if c.Note != "" && strings.Contains(c.Note, "dangerous") {
				comments = append(comments, fmt.Sprintf("      # WARNING: %s allows arbitrary network access", c.Command))
				comments = append(comments, fmt.Sprintf("      # - %s", c.Command))
			} else {
				commands = append(commands, c.Command)
			}
		}

		if len(commands) > 0 {
			exec["commands"] = commands
		}
		if len(comments) > 0 {
			exec["_commands_comments"] = comments
		}

		permissions["execution"] = exec
	}

	if len(permissions) > 0 {
		profile["permissions"] = permissions
	}

	return profile
}

func generateProfileHeader(workflowPath string) string {
	workflowName := filepath.Base(workflowPath)
	return fmt.Sprintf(`# Auto-generated security profile for: %s
# Review and adjust before production use
#
# To use this profile:
#   1. Save to ~/.config/conductor/security/profiles/
#   2. Run with: conductor run --security=%s workflow.yaml
#   3. Or set in config: security.default_profile

`, workflowName, sanitizeProfileName(filepath.Base(workflowPath)))
}

func sanitizeProfileName(name string) string {
	// Remove file extension
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// Replace non-alphanumeric characters with hyphens
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Collapse multiple hyphens
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}

	return name
}

// Placeholder functions for permission management
func newSecurityListPermissionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-permissions",
		Short: "List stored permission grants",
		Long: `List all permission grants that have been saved with "always" or "never".

Permissions are stored in ~/.config/conductor/permissions.yaml and are keyed
by workflow content hash. When a workflow is modified, stored permissions
are reset.

Example:
  conductor security list-permissions
  conductor security list-permissions --json`,
		RunE: runSecurityListPermissions,
	}

	return cmd
}

func runSecurityListPermissions(cmd *cobra.Command, args []string) error {
	// TODO: Implement permission persistence (P4-T7)
	return fmt.Errorf("not implemented - permission persistence coming in P4-T7")
}

func newSecurityRevokeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <workflow-id>",
		Short: "Revoke a stored permission grant",
		Long: `Revoke a permission grant for a workflow.

This removes the "always" or "never" decision saved for a workflow,
requiring a new prompt on next run.

Example:
  conductor security revoke my-workflow-abc123`,
		Args: cobra.ExactArgs(1),
		RunE: runSecurityRevoke,
	}

	return cmd
}

func runSecurityRevoke(cmd *cobra.Command, args []string) error {
	// TODO: Implement permission persistence (P4-T7)
	return fmt.Errorf("not implemented - permission persistence coming in P4-T7")
}
