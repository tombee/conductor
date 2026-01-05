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

// NewHistoryCommand creates the history command group.
func NewHistoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "history",
		Annotations: map[string]string{
			"group": "management",
		},
		Short: "View workflow execution history",
		Long: `Commands for listing, viewing, and managing past workflow executions.

Use 'conductor run' to execute a workflow. Use 'conductor history' to view past executions.`,
	}

	cmd.AddCommand(newHistoryListCommand())
	cmd.AddCommand(newHistoryShowCommand())
	cmd.AddCommand(newHistoryOutputCommand())
	cmd.AddCommand(newHistoryLogsCommand())
	cmd.AddCommand(newHistoryCancelCommand())

	return cmd
}

func newHistoryListCommand() *cobra.Command {
	var status string
	var workflow string
	var failed bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List past workflow executions",
		Long: `List all workflow executions, optionally filtered by status or workflow.

See also: conductor history show, conductor run, conductor controller status`,
		Example: `  # Example 1: List all workflow executions
  conductor history list

  # Example 2: Filter by status
  conductor history list --status running

  # Example 3: Filter by workflow name
  conductor history list --workflow my-workflow

  # Example 4: List failed executions (shorthand)
  conductor history list --failed

  # Example 5: Get executions as JSON for monitoring
  conductor history list --json | jq '.runs[] | select(.status=="failed")'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// If --failed flag is set, override status to "failed"
			if failed {
				status = "failed"
			}
			return historyList(status, workflow)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending, running, completed, failed, cancelled)")
	cmd.Flags().StringVar(&workflow, "workflow", "", "Filter by workflow name")
	cmd.Flags().BoolVar(&failed, "failed", false, "Show only failed executions (shorthand for --status failed)")

	return cmd
}

func newHistoryShowCommand() *cobra.Command {
	var failed bool

	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show execution details",
		Long: `Display detailed information about a specific workflow execution.

See also: conductor history list, conductor history logs, conductor history output`,
		Example: `  # Example 1: Show execution details
  conductor history show abc123

  # Example 2: Get execution details as JSON
  conductor history show abc123 --json

  # Example 3: Extract execution status
  conductor history show abc123 --json | jq -r '.status'

  # Example 4: Check if execution is complete
  conductor history show abc123 --json | jq -e '.status == "completed"'

  # Example 5: Show failure details with suggested replay command
  conductor history show abc123 --failed`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteRunIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return historyShow(args[0], failed)
		},
	}

	cmd.Flags().BoolVar(&failed, "failed", false, "Show failure point details and suggest replay command")

	return cmd
}

func newHistoryOutputCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "output <run-id>",
		Short:             "Get execution output",
		Long:              `Display the output of a completed workflow execution.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteRunIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return historyOutput(args[0])
		},
	}
}

func newHistoryLogsCommand() *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:               "logs <run-id>",
		Short:             "View execution logs",
		Long:              `Display logs from a workflow execution. Use -f to follow/stream logs.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteRunIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return historyLogs(args[0], follow)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")

	return cmd
}

func newHistoryCancelCommand() *cobra.Command {
	return &cobra.Command{
		Use:               "cancel <run-id>",
		Short:             "Cancel a running workflow",
		Long:              `Cancel a pending or running workflow execution.`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteActiveRunIDs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return historyCancel(args[0])
		},
	}
}

func historyList(status, workflow string) error {
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
		return fmt.Errorf("failed to list executions: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	runs, ok := resp["runs"].([]any)
	if !ok {
		runs = []any{}
	}

	if len(runs) == 0 {
		fmt.Println(shared.Muted.Render("No executions found"))
		return nil
	}

	fmt.Println(shared.Header.Render("Execution History"))
	fmt.Println()
	fmt.Printf("%s %s %s %s\n",
		shared.Bold.Render(fmt.Sprintf("%-8s", "ID")),
		shared.Bold.Render(fmt.Sprintf("%-11s", "STATUS")),
		shared.Bold.Render(fmt.Sprintf("%-20s", "WORKFLOW")),
		shared.Bold.Render("STARTED"))
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
		// Color-code status
		statusStyled := formatRunStatus(status)
		fmt.Printf("%-8s %s %-20s %s\n", id, statusStyled, truncate(workflow, 20), shared.Muted.Render(startedAt))
	}

	return nil
}

// formatRunStatus returns a styled status string
func formatRunStatus(status string) string {
	switch status {
	case "completed":
		return shared.StatusOK.Render(fmt.Sprintf("%-11s", status))
	case "failed":
		return shared.StatusError.Render(fmt.Sprintf("%-11s", status))
	case "running":
		return shared.StatusInfo.Render(fmt.Sprintf("%-11s", status))
	case "pending":
		return shared.StatusWarn.Render(fmt.Sprintf("%-11s", status))
	case "cancelled":
		return shared.Muted.Render(fmt.Sprintf("%-11s", status))
	default:
		return fmt.Sprintf("%-11s", status)
	}
}

func historyShow(id string, showFailureDetails bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := c.Get(ctx, "/v1/runs/"+id)
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	fmt.Println(shared.Header.Render("Execution Details"))
	fmt.Println()
	fmt.Printf("%s %s\n", shared.Muted.Render("Run ID:"), resp["id"])
	fmt.Printf("%s %s\n", shared.Muted.Render("Workflow:"), shared.Bold.Render(fmt.Sprint(resp["workflow"])))
	status, _ := resp["status"].(string)
	fmt.Printf("%s %s\n", shared.Muted.Render("Status:"), formatRunStatus(status))
	if cid, ok := resp["correlation_id"].(string); ok && cid != "" {
		fmt.Printf("%s %s\n", shared.Muted.Render("Correlation ID:"), cid)
	}

	if s, ok := resp["created_at"].(string); ok {
		fmt.Printf("%s %s\n", shared.Muted.Render("Created:"), s)
	}
	if s, ok := resp["started_at"].(string); ok && s != "" {
		fmt.Printf("%s %s\n", shared.Muted.Render("Started:"), s)
	}
	if s, ok := resp["completed_at"].(string); ok && s != "" {
		fmt.Printf("%s %s\n", shared.Muted.Render("Completed:"), s)
	}
	if e, ok := resp["error"].(string); ok && e != "" {
		fmt.Printf("%s %s\n", shared.Muted.Render("Error:"), shared.StatusError.Render(e))
	}

	if progress, ok := resp["progress"].(map[string]any); ok {
		completed := int(progress["completed"].(float64))
		total := int(progress["total"].(float64))
		current := progress["current_step"]
		progressStr := fmt.Sprintf("%d/%d", completed, total)
		if current != nil && current != "" {
			progressStr += fmt.Sprintf(" %s", shared.Muted.Render(fmt.Sprintf("(current: %s)", current)))
		}
		fmt.Printf("%s %s\n", shared.Muted.Render("Progress:"), progressStr)
	}

	// If --failed flag is set and the run failed, show failure details
	if showFailureDetails {
		runStatus, _ := resp["status"].(string)
		if runStatus == "failed" {
			fmt.Println()
			fmt.Println(shared.Bold.Render("Failure Details"))

			// Show error message
			if errorMsg, ok := resp["error"].(string); ok && errorMsg != "" {
				fmt.Printf("  %s %s\n", shared.Muted.Render("Error Message:"), shared.StatusError.Render(errorMsg))
			}

			// Show the step that failed (from progress.current_step)
			var failedStep string
			if progress, ok := resp["progress"].(map[string]any); ok {
				if current, ok := progress["current_step"].(string); ok && current != "" {
					failedStep = current
					fmt.Printf("  %s %s\n", shared.Muted.Render("Failed At:"), failedStep)
				}
			}

			// Suggest replay command
			fmt.Println()
			fmt.Println(shared.Bold.Render("Suggested Replay Command"))
			if failedStep != "" {
				fmt.Printf("  %s\n", shared.StatusInfo.Render(fmt.Sprintf("conductor run replay %s --from %s", id, failedStep)))
			} else {
				fmt.Printf("  %s\n", shared.StatusInfo.Render(fmt.Sprintf("conductor run replay %s", id)))
			}

			// Show cost estimation command
			fmt.Println()
			fmt.Println(shared.Muted.Render("To estimate replay cost:"))
			if failedStep != "" {
				fmt.Printf("  conductor run replay %s --from %s --estimate\n", id, failedStep)
			} else {
				fmt.Printf("  conductor run replay %s --estimate\n", id)
			}
		} else {
			fmt.Printf("\n%s Execution status is '%s', not 'failed'. Use --failed only with failed executions.\n",
				shared.StatusInfo.Render(shared.SymbolInfo), runStatus)
		}
	}

	return nil
}

func historyOutput(id string) error {
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

func historyLogs(id string, follow bool) error {
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
	timestamp, _ := log["timestamp"].(string)
	entryType, _ := log["type"].(string)
	level, _ := log["level"].(string)
	message, _ := log["message"].(string)
	stepID, _ := log["step_id"].(string)
	status, _ := log["status"].(string)

	// Parse and format timestamp
	if t, err := time.Parse(time.RFC3339Nano, timestamp); err == nil {
		timestamp = t.Local().Format("15:04:05")
	}

	// Format based on entry type
	switch entryType {
	case "step_start":
		fmt.Printf("%s [STEP] Starting: %s\n", timestamp, stepID)
	case "step_complete":
		durationMs, _ := log["duration_ms"].(float64)
		if status == "success" {
			fmt.Printf("%s [STEP] Completed: %s (%.1fs)\n", timestamp, stepID, durationMs/1000)
		} else {
			errMsg, _ := log["error"].(string)
			fmt.Printf("%s [STEP] %s: %s - %s\n", timestamp, strings.ToUpper(status), stepID, errMsg)
		}
		// Show output if present (essential for debugging data flow)
		if output, ok := log["output"].(map[string]any); ok && len(output) > 0 {
			if response, ok := output["response"].(string); ok && response != "" {
				fmt.Println()
				fmt.Println("         ┌─ Output ─────────────────────────────────────────────")
				// Indent each line of output
				lines := strings.Split(response, "\n")
				for _, line := range lines {
					fmt.Printf("         │ %s\n", line)
				}
				fmt.Println("         └─────────────────────────────────────────────────────")
				fmt.Println()
			}
		}
	case "status":
		fmt.Printf("%s [STATUS] %s\n", timestamp, status)
	default:
		// Regular log entry
		levelStr := strings.ToUpper(level)
		if levelStr == "" {
			levelStr = "INFO"
		}
		if stepID != "" {
			fmt.Printf("%s [%s] [%s] %s\n", timestamp, levelStr, stepID, message)
		} else {
			fmt.Printf("%s [%s] %s\n", timestamp, levelStr, message)
		}
	}
}

func historyCancel(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if err := c.Delete(ctx, "/v1/runs/"+id); err != nil {
		return fmt.Errorf("failed to cancel execution: %w", err)
	}

	fmt.Printf("%s Execution %s cancelled\n", shared.StatusOK.Render(shared.SymbolOK), id)
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
