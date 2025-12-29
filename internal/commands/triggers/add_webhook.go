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
	webhookPath   string
	webhookSource string
	webhookSecret string
	webhookEvents []string
	webhookMap    []string
	dryRun        bool
)

func newAddWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook WORKFLOW",
		Short: "Add a webhook trigger",
		Long: `Add a webhook trigger that invokes a workflow when HTTP requests arrive.

The webhook path must be unique. Common sources include:
  - github: GitHub webhook events
  - slack: Slack event subscriptions
  - generic: Generic HTTP POST with JSON payload

Input mapping uses JSONPath to extract values from the webhook payload.`,
		Example: `  # Add GitHub PR webhook
  conductor triggers add webhook security-review.yaml \
    --path=/webhooks/pr-review \
    --source=github \
    --secret='${GITHUB_WEBHOOK_SECRET}' \
    --events=pull_request.opened,pull_request.synchronize \
    --map owner=$.repository.owner.login \
    --map repo=$.repository.name \
    --map pr_number=$.pull_request.number

  # Add generic webhook with no authentication
  conductor triggers add webhook notify.yaml \
    --path=/webhooks/notify \
    --source=generic

  # Dry-run to preview changes
  conductor triggers add webhook deploy.yaml \
    --path=/webhooks/deploy \
    --source=github \
    --dry-run`,
		Args: cobra.ExactArgs(1),
		RunE: runAddWebhook,
	}

	cmd.Flags().StringVar(&webhookPath, "path", "", "Webhook URL path (required)")
	cmd.Flags().StringVar(&webhookSource, "source", "", "Webhook source type: github, slack, generic (required)")
	cmd.Flags().StringVar(&webhookSecret, "secret", "", "Secret for signature verification (e.g., ${VAR_NAME})")
	cmd.Flags().StringSliceVar(&webhookEvents, "events", nil, "Event types to handle (comma-separated)")
	cmd.Flags().StringSliceVar(&webhookMap, "map", nil, "Input mapping: key=jsonpath (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview changes without writing")

	cmd.MarkFlagRequired("path")
	cmd.MarkFlagRequired("source")

	return cmd
}

func runAddWebhook(cmd *cobra.Command, args []string) error {
	workflow := args[0]

	// Parse input mapping
	inputMapping := make(map[string]string)
	for _, mapping := range webhookMap {
		parts := strings.SplitN(mapping, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid mapping format: %s (expected key=jsonpath)", mapping)
		}
		inputMapping[parts[0]] = parts[1]
	}

	req := triggers.CreateWebhookRequest{
		Workflow:     workflow,
		Path:         webhookPath,
		Source:       webhookSource,
		Events:       webhookEvents,
		Secret:       webhookSecret,
		InputMapping: inputMapping,
	}

	if dryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Dry-run: Would add webhook trigger:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Path: %s\n", req.Path)
		fmt.Fprintf(cmd.OutOrStdout(), "  Source: %s\n", req.Source)
		fmt.Fprintf(cmd.OutOrStdout(), "  Workflow: %s\n", req.Workflow)
		if len(req.Events) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Events: %s\n", strings.Join(req.Events, ", "))
		}
		if req.Secret != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "  Secret: %s\n", req.Secret)
		}
		if len(req.InputMapping) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "  Input Mapping:\n")
			for k, v := range req.InputMapping {
				fmt.Fprintf(cmd.OutOrStdout(), "    %s = %s\n", k, v)
			}
		}
		return nil
	}

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := mgr.AddWebhook(ctx, req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Webhook trigger created!\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Configure in %s: %s\n", webhookSource, getWebhookURL(webhookPath))
	if webhookSecret != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Secret: Set %s environment variable\n", strings.Trim(webhookSecret, "${}"))
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}
