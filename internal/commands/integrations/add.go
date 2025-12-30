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

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/workspace"
)

// NewAddCommand creates the integrations add command.
func NewAddCommand() *cobra.Command {
	var (
		workspaceName string
		name          string
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
		Use:   "add <type>",
		Short: "Add a new integration",
		Long: `Add a new integration to connect to an external service.

The integration type determines which service to connect to (github, slack, jira, etc.).
You can configure authentication, base URL, headers, and timeout.

Authentication types:
  token  - Bearer token authentication (most APIs)
  basic  - Username/password authentication
  api-key - Custom header-based authentication
  none   - No authentication

Secret references:
  Instead of hardcoding secrets, you can reference them:
    ${VAR}        - Environment variable
    env:VAR       - Explicit environment variable
    file:/path    - Read from file
    keychain:name - System keychain entry

Examples:
  # Add GitHub integration with environment variable
  conductor integrations add github --token '${GITHUB_TOKEN}'

  # Add GitHub Enterprise with explicit configuration
  conductor integrations add github --name work \
    --base-url "https://github.mycompany.com/api/v3" \
    --token '${WORK_GITHUB_TOKEN}'

  # Add Jira with basic auth
  conductor integrations add jira \
    --base-url "https://mycompany.atlassian.net" \
    --auth-type basic \
    --username "user@company.com" \
    --password '${JIRA_API_TOKEN}'

  # Add custom API with API key
  conductor integrations add custom \
    --base-url "https://api.example.com" \
    --auth-type api-key \
    --api-key-header "X-API-Key" \
    --api-key-value '${API_KEY}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			integrationType := args[0]
			workspaceName = getWorkspaceName(workspaceName)

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			storage, err := getStorage(ctx)
			if err != nil {
				return err
			}
			defer storage.Close()

			// Default name to type if not specified
			if name == "" {
				name = integrationType
			}

			// Default base URL from type if not specified
			if baseURL == "" {
				baseURL = workspace.DefaultBaseURL(integrationType)
			}

			// Parse auth configuration
			var authConfig workspace.AuthConfig
			switch authType {
			case "", "token":
				authConfig = workspace.AuthConfig{
					Type:  workspace.AuthTypeToken,
					Token: token,
				}
			case "basic":
				authConfig = workspace.AuthConfig{
					Type:     workspace.AuthTypeBasic,
					Username: username,
					Password: password,
				}
			case "api-key":
				authConfig = workspace.AuthConfig{
					Type:         workspace.AuthTypeAPIKey,
					APIKeyHeader: apiKeyHeader,
					APIKeyValue:  apiKeyValue,
				}
			case "none":
				authConfig = workspace.AuthConfig{
					Type: workspace.AuthTypeNone,
				}
			default:
				return fmt.Errorf("invalid auth type %q, must be one of: token, basic, api-key, none", authType)
			}

			// Validate auth configuration
			if err := validateAuthConfig(authConfig); err != nil {
				return err
			}

			// Parse headers
			headerMap := make(map[string]string)
			for _, header := range headers {
				parts := strings.SplitN(header, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid header format %q, expected key=value", header)
				}
				headerMap[parts[0]] = parts[1]
			}

			// Default timeout
			if timeout == 0 {
				timeout = workspace.DefaultTimeout
			}

			// Create integration
			integration := &workspace.Integration{
				ID:             uuid.New().String(),
				WorkspaceName:  workspaceName,
				Name:           name,
				Type:           integrationType,
				BaseURL:        baseURL,
				Auth:           authConfig,
				Headers:        headerMap,
				TimeoutSeconds: timeout,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			}

			if err := storage.CreateIntegration(ctx, integration); err != nil {
				if err == workspace.ErrWorkspaceNotFound {
					return fmt.Errorf("workspace %q not found\n\nTo create: conductor workspace create %s", workspaceName, workspaceName)
				}
				if err == workspace.ErrIntegrationExists {
					return fmt.Errorf("integration %q already exists in workspace %q\n\nTo update: conductor integrations update %s --workspace %s", name, workspaceName, name, workspaceName)
				}
				return fmt.Errorf("failed to create integration: %w", err)
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
					"created_at":      integration.CreatedAt,
				}
				return json.NewEncoder(os.Stdout).Encode(output)
			}

			fmt.Printf("✓ Added integration '%s' to workspace '%s'\n", name, workspaceName)
			fmt.Printf("\nType:     %s\n", integrationType)
			fmt.Printf("Auth:     %s\n", redactAuth(authConfig))
			if baseURL != "" {
				fmt.Printf("Base URL: %s\n", baseURL)
			}
			fmt.Printf("\nTest it with:\n")
			fmt.Printf("  conductor integrations test %s", name)
			if workspaceName != "default" {
				fmt.Printf(" --workspace %s", workspaceName)
			}
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Workspace to add integration to (defaults to current workspace)")
	cmd.Flags().StringVar(&name, "name", "", "Integration name (defaults to type)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "API base URL (has sensible defaults per type)")
	cmd.Flags().StringVar(&authType, "auth-type", "token", "Authentication type: token, basic, api-key, none")
	cmd.Flags().StringVar(&token, "token", "", "Bearer token (for token auth)")
	cmd.Flags().StringVar(&username, "username", "", "Username (for basic auth)")
	cmd.Flags().StringVar(&password, "password", "", "Password (for basic auth)")
	cmd.Flags().StringVar(&apiKeyHeader, "api-key-header", "", "Header name (for api-key auth)")
	cmd.Flags().StringVar(&apiKeyValue, "api-key-value", "", "Header value (for api-key auth)")
	cmd.Flags().StringArrayVar(&headers, "header", []string{}, "Additional header key=value (can be specified multiple times)")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Request timeout in seconds (default 30)")

	return cmd
}

// validateAuthConfig validates that required fields are present for the auth type.
func validateAuthConfig(auth workspace.AuthConfig) error {
	switch auth.Type {
	case workspace.AuthTypeToken:
		if auth.Token == "" {
			return fmt.Errorf("--token is required for token authentication")
		}
	case workspace.AuthTypeBasic:
		if auth.Username == "" {
			return fmt.Errorf("--username is required for basic authentication")
		}
		if auth.Password == "" {
			return fmt.Errorf("--password is required for basic authentication")
		}
	case workspace.AuthTypeAPIKey:
		if auth.APIKeyHeader == "" {
			return fmt.Errorf("--api-key-header is required for api-key authentication")
		}
		if auth.APIKeyValue == "" {
			return fmt.Errorf("--api-key-value is required for api-key authentication")
		}
	case workspace.AuthTypeNone:
		// No validation needed
	}
	return nil
}
