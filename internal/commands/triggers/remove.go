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

	"github.com/spf13/cobra"
)

func newRemoveCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a trigger",
		Long: `Remove a trigger configuration.

Subcommands:
  webhook   - Remove a webhook trigger
  schedule  - Remove a schedule trigger
  api       - Remove an API trigger
  file      - Remove a file watcher trigger`,
	}

	cmd.AddCommand(newRemoveWebhookCommand())
	cmd.AddCommand(newRemoveScheduleCommand())
	cmd.AddCommand(newRemoveAPICommand())
	cmd.AddCommand(newRemoveFileCommand())

	return cmd
}

func newRemoveWebhookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "webhook PATH",
		Short: "Remove a webhook trigger",
		Long:  `Remove a webhook trigger by its URL path.`,
		Example: `  # Remove webhook
  conductor triggers remove webhook /webhooks/pr-review`,
		Args: cobra.ExactArgs(1),
		RunE: runRemoveWebhook,
	}

	return cmd
}

func runRemoveWebhook(cmd *cobra.Command, args []string) error {
	path := args[0]

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := mgr.RemoveWebhook(ctx, path); err != nil {
		return fmt.Errorf("failed to remove webhook: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Webhook trigger removed: %s\n", path)
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}

func newRemoveScheduleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule NAME",
		Short: "Remove a schedule trigger",
		Long:  `Remove a schedule trigger by its name.`,
		Example: `  # Remove schedule
  conductor triggers remove schedule daily-report`,
		Args: cobra.ExactArgs(1),
		RunE: runRemoveSchedule,
	}

	return cmd
}

func runRemoveSchedule(cmd *cobra.Command, args []string) error {
	name := args[0]

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := mgr.RemoveSchedule(ctx, name); err != nil {
		return fmt.Errorf("failed to remove schedule: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Schedule trigger removed: %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}

func newRemoveAPICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api NAME",
		Short: "Remove an API trigger",
		Long:  `Remove an API trigger by its name.`,
		Example: `  # Remove API trigger
  conductor triggers remove api deploy-trigger`,
		Args: cobra.ExactArgs(1),
		RunE: runRemoveAPI,
	}

	return cmd
}

func runRemoveAPI(cmd *cobra.Command, args []string) error {
	name := args[0]

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := mgr.RemoveEndpoint(ctx, name); err != nil {
		return fmt.Errorf("failed to remove API trigger: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "API trigger removed: %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}

func newRemoveFileCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "file NAME",
		Short: "Remove a file watcher trigger",
		Long:  `Remove a file watcher trigger by its name.`,
		Example: `  # Remove file watcher
  conductor triggers remove file file-abc12345`,
		Args: cobra.ExactArgs(1),
		RunE: runRemoveFile,
	}

	return cmd
}

func runRemoveFile(cmd *cobra.Command, args []string) error {
	name := args[0]

	mgr, err := getManager()
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := mgr.RemoveFileWatcher(ctx, name); err != nil {
		return fmt.Errorf("failed to remove file watcher: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "File watcher trigger removed: %s\n", name)
	fmt.Fprintf(cmd.OutOrStdout(), "\nRestart the controller for changes to take effect:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  conductor controller restart\n")

	return nil
}
