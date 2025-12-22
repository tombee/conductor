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

import "context"

// MCPManagerProvider defines the interface for managing MCP server lifecycle.
// This interface enables dependency injection and testing with mock implementations.
type MCPManagerProvider interface {
	// Start starts an MCP server with the given configuration.
	Start(config ServerConfig) error

	// Stop stops an MCP server by name.
	Stop(name string) error

	// GetClient returns the MCP client for a server by name.
	GetClient(name string) (ClientProvider, error)

	// ListServers returns the names of all managed servers.
	ListServers() []string

	// IsRunning returns true if the named server is running.
	IsRunning(name string) bool
}

// ClientProvider defines the interface for interacting with an MCP client.
// This interface enables dependency injection and testing with mock implementations.
type ClientProvider interface {
	// ListTools retrieves the list of available tools from the MCP server.
	ListTools(ctx context.Context) ([]ToolDefinition, error)

	// CallTool executes an MCP tool with the given arguments.
	CallTool(ctx context.Context, req ToolCallRequest) (*ToolCallResponse, error)

	// Close closes the connection to the MCP server.
	Close() error

	// Ping checks if the server is still responsive.
	Ping(ctx context.Context) error

	// ServerName returns the unique identifier for this server.
	ServerName() string

	// Capabilities returns the server's capabilities.
	Capabilities() *ServerCapabilities
}
