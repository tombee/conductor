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
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// newMCPListCommand creates the 'mcp list' command.
func newMCPListCommand() *cobra.Command {
	var showAll bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all registered MCP servers",
		Long: `List all registered MCP servers with their status.

See also: conductor mcp add, conductor mcp status, conductor controller status`,
		Example: `  # Example 1: List registered MCP servers
  conductor mcp list

  # Example 2: Include workflow-scoped servers
  conductor mcp list --all

  # Example 3: Get server list as JSON
  conductor mcp list --json

  # Example 4: Extract server names for scripting
  conductor mcp list --json | jq -r '.servers[].name'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPList(showAll)
		},
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "Include workflow-scoped servers")

	return cmd
}

func runMCPList(showAll bool) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	data, err := client.get(ctx, "/v1/mcp/servers")
	if err != nil {
		return err
	}

	var resp struct {
		Servers []struct {
			Name          string `json:"name"`
			Status        string `json:"status"`
			UptimeSeconds int64  `json:"uptime_seconds"`
			ToolCount     int    `json:"tool_count"`
			FailureCount  int    `json:"failure_count"`
			LastError     string `json:"last_error,omitempty"`
		} `json:"servers"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if shared.GetJSON() {
		fmt.Println(string(data))
		return nil
	}

	if len(resp.Servers) == 0 {
		fmt.Println("No MCP servers registered.")
		fmt.Println("\nTo add a server:")
		fmt.Println("  conductor mcp add <name> --command <cmd>")
		return nil
	}

	// Print table header
	fmt.Printf("%-20s %-12s %-12s %-8s %s\n", "NAME", "STATUS", "UPTIME", "TOOLS", "ERRORS")
	fmt.Println(strings.Repeat("-", 70))

	for _, s := range resp.Servers {
		uptime := formatDuration(time.Duration(s.UptimeSeconds) * time.Second)
		errInfo := ""
		if s.FailureCount > 0 {
			errInfo = fmt.Sprintf("%d failures", s.FailureCount)
		}

		fmt.Printf("%-20s %-12s %-12s %-8d %s\n",
			truncate(s.Name, 20),
			s.Status,
			uptime,
			s.ToolCount,
			errInfo,
		)
	}

	return nil
}
