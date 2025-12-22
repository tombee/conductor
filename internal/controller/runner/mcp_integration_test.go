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

package runner

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tombee/conductor/internal/mcp"
	mcptesting "github.com/tombee/conductor/internal/mcp/testing"
)

// TestMCPWorkflowIntegration tests end-to-end MCP server integration in workflow execution.
// This test creates a simple echo MCP server and verifies that:
// 1. MCP servers are started when workflow begins
// 2. MCP tools are registered in the tool registry
// 3. MCP servers are stopped when workflow completes
func TestMCPWorkflowIntegration(t *testing.T) {
	// Create a test workflow with MCP server configuration
	workflowYAML := []byte(`
name: test-mcp-workflow
version: "1.0"
description: Test workflow with MCP server

mcp_servers:
  - name: echo
    command: echo
    args: ["mock"]
    timeout: 30

steps:
  - id: step1
    type: llm
    prompt: "test"
    inputs:
      message: "Hello from MCP"
`)

	// Track when the server starts
	var serverStarted bool
	var serverStartedCh = make(chan struct{})

	// Create mock MCP manager with echo tool
	mockMgr := mcptesting.NewMockManager()
	mockMgr.AddServer(mcptesting.MockServerConfig{
		Name: "echo",
		Tools: []mcp.ToolDefinition{
			{
				Name:        "echo",
				Description: "Echo back input",
				InputSchema: []byte(`{"type":"object","properties":{"message":{"type":"string"}}}`),
			},
		},
		CallHandler: func(ctx context.Context, req mcp.ToolCallRequest) (*mcp.ToolCallResponse, error) {
			message := "echoed"
			if msg, ok := req.Arguments["message"].(string); ok {
				message = msg
			}
			return &mcp.ToolCallResponse{
				Content: []mcp.ContentItem{
					{
						Type: "text",
						Text: message,
					},
				},
			}, nil
		},
		OnStart: func(config mcp.ServerConfig) error {
			if config.Name == "echo" {
				serverStarted = true
				close(serverStartedCh)
			}
			return nil
		},
	})

	// Create runner with mock manager
	cfg := Config{
		MaxParallel:    1,
		DefaultTimeout: 1 * time.Minute,
	}
	runner := New(cfg, nil, nil, WithMCPManager(mockMgr))

	// Submit workflow
	ctx := context.Background()
	run, err := runner.Submit(ctx, SubmitRequest{
		WorkflowYAML: workflowYAML,
		Inputs:       map[string]any{},
	})
	if err != nil {
		t.Fatalf("Failed to submit workflow: %v", err)
	}

	// Wait for the server to start (hook called)
	select {
	case <-serverStartedCh:
	case <-time.After(5 * time.Second):
		t.Fatal("MCP server was not started within timeout")
	}

	// Verify the server started
	require.True(t, serverStarted, "Expected server start hook to be called")

	// Wait for tools to be registered (this is the key verification)
	require.Eventually(t, func() bool {
		toolNames := runner.ToolRegistry().List()
		for _, name := range toolNames {
			if name == "echo.echo" {
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond, "Expected echo.echo tool to be registered")

	// The test verifies:
	// 1. MCP server was started (serverStarted = true)
	// 2. MCP tools were registered in the tool registry (echo.echo found)
	// This demonstrates successful integration between Runner and MCP system

	// Cancel the workflow since we don't have a real LLM backend
	runner.Cancel(run.ID)

	_ = run // Use run to avoid unused variable error
}

// TestMCPServerLifecycle tests that MCP servers are properly cleaned up after workflow execution.
func TestMCPServerLifecycle(t *testing.T) {
	// Track lifecycle events
	var startCalled, stopCalled bool
	var startCalledCh = make(chan struct{})
	var stopCalledCh = make(chan struct{})

	mockMgr := mcptesting.NewMockManager()
	mockMgr.AddServer(mcptesting.MockServerConfig{
		Name: "test-server",
		OnStart: func(config mcp.ServerConfig) error {
			startCalled = true
			close(startCalledCh)
			return nil
		},
		OnStop: func(name string) error {
			stopCalled = true
			close(stopCalledCh)
			return nil
		},
	})

	cfg := Config{
		MaxParallel:    1,
		DefaultTimeout: 1 * time.Minute,
	}
	runner := New(cfg, nil, nil, WithMCPManager(mockMgr))

	// Start the server
	err := runner.lifecycle.mcpManager.Start(mcp.ServerConfig{
		Name:    "test-server",
		Command: "echo",
		Args:    []string{"test"},
	})
	require.NoError(t, err)

	// Wait for start to be called
	select {
	case <-startCalledCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Start hook was not called")
	}

	// Verify server is running
	require.True(t, mockMgr.IsRunning("test-server"))

	// Get client to verify it's accessible
	client, err := runner.lifecycle.mcpManager.GetClient("test-server")
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, "test-server", client.ServerName())

	// Stop the server
	err = runner.lifecycle.mcpManager.Stop("test-server")
	require.NoError(t, err)

	// Wait for stop to be called
	select {
	case <-stopCalledCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop hook was not called")
	}

	// Verify server is stopped
	require.False(t, mockMgr.IsRunning("test-server"))

	// Verify lifecycle hooks were called
	require.True(t, startCalled, "Start hook should have been called")
	require.True(t, stopCalled, "Stop hook should have been called")

	// Verify GetClient returns error after stop
	_, err = runner.lifecycle.mcpManager.GetClient("test-server")
	require.Error(t, err)
}

// TestMCPServerEnvironmentVariables tests that environment variables are properly passed to MCP servers.
func TestMCPServerEnvironmentVariables(t *testing.T) {
	// Capture the config passed to Start
	var capturedConfig mcp.ServerConfig
	var configCapturedCh = make(chan struct{})

	mockMgr := mcptesting.NewMockManager()
	mockMgr.AddServer(mcptesting.MockServerConfig{
		Name: "test-server",
		OnStart: func(config mcp.ServerConfig) error {
			capturedConfig = config
			close(configCapturedCh)
			return nil
		},
	})

	cfg := Config{
		MaxParallel:    1,
		DefaultTimeout: 1 * time.Minute,
	}
	runner := New(cfg, nil, nil, WithMCPManager(mockMgr))

	// Start server with environment variables
	err := runner.lifecycle.mcpManager.Start(mcp.ServerConfig{
		Name:    "test-server",
		Command: "echo",
		Args:    []string{"test"},
		Env:     []string{"API_KEY=test-key-123", "DEBUG=true"},
	})
	require.NoError(t, err)

	// Wait for config to be captured
	select {
	case <-configCapturedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Start hook was not called")
	}

	// Verify environment variables were passed correctly
	require.Equal(t, "test-server", capturedConfig.Name)
	require.Equal(t, "echo", capturedConfig.Command)
	require.Contains(t, capturedConfig.Env, "API_KEY=test-key-123")
	require.Contains(t, capturedConfig.Env, "DEBUG=true")

	// Clean up
	err = runner.lifecycle.mcpManager.Stop("test-server")
	require.NoError(t, err)
}

// TestMCPMultipleServers tests workflow execution with multiple MCP servers.
func TestMCPMultipleServers(t *testing.T) {
	// Create mock manager with two servers
	mockMgr := mcptesting.NewMockManager()

	// Configure server1 with tool1
	mockMgr.AddServer(mcptesting.MockServerConfig{
		Name: "server1",
		Tools: []mcp.ToolDefinition{
			{
				Name:        "tool1",
				Description: "Tool from server1",
				InputSchema: []byte(`{"type":"object"}`),
			},
		},
		CallHandler: func(ctx context.Context, req mcp.ToolCallRequest) (*mcp.ToolCallResponse, error) {
			return &mcp.ToolCallResponse{
				Content: []mcp.ContentItem{
					{Type: "text", Text: "response from server1"},
				},
			}, nil
		},
	})

	// Configure server2 with tool2
	mockMgr.AddServer(mcptesting.MockServerConfig{
		Name: "server2",
		Tools: []mcp.ToolDefinition{
			{
				Name:        "tool2",
				Description: "Tool from server2",
				InputSchema: []byte(`{"type":"object"}`),
			},
		},
		CallHandler: func(ctx context.Context, req mcp.ToolCallRequest) (*mcp.ToolCallResponse, error) {
			return &mcp.ToolCallResponse{
				Content: []mcp.ContentItem{
					{Type: "text", Text: "response from server2"},
				},
			}, nil
		},
	})

	cfg := Config{
		MaxParallel:    1,
		DefaultTimeout: 1 * time.Minute,
	}
	runner := New(cfg, nil, nil, WithMCPManager(mockMgr))

	// Start both servers
	err := runner.lifecycle.mcpManager.Start(mcp.ServerConfig{
		Name:    "server1",
		Command: "echo",
		Args:    []string{"server1"},
	})
	require.NoError(t, err)

	err = runner.lifecycle.mcpManager.Start(mcp.ServerConfig{
		Name:    "server2",
		Command: "echo",
		Args:    []string{"server2"},
	})
	require.NoError(t, err)

	// Verify both servers are running
	require.True(t, mockMgr.IsRunning("server1"))
	require.True(t, mockMgr.IsRunning("server2"))

	// Verify we have both servers in the list
	servers := mockMgr.ListServers()
	require.Len(t, servers, 2)
	require.Contains(t, servers, "server1")
	require.Contains(t, servers, "server2")

	// Get clients for both servers
	client1, err := mockMgr.GetClient("server1")
	require.NoError(t, err)
	require.Equal(t, "server1", client1.ServerName())

	client2, err := mockMgr.GetClient("server2")
	require.NoError(t, err)
	require.Equal(t, "server2", client2.ServerName())

	// Verify tools are distinct (no state leakage)
	tools1, err := client1.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools1, 1)
	require.Equal(t, "tool1", tools1[0].Name)

	tools2, err := client2.ListTools(context.Background())
	require.NoError(t, err)
	require.Len(t, tools2, 1)
	require.Equal(t, "tool2", tools2[0].Name)

	// Call tools to verify no cross-contamination
	ctx := context.Background()
	resp1, err := client1.CallTool(ctx, mcp.ToolCallRequest{Name: "tool1", Arguments: map[string]interface{}{}})
	require.NoError(t, err)
	require.Contains(t, resp1.Content[0].Text, "server1")

	resp2, err := client2.CallTool(ctx, mcp.ToolCallRequest{Name: "tool2", Arguments: map[string]interface{}{}})
	require.NoError(t, err)
	require.Contains(t, resp2.Content[0].Text, "server2")

	// Clean up
	err = runner.lifecycle.mcpManager.Stop("server1")
	require.NoError(t, err)
	err = runner.lifecycle.mcpManager.Stop("server2")
	require.NoError(t, err)

	// Verify both servers are stopped
	require.False(t, mockMgr.IsRunning("server1"))
	require.False(t, mockMgr.IsRunning("server2"))
}
