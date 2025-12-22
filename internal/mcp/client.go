package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// Client wraps an MCP server connection and provides methods to interact with it.
type Client struct {
	// serverName is the unique identifier for this MCP server
	serverName string

	// client is the underlying MCP protocol client
	client *client.Client

	// capabilities tracks what features the server supports
	capabilities *ServerCapabilities

	// timeout is the default timeout for tool calls
	timeout time.Duration

	// process is the underlying OS process (for force-kill during shutdown)
	process ProcessHandle
}

// ClientConfig configures an MCP client connection.
type ClientConfig struct {
	// ServerName is the unique identifier for this server
	ServerName string

	// Command is the executable to run
	Command string

	// Args are the command-line arguments
	Args []string

	// Env are environment variables to pass to the server
	Env []string

	// Timeout is the default timeout for tool calls (defaults to 30s)
	Timeout time.Duration
}

// NewClient creates a new MCP client and starts the server process.
func NewClient(ctx context.Context, config ClientConfig) (*Client, error) {
	if config.ServerName == "" {
		return nil, fmt.Errorf("server name is required")
	}
	if config.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Default timeout
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// Create the MCP client
	mcpClient, err := client.NewStdioMCPClient(config.Command, config.Env, config.Args...)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}

	// Start the connection
	if err := mcpClient.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start MCP client: %w", err)
	}

	c := &Client{
		serverName: config.ServerName,
		client:     mcpClient,
		timeout:    timeout,
		process:    extractProcess(mcpClient),
	}

	// Initialize the server (sends initialize request)
	if err := c.initialize(ctx); err != nil {
		c.Close()
		return nil, fmt.Errorf("failed to initialize MCP server: %w", err)
	}

	return c, nil
}

// extractProcess attempts to extract the underlying OS process from the MCP client.
// Uses reflection to access the stdio transport's process field.
// Returns nil if extraction fails (non-fatal - we just won't be able to force-kill).
func extractProcess(mcpClient *client.Client) ProcessHandle {
	if mcpClient == nil {
		return nil
	}

	// Get transport using GetTransport() method
	transport := mcpClient.GetTransport()
	if transport == nil {
		return nil
	}

	// Use reflection to access the Cmd field from StdioTransport
	// The transport should be *transport.StdioTransport which has a Cmd *exec.Cmd field
	transportVal := reflect.ValueOf(transport)
	if transportVal.Kind() == reflect.Ptr {
		transportVal = transportVal.Elem()
	}

	// Look for Cmd field
	cmdField := transportVal.FieldByName("Cmd")
	if !cmdField.IsValid() || cmdField.IsNil() {
		return nil
	}

	// The Cmd is *exec.Cmd, which has a Process field
	if cmdField.Kind() == reflect.Ptr {
		cmdVal := cmdField.Elem()
		processField := cmdVal.FieldByName("Process")
		if !processField.IsValid() || processField.IsNil() {
			return nil
		}

		// Extract *os.Process
		if proc, ok := processField.Interface().(*os.Process); ok {
			return proc
		}
	}

	return nil
}

// initialize sends the initialize request to the MCP server.
func (c *Client) initialize(ctx context.Context) error {
	// Send initialize request with client capabilities
	initReq := mcp.InitializeRequest{
		Params: mcp.InitializeParams{
			ProtocolVersion: mcp.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcp.ClientCapabilities{
				// Minimal capabilities for tool usage
			},
			ClientInfo: mcp.Implementation{
				Name:    "conductor",
				Version: "0.1.0",
			},
		},
	}

	_, err := c.client.Initialize(ctx, initReq)
	if err != nil {
		return fmt.Errorf("initialize request failed: %w", err)
	}

	// Store server capabilities
	serverCaps := c.client.GetServerCapabilities()
	c.capabilities = &ServerCapabilities{}
	if serverCaps.Tools != nil {
		c.capabilities.Tools = &ToolsCapability{
			ListChanged: serverCaps.Tools.ListChanged,
		}
	}
	if serverCaps.Resources != nil {
		c.capabilities.Resources = &ResourcesCapability{
			Subscribe:   serverCaps.Resources.Subscribe,
			ListChanged: serverCaps.Resources.ListChanged,
		}
	}
	if serverCaps.Prompts != nil {
		c.capabilities.Prompts = &PromptsCapability{
			ListChanged: serverCaps.Prompts.ListChanged,
		}
	}

	return nil
}

// ListTools retrieves the list of available tools from the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	result, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	tools := make([]ToolDefinition, len(result.Tools))
	for i, tool := range result.Tools {
		// Use RawInputSchema if available, otherwise marshal InputSchema
		var schemaBytes []byte
		if len(tool.RawInputSchema) > 0 {
			schemaBytes = tool.RawInputSchema
		} else {
			// Marshal the tool to JSON and extract inputSchema
			toolBytes, err := tool.MarshalJSON()
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool %s: %w", tool.Name, err)
			}
			// Parse to extract inputSchema field
			var toolMap map[string]interface{}
			if err := json.Unmarshal(toolBytes, &toolMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool %s: %w", tool.Name, err)
			}
			if inputSchema, ok := toolMap["inputSchema"]; ok {
				schemaBytes, err = json.Marshal(inputSchema)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal input schema for %s: %w", tool.Name, err)
				}
			}
		}

		tools[i] = ToolDefinition{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schemaBytes,
		}
	}

	return tools, nil
}

// CallTool executes an MCP tool with the given arguments.
func (c *Client) CallTool(ctx context.Context, req ToolCallRequest) (*ToolCallResponse, error) {
	// Apply timeout
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// Convert arguments to MCP format
	mcpReq := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      req.Name,
			Arguments: req.Arguments,
		},
	}

	result, err := c.client.CallTool(ctx, mcpReq)
	if err != nil {
		return nil, fmt.Errorf("tool call failed: %w", err)
	}

	// Convert response
	response := &ToolCallResponse{
		IsError: result.IsError,
		Content: make([]ContentItem, len(result.Content)),
	}

	for i, content := range result.Content {
		item := ContentItem{}

		// Use type assertions to determine content type
		if textContent, ok := mcp.AsTextContent(content); ok {
			item.Type = textContent.Type
			item.Text = textContent.Text
		} else if imageContent, ok := mcp.AsImageContent(content); ok {
			item.Type = imageContent.Type
			item.Data = imageContent.Data
			item.MimeType = imageContent.MIMEType
		} else {
			// Fallback: marshal to JSON to extract fields
			contentBytes, err := json.Marshal(content)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal content: %w", err)
			}
			var contentMap map[string]interface{}
			if err := json.Unmarshal(contentBytes, &contentMap); err != nil {
				return nil, fmt.Errorf("failed to unmarshal content: %w", err)
			}

			if contentType, ok := contentMap["type"].(string); ok {
				item.Type = contentType
			}
			if text, ok := contentMap["text"].(string); ok {
				item.Text = text
			}
			if data, ok := contentMap["data"].(string); ok {
				item.Data = data
			}
			if mimeType, ok := contentMap["mimeType"].(string); ok {
				item.MimeType = mimeType
			}
		}

		response.Content[i] = item
	}

	return response, nil
}

// ListResources retrieves the list of available resources from the MCP server.
func (c *Client) ListResources(ctx context.Context) ([]ResourceDefinition, error) {
	if c.capabilities == nil || c.capabilities.Resources == nil {
		return nil, fmt.Errorf("server does not support resources")
	}

	result, err := c.client.ListResources(ctx, mcp.ListResourcesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	resources := make([]ResourceDefinition, len(result.Resources))
	for i, resource := range result.Resources {
		resources[i] = ResourceDefinition{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: resource.Description,
			MimeType:    resource.MIMEType,
		}
	}

	return resources, nil
}

// ReadResource reads the content of an MCP resource.
func (c *Client) ReadResource(ctx context.Context, req ResourceReadRequest) (*ResourceReadResponse, error) {
	if c.capabilities == nil || c.capabilities.Resources == nil {
		return nil, fmt.Errorf("server does not support resources")
	}

	result, err := c.client.ReadResource(ctx, mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: req.URI,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	response := &ResourceReadResponse{
		Contents: make([]ResourceContent, len(result.Contents)),
	}

	for i, content := range result.Contents {
		item := ResourceContent{}

		// Extract content based on type
		if textContent, ok := mcp.AsTextResourceContents(content); ok {
			item.URI = textContent.URI
			item.MimeType = textContent.MIMEType
			item.Text = textContent.Text
		} else if blobContent, ok := mcp.AsBlobResourceContents(content); ok {
			item.URI = blobContent.URI
			item.MimeType = blobContent.MIMEType
			item.Blob = blobContent.Blob
		}

		response.Contents[i] = item
	}

	return response, nil
}

// Capabilities returns the server's capabilities.
func (c *Client) Capabilities() *ServerCapabilities {
	return c.capabilities
}

// ServerName returns the unique identifier for this server.
func (c *Client) ServerName() string {
	return c.serverName
}

// Close closes the connection to the MCP server and stops the process.
func (c *Client) Close() error {
	if c.client == nil {
		return nil
	}

	// Close the client connection
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("failed to close MCP client: %w", err)
	}

	return nil
}

// Process returns the underlying OS process for this MCP server.
// Returns nil if the process is not available (e.g., not a stdio transport).
func (c *Client) Process() ProcessHandle {
	return c.process
}

// Ping checks if the server is still responsive.
func (c *Client) Ping(ctx context.Context) error {
	// Send a ping request
	if err := c.client.Ping(ctx); err != nil {
		// Check if it's an EOF error (server closed)
		if err == io.EOF {
			return fmt.Errorf("server connection closed")
		}
		return fmt.Errorf("ping failed: %w", err)
	}

	return nil
}
