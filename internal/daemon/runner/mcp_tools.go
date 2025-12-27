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

// MCP server lifecycle and tool registration.
// Manages starting MCP servers defined in workflows, discovering their tools,
// and registering them in the tool registry for use during workflow execution.
package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/mcp"
)

// startMCPServers starts all MCP servers defined in the workflow and registers their tools.
func (r *Runner) startMCPServers(run *Run) error {
	if len(run.definition.MCPServers) == 0 {
		return nil
	}

	r.addLog(run, "info", fmt.Sprintf("Starting %d MCP server(s)", len(run.definition.MCPServers)), "")

	// Start each MCP server
	for _, mcpServerDef := range run.definition.MCPServers {
		r.addLog(run, "info", fmt.Sprintf("Starting MCP server: %s", mcpServerDef.Name), "")

		// Convert workflow MCPServerConfig to mcp.ServerConfig
		serverConfig := mcp.ServerConfig{
			Name:    mcpServerDef.Name,
			Command: mcpServerDef.Command,
			Args:    mcpServerDef.Args,
			Env:     mcpServerDef.Env,
			Timeout: time.Duration(mcpServerDef.Timeout) * time.Second,
		}

		// Start the server
		if err := r.lifecycle.mcpManager.Start(serverConfig); err != nil {
			return fmt.Errorf("failed to start MCP server %s: %w", mcpServerDef.Name, err)
		}

		// Wait for the server to be ready and register its tools
		if err := r.registerMCPTools(run, mcpServerDef.Name); err != nil {
			return fmt.Errorf("failed to register tools for MCP server %s: %w", mcpServerDef.Name, err)
		}

		r.addLog(run, "info", fmt.Sprintf("MCP server started: %s", mcpServerDef.Name), "")
	}

	return nil
}

// registerMCPTools discovers and registers tools from an MCP server.
func (r *Runner) registerMCPTools(run *Run, serverName string) error {
	// Get the MCP client for this server
	// We need to wait a bit for the server to initialize
	var client mcp.ClientProvider
	var err error

	// Retry for up to 10 seconds
	maxAttempts := 20
	for attempt := 0; attempt < maxAttempts; attempt++ {
		client, err = r.lifecycle.mcpManager.GetClient(serverName)
		if err == nil {
			break
		}
		if attempt < maxAttempts-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to get client for server %s: %w", serverName, err)
	}

	// List available tools from the MCP server
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	toolDefs, err := client.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tools from server %s: %w", serverName, err)
	}

	r.addLog(run, "info", fmt.Sprintf("Registering %d tool(s) from MCP server %s", len(toolDefs), serverName), "")

	// Register each tool in the registry
	for _, toolDef := range toolDefs {
		mcpTool := mcp.NewMCPTool(serverName, toolDef, client)
		r.lifecycle.toolRegistry.Register(mcpTool)
		r.addLog(run, "debug", fmt.Sprintf("Registered MCP tool: %s", mcpTool.Name()), "")
	}

	return nil
}
