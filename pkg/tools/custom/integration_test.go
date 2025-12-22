package custom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

// TestHTTPCustomTool_Integration tests end-to-end HTTP tool workflow.
func TestHTTPCustomTool_Integration(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Expected Authorization header, got: %s", auth)
		}

		// Parse request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify inputs were passed
		if reqBody["query"] != "test query" {
			t.Errorf("Expected query in body, got: %v", reqBody)
		}

		// Return JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "success",
			"data":   "test data",
		})
	}))
	defer server.Close()

	// Create tool definition
	toolDef := workflow.FunctionDefinition{
		Name:        "search-api",
		Type:        workflow.ToolTypeHTTP,
		Description: "Search API tool",
		Method:      "POST",
		URL:         server.URL + "/search",
		Headers: map[string]string{
			"Authorization": "Bearer test-token",
			"Content-Type":  "application/json",
		},
		Timeout:         5,
		MaxResponseSize: 1024 * 1024,
	}

	// Create tool
	tool, err := NewHTTPCustomTool(toolDef)
	if err != nil {
		t.Fatalf("NewHTTPCustomTool() error = %v", err)
	}

	// Execute tool
	ctx := context.Background()
	result, err := tool.Execute(ctx, map[string]interface{}{
		"query": "test query",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify result structure
	if result["status_code"] != 200 {
		t.Errorf("Expected status 200, got: %v", result["status_code"])
	}

	response, ok := result["response"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected response to be map, got: %T", result["response"])
	}

	if response["result"] != "success" {
		t.Errorf("Expected result=success, got: %v", response["result"])
	}

	if response["data"] != "test data" {
		t.Errorf("Expected data='test data', got: %v", response["data"])
	}
}

// TestHTTPCustomTool_ErrorHandling tests error propagation.
func TestHTTPCustomTool_ErrorHandling(t *testing.T) {
	// Create mock server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found"))
	}))
	defer server.Close()

	toolDef := workflow.FunctionDefinition{
		Name:        "failing-api",
		Type:        workflow.ToolTypeHTTP,
		Description: "API that fails",
		Method:      "GET",
		URL:         server.URL + "/notfound",
	}

	tool, err := NewHTTPCustomTool(toolDef)
	if err != nil {
		t.Fatalf("NewHTTPCustomTool() error = %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, nil)

	if err == nil {
		t.Fatal("Expected error for 404 response")
	}

	// Error should include status code
	if !contains(err.Error(), "404") {
		t.Errorf("Expected error to mention 404, got: %v", err)
	}
}

// TestHTTPCustomTool_Timeout tests timeout enforcement.
func TestHTTPCustomTool_Timeout(t *testing.T) {
	// Create slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	toolDef := workflow.FunctionDefinition{
		Name:        "slow-api",
		Type:        workflow.ToolTypeHTTP,
		Description: "Slow API",
		Method:      "GET",
		URL:         server.URL,
		Timeout:     1, // 1 second timeout
	}

	tool, err := NewHTTPCustomTool(toolDef)
	if err != nil {
		t.Fatalf("NewHTTPCustomTool() error = %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, nil)

	if err == nil {
		t.Fatal("Expected timeout error")
	}
}

// TestHTTPCustomTool_ResponseSizeLimit tests size limit enforcement.
func TestHTTPCustomTool_ResponseSizeLimit(t *testing.T) {
	// Create server that returns large response
	largeData := make([]byte, 2*1024*1024) // 2MB
	for i := range largeData {
		largeData[i] = 'a'
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write(largeData)
	}))
	defer server.Close()

	toolDef := workflow.FunctionDefinition{
		Name:            "large-response",
		Type:            workflow.ToolTypeHTTP,
		Description:     "Returns large response",
		Method:          "GET",
		URL:             server.URL,
		MaxResponseSize: 1024, // 1KB limit
	}

	tool, err := NewHTTPCustomTool(toolDef)
	if err != nil {
		t.Fatalf("NewHTTPCustomTool() error = %v", err)
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, nil)

	// Should not error, but response should be truncated
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	response, ok := result["response"].(string)
	if !ok {
		t.Fatalf("Expected string response")
	}

	// Response should be exactly maxResponseSize
	if len(response) != 1024 {
		t.Errorf("Expected response size 1024, got: %d", len(response))
	}
}

// TestScriptCustomTool_Integration tests end-to-end script tool workflow.
func TestScriptCustomTool_Integration(t *testing.T) {
	// Create temporary directory for test
	tmpDir, err := os.MkdirTemp("", "script-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test script that echoes JSON
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	scriptContent := `#!/bin/bash
read input
echo "$input" | jq '{result: "processed", input: .}'
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	// Create tool definition
	toolDef := workflow.FunctionDefinition{
		Name:        "json-processor",
		Type:        workflow.ToolTypeScript,
		Description: "Processes JSON input",
		Command:     "test-script.sh",
		Timeout:     5,
	}

	// Create tool
	tool, err := NewScriptCustomTool(toolDef, tmpDir)
	if err != nil {
		t.Fatalf("NewScriptCustomTool() error = %v", err)
	}

	// Execute tool
	ctx := context.Background()
	result, err := tool.Execute(ctx, map[string]interface{}{
		"message": "hello",
		"count":   42,
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify result
	output, ok := result["output"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected output to be map, got: %T", result["output"])
	}

	if output["result"] != "processed" {
		t.Errorf("Expected result=processed, got: %v", output["result"])
	}
}

// TestScriptCustomTool_ErrorPropagation tests error handling.
func TestScriptCustomTool_ErrorPropagation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "script-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create script that exits with error
	scriptPath := filepath.Join(tmpDir, "failing-script.sh")
	scriptContent := `#!/bin/bash
echo "Error message" >&2
exit 1
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	toolDef := workflow.FunctionDefinition{
		Name:        "failing-script",
		Type:        workflow.ToolTypeScript,
		Description: "Script that fails",
		Command:     "failing-script.sh",
	}

	tool, err := NewScriptCustomTool(toolDef, tmpDir)
	if err != nil {
		t.Fatalf("NewScriptCustomTool() error = %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, nil)

	if err == nil {
		t.Fatal("Expected error from failing script")
	}

	// Error should include stderr
	if !contains(err.Error(), "Error message") {
		t.Errorf("Expected error to include stderr, got: %v", err)
	}
}

// TestScriptCustomTool_Timeout tests timeout enforcement.
func TestScriptCustomTool_Timeout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "script-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create slow script
	scriptPath := filepath.Join(tmpDir, "slow-script.sh")
	scriptContent := `#!/bin/bash
sleep 5
echo '{"result": "done"}'
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	toolDef := workflow.FunctionDefinition{
		Name:        "slow-script",
		Type:        workflow.ToolTypeScript,
		Description: "Slow script",
		Command:     "slow-script.sh",
		Timeout:     1, // 1 second timeout
	}

	tool, err := NewScriptCustomTool(toolDef, tmpDir)
	if err != nil {
		t.Fatalf("NewScriptCustomTool() error = %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, nil)

	if err == nil {
		t.Fatal("Expected timeout error")
	}

	if !contains(err.Error(), "timed out") {
		t.Errorf("Expected timeout error message, got: %v", err)
	}
}

// TestScriptCustomTool_OutputSizeLimit tests size limit enforcement.
func TestScriptCustomTool_OutputSizeLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "script-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create script that outputs large data
	scriptPath := filepath.Join(tmpDir, "large-output.sh")
	scriptContent := `#!/bin/bash
for i in {1..100000}; do
  echo "This is line $i with lots of data"
done
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write script: %v", err)
	}

	toolDef := workflow.FunctionDefinition{
		Name:            "large-output",
		Type:            workflow.ToolTypeScript,
		Description:     "Produces large output",
		Command:         "large-output.sh",
		MaxResponseSize: 1024, // 1KB limit
	}

	tool, err := NewScriptCustomTool(toolDef, tmpDir)
	if err != nil {
		t.Fatalf("NewScriptCustomTool() error = %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, nil)

	if err == nil {
		t.Fatal("Expected error for output exceeding size limit")
	}

	if !contains(err.Error(), "exceeds maximum size") {
		t.Errorf("Expected size limit error, got: %v", err)
	}
}

// TestScriptCustomTool_PathTraversalPrevention tests security.
func TestScriptCustomTool_PathTraversalPrevention(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "script-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Try to access file outside workflow directory
	toolDef := workflow.FunctionDefinition{
		Name:        "malicious-script",
		Type:        workflow.ToolTypeScript,
		Description: "Tries path traversal",
		Command:     "../../etc/passwd",
	}

	tool, err := NewScriptCustomTool(toolDef, tmpDir)
	if err != nil {
		t.Fatalf("NewScriptCustomTool() error = %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, nil)

	if err == nil {
		t.Fatal("Expected error for path traversal attempt")
	}

	if !contains(err.Error(), "security violation") {
		t.Errorf("Expected security violation error, got: %v", err)
	}
}

// TestToolFactory tests the factory function.
func TestToolFactory(t *testing.T) {
	tests := []struct {
		name        string
		toolDef     workflow.FunctionDefinition
		workflowDir string
		wantErr     bool
		wantType    string
	}{
		{
			name: "HTTP tool",
			toolDef: workflow.FunctionDefinition{
				Name:        "http-tool",
				Type:        workflow.ToolTypeHTTP,
				Description: "HTTP tool",
				Method:      "GET",
				URL:         "https://api.example.com",
			},
			workflowDir: "/tmp",
			wantErr:     false,
			wantType:    "*custom.HTTPCustomTool",
		},
		{
			name: "Script tool",
			toolDef: workflow.FunctionDefinition{
				Name:        "script-tool",
				Type:        workflow.ToolTypeScript,
				Description: "Script tool",
				Command:     "script.sh",
			},
			workflowDir: "/tmp",
			wantErr:     false,
			wantType:    "*custom.ScriptCustomTool",
		},
		{
			name: "Invalid tool type",
			toolDef: workflow.FunctionDefinition{
				Name:        "invalid-tool",
				Type:        workflow.ToolType("invalid"),
				Description: "Invalid tool",
			},
			workflowDir: "/tmp",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := NewToolFromDefinition(tt.toolDef, tt.workflowDir)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NewToolFromDefinition() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("NewToolFromDefinition() error = %v", err)
			}

			if tool == nil {
				t.Fatal("NewToolFromDefinition() returned nil tool")
			}

			if tool.Name() != tt.toolDef.Name {
				t.Errorf("Tool name = %s, want %s", tool.Name(), tt.toolDef.Name)
			}

			actualType := fmt.Sprintf("%T", tool)
			if actualType != tt.wantType {
				t.Errorf("Tool type = %s, want %s", actualType, tt.wantType)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
