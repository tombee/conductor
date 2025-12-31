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

/*
Package mcp implements the Model Context Protocol (MCP) for Conductor.

MCP enables workflows to integrate with external tool servers that expose
capabilities like file system access, database queries, or custom operations.
This package handles server lifecycle, client communication, and tool adaptation.

# Overview

The MCP implementation consists of several components:

  - Manager: Lifecycle management for MCP servers (start, stop, restart)
  - Client: Communication with MCP server processes via stdio
  - Registry: Global registration and discovery of MCP servers
  - Tool Adapter: Converts MCP tools to Conductor's tool interface

# Server Lifecycle

Start an MCP server:

	mgr := mcp.NewManager(mcp.ManagerConfig{Logger: logger})

	err := mgr.Start(mcp.ServerConfig{
	    Name:    "filesystem",
	    Command: "npx",
	    Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
	    Env:     []string{"HOME=/home/user"},
	})

The manager handles:

  - Process spawning and monitoring
  - Health checking via ping
  - Automatic restart with exponential backoff
  - Graceful shutdown

# Tool Discovery

Once a server is running, discover its tools:

	client, err := mgr.GetClient("filesystem")
	tools, err := client.ListTools(ctx)

	for _, tool := range tools {
	    fmt.Printf("Tool: %s - %s\n", tool.Name, tool.Description)
	}

# Tool Invocation

Call a tool on the server:

	result, err := client.CallTool(ctx, "read_file", map[string]any{
	    "path": "/etc/hosts",
	})

# Integration with Workflows

Workflows define MCP servers in their configuration:

	name: my-workflow
	mcp_servers:
	  - name: filesystem
	    command: npx
	    args: ["-y", "@modelcontextprotocol/server-filesystem"]

The runner automatically:

 1. Starts defined servers before workflow execution
 2. Registers discovered tools in the tool registry
 3. Stops servers after workflow completion

# Server States

MCP servers transition through states:

  - stopped: Not running
  - starting: Process spawning
  - running: Healthy and accepting requests
  - restarting: Restarting after failure or request
  - error: Failed to start or crashed

# Registry

The Registry provides controller-level MCP server management:

	registry, _ := mcp.NewRegistry(mcp.RegistryConfig{Logger: logger})
	registry.Start(ctx)  // Starts auto-start servers

	// Query server status
	summary := registry.GetSummary()
	fmt.Printf("Running: %d/%d\n", summary.Running, summary.Total)

# Configuration

MCP servers can be configured at the controller level:

	~/.config/conductor/mcp-servers.yaml

	servers:
	  - name: filesystem
	    command: npx
	    args: ["-y", "@modelcontextprotocol/server-filesystem"]
	    auto_start: true
*/
package mcp
