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
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewListCommand creates the workspace list command.
func NewListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workspaces",
		Long: `List all workspaces.

Displays workspace name, description, and integration count.
The current active workspace is marked with an asterisk (*).

Examples:
  # List all workspaces
  conductor workspace list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			storage, err := getStorage(ctx)
			if err != nil {
				return err
			}
			defer storage.Close()

			// Get current workspace
			currentWorkspace, err := storage.GetCurrentWorkspace(ctx)
			if err != nil {
				return fmt.Errorf("failed to get current workspace: %w", err)
			}

			// List all workspaces
			workspaces, err := storage.ListWorkspaces(ctx)
			if err != nil {
				return fmt.Errorf("failed to list workspaces: %w", err)
			}

			if shared.GetJSON() {
				output := make([]map[string]interface{}, len(workspaces))
				for i, ws := range workspaces {
					integrations, _ := storage.ListIntegrations(ctx, ws.Name)
					output[i] = map[string]interface{}{
						"name":              ws.Name,
						"description":       ws.Description,
						"integration_count": len(integrations),
						"current":           ws.Name == currentWorkspace,
						"created_at":        ws.CreatedAt,
						"updated_at":        ws.UpdatedAt,
					}
				}
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"workspaces": output,
					"current":    currentWorkspace,
				})
			}

			if len(workspaces) == 0 {
				fmt.Println(shared.Muted.Render("No workspaces found"))
				return nil
			}

			fmt.Printf("%s %s\n\n",
				shared.Header.Render("Workspaces"),
				shared.Muted.Render(fmt.Sprintf("(current: %s)", currentWorkspace)))
			fmt.Printf("  %s %s %s\n",
				shared.Bold.Render(fmt.Sprintf("%-15s", "NAME")),
				shared.Bold.Render(fmt.Sprintf("%-34s", "DESCRIPTION")),
				shared.Bold.Render("INTEGRATIONS"))

			for _, ws := range workspaces {
				integrations, _ := storage.ListIntegrations(ctx, ws.Name)
				var marker string
				var nameDisplay string
				if ws.Name == currentWorkspace {
					marker = shared.StatusOK.Render("*")
					nameDisplay = shared.Bold.Render(truncate(ws.Name, 15))
				} else {
					marker = " "
					nameDisplay = truncate(ws.Name, 15)
				}
				description := ws.Description
				if description == "" {
					description = shared.Muted.Render("-")
				} else {
					description = truncate(description, 34)
				}
				fmt.Printf("%s %-15s %-34s %d\n",
					marker,
					nameDisplay,
					description,
					len(integrations))
			}

			return nil
		},
	}

	return cmd
}

// truncate truncates a string to the specified length.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
