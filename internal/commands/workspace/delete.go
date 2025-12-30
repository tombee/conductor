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

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/workspace"
)

// NewDeleteCommand creates the workspace delete command.
func NewDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a workspace",
		Long: `Delete a workspace and all its integrations.

This operation cannot be undone. All integrations in the workspace will be deleted.
The default workspace cannot be deleted.

Examples:
  # Delete a workspace
  conductor workspace delete frontend`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceName := args[0]

			// Prevent deletion of default workspace
			if workspaceName == "default" {
				return fmt.Errorf("cannot delete the default workspace")
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			storage, err := getStorage(ctx)
			if err != nil {
				return err
			}
			defer storage.Close()

			// Check if this is the current workspace
			currentWorkspace, err := storage.GetCurrentWorkspace(ctx)
			if err != nil {
				return fmt.Errorf("failed to get current workspace: %w", err)
			}

			// Delete workspace
			err = storage.DeleteWorkspace(ctx, workspaceName)
			if err != nil {
				if errors.Is(err, workspace.ErrWorkspaceNotFound) {
					return fmt.Errorf("workspace '%s' not found", workspaceName)
				}
				return fmt.Errorf("failed to delete workspace: %w", err)
			}

			// If we deleted the current workspace, switch to default
			if workspaceName == currentWorkspace {
				if err := storage.SetCurrentWorkspace(ctx, "default"); err != nil {
					return fmt.Errorf("failed to switch to default workspace: %w", err)
				}
			}

			if shared.GetJSON() {
				output := map[string]interface{}{
					"deleted":          workspaceName,
					"switched_default": workspaceName == currentWorkspace,
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Printf("Deleted workspace '%s'\n", workspaceName)
			if workspaceName == currentWorkspace {
				fmt.Println("Switched to 'default' workspace")
			}

			return nil
		},
	}

	return cmd
}
