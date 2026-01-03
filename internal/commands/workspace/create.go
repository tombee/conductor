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

// NewCreateCommand creates the workspace create command.
func NewCreateCommand() *cobra.Command {
	var description string

	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new workspace",
		Long: `Create a new workspace for organizing integrations.

A workspace is a configuration boundary containing named integrations.
You can switch between workspaces using 'conductor workspace use <name>'.

Examples:
  # Create a workspace
  conductor workspace create frontend

  # Create with description
  conductor workspace create frontend --description "Frontend team workspace"`,
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

			// Create workspace
			ws := &workspace.Workspace{
				Name:        workspaceName,
				Description: description,
			}

			err = storage.CreateWorkspace(ctx, ws)
			if err != nil {
				if errors.Is(err, workspace.ErrWorkspaceExists) {
					return fmt.Errorf("workspace '%s' already exists", workspaceName)
				}
				return fmt.Errorf("failed to create workspace: %w", err)
			}

			if shared.GetJSON() {
				output := map[string]interface{}{
					"name":        ws.Name,
					"description": ws.Description,
					"created_at":  ws.CreatedAt,
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Printf("%s Created workspace %s\n",
				shared.StatusOK.Render(shared.SymbolOK),
				shared.Bold.Render(workspaceName))
			if description != "" {
				fmt.Printf("  %s %s\n", shared.Muted.Render("Description:"), description)
			}
			fmt.Printf("\n%s Switch to this workspace: %s\n",
				shared.StatusInfo.Render(shared.SymbolInfo),
				shared.Bold.Render(fmt.Sprintf("conductor workspace use %s", workspaceName)))

			return nil
		},
	}

	cmd.Flags().StringVar(&description, "description", "", "Description of the workspace")

	return cmd
}
