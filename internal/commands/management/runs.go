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

package management

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/client"
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewRunsCommand creates the runs command group.
func NewRunsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Annotations: map[string]string{
			"group": "management",
		},
		Short: "Manage workflow runs",
		Long: `Commands for listing, viewing, and managing workflow runs.

Runs are workflow executions managed by the conductor controller.`,
	}

	cmd.AddCommand(newRunsListCommand())
	cmd.AddCommand(newRunsShowCommand())
	cmd.AddCommand(newRunsOutputCommand())
	cmd.AddCommand(newRunsLogsCommand())
	cmd.AddCommand(newRunsCancelCommand())

	return cmd
}

func newRunsListCommand() *cobra.Command {
	var status string
	var workflow string
	var failed bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflow runs",
		Long: `List all workflow runs, optionally filtered by status or workflow.

See also: conductor runs show, conductor run, conductor controller status`,
		Example: `  # Example 1: List all workflow runs
  conductor runs list

  # Example 2: Filter by status
  conductor runs list --status running

  # Example 3: Filter by workflow name
  conductor runs list --workflow my-workflow

  # Example 4: List failed runs (shorthand)
  conductor runs list --failed

  # Example 5: Get runs as JSON for monitoring
  conductor runs list --json | jq '.runs[] | select(.status=="failed")'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --failed flag is set, override status to "failed"
			if failed {
				status = "failed"
			}
			return runsList(status, workflow)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending, running, completed, failed, cancelled)")
	cmd.Flags().StringVar(&workflow, "workflow", "", "Filter by workflow name")
	cmd.Flags().BoolVar(&failed, "failed", false, "Show only failed runs (shorthand for --status failed)")

	return cmd
}

func newRunsShowCommand() *cobra.Command {
	var failed bool

	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show run details",
		Long: `Display detailed information about a specific workflow run.

See also: conductor runs list, conductor runs logs, conductor runs output`,
		Example: `  # Example 1: Show run details
  conductor runs show abc123

  # Example 2: Get run details as JSON
  conductor runs show abc123 --json

  # Example 3: Extract run status
  conductor runs show abc123 --json | jq -r '.status'

  # Example 4: Check if run is complete
  conductor runs show abc123 --json | jq -e '.status == "completed"'

  # Example 5: Show failure details with suggested replay command
  conductor runs show abc123 --failed`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteRunIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runsShow(args[0], failed)
		},
	}

	cmd.Flags().BoolVar(&failed, "failed", false, "Show failure point details and suggest replay command")

	return cmd
}

func newRunsOutputCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "output <run-id>",
		Short:             "Get run output",
		Long:              `Display the output of a completed workflow run.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteRunIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runsOutput(args[0])
		},
	}
}

func newRunsLogsCommand() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:               "logs <run-id>",
		Short:             "View run logs",
		Long:              `Display logs from a workflow run. Use -f to follow/stream logs.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteRunIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runsLogs(args[0], follow)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")

	return cmd
}

func newRunsCancelCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "cancel <run-id>",
		Short:             "Cancel a running workflow",
		Long:              `Cancel a pending or running workflow execution.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteActiveRunIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runsCancel(args[0])
		},
	}
}

func runsList(status, workflow string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Build query params
	path := "/v1/runs"
	var params []string
	if status != "" {
		params = append(params, "status="+status)
	}
	if workflow != "" {
		params = append(params, "workflow="+workflow)
	}
	if len(params) > 0 {
		path += "?" + strings.Join(params, "&")
	}

	resp, err := c.Get(ctx, path)
	if err != nil {
		return fmt.Errorf("failed to list runs: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	runs, ok := resp["runs"].([]any)
	if !ok {
		runs = []any{}
	}

	if len(runs) == 0 {
		fmt.Println("No runs found")
		return nil
	}

	fmt.Println("ID       STATUS      WORKFLOW             STARTED")
	fmt.Println("-------- ----------- -------------------- -------------------")
	for _, r := range runs {
		run := r.(map[string]any)
		id := run["id"].(string)
		status := run["status"].(string)
		workflow := run["workflow"].(string)
		startedAt := "-"
		if s, ok := run["started_at"].(string); ok && s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				startedAt = t.Local().Format("2006-01-02 15:04:05")
			}
		}
		fmt.Printf("%-8s %-11s %-20s %s\n", id, status, truncate(workflow, 20), startedAt)
	}

	return nil
}

func runsShow(id string, showFailureDetails bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := c.Get(ctx, "/v1/runs/"+id)
	if err != nil {
		return fmt.Errorf("failed to get run: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	fmt.Printf("Run ID:     %s\n", resp["id"])
	fmt.Printf("Workflow:   %s\n", resp["workflow"])
	fmt.Printf("Status:     %s\n", resp["status"])
	if cid, ok := resp["correlation_id"].(string); ok && cid != "" {
		fmt.Printf("Correlation ID: %s\n", cid)
	}

	if s, ok := resp["created_at"].(string); ok {
		fmt.Printf("Created:    %s\n", s)
	}
	if s, ok := resp["started_at"].(string); ok && s != "" {
		fmt.Printf("Started:    %s\n", s)
	}
	if s, ok := resp["completed_at"].(string); ok && s != "" {
		fmt.Printf("Completed:  %s\n", s)
	}
	if e, ok := resp["error"].(string); ok && e != "" {
		fmt.Printf("Error:      %s\n", e)
	}

	if progress, ok := resp["progress"].(map[string]any); ok {
		completed := int(progress["completed"].(float64))
		total := int(progress["total"].(float64))
		current := progress["current_step"]
		fmt.Printf("Progress:   %d/%d", completed, total)
		if current != nil && current != "" {
			fmt.Printf(" (current: %s)", current)
		}
		fmt.Println()
	}

	// If --failed flag is set and the run failed, show failure details
	if showFailureDetails {
		status, _ := resp["status"].(string)
		if status == "failed" {
			fmt.Println("\n--- Failure Details ---")

			// Show error message
			if errorMsg, ok := resp["error"].(string); ok && errorMsg != "" {
				fmt.Printf("Error Message: %s\n", errorMsg)
			}

			// Show the step that failed (from progress.current_step)
			var failedStep string
			if progress, ok := resp["progress"].(map[string]any); ok {
				if current, ok := progress["current_step"].(string); ok && current != "" {
					failedStep = current
					fmt.Printf("Failed At:     %s\n", failedStep)
				}
			}

			// Suggest replay command
			fmt.Println("\n--- Suggested Replay Command ---")
			if failedStep != "" {
				fmt.Printf("conductor run replay %s --from %s\n", id, failedStep)
			} else {
				fmt.Printf("conductor run replay %s\n", id)
			}

			// Show cost estimation command
			fmt.Println("\nTo estimate replay cost:")
			if failedStep != "" {
				fmt.Printf("conductor run replay %s --from %s --estimate\n", id, failedStep)
			} else {
				fmt.Printf("conductor run replay %s --estimate\n", id)
			}
		} else {
			fmt.Printf("\nNote: Run status is '%s', not 'failed'. Use --failed only with failed runs.\n", status)
		}
	}

	return nil
}

func runsOutput(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := c.Get(ctx, "/v1/runs/"+id+"/output")
	if err != nil {
		return fmt.Errorf("failed to get output: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

func runsLogs(id string, follow bool) error {
	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if follow {
		return streamLogs(c, id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := c.Get(ctx, "/v1/runs/"+id+"/logs")
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	logs, ok := resp["logs"].([]any)
	if !ok || len(logs) == 0 {
		fmt.Println("No logs available")
		return nil
	}

	for _, l := range logs {
		log := l.(map[string]any)
		printLogEntry(log)
	}

	return nil
}

func streamLogs(c *client.Client, id string) error {
	ctx := context.Background()

	resp, err := c.GetStream(ctx, "/v1/runs/"+id+"/logs", "text/event-stream")
	if err != nil {
		return fmt.Errorf("failed to stream logs: %w", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "event: done") {
			return nil
		}

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			var log map[string]any
			if err := json.Unmarshal([]byte(data), &log); err != nil {
				continue
			}
			printLogEntry(log)
		}
	}
}

func printLogEntry(log map[string]any) {
	timestamp := log["timestamp"].(string)
	level := log["level"].(string)
	message := log["message"].(string)
	stepID := ""
	if s, ok := log["step_id"].(string); ok {
		stepID = s
	}

	// Parse and format timestamp
	if t, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
		timestamp = t.Local().Format("15:04:05")
	}

	// Color based on level
	levelStr := strings.ToUpper(level)
	if stepID != "" {
		fmt.Printf("%s [%s] [%s] %s\n", timestamp, levelStr, stepID, message)
	} else {
		fmt.Printf("%s [%s] %s\n", timestamp, levelStr, message)
	}
}

func runsCancel(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := c.Delete(ctx, "/v1/runs/"+id); err != nil {
		return fmt.Errorf("failed to cancel run: %w", err)
	}

	fmt.Printf("Run %s cancelled\n", id)
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
