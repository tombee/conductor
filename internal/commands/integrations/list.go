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
				fmt.Printf("No integrations configured in workspace '%s'\n", workspaceName)
				fmt.Println("\nAdd one with: conductor integrations add <type>")
				return nil
			}

			fmt.Printf("Integrations in workspace '%s':\n\n", workspaceName)
			fmt.Println("NAME            TYPE            BASE URL                               AUTH")
			fmt.Println("--------------- --------------- -------------------------------------- --------------------")
			for _, integration := range integrations {
				baseURL := integration.BaseURL
				if baseURL == "" {
					baseURL = "-"
				}
				fmt.Printf("%-15s %-15s %-38s %s\n",
					truncate(integration.Name, 15),
					truncate(integration.Type, 15),
					truncate(baseURL, 38),
					truncate(redactAuth(integration.Auth), 20))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Workspace to list integrations from (defaults to current workspace)")

	return cmd
}
