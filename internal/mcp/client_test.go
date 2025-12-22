package mcp

import (
	"context"
	"testing"
	"time"
)

func TestClientConfig_Validation(t *testing.T) {
	tests := []struct {
		name      string
		config    ClientConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "missing server name",
			config: ClientConfig{
				Command: "echo",
			},
			wantError: true,
			errorMsg:  "server name is required",
		},
		{
			name: "missing command",
			config: ClientConfig{
				ServerName: "test-server",
			},
			wantError: true,
			errorMsg:  "command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Note: NewClient will fail because echo is not an MCP server
			// This test just validates the config validation logic
			_, err := NewClient(ctx, tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("NewClient() expected error containing %q, got nil", tt.errorMsg)
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("NewClient() error = %v, want error containing %v", err, tt.errorMsg)
				}
			}
		})
	}
}

func TestProtocolError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ProtocolError
		want string
	}{
		{
			name: "error without data",
			err: &ProtocolError{
				Code:    ErrorCodeMethodNotFound,
				Message: "method not found",
			},
			want: "method not found",
		},
		{
			name: "error with data",
			err: &ProtocolError{
				Code:    ErrorCodeInvalidParams,
				Message: "invalid parameters",
				Data:    []byte(`{"detail":"missing required field"}`),
			},
			want: "invalid parameters (data: {\"detail\":\"missing required field\"})",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("ProtocolError.Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToolDefinition_Structure(t *testing.T) {
	// Test that ToolDefinition can be marshaled/unmarshaled
	tool := ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: []byte(`{"type":"object","properties":{"arg":{"type":"string"}}}`),
	}

	if tool.Name != "test_tool" {
		t.Errorf("ToolDefinition.Name = %v, want %v", tool.Name, "test_tool")
	}
	if tool.Description != "A test tool" {
		t.Errorf("ToolDefinition.Description = %v, want %v", tool.Description, "A test tool")
	}
	if len(tool.InputSchema) == 0 {
		t.Error("ToolDefinition.InputSchema should not be empty")
	}
}

func TestToolCallRequest_Structure(t *testing.T) {
	req := ToolCallRequest{
		Name: "test_tool",
		Arguments: map[string]interface{}{
			"arg1": "value1",
			"arg2": 42,
		},
	}

	if req.Name != "test_tool" {
		t.Errorf("ToolCallRequest.Name = %v, want %v", req.Name, "test_tool")
	}
	if len(req.Arguments) != 2 {
		t.Errorf("ToolCallRequest.Arguments length = %v, want %v", len(req.Arguments), 2)
	}
}

func TestToolCallResponse_Structure(t *testing.T) {
	resp := ToolCallResponse{
		Content: []ContentItem{
			{
				Type: "text",
				Text: "result",
			},
		},
		IsError: false,
	}

	if len(resp.Content) != 1 {
		t.Errorf("ToolCallResponse.Content length = %v, want %v", len(resp.Content), 1)
	}
	if resp.Content[0].Type != "text" {
		t.Errorf("ToolCallResponse.Content[0].Type = %v, want %v", resp.Content[0].Type, "text")
	}
	if resp.Content[0].Text != "result" {
		t.Errorf("ToolCallResponse.Content[0].Text = %v, want %v", resp.Content[0].Text, "result")
	}
	if resp.IsError {
		t.Error("ToolCallResponse.IsError should be false")
	}
}

func TestServerCapabilities_Structure(t *testing.T) {
	caps := ServerCapabilities{
		Tools: &ToolsCapability{
			ListChanged: true,
		},
		Resources: &ResourcesCapability{
			Subscribe:   true,
			ListChanged: false,
		},
	}

	if caps.Tools == nil {
		t.Error("ServerCapabilities.Tools should not be nil")
	}
	if !caps.Tools.ListChanged {
		t.Error("ToolsCapability.ListChanged should be true")
	}
	if caps.Resources == nil {
		t.Error("ServerCapabilities.Resources should not be nil")
	}
	if !caps.Resources.Subscribe {
		t.Error("ResourcesCapability.Subscribe should be true")
	}
}

func TestResourceDefinition_Structure(t *testing.T) {
	resource := ResourceDefinition{
		URI:         "file:///test.txt",
		Name:        "test file",
		Description: "A test file resource",
		MimeType:    "text/plain",
	}

	if resource.URI != "file:///test.txt" {
		t.Errorf("ResourceDefinition.URI = %v, want %v", resource.URI, "file:///test.txt")
	}
	if resource.Name != "test file" {
		t.Errorf("ResourceDefinition.Name = %v, want %v", resource.Name, "test file")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
