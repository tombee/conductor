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
	"github.com/spf13/cobra"
)

// NewMCPCommand creates the mcp command for MCP server management.
func NewMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "mcp",
		Annotations: map[string]string{
			"group": "mcp",
		},
		Short: "Manage MCP (Model Context Protocol) servers",
		Long: `Manage MCP servers for use in Conductor workflows.

MCP servers provide tools that can be used in workflow steps.

Commands:
  init      Create a new MCP server project from a template
  list      List all registered MCP servers
  status    Show detailed status of an MCP server
  tools     List tools available from an MCP server
  start     Start a global MCP server
  stop      Stop a running MCP server
  restart   Restart an MCP server
  add       Register a new global MCP server
  remove    Remove a global MCP server
  validate  Validate an MCP server configuration
  test      Test an MCP server by starting it and checking connectivity
  logs      View logs from an MCP server`,
	}

	cmd.AddCommand(newMCPInitCommand())
	cmd.AddCommand(newMCPListCommand())
	cmd.AddCommand(newMCPStatusCommand())
	cmd.AddCommand(newMCPToolsCommand())
	cmd.AddCommand(newMCPStartCommand())
	cmd.AddCommand(newMCPStopCommand())
	cmd.AddCommand(newMCPRestartCommand())
	cmd.AddCommand(newMCPAddCommand())
	cmd.AddCommand(newMCPRemoveCommand())
	cmd.AddCommand(newMCPValidateCommand())
	cmd.AddCommand(newMCPTestCommand())
	cmd.AddCommand(newMCPLogsCommand())

	return cmd
}
