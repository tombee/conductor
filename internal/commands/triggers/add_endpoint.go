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

package triggers

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/triggers"
)

var (
	endpointName   string
	endpointSecret string
)

func newAddEndpointCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoint WORKFLOW",
		Short: "Add an API endpoint trigger",
		Long: `Add an API endpoint trigger that invokes a workflow via HTTP API.

API endpoints provide a stable, named way to invoke workflows programmatically.
They can be called with: POST /v1/endpoints/{name}`,
		Example: `  # Add deployment endpoint
  conductor triggers add endpoint deploy.yaml \
    --name=deploy-trigger \
    --secret='${DEPLOY_SECRET}'

  # Add public endpoint (no auth)
  conductor triggers add endpoint health.yaml \
    --name=health-check

  # Dry-run to preview
  conductor triggers add endpoint test.yaml \
    --name=test-endpoint \
    --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: runAddEndpoint,
	}

	cmd.Flags().StringVar(&endpointName, "name", "", "Unique endpoint name (required)")
	cmd.Flags().StringVar(&endpointSecret, "secret", "", "Secret for authentication (e.g., ${VAR_NAME})")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")

	cmd.MarkFlagRequired("name")

	return cmd
}

func runAddEndpoint(cmd *cobra.Command, args []string) error {
	workflow := args[0]

	req := triggers.CreateEndpointRequest{
		Workflow: workflow,
		Name:     endpointName,
		Secret:   endpointSecret,
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry-run: Would add endpoint trigger:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Name: %s\n", req.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "  Workflow: %s\n", req.Workflow)
		if req.Secret != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Secret: %s\n", req.Secret)
		}
		return nil
	}

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := mgr.AddEndpoint(ctx, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Endpoint trigger created!\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Endpoint URL: %s\n", getEndpointURL(endpointName))
	if endpointSecret != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Secret: Set %s environment variable\n", strings.Trim(endpointSecret, "${}"))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}
