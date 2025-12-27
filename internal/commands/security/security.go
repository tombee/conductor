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
	"github.com/spf13/cobra"
)

// NewCommand creates the security command for security operations.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security",
		Short: "Manage Conductor security settings and permissions",
		Annotations: map[string]string{
			"group": "configuration",
		},
		Long: `Manage security profiles, permissions, and analyze workflow security requirements.

Security profiles control what filesystem paths, network hosts, and commands
agents can access during workflow execution.

Commands:
  status           Show current security profile and permissions
  analyze          Analyze a workflow's security requirements
  generate-profile Generate a custom security profile from workflow analysis
  list-permissions List stored permission grants
  revoke           Revoke a stored permission grant`,
	}

	cmd.AddCommand(newSecurityStatusCommand())
	cmd.AddCommand(newSecurityAnalyzeCommand())
	cmd.AddCommand(newSecurityGenerateProfileCommand())
	cmd.AddCommand(newSecurityListPermissionsCommand())
	cmd.AddCommand(newSecurityRevokeCommand())

	return cmd
}
