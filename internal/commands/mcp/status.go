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
)
// newMCPStatusCommand creates the 'mcp status' command.
func newMCPStatusCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "status <name>",
		Short: "Show detailed status of an MCP server",
		Long: `Show detailed status of an MCP server including configuration,
uptime, failure history, and capabilities.

Examples:
  conductor mcp status github
  conductor mcp status github --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPStatus(args[0], jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")

	return cmd
}

func runMCPStatus(name string, jsonOutput bool) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	data, err := client.get(ctx, "/v1/mcp/servers/"+name)
	if err != nil {
		return err
	}

	if jsonOutput {
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

	fmt.Printf("Name:     %s\n", resp.Name)
	fmt.Printf("Status:   %s\n", resp.Status)
	fmt.Printf("Uptime:   %s\n", formatDuration(time.Duration(resp.UptimeSeconds)*time.Second))
	fmt.Printf("Tools:    %d\n", resp.ToolCount)

	if resp.Config != nil {
		fmt.Println("\nConfiguration:")
		fmt.Printf("  Command:  %s\n", resp.Config.Command)
		if len(resp.Config.Args) > 0 {
			fmt.Printf("  Args:     %s\n", strings.Join(resp.Config.Args, " "))
		}
		if len(resp.Config.Env) > 0 {
			fmt.Printf("  Env:      %s\n", strings.Join(resp.Config.Env, ", "))
		}
		fmt.Printf("  Timeout:  %ds\n", resp.Config.Timeout)
	}

	if resp.Capabilities != nil {
		fmt.Println("\nCapabilities:")
		fmt.Printf("  Tools:     %v\n", resp.Capabilities.Tools)
		fmt.Printf("  Resources: %v\n", resp.Capabilities.Resources)
		fmt.Printf("  Prompts:   %v\n", resp.Capabilities.Prompts)
	}

	if resp.FailureCount > 0 {
		fmt.Println("\nFailure History:")
		fmt.Printf("  Count:      %d\n", resp.FailureCount)
		if resp.LastError != "" {
			fmt.Printf("  Last Error: %s\n", resp.LastError)
		}
	}

	return nil
}
