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
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/workspace"
)

// NewUpdateCommand creates the integrations update command.
func NewUpdateCommand() *cobra.Command {
	var (
		workspaceName string
		baseURL       string
		authType      string
		token         string
		username      string
		password      string
		apiKeyHeader  string
		apiKeyValue   string
		headers       []string
		timeout       int
	)

	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an integration",
		Long: `Update an existing integration's configuration.

Only specified flags are updated; unspecified fields remain unchanged.

Examples:
  # Update GitHub token
  conductor integrations update github --token '${NEW_TOKEN}'

  # Update base URL
  conductor integrations update github --base-url "https://github.example.com/api/v3"

  # Update timeout
  conductor integrations update github --timeout 60

  # Switch to basic auth
  conductor integrations update myapi \
    --auth-type basic \
    --username "user@example.com" \
    --password '${NEW_PASSWORD}'`,
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

			// Get existing integration
			integration, err := storage.GetIntegration(ctx, workspaceName, name)
			if err != nil {
				if err == workspace.ErrIntegrationNotFound {
					return fmt.Errorf("integration %q not found in workspace %q\n\nTo add: conductor integrations add <type> --name %s", name, workspaceName, name)
				}
				return fmt.Errorf("failed to get integration: %w", err)
			}

			// Track if any changes were made
			changed := false

			// Update base URL if specified
			if cmd.Flags().Changed("base-url") {
				integration.BaseURL = baseURL
				changed = true
			}

			// Update auth if any auth flags were specified
			if cmd.Flags().Changed("auth-type") ||
				cmd.Flags().Changed("token") ||
				cmd.Flags().Changed("username") ||
				cmd.Flags().Changed("password") ||
				cmd.Flags().Changed("api-key-header") ||
				cmd.Flags().Changed("api-key-value") {

				// If auth-type changed, build new config; otherwise update existing
				if cmd.Flags().Changed("auth-type") {
					switch authType {
					case "token":
						integration.Auth = workspace.AuthConfig{
							Type:  workspace.AuthTypeToken,
							Token: token,
						}
					case "basic":
						integration.Auth = workspace.AuthConfig{
							Type:     workspace.AuthTypeBasic,
							Username: username,
							Password: password,
						}
					case "api-key":
						integration.Auth = workspace.AuthConfig{
							Type:         workspace.AuthTypeAPIKey,
							APIKeyHeader: apiKeyHeader,
							APIKeyValue:  apiKeyValue,
						}
					case "none":
						integration.Auth = workspace.AuthConfig{
							Type: workspace.AuthTypeNone,
						}
					default:
						return fmt.Errorf("invalid auth type %q, must be one of: token, basic, api-key, none", authType)
					}
				} else {
					// Update individual auth fields based on current type
					switch integration.Auth.Type {
					case workspace.AuthTypeToken:
						if cmd.Flags().Changed("token") {
							integration.Auth.Token = token
						}
					case workspace.AuthTypeBasic:
						if cmd.Flags().Changed("username") {
							integration.Auth.Username = username
						}
						if cmd.Flags().Changed("password") {
							integration.Auth.Password = password
						}
					case workspace.AuthTypeAPIKey:
						if cmd.Flags().Changed("api-key-header") {
							integration.Auth.APIKeyHeader = apiKeyHeader
						}
						if cmd.Flags().Changed("api-key-value") {
							integration.Auth.APIKeyValue = apiKeyValue
						}
					}
				}

				// Validate auth configuration
				if err := validateAuthConfig(integration.Auth); err != nil {
					return err
				}
				changed = true
			}

			// Update headers if specified
			if cmd.Flags().Changed("header") {
				if integration.Headers == nil {
					integration.Headers = make(map[string]string)
				}
				for _, header := range headers {
					parts := strings.SplitN(header, "=", 2)
					if len(parts) != 2 {
						return fmt.Errorf("invalid header format %q, expected key=value", header)
					}
					integration.Headers[parts[0]] = parts[1]
				}
				changed = true
			}

			// Update timeout if specified
			if cmd.Flags().Changed("timeout") {
				integration.TimeoutSeconds = timeout
				changed = true
			}

			if !changed {
				return fmt.Errorf("no changes specified\n\nSpecify at least one flag to update (--token, --base-url, etc.)")
			}

			// Update timestamp
			integration.UpdatedAt = time.Now()

			if err := storage.UpdateIntegration(ctx, integration); err != nil {
				return fmt.Errorf("failed to update integration: %w", err)
			}

			if shared.GetJSON() {
				output := map[string]interface{}{
					"id":              integration.ID,
					"workspace":       integration.WorkspaceName,
					"name":            integration.Name,
					"type":            integration.Type,
					"base_url":        integration.BaseURL,
					"auth_configured": redactAuth(integration.Auth),
					"timeout":         integration.TimeoutSeconds,
					"updated_at":      integration.UpdatedAt,
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Println(shared.RenderOK(fmt.Sprintf("Updated integration '%s' in workspace '%s'", name, workspaceName)))
			fmt.Printf("\n%s %s\n", shared.Muted.Render("Auth:"), redactAuth(integration.Auth))
			if integration.BaseURL != "" {
				fmt.Printf("%s %s\n", shared.Muted.Render("Base URL:"), integration.BaseURL)
			}
			fmt.Printf("%s %ds\n", shared.Muted.Render("Timeout:"), integration.TimeoutSeconds)

			return nil
		},
	}

	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Workspace containing the integration (defaults to current workspace)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "API base URL")
	cmd.Flags().StringVar(&authType, "auth-type", "", "Authentication type: token, basic, api-key, none")
	cmd.Flags().StringVar(&token, "token", "", "Bearer token (for token auth)")
	cmd.Flags().StringVar(&username, "username", "", "Username (for basic auth)")
	cmd.Flags().StringVar(&password, "password", "", "Password (for basic auth)")
	cmd.Flags().StringVar(&apiKeyHeader, "api-key-header", "", "Header name (for api-key auth)")
	cmd.Flags().StringVar(&apiKeyValue, "api-key-value", "", "Header value (for api-key auth)")
	cmd.Flags().StringArrayVar(&headers, "header", []string{}, "Additional header key=value (can be specified multiple times)")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Request timeout in seconds")

	return cmd
}
