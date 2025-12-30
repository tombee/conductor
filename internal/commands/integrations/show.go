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

// NewShowCommand creates the integrations show command.
func NewShowCommand() *cobra.Command {
	var workspaceName string

	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show integration details",
		Long: `Show detailed information about an integration.

Displays all configuration including type, base URL, headers, and timeout.
Credentials are redacted and never shown in the output.

Examples:
  # Show integration details
  conductor integrations show github

  # Show integration in specific workspace
  conductor integrations show github --workspace frontend`,
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

			integration, err := storage.GetIntegration(ctx, workspaceName, name)
			if err != nil {
				if err == workspace.ErrIntegrationNotFound {
					return fmt.Errorf("integration %q not found in workspace %q\n\nTo add: conductor integrations add <type> --name %s", name, workspaceName, name)
				}
				return fmt.Errorf("failed to get integration: %w", err)
			}

			if shared.GetJSON() {
				output := map[string]interface{}{
					"id":              integration.ID,
					"workspace":       integration.WorkspaceName,
					"name":            integration.Name,
					"type":            integration.Type,
					"base_url":        integration.BaseURL,
					"auth_configured": redactAuth(integration.Auth),
					"headers":         integration.Headers,
					"timeout":         integration.TimeoutSeconds,
					"created_at":      integration.CreatedAt,
					"updated_at":      integration.UpdatedAt,
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Printf("Integration: %s\n\n", integration.Name)
			fmt.Printf("Workspace:  %s\n", integration.WorkspaceName)
			fmt.Printf("Type:       %s\n", integration.Type)
			if integration.BaseURL != "" {
				fmt.Printf("Base URL:   %s\n", integration.BaseURL)
			}
			fmt.Printf("Auth:       %s\n", redactAuth(integration.Auth))
			if len(integration.Headers) > 0 {
				fmt.Printf("Headers:\n")
				for k, v := range integration.Headers {
					fmt.Printf("  %s: %s\n", k, v)
				}
			}
			fmt.Printf("Timeout:    %ds\n", integration.TimeoutSeconds)
			fmt.Printf("\nCreated:    %s\n", integration.CreatedAt.Format(time.RFC3339))
			fmt.Printf("Updated:    %s\n", integration.UpdatedAt.Format(time.RFC3339))

			return nil
		},
	}

	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Workspace to show integration from (defaults to current workspace)")

	return cmd
}
