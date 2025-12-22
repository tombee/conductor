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

package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/workspace"
)

// NewRemoveCommand creates the integrations remove command.
func NewRemoveCommand() *cobra.Command {
	var workspaceName string

	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an integration",
		Long: `Remove an integration from a workspace.

This permanently deletes the integration and all its configuration.
Workflows that depend on this integration will fail to run.

Examples:
  # Remove an integration
  conductor integrations remove github

  # Remove from specific workspace
  conductor integrations remove github --workspace frontend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			workspaceName = getWorkspaceName(workspaceName)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			storage, err := getStorage(ctx)
			if err != nil {
				return err
			}
			defer storage.Close()

			// Verify integration exists before deleting
			_, err = storage.GetIntegration(ctx, workspaceName, name)
			if err != nil {
				if err == workspace.ErrIntegrationNotFound {
					return fmt.Errorf("integration %q not found in workspace %q", name, workspaceName)
				}
				return fmt.Errorf("failed to get integration: %w", err)
			}

			if err := storage.DeleteIntegration(ctx, workspaceName, name); err != nil {
				return fmt.Errorf("failed to remove integration: %w", err)
			}

			if shared.GetJSON() {
				output := map[string]interface{}{
					"workspace": workspaceName,
					"name":      name,
					"status":    "removed",
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Println(shared.RenderOK(fmt.Sprintf("Removed integration '%s' from workspace '%s'", name, workspaceName)))

			return nil
		},
	}

	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Workspace to remove integration from (defaults to current workspace)")

	return cmd
}
