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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
)

// Logs command

var (
	mcpLogsFollow bool
	mcpLogsSince  string
	mcpLogsLines  int
)

func newMCPLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "View logs from an MCP server",
		Long: `View logs from an MCP server.

Examples:
  conductor mcp logs github
  conductor mcp logs github --lines 50
  conductor mcp logs github --since 5m
  conductor mcp logs github --follow`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteMCPServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPLogs(args[0], mcpLogsFollow, mcpLogsSince, mcpLogsLines)
		},
	}

	cmd.Flags().BoolVarP(&mcpLogsFollow, "follow", "f", false, "Follow log output")
	cmd.Flags().StringVar(&mcpLogsSince, "since", "", "Show logs since timestamp (e.g., 5m, 1h)")
	cmd.Flags().IntVarP(&mcpLogsLines, "lines", "n", 100, "Number of lines to show")

	return cmd
}

func runMCPLogs(name string, follow bool, since string, lines int) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	// Build query parameters
	path := fmt.Sprintf("/v1/mcp/servers/%s/logs?lines=%d", name, lines)
	if since != "" {
		path += "&since=" + since
	}

	if follow {
		// For follow mode, we'd need SSE streaming
		// For now, just poll periodically
		fmt.Printf("Following logs for %s (Ctrl+C to stop)...\n\n", name)

		lastTimestamp := ""
		for {
			data, err := client.get(ctx, path)
			if err != nil {
				return err
			}

			var logsResp struct {
				Logs []struct {
					Timestamp string `json:"timestamp"`
					Level     string `json:"level"`
					Message   string `json:"message"`
				} `json:"logs"`
			}

			if err := json.Unmarshal(data, &logsResp); err != nil {
				return fmt.Errorf("failed to parse logs: %w", err)
			}

			// Print new logs
			for _, log := range logsResp.Logs {
				if log.Timestamp > lastTimestamp {
					fmt.Printf("[%s] %s: %s\n", log.Timestamp, log.Level, log.Message)
					lastTimestamp = log.Timestamp
				}
			}

			time.Sleep(1 * time.Second)
		}
	}

	// One-shot log retrieval
	data, err := client.get(ctx, path)
	if err != nil {
		return err
	}

	var logsResp struct {
		Logs []struct {
			Timestamp string `json:"timestamp"`
			Level     string `json:"level"`
			Message   string `json:"message"`
		} `json:"logs"`
	}

	if err := json.Unmarshal(data, &logsResp); err != nil {
		return fmt.Errorf("failed to parse logs: %w", err)
	}

	if len(logsResp.Logs) == 0 {
		fmt.Printf("No logs available for MCP server: %s\n", name)
		return nil
	}

	for _, log := range logsResp.Logs {
		fmt.Printf("[%s] %s: %s\n", log.Timestamp, log.Level, log.Message)
	}

	return nil
}
