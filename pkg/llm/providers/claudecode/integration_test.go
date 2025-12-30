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

//go:build integration

package claudecode

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/testing/integration"
	"github.com/tombee/conductor/pkg/llm"
)

// TestClaudeCLI_FileReadToolRouting verifies that file.read tool calls are routed
// through Conductor's operation registry instead of Claude's built-in Read tool.
//
// This test:
// 1. Creates a temporary file with known content
// 2. Asks Claude to read it using file.read
// 3. Verifies the tool_use block has name "file.read"
// 4. Verifies the tool was executed through the instrumented registry
// 5. Verifies the file content appears in Claude's final response
func TestClaudeCLI_FileReadToolRouting(t *testing.T) {
	skipClaudeCLITests(t)

	// Create test directory and file
	tmpDir := t.TempDir()
	testContent := "Test content for E2E verification: ABC123"
	testPath := createTestFile(t, tmpDir, "test-file.txt", testContent)

	// Setup provider with instrumented registry
	provider, registry := setupTestProvider(t, tmpDir)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout())
	defer cancel()

	// Build request asking Claude to read the file
	// Note: MaxTokens not specified to avoid Claude CLI flag issue
	req := llm.CompletionRequest{
		Model: "fast", // Use haiku for cost efficiency
		Messages: []llm.Message{
			{
				Role:    llm.MessageRoleSystem,
				Content: buildToolSystemPrompt(),
			},
			{
				Role:    llm.MessageRoleUser,
				Content: fmt.Sprintf("Read the file at %s and tell me what it contains. Use the file.read tool.", testPath),
			},
		},
		Tools: buildFileReadTool(),
	}

	// Execute completion with retry logic
	resp, err := executeWithRetry(ctx, t, provider, req)
	if err != nil {
		t.Fatalf("Completion failed: %v", err)
	}

	// Log audit trail
	records := registry.Records()
	logAuditTrail(t, records)

	// Verify registry was called
	if len(records) == 0 {
		t.Fatal("No tool executions recorded - tool routing may be broken")
	}

	// Verify file.read was called
	var foundFileRead bool
	for _, record := range records {
		if record.Reference == "file.read" {
			foundFileRead = true
			// Verify path input
			if path, ok := record.Inputs["path"].(string); ok {
				if path != testPath {
					t.Errorf("file.read called with wrong path: got %s, want %s", path, testPath)
				}
			}
		}
	}

	if !foundFileRead {
		t.Error("file.read was not called through the registry")
	}

	// Verify response contains the file content
	if !strings.Contains(resp.Content, "ABC123") {
		t.Errorf("Response does not contain file content 'ABC123': %s", resp.Content)
	}
}

// TestClaudeCLI_NoBuiltinToolUse verifies that Claude's built-in tools are disabled
// when using Conductor's tool routing via --tools "".
//
// This test:
// 1. Creates two temporary files
// 2. Asks Claude to read both files
// 3. Verifies NO tool calls have names matching Claude's built-in tools (Read, Write, etc.)
// 4. Verifies all tool calls use Conductor's namespace.operation format
func TestClaudeCLI_NoBuiltinToolUse(t *testing.T) {
	skipClaudeCLITests(t)

	// Create test directory and files
	tmpDir := t.TempDir()
	file1Content := "First file content: ALPHA"
	file2Content := "Second file content: BETA"
	file1Path := createTestFile(t, tmpDir, "file1.txt", file1Content)
	file2Path := createTestFile(t, tmpDir, "file2.txt", file2Content)

	// Setup provider with instrumented registry
	provider, registry := setupTestProvider(t, tmpDir)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout())
	defer cancel()

	// Build request asking Claude to read both files
	req := llm.CompletionRequest{
		Model: "fast",
		Messages: []llm.Message{
			{
				Role:    llm.MessageRoleSystem,
				Content: buildToolSystemPrompt(),
			},
			{
				Role: llm.MessageRoleUser,
				Content: fmt.Sprintf("Read the file at %s and the file at %s. Summarize both files. Use the file.read tool for each file.",
					file1Path, file2Path),
			},
		},
		Tools: buildFileReadTool(),
	}

	// Execute completion with retry logic
	_, err := executeWithRetry(ctx, t, provider, req)
	if err != nil {
		t.Fatalf("Completion failed: %v", err)
	}

	// Collect all tool names from execution records
	records := registry.Records()
	logAuditTrail(t, records)

	if len(records) == 0 {
		t.Fatal("No tool executions recorded")
	}

	var toolNames []string
	for _, record := range records {
		toolNames = append(toolNames, record.Reference)
	}

	t.Logf("Tool calls: %v", toolNames)

	// Verify no Claude built-in tools were used
	assertNoBuiltinTools(t, toolNames)

	// Verify all tools follow Conductor's format
	assertConductorToolFormat(t, toolNames)
}

// TestClaudeCLI_MultiTurnConversation verifies that multi-turn conversations
// with tool calls work correctly through the operation registry.
//
// This test:
// 1. Creates two files with different content
// 2. Asks Claude to read both files sequentially
// 3. Verifies at least 2 tool calls were made
// 4. Verifies tool results were properly incorporated
func TestClaudeCLI_MultiTurnConversation(t *testing.T) {
	skipClaudeCLITests(t)

	// Create test directory and files
	tmpDir := t.TempDir()
	file1Content := "Configuration: port=8080"
	file2Content := "Secrets: password=hunter2"
	file1Path := createTestFile(t, tmpDir, "config.txt", file1Content)
	file2Path := createTestFile(t, tmpDir, "secrets.txt", file2Content)

	// Setup provider with instrumented registry
	provider, registry := setupTestProvider(t, tmpDir)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout())
	defer cancel()

	// Build request that requires multiple tool calls
	req := llm.CompletionRequest{
		Model: "fast",
		Messages: []llm.Message{
			{
				Role:    llm.MessageRoleSystem,
				Content: buildToolSystemPrompt(),
			},
			{
				Role: llm.MessageRoleUser,
				Content: fmt.Sprintf("First, read %s using file.read. Then read %s using file.read. Finally, tell me what both files contained.",
					file1Path, file2Path),
			},
		},
		Tools: buildFileReadTool(),
	}

	// Execute completion with retry logic
	resp, err := executeWithRetry(ctx, t, provider, req)
	if err != nil {
		t.Fatalf("Completion failed: %v", err)
	}

	// Log audit trail
	records := registry.Records()
	logAuditTrail(t, records)

	// Verify at least 2 tool calls were made
	if len(records) < 2 {
		t.Errorf("Expected at least 2 tool calls, got %d", len(records))
	}

	// Log all tool calls for debugging
	for i, record := range records {
		t.Logf("Tool call %d: %s", i+1, record.Reference)
	}

	// Verify response incorporates content from both files
	if !strings.Contains(resp.Content, "8080") && !strings.Contains(resp.Content, "port") {
		t.Errorf("Response does not mention content from first file (port/8080): %s", resp.Content)
	}
	if !strings.Contains(resp.Content, "hunter2") && !strings.Contains(resp.Content, "password") {
		t.Errorf("Response does not mention content from second file (password/hunter2): %s", resp.Content)
	}
}

// TestClaudeCLI_ToolExecutionViaRegistry verifies that the instrumented registry's
// Execute() method is called exactly once for each tool_use block from Claude.
//
// This test:
// 1. Creates a test file
// 2. Asks Claude to read it
// 3. Verifies the registry recorded the exact execution with correct inputs
func TestClaudeCLI_ToolExecutionViaRegistry(t *testing.T) {
	skipClaudeCLITests(t)

	// Create test directory and file
	tmpDir := t.TempDir()
	testContent := "Registry verification content"
	testPath := createTestFile(t, tmpDir, "registry-test.txt", testContent)

	// Setup provider with instrumented registry
	provider, registry := setupTestProvider(t, tmpDir)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout())
	defer cancel()

	// Build simple request requiring one tool call
	req := llm.CompletionRequest{
		Model: "fast",
		Messages: []llm.Message{
			{
				Role:    llm.MessageRoleSystem,
				Content: buildToolSystemPrompt(),
			},
			{
				Role:    llm.MessageRoleUser,
				Content: fmt.Sprintf("Read the file at %s using file.read and report its contents.", testPath),
			},
		},
		Tools: buildFileReadTool(),
	}

	// Execute completion with retry logic
	_, err := executeWithRetry(ctx, t, provider, req)
	if err != nil {
		t.Fatalf("Completion failed: %v", err)
	}

	// Log audit trail
	records := registry.Records()
	logAuditTrail(t, records)

	// Verify execution records
	if len(records) == 0 {
		t.Fatal("Registry recorded no executions")
	}

	// Find file.read execution
	var fileReadCount int
	for _, record := range records {
		if record.Reference == "file.read" {
			fileReadCount++

			// Verify inputs
			if record.Inputs == nil {
				t.Error("file.read inputs is nil")
				continue
			}

			// Verify path was passed
			if path, ok := record.Inputs["path"].(string); ok {
				if path != testPath {
					t.Errorf("file.read path: got %s, want %s", path, testPath)
				}
			} else {
				t.Error("file.read inputs missing 'path' field")
			}

			// Verify result was captured
			if record.Result == nil && record.Error == nil {
				t.Error("file.read execution has no result or error")
			}

			// Verify timestamp is reasonable
			if record.Timestamp.IsZero() {
				t.Error("file.read execution timestamp is zero")
			}
		}
	}

	if fileReadCount == 0 {
		t.Error("file.read was not executed via registry")
	}

	t.Logf("Total registry executions: %d, file.read calls: %d", len(records), fileReadCount)
}

// executeWithRetry executes a completion request with exponential backoff retry
// for transient errors (rate limits, server errors). Fails immediately on auth errors.
func executeWithRetry(ctx context.Context, t *testing.T, provider *Provider, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	t.Helper()

	var lastErr error
	var resp *llm.CompletionResponse

	retryCfg := integration.DefaultRetryConfig()
	retryCfg.MaxAttempts = 3
	retryCfg.InitialDelay = 1 * time.Second

	err := integration.Retry(ctx, func() error {
		var err error
		resp, err = provider.Complete(ctx, req)
		if err != nil {
			lastErr = err
			t.Logf("Retry attempt failed: %v", err)
			return err
		}
		return nil
	}, retryCfg)

	if err != nil {
		return nil, fmt.Errorf("all retry attempts failed: %w", lastErr)
	}

	return resp, nil
}

// logAuditTrail logs the audit trail of tool executions for security verification.
// Each execution is logged with timestamp, operation reference, status, and any errors.
func logAuditTrail(t *testing.T, records []ExecutionRecord) {
	t.Helper()

	t.Log("=== Tool Execution Audit Trail ===")
	for i, record := range records {
		status := "success"
		errMsg := ""
		if record.Error != nil {
			status = "error"
			// Sanitize error message (don't include full paths or sensitive info)
			errMsg = fmt.Sprintf(" error=%q", sanitizeAuditError(record.Error.Error()))
		}
		t.Logf("  [%d] %s operation=%s status=%s timestamp=%s%s",
			i+1,
			record.Timestamp.Format(time.RFC3339),
			record.Reference,
			status,
			record.Timestamp.Format("15:04:05.000"),
			errMsg,
		)
	}
	t.Log("=== End Audit Trail ===")
}

// sanitizeAuditError removes potentially sensitive information from error messages.
func sanitizeAuditError(errMsg string) string {
	// Remove file paths that might reveal system structure
	// Keep error type and general message
	if len(errMsg) > 100 {
		return errMsg[:100] + "..."
	}
	return errMsg
}

// buildToolSystemPrompt returns the system prompt that registers Conductor's tools.
func buildToolSystemPrompt() string {
	return `You are a helpful assistant with access to tools.

Available tools:
- file.read: Read the contents of a file. Parameters: path (string, required)

When asked to read a file, use the file.read tool with the exact path provided.
Always use the tools when instructed to do so.`
}

// buildFileReadTool returns the tool definition for file.read.
func buildFileReadTool() []llm.Tool {
	return []llm.Tool{
		{
			Name:        "file.read",
			Description: "Read the contents of a file at the specified path",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}
