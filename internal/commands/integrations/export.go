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
	"gopkg.in/yaml.v3"
)

// NewExportCommand creates the integrations export command.
func NewExportCommand() *cobra.Command {
	var (
		workspaceName string
		format        string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export integrations configuration",
		Long: `Export integrations configuration for backup or migration.

Credentials are redacted in the export. The exported configuration can be used
as a template for setting up integrations in another workspace or environment.

Supported formats:
  yaml - YAML format (default)
  json - JSON format

Examples:
  # Export integrations as YAML
  conductor integrations export

  # Export as JSON
  conductor integrations export --format json

  # Export specific workspace
  conductor integrations export --workspace frontend > frontend-integrations.yaml`,
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

			// Build export structure with redacted credentials
			export := map[string]interface{}{
				"workspace":    workspaceName,
				"exported_at":  time.Now().Format(time.RFC3339),
				"integrations": make([]map[string]interface{}, 0, len(integrations)),
			}

			for _, integration := range integrations {
				item := map[string]interface{}{
					"name":    integration.Name,
					"type":    integration.Type,
					"timeout": integration.TimeoutSeconds,
				}

				if integration.BaseURL != "" {
					item["base_url"] = integration.BaseURL
				}

				// Redact auth - only show structure, not values
				authInfo := map[string]interface{}{
					"type": string(integration.Auth.Type),
				}
				switch integration.Auth.Type {
				case "token":
					authInfo["token"] = "REDACTED - configure with --token flag"
				case "basic":
					authInfo["username"] = integration.Auth.Username
					authInfo["password"] = "REDACTED - configure with --password flag"
				case "api-key":
					authInfo["header"] = integration.Auth.APIKeyHeader
					authInfo["value"] = "REDACTED - configure with --api-key-value flag"
				}
				item["auth"] = authInfo

				if len(integration.Headers) > 0 {
					item["headers"] = integration.Headers
				}

				export["integrations"] = append(export["integrations"].([]map[string]interface{}), item)
			}

			// Output in requested format
			switch format {
			case "json":
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(export)
			case "yaml":
				encoder := yaml.NewEncoder(os.Stdout)
				encoder.SetIndent(2)
				defer encoder.Close()
				return encoder.Encode(export)
			default:
				return fmt.Errorf("unsupported format %q, must be yaml or json", format)
			}
		},
	}

	cmd.Flags().StringVar(&workspaceName, "workspace", "", "Workspace to export integrations from (defaults to current workspace)")
	cmd.Flags().StringVar(&format, "format", "yaml", "Output format: yaml, json")

	return cmd
}
