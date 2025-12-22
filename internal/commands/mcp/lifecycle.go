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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/completion"
)

// newMCPStartCommand creates the 'mcp start' command.
func newMCPStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a global MCP server",
		Long: `Start a global MCP server by name.

The server must be registered in the global configuration.

Examples:
  conductor mcp start github`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteMCPServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPStart(args[0])
		},
	}

	return cmd
}

func runMCPStart(name string) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	_, err := client.post(ctx, "/v1/mcp/servers/"+name+"/start", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Started MCP server: %s\n", name)
	return nil
}

// newMCPStopCommand creates the 'mcp stop' command.
func newMCPStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop <name>",
		Short: "Stop a running MCP server",
		Long: `Stop a running MCP server.

Examples:
  conductor mcp stop github`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteMCPServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPStop(args[0])
		},
	}

	return cmd
}

func runMCPStop(name string) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	_, err := client.post(ctx, "/v1/mcp/servers/"+name+"/stop", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Stopped MCP server: %s\n", name)
	return nil
}

// newMCPRestartCommand creates the 'mcp restart' command.
func newMCPRestartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart <name>",
		Short: "Restart an MCP server",
		Long: `Restart an MCP server.

Examples:
  conductor mcp restart github`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.CompleteMCPServerNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPRestart(args[0])
		},
	}

	return cmd
}

func runMCPRestart(name string) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	_, err := client.post(ctx, "/v1/mcp/servers/"+name+"/restart", nil)
	if err != nil {
		return err
	}

	fmt.Printf("Restarting MCP server: %s\n", name)
	return nil
}
