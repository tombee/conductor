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
	"github.com/tombee/conductor/internal/commands/completion"
	"github.com/tombee/conductor/internal/commands/shared"
)

// newMCPStatusCommand creates the 'mcp status' command.
func newMCPStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show detailed status of an MCP server",
		Long: `Show detailed status of an MCP server including configuration,
uptime, failure history, and capabilities.

Examples:
  conductor mcp status github
  conductor mcp status github --json`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteMCPServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPStatus(args[0])
		},
	}

	return cmd
}

func runMCPStatus(name string) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	data, err := client.get(ctx, "/v1/mcp/servers/"+name)
	if err != nil {
		return err
	}

	if shared.GetJSON() {
		fmt.Println(string(data))
		return nil
	}

	var resp struct {
		Name          string `json:"name"`
		Status        string `json:"status"`
		UptimeSeconds int64  `json:"uptime_seconds"`
		ToolCount     int    `json:"tool_count"`
		FailureCount  int    `json:"failure_count"`
		LastError     string `json:"last_error,omitempty"`
		Config        *struct {
			Command string   `json:"command"`
			Args    []string `json:"args,omitempty"`
			Env     []string `json:"env,omitempty"`
			Timeout int      `json:"timeout"`
		} `json:"config,omitempty"`
		Capabilities *struct {
			Tools     bool `json:"tools"`
			Resources bool `json:"resources"`
			Prompts   bool `json:"prompts"`
		} `json:"capabilities,omitempty"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	fmt.Println(shared.Header.Render("MCP Server: " + resp.Name))
	fmt.Println()

	// Status with color
	var statusStyled string
	switch resp.Status {
	case "running":
		statusStyled = shared.StatusOK.Render(resp.Status)
	case "stopped", "disabled":
		statusStyled = shared.Muted.Render(resp.Status)
	case "error", "failed":
		statusStyled = shared.StatusError.Render(resp.Status)
	default:
		statusStyled = resp.Status
	}

	fmt.Printf("%s %s\n", shared.Muted.Render("Status:"), statusStyled)
	fmt.Printf("%s %s\n", shared.Muted.Render("Uptime:"), formatDuration(time.Duration(resp.UptimeSeconds)*time.Second))
	fmt.Printf("%s %d\n", shared.Muted.Render("Tools:"), resp.ToolCount)

	if resp.Config != nil {
		fmt.Println()
		fmt.Println(shared.Bold.Render("Configuration:"))
		fmt.Printf("  %s %s\n", shared.Muted.Render("Command:"), resp.Config.Command)
		if len(resp.Config.Args) > 0 {
			fmt.Printf("  %s %s\n", shared.Muted.Render("Args:"), strings.Join(resp.Config.Args, " "))
		}
		if len(resp.Config.Env) > 0 {
			fmt.Printf("  %s %s\n", shared.Muted.Render("Env:"), strings.Join(resp.Config.Env, ", "))
		}
		fmt.Printf("  %s %ds\n", shared.Muted.Render("Timeout:"), resp.Config.Timeout)
	}

	if resp.Capabilities != nil {
		fmt.Println()
		fmt.Println(shared.Bold.Render("Capabilities:"))
		fmt.Printf("  %s %s\n", shared.Muted.Render("Tools:"), formatBool(resp.Capabilities.Tools))
		fmt.Printf("  %s %s\n", shared.Muted.Render("Resources:"), formatBool(resp.Capabilities.Resources))
		fmt.Printf("  %s %s\n", shared.Muted.Render("Prompts:"), formatBool(resp.Capabilities.Prompts))
	}

	if resp.FailureCount > 0 {
		fmt.Println()
		fmt.Println(shared.Bold.Render("Failure History:"))
		fmt.Printf("  %s %s\n", shared.Muted.Render("Count:"), shared.StatusError.Render(fmt.Sprintf("%d", resp.FailureCount)))
		if resp.LastError != "" {
			fmt.Printf("  %s %s\n", shared.Muted.Render("Last Error:"), shared.StatusError.Render(resp.LastError))
		}
	}

	return nil
}

// formatBool returns a styled boolean display
func formatBool(b bool) string {
	if b {
		return shared.StatusOK.Render(shared.SymbolOK)
	}
	return shared.Muted.Render(shared.SymbolError)
}
