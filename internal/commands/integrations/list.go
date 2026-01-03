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
)

// NewListCommand creates the integrations list command.
func NewListCommand() *cobra.Command {
	var workspaceName string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List integrations",
		Long: `List all integrations in a workspace.

Displays integration name, type, base URL, and auth status.
Credentials are never shown in the output.

Examples:
  # List integrations in default workspace
  conductor integrations list

  # List integrations in specific workspace
  conductor integrations list --workspace frontend`,
		RunE: func(cmd *cobra.Command, args []string) error {
			workspaceName = getWorkspaceName(workspaceName)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			storage, err := getStorage(ctx)
			if err != nil {
				return err
			}
			defer storage.Close()

			integrations, err := storage.ListIntegrations(ctx, workspaceName)
			if err != nil {
				return fmt.Errorf("failed to list integrations: %w", err)
			}

			if shared.GetJSON() {
				output := make([]map[string]interface{}, len(integrations))
				for i, integration := range integrations {
					output[i] = map[string]interface{}{
						"name":            integration.Name,
						"type":            integration.Type,
						"base_url":        integration.BaseURL,
						"auth_configured": redactAuth(integration.Auth),
						"timeout":         integration.TimeoutSeconds,
						"created_at":      integration.CreatedAt,
						"updated_at":      integration.UpdatedAt,
					}
				}
				return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"workspace":    workspaceName,
					"integrations": output,
				})
			}

			if len(integrations) == 0 {
				fmt.Printf("%s No integrations configured in workspace %s\n",
					shared.Muted.Render(shared.SymbolInfo),
					shared.Bold.Render(workspaceName))
				fmt.Println()
				fmt.Printf("Add one with: %s\n", shared.Bold.Render("conductor integrations add <type>"))
				return nil
			}

			fmt.Printf("%s %s\n\n",
				shared.Header.Render("Integrations"),
				shared.Muted.Render("(workspace: "+workspaceName+")"))
			fmt.Printf("%s %s %s %s\n",
				shared.Bold.Render(fmt.Sprintf("%-15s", "NAME")),
				shared.Bold.Render(fmt.Sprintf("%-15s", "TYPE")),
				shared.Bold.Render(fmt.Sprintf("%-38s", "BASE URL")),
				shared.Bold.Render("AUTH"))
			for _, integration := range integrations {
				baseURL := integration.BaseURL
				if baseURL == "" {
					baseURL = shared.Muted.Render("-")
				} else {
					baseURL = truncate(baseURL, 38)
				}
				authStatus := redactAuth(integration.Auth)
				var authStyled string
				if authStatus == "(none)" {
					authStyled = shared.Muted.Render(authStatus)
				} else {
					authStyled = shared.StatusOK.Render(authStatus)
				}
				fmt.Printf("%-15s %s %-38s %s\n",
					shared.Bold.Render(truncate(integration.Name, 15)),
					shared.Muted.Render(fmt.Sprintf("%-15s", truncate(integration.Type, 15))),
					baseURL,
					authStyled)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Workspace to list integrations from (defaults to current workspace)")

	return cmd
}
