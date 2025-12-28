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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

func newRevokeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke <type>",
		Short: "Revoke a security override",
		Long: `Revoke an active security override.

Available override types:
  - disable-enforcement: Bypass security policy enforcement
  - disable-sandbox: Bypass sandboxing restrictions`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			overrideType := args[0]

			// Validate override type
			if !isValidOverrideType(overrideType) {
				return fmt.Errorf("invalid override type: %s (valid types: disable-enforcement, disable-sandbox)", overrideType)
			}

			// Revoke override via daemon API
			url := shared.BuildAPIURL(fmt.Sprintf("/v1/override/%s", overrideType), nil)
			_, err := shared.MakeAPIRequest("DELETE", url, nil)
			if err != nil {
				return fmt.Errorf("failed to revoke override: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Override '%s' revoked successfully\n", overrideType)

			return nil
		},
	}

	return cmd
}
