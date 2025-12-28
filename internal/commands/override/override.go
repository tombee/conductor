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

package override

import (
	"github.com/spf13/cobra"
)

// NewCommand creates the override command with subcommands.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "override",
		Short: "Manage security overrides",
		Long: `Manage security overrides for emergency situations.

Security overrides allow bypassing specific security controls with audit logging.
All override operations are logged and time-limited.

Available override types:
  - disable-enforcement: Bypass security policy enforcement
  - disable-sandbox: Bypass sandboxing restrictions

Note: disable-audit type is not available for security reasons.`,
	}

	// Add subcommands
	cmd.AddCommand(newCreateCommand())
	cmd.AddCommand(newListCommand())
	cmd.AddCommand(newRevokeCommand())

	return cmd
}
