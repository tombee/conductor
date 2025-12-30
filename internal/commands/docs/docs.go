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

package docs

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

const docsBaseURL = "https://tombee.github.io/conductor"

// DocResource represents a documentation resource with its URL
type DocResource struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// DocsResponse is the JSON response for docs commands
type DocsResponse struct {
	shared.JSONResponse
	Resources []DocResource `json:"resources"`
}

// NewDocsCommand creates the docs command
func NewDocsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Annotations: map[string]string{
			"group": "documentation",
		},
		Short: "Show documentation URLs",
		Long: `Display URLs to various documentation resources.

Use subcommands to get specific documentation sections:
  conductor docs cli        - CLI reference documentation
  conductor docs schema     - Workflow schema documentation
  conductor docs config     - Configuration file documentation
  conductor docs workflows  - Workflow examples and guides`,
		RunE: func(cmd *cobra.Command, args []string) error {
			useJSON := shared.GetJSON()
			out := cmd.OutOrStdout()

			resources := []DocResource{
				{
					Name:        "Getting Started",
					Description: "Installation and quickstart guide",
					URL:         docsBaseURL + "/getting-started/",
				},
				{
					Name:        "CLI Reference",
					Description: "Complete command-line interface reference",
					URL:         docsBaseURL + "/reference/cli/",
				},
				{
					Name:        "Workflow Schema",
					Description: "YAML workflow file schema and examples",
					URL:         docsBaseURL + "/reference/schema/",
				},
				{
					Name:        "Configuration",
					Description: "Configuration file format and options",
					URL:         docsBaseURL + "/reference/configuration/",
				},
				{
					Name:        "Workflow Examples",
					Description: "Example workflows and patterns",
					URL:         docsBaseURL + "/workflows/",
				},
			}

			if useJSON {
				resp := DocsResponse{
					JSONResponse: shared.JSONResponse{
						Version: "1.0",
						Command: "docs",
						Success: true,
					},
					Resources: resources,
				}
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(resp)
			}

			// Human-readable output
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Conductor Documentation:")
			fmt.Fprintln(out)
			for _, r := range resources {
				fmt.Fprintf(out, "  %s\n", r.Name)
				fmt.Fprintf(out, "    %s\n", r.Description)
				fmt.Fprintf(out, "    %s\n", r.URL)
				fmt.Fprintln(out)
			}

			return nil
		},
	}

	// Add subcommands
	cmd.AddCommand(newDocsCLICmd())
	cmd.AddCommand(newDocsSchemaCmd())
	cmd.AddCommand(newDocsConfigCmd())
	cmd.AddCommand(newDocsWorkflowsCmd())

	return cmd
}

func newDocsCLICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli",
		Short: "Show CLI reference documentation URL",
		Long:  "Display the URL for the complete CLI command reference documentation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			useJSON := shared.GetJSON()
			out := cmd.OutOrStdout()

			resource := DocResource{
				Name:        "CLI Reference",
				Description: "Complete command-line interface reference",
				URL:         docsBaseURL + "/reference/cli/",
			}

			if useJSON {
				resp := DocsResponse{
					JSONResponse: shared.JSONResponse{
						Version: "1.0",
						Command: "docs cli",
						Success: true,
					},
					Resources: []DocResource{resource},
				}
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(resp)
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "%s\n", resource.Name)
			fmt.Fprintf(out, "  %s\n", resource.Description)
			fmt.Fprintf(out, "  %s\n", resource.URL)
			fmt.Fprintln(out)

			return nil
		},
	}

	return cmd
}

func newDocsSchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Show workflow schema documentation URL",
		Long:  "Display the URL for workflow YAML schema documentation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			useJSON := shared.GetJSON()
			out := cmd.OutOrStdout()

			resource := DocResource{
				Name:        "Workflow Schema",
				Description: "YAML workflow file schema and examples",
				URL:         docsBaseURL + "/reference/schema/",
			}

			if useJSON {
				resp := DocsResponse{
					JSONResponse: shared.JSONResponse{
						Version: "1.0",
						Command: "docs schema",
						Success: true,
					},
					Resources: []DocResource{resource},
				}
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(resp)
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "%s\n", resource.Name)
			fmt.Fprintf(out, "  %s\n", resource.Description)
			fmt.Fprintf(out, "  %s\n", resource.URL)
			fmt.Fprintln(out)

			return nil
		},
	}

	return cmd
}

func newDocsConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show configuration documentation URL",
		Long:  "Display the URL for configuration file format and options documentation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			useJSON := shared.GetJSON()
			out := cmd.OutOrStdout()

			resource := DocResource{
				Name:        "Configuration",
				Description: "Configuration file format and options",
				URL:         docsBaseURL + "/reference/configuration/",
			}

			if useJSON {
				resp := DocsResponse{
					JSONResponse: shared.JSONResponse{
						Version: "1.0",
						Command: "docs config",
						Success: true,
					},
					Resources: []DocResource{resource},
				}
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(resp)
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "%s\n", resource.Name)
			fmt.Fprintf(out, "  %s\n", resource.Description)
			fmt.Fprintf(out, "  %s\n", resource.URL)
			fmt.Fprintln(out)

			return nil
		},
	}

	return cmd
}

func newDocsWorkflowsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflows",
		Short: "Show workflow examples documentation URL",
		Long:  "Display the URL for workflow examples and patterns documentation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			useJSON := shared.GetJSON()
			out := cmd.OutOrStdout()

			resource := DocResource{
				Name:        "Workflow Examples",
				Description: "Example workflows and patterns",
				URL:         docsBaseURL + "/workflows/",
			}

			if useJSON {
				resp := DocsResponse{
					JSONResponse: shared.JSONResponse{
						Version: "1.0",
						Command: "docs workflows",
						Success: true,
					},
					Resources: []DocResource{resource},
				}
				encoder := json.NewEncoder(out)
				encoder.SetIndent("", "  ")
				return encoder.Encode(resp)
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "%s\n", resource.Name)
			fmt.Fprintf(out, "  %s\n", resource.Description)
			fmt.Fprintf(out, "  %s\n", resource.URL)
			fmt.Fprintln(out)

			return nil
		},
	}

	return cmd
}
