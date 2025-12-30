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

// NewTestCommand creates the integrations test command.
func NewTestCommand() *cobra.Command {
	var workspaceName string

	cmd := &cobra.Command{
		Use:   "test <name>",
		Short: "Test integration connectivity",
		Long: `Test connectivity to an integration's API.

This validates that:
  - The integration is configured correctly
  - Authentication credentials are valid
  - The API endpoint is reachable

Credentials are never exposed in the output.

Examples:
  # Test integration connectivity
  conductor integrations test github

  # Test integration in specific workspace
  conductor integrations test github --workspace frontend`,
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

			// TODO: Implement actual connectivity testing
			// For now, just validate the configuration
			status := "unknown"
			message := "Connectivity testing not yet implemented"

			// Basic validation checks
			if integration.BaseURL == "" && workspace.DefaultBaseURL(integration.Type) == "" {
				status = "invalid_config"
				message = "Base URL not configured and no default available for this integration type"
			} else if integration.Auth.Type == workspace.AuthTypeNone {
				status = "unknown"
				message = "No authentication configured - cannot test connectivity"
			} else {
				// Validate auth has required fields
				if err := validateAuthConfig(integration.Auth); err != nil {
					status = "invalid_config"
					message = err.Error()
				} else {
					status = "not_implemented"
					message = "Integration test functionality will be implemented in a future phase"
				}
			}

			if shared.GetJSON() {
				output := map[string]interface{}{
					"workspace": workspaceName,
					"name":      name,
					"type":      integration.Type,
					"status":    status,
					"message":   message,
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Printf("Testing integration '%s' (%s)\n\n", name, integration.Type)
			fmt.Printf("Workspace:  %s\n", workspaceName)
			if integration.BaseURL != "" {
				fmt.Printf("Base URL:   %s\n", integration.BaseURL)
			} else if defaultURL := workspace.DefaultBaseURL(integration.Type); defaultURL != "" {
				fmt.Printf("Base URL:   %s (default)\n", defaultURL)
			}
			fmt.Printf("Auth:       %s\n", redactAuth(integration.Auth))
			fmt.Printf("\nStatus:     %s\n", status)
			fmt.Printf("Message:    %s\n", message)

			if status == "invalid_config" {
				return fmt.Errorf("integration configuration is invalid")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Workspace containing the integration (defaults to current workspace)")

	return cmd
}
