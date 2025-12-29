package sdk

import (
	"time"
)

// MCPConfig configures an MCP server connection.
type MCPConfig struct {
	// Transport is "stdio" or "sse"
	Transport string

	// Command is the command to run (for stdio transport)
	Command string
	Args    []string
	Env     map[string]string

	// URL is the SSE endpoint (for sse transport)
	URL string

	// ToolFilter limits which tools to expose (empty = all)
	ToolFilter []string

	// Security constraints (required for untrusted MCP servers)
	ConnectTimeout time.Duration // Max time to establish connection (default 5s)
	RequestTimeout time.Duration // Max time for tool execution (default 30s)
	MaxOutputSize  int64         // Max bytes tool can return (default 10MB)
}

// ConnectMCP connects to an MCP server and registers its tools.
// Returns an error if the server is unreachable within ConnectTimeout.
//
// The server must have been registered via WithMCPServer() during SDK construction.
//
// Example:
//
//	config := sdk.MCPConfig{
//		Transport: "stdio",
//		Command:   "mcp-server-gh",
//		Args:      []string{"--token", token},
//		ConnectTimeout: 10 * time.Second,
//	}
//	if err := s.ConnectMCP("github", config); err != nil {
//		return err
//	}
func (s *SDK) ConnectMCP(name string, config MCPConfig) error {
	// TODO: Implement in Phase 2
	// This will connect to the MCP server and register its tools
	return nil
}

// DisconnectMCP disconnects from an MCP server.
// Gracefully closes the connection; pending tool calls fail with error.
//
// Example:
//
//	if err := s.DisconnectMCP("github"); err != nil {
//		return err
//	}
func (s *SDK) DisconnectMCP(name string) error {
	// TODO: Implement in Phase 2
	// This will disconnect from the MCP server and unregister its tools
	return nil
}
