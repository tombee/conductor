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

// NewUseCommand creates the workspace use command.
func NewUseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Set the current workspace",
		Long: `Set the current active workspace.

The current workspace is used by default for all commands that accept a --workspace flag.

Examples:
  # Switch to frontend workspace
  conductor workspace use frontend

  # Switch back to default
  conductor workspace use default`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceName := args[0]

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			storage, err := getStorage(ctx)
			if err != nil {
				return err
			}
			defer storage.Close()

			// Set current workspace
			err = storage.SetCurrentWorkspace(ctx, workspaceName)
			if err != nil {
				if errors.Is(err, workspace.ErrWorkspaceNotFound) {
					// List available workspaces
					workspaces, listErr := storage.ListWorkspaces(ctx)
					if listErr == nil && len(workspaces) > 0 {
						fmt.Fprintf(os.Stderr, "Error: Workspace '%s' not found.\n\n", workspaceName)
						fmt.Fprintf(os.Stderr, "Available workspaces:\n")
						for _, ws := range workspaces {
							fmt.Fprintf(os.Stderr, "  - %s\n", ws.Name)
						}
						fmt.Fprintf(os.Stderr, "\nTo create: conductor workspace create %s\n", workspaceName)
					}
					return fmt.Errorf("workspace '%s' not found", workspaceName)
				}
				return fmt.Errorf("failed to set current workspace: %w", err)
			}

			if shared.GetJSON() {
				output := map[string]interface{}{
					"workspace": workspaceName,
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Printf("Switched to workspace '%s'\n", workspaceName)

			return nil
		},
	}

	return cmd
}
