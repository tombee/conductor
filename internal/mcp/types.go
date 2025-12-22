// Package mcp provides Model Context Protocol integration for Conductor.
//
// MCP (Model Context Protocol) defines a standard way for LLMs to interact with
// external tools and data sources. This package implements MCP client functionality
// to communicate with MCP servers over stdio, manage server lifecycles, and bridge
// MCP tools to Conductor's tool registry.
package mcp

import (
	"encoding/json"
)

// ToolDefinition represents an MCP tool definition.
// Maps to the MCP protocol's Tool schema.
type ToolDefinition struct {
	// Name is the unique identifier for this tool
	Name string `json:"name"`

	// Description explains what the tool does
	Description string `json:"description"`

	// InputSchema defines the expected input parameters using JSON Schema
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolCallRequest represents a request to execute an MCP tool.
type ToolCallRequest struct {
	// Name is the tool to execute
	Name string `json:"name"`

	// Arguments contains the input parameters for the tool
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResponse represents the result of an MCP tool execution.
type ToolCallResponse struct {
	// Content contains the tool's output
	Content []ContentItem `json:"content"`

	// IsError indicates if the tool execution failed
	IsError bool `json:"isError,omitempty"`
}

// ContentItem represents a piece of content in an MCP response.
type ContentItem struct {
	// Type is the content type (text, image, resource)
	Type string `json:"type"`

	// Text is the text content (for type="text")
	Text string `json:"text,omitempty"`

	// Data is the base64-encoded data (for type="image")
	Data string `json:"data,omitempty"`

	// MimeType is the MIME type for binary content
	MimeType string `json:"mimeType,omitempty"`
}

// ResourceDefinition represents an MCP resource definition.
type ResourceDefinition struct {
	// URI is the unique identifier for this resource
	URI string `json:"uri"`

	// Name is a human-readable name
	Name string `json:"name"`

	// Description explains what this resource contains
	Description string `json:"description,omitempty"`

	// MimeType indicates the content type
	MimeType string `json:"mimeType,omitempty"`
}

// ResourceReadRequest represents a request to read an MCP resource.
type ResourceReadRequest struct {
	// URI is the resource to read
	URI string `json:"uri"`
}

// ResourceReadResponse represents the result of reading an MCP resource.
type ResourceReadResponse struct {
	// Contents contains the resource data
	Contents []ResourceContent `json:"contents"`
}

// ResourceContent represents the content of an MCP resource.
type ResourceContent struct {
	// URI is the resource identifier
	URI string `json:"uri"`

	// MimeType indicates the content type
	MimeType string `json:"mimeType,omitempty"`

	// Text is the text content (for text resources)
	Text string `json:"text,omitempty"`

	// Blob is the base64-encoded binary content (for binary resources)
	Blob string `json:"blob,omitempty"`
}

// ServerCapabilities describes what features an MCP server supports.
type ServerCapabilities struct {
	// Tools indicates if the server provides tools
	Tools *ToolsCapability `json:"tools,omitempty"`

	// Resources indicates if the server provides resources
	Resources *ResourcesCapability `json:"resources,omitempty"`

	// Prompts indicates if the server provides prompts
	Prompts *PromptsCapability `json:"prompts,omitempty"`
}

// ToolsCapability describes tool-related capabilities.
type ToolsCapability struct {
	// ListChanged indicates if the server sends notifications when tools change
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability describes resource-related capabilities.
type ResourcesCapability struct {
	// Subscribe indicates if clients can subscribe to resource updates
	Subscribe bool `json:"subscribe,omitempty"`

	// ListChanged indicates if the server sends notifications when resources change
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability describes prompt-related capabilities.
type PromptsCapability struct {
	// ListChanged indicates if the server sends notifications when prompts change
	ListChanged bool `json:"listChanged,omitempty"`
}

// ProtocolError represents an MCP protocol error.
type ProtocolError struct {
	// Code is the error code
	Code int `json:"code"`

	// Message describes the error
	Message string `json:"message"`

	// Data contains additional error details
	Data interface{} `json:"data,omitempty"`
}

// Error implements the error interface.
func (e *ProtocolError) Error() string {
	if e.Data != nil {
		return e.Message + " (data: " + string(e.Data.([]byte)) + ")"
	}
	return e.Message
}

// Common MCP error codes.
const (
	// ErrorCodeParse indicates a JSON parsing error
	ErrorCodeParse = -32700

	// ErrorCodeInvalidRequest indicates an invalid JSON-RPC request
	ErrorCodeInvalidRequest = -32600

	// ErrorCodeMethodNotFound indicates the method doesn't exist
	ErrorCodeMethodNotFound = -32601

	// ErrorCodeInvalidParams indicates invalid method parameters
	ErrorCodeInvalidParams = -32602

	// ErrorCodeInternal indicates an internal server error
	ErrorCodeInternal = -32603
)
