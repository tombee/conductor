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
	"strings"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/triggers"
)

var (
	apiName   string
	apiSecret string
)

func newAddAPICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api WORKFLOW",
		Short: "Add an API trigger",
		Long: `Add an API trigger that invokes a workflow via HTTP API.

API triggers provide a stable, named way to invoke workflows programmatically.
They can be called with: POST /v1/triggers/{name}`,
		Example: `  # Add deployment API trigger
  conductor triggers add api deploy.yaml \
    --name=deploy-trigger \
    --secret='${DEPLOY_SECRET}'

  # Add public API trigger (no auth)
  conductor triggers add api health.yaml \
    --name=health-check

  # Dry-run to preview
  conductor triggers add api test.yaml \
    --name=test-trigger \
    --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: runAddAPI,
	}

	cmd.Flags().StringVar(&apiName, "name", "", "Unique trigger name (required)")
	cmd.Flags().StringVar(&apiSecret, "secret", "", "Secret for authentication (e.g., ${VAR_NAME})")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")

	cmd.MarkFlagRequired("name")

	return cmd
}

func runAddAPI(cmd *cobra.Command, args []string) error {
	workflow := args[0]

	req := triggers.CreateEndpointRequest{
		Workflow: workflow,
		Name:     apiName,
		Secret:   apiSecret,
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry-run: Would add API trigger:\n")
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
		return fmt.Errorf("failed to add API trigger: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "API trigger created!\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Trigger URL: %s\n", getEndpointURL(apiName))
	if apiSecret != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Secret: Set %s environment variable\n", strings.Trim(apiSecret, "${}"))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}
