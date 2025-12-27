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

package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/pkg/workflow/schema"
	"gopkg.in/yaml.v3"
)

// NewSchemaCommand creates the schema command
func NewSchemaCommand() *cobra.Command {
	var (
		outputFormat string
		writeToFile  bool
		force        bool
	)

	cmd := &cobra.Command{
		Use:   "schema",
		Annotations: map[string]string{
			"group": "workflow",
		},
		Short: "Output the workflow JSON Schema",
		Long: `Output the embedded JSON Schema for Conductor workflow definitions.

The schema can be used for IDE autocompletion, validation, and AI-assisted
workflow authoring. By default, it outputs to stdout in JSON format.

Use the --write flag to save the schema to ./schemas/workflow.schema.json
in the current directory.

See also: conductor validate, conductor examples list`,
		Example: `  # Example 1: Output schema to stdout
  conductor schema

  # Example 2: Save schema to file for IDE integration
  conductor schema --write

  # Example 3: Output schema in YAML format
  conductor schema --output yaml

  # Example 4: Extract specific schema properties
  conductor schema | jq '.properties.steps'

  # Example 5: Validate workflow using extracted schema
  conductor schema > workflow-schema.json
  conductor validate workflow.yaml --schema workflow-schema.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Get the embedded schema
			schemaBytes := schema.GetEmbeddedSchema()

			// Handle output format
			var output []byte
			var err error

			switch outputFormat {
			case "json":
				// Pretty-print JSON
				var schemaObj interface{}
				if err := json.Unmarshal(schemaBytes, &schemaObj); err != nil {
					return fmt.Errorf("failed to parse embedded schema: %w", err)
				}
				output, err = json.MarshalIndent(schemaObj, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to format JSON: %w", err)
				}

			case "yaml":
				// Convert JSON to YAML
				var schemaObj interface{}
				if err := json.Unmarshal(schemaBytes, &schemaObj); err != nil {
					return fmt.Errorf("failed to parse embedded schema: %w", err)
				}
				output, err = yaml.Marshal(schemaObj)
				if err != nil {
					return fmt.Errorf("failed to convert to YAML: %w", err)
				}

			default:
				return &shared.ExitError{
					Code:    2,
					Message: fmt.Sprintf("invalid output format: %s (must be 'json' or 'yaml')", outputFormat),
				}
			}

			// Write to file if requested
			if writeToFile {
				destPath := filepath.Join(".", "schemas", "workflow.schema.json")

				// Check if file exists
				if _, err := os.Stat(destPath); err == nil && !force {
					return &shared.ExitError{
						Code:    1,
						Message: fmt.Sprintf("file already exists: %s (use --force to overwrite)", destPath),
					}
				}

				// Create directory if it doesn't exist
				destDir := filepath.Dir(destPath)
				if err := os.MkdirAll(destDir, 0755); err != nil {
					return &shared.ExitError{
						Code:    1,
						Message: fmt.Sprintf("failed to create directory: %s", destDir),
						Cause:   err,
					}
				}

				// Write the file (always JSON, regardless of output format)
				if err := os.WriteFile(destPath, schemaBytes, 0644); err != nil {
					return &shared.ExitError{
						Code:    1,
						Message: fmt.Sprintf("failed to write file: %s", destPath),
						Cause:   err,
					}
				}

				cmd.Printf("✓ Schema written to %s\n", destPath)
				return nil
			}

			// Output to stdout
			cmd.Println(string(output))
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "json", "Output format: json (default), yaml")
	cmd.Flags().BoolVarP(&writeToFile, "write", "w", false, "Write to ./schemas/workflow.schema.json in current directory")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing file (only with --write)")

	return cmd
}
