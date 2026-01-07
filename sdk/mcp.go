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
