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
	"errors"
	"testing"

	"github.com/tombee/conductor/internal/daemon/checkpoint"
	"github.com/tombee/conductor/internal/mcp"
	mcptesting "github.com/tombee/conductor/internal/mcp/testing"
	"github.com/tombee/conductor/pkg/tools"
	"github.com/tombee/conductor/pkg/workflow"
)

func TestNewLifecycleManager(t *testing.T) {
	lm := NewLifecycleManager(nil, nil, nil)
	if lm == nil {
		t.Fatal("expected non-nil LifecycleManager")
	}
	if lm.mcpManager == nil {
		t.Error("expected default MCP manager to be set")
	}
	if lm.toolRegistry == nil {
		t.Error("expected default tool registry to be set")
	}
}

func TestNewLifecycleManager_WithProvided(t *testing.T) {
	mockMCP := mcptesting.NewMockManager()
	tr := tools.NewRegistry()

	lm := NewLifecycleManager(mockMCP, nil, tr)

	if lm.mcpManager == nil {
		t.Error("expected MCP manager to be set")
	}
	if lm.toolRegistry != tr {
		t.Error("expected provided tool registry")
	}
}

func TestLifecycleManager_ToolRegistry(t *testing.T) {
	tr := tools.NewRegistry()
	lm := NewLifecycleManager(nil, nil, tr)

	if lm.ToolRegistry() != tr {
		t.Error("expected same tool registry")
	}
}

func TestLifecycleManager_MCPManager(t *testing.T) {
	mockMCP := mcptesting.NewMockManager()
	lm := NewLifecycleManager(mockMCP, nil, nil)

	if lm.MCPManager() == nil {
		t.Error("expected MCP manager to be set")
	}
}

func TestLifecycleManager_StartMCPServers_NoServers(t *testing.T) {
	lm := NewLifecycleManager(nil, nil, nil)

	def := &workflow.Definition{Name: "test"}
	names, err := lm.StartMCPServers(context.Background(), def, nil)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected no server names, got %d", len(names))
	}
}

func TestLifecycleManager_StartMCPServers_Success(t *testing.T) {
	mockMCP := mcptesting.NewMockManager()
	mockMCP.AddServer(mcptesting.MockServerConfig{
		Name: "test-server",
		Tools: []mcp.ToolDefinition{
			{Name: "tool1"},
			{Name: "tool2"},
		},
	})

	lm := NewLifecycleManager(mockMCP, nil, nil)

	def := &workflow.Definition{
		Name: "test",
		MCPServers: []workflow.MCPServerConfig{
			{Name: "test-server", Command: "echo"},
		},
	}

	var logs []string
	logFn := func(level, message, stepID string) {
		logs = append(logs, message)
	}

	names, err := lm.StartMCPServers(context.Background(), def, logFn)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 || names[0] != "test-server" {
		t.Errorf("expected ['test-server'], got %v", names)
	}

	// Verify logs were captured
	if len(logs) == 0 {
		t.Error("expected log messages")
	}
}

func TestLifecycleManager_StartMCPServers_StartError(t *testing.T) {
	mockMCP := mcptesting.NewMockManager()
	mockMCP.AddServer(mcptesting.MockServerConfig{
		Name:       "test-server",
		StartError: errors.New("start failed"),
	})

	lm := NewLifecycleManager(mockMCP, nil, nil)

	def := &workflow.Definition{
		Name: "test",
		MCPServers: []workflow.MCPServerConfig{
			{Name: "test-server", Command: "echo"},
		},
	}

	_, err := lm.StartMCPServers(context.Background(), def, nil)

	if err == nil {
		t.Error("expected error")
	}
}

func TestLifecycleManager_StopMCPServers(t *testing.T) {
	mockMCP := mcptesting.NewMockManager()
	mockMCP.AddServer(mcptesting.MockServerConfig{Name: "server1"})
	mockMCP.AddServer(mcptesting.MockServerConfig{Name: "server2"})
	// Start the servers first
	_ = mockMCP.Start(mcp.ServerConfig{Name: "server1"})
	_ = mockMCP.Start(mcp.ServerConfig{Name: "server2"})

	lm := NewLifecycleManager(mockMCP, nil, nil)

	var logs []string
	logFn := func(level, message, stepID string) {
		logs = append(logs, level+": "+message)
	}

	lm.StopMCPServers([]string{"server1", "server2"}, logFn)

	// Verify servers were stopped
	if mockMCP.IsRunning("server1") {
		t.Error("expected server1 to be stopped")
	}
	if mockMCP.IsRunning("server2") {
		t.Error("expected server2 to be stopped")
	}
}

func TestLifecycleManager_StopMCPServers_WithError(t *testing.T) {
	mockMCP := mcptesting.NewMockManager()
	mockMCP.WithOnStop(func(name string) error {
		return errors.New("stop failed")
	})
	// Start a server first
	_ = mockMCP.Start(mcp.ServerConfig{Name: "server1"})

	lm := NewLifecycleManager(mockMCP, nil, nil)

	var logs []string
	logFn := func(level, message, stepID string) {
		logs = append(logs, level+": "+message)
	}

	// Should not panic, just log warning
	lm.StopMCPServers([]string{"server1"}, logFn)

	// Verify warning was logged
	found := false
	for _, log := range logs {
		if len(log) >= 4 && log[:4] == "warn" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning log for stop error")
	}
}

func TestLifecycleManager_SaveCheckpoint_NilManager(t *testing.T) {
	lm := NewLifecycleManager(nil, nil, nil)

	run := &Run{ID: "test-run", Progress: &Progress{CurrentStep: "step1"}}
	err := lm.SaveCheckpoint(context.Background(), run, 0, nil)

	if err != nil {
		t.Errorf("expected no error for nil checkpoint manager, got %v", err)
	}
}

func TestLifecycleManager_CleanupCheckpoint_NilManager(t *testing.T) {
	lm := NewLifecycleManager(nil, nil, nil)

	err := lm.CleanupCheckpoint(context.Background(), "test-run")

	if err != nil {
		t.Errorf("expected no error for nil checkpoint manager, got %v", err)
	}
}

func TestLifecycleManager_ResumeInterrupted_NilManager(t *testing.T) {
	lm := NewLifecycleManager(nil, nil, nil)

	err := lm.ResumeInterrupted(context.Background())

	if err != nil {
		t.Errorf("expected no error for nil checkpoint manager, got %v", err)
	}
}

func TestLifecycleManager_StartMCPServers_WithLogFunc(t *testing.T) {
	mockMCP := mcptesting.NewMockManager()
	mockMCP.AddServer(mcptesting.MockServerConfig{
		Name:  "test-server",
		Tools: []mcp.ToolDefinition{{Name: "test-tool"}},
	})

	lm := NewLifecycleManager(mockMCP, nil, nil)

	def := &workflow.Definition{
		Name: "test",
		MCPServers: []workflow.MCPServerConfig{
			{Name: "test-server", Command: "echo"},
		},
	}

	var logs []struct{ level, message, stepID string }
	logFn := func(level, message, stepID string) {
		logs = append(logs, struct{ level, message, stepID string }{level, message, stepID})
	}

	_, err := lm.StartMCPServers(context.Background(), def, logFn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have at least some info logs
	hasInfoLog := false
	for _, log := range logs {
		if log.level == "info" {
			hasInfoLog = true
			break
		}
	}
	if !hasInfoLog {
		t.Error("expected at least one info log")
	}
}

func TestLifecycleManager_StartMCPServers_NilLogFunc(t *testing.T) {
	mockMCP := mcptesting.NewMockManager()
	mockMCP.AddServer(mcptesting.MockServerConfig{
		Name:  "test-server",
		Tools: []mcp.ToolDefinition{{Name: "test-tool"}},
	})

	lm := NewLifecycleManager(mockMCP, nil, nil)

	def := &workflow.Definition{
		Name: "test",
		MCPServers: []workflow.MCPServerConfig{
			{Name: "test-server", Command: "echo"},
		},
	}

	// Should not panic with nil logFn
	names, err := lm.StartMCPServers(context.Background(), def, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 1 {
		t.Errorf("expected 1 server, got %d", len(names))
	}
}

func TestLifecycleManager_SaveCheckpoint_WithManager(t *testing.T) {
	// Create a checkpoint manager with a temp directory
	tmpDir := t.TempDir()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{Dir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create checkpoint manager: %v", err)
	}

	lm := NewLifecycleManager(nil, cm, nil)

	run := &Run{
		ID:         "test-run",
		WorkflowID: "test-workflow",
		Progress: &Progress{
			CurrentStep: "step1",
		},
	}
	workflowCtx := map[string]any{
		"inputs": map[string]any{"key": "value"},
		"step1":  "output1",
	}

	err = lm.SaveCheckpoint(context.Background(), run, 0, workflowCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify checkpoint was saved
	cp, err := cm.Load(context.Background(), "test-run")
	if err != nil {
		t.Errorf("failed to load checkpoint: %v", err)
	}
	if cp == nil {
		t.Error("expected checkpoint to be saved")
	}
	if cp.RunID != "test-run" {
		t.Errorf("expected run ID 'test-run', got %s", cp.RunID)
	}
	if cp.StepID != "step1" {
		t.Errorf("expected step ID 'step1', got %s", cp.StepID)
	}
}

func TestLifecycleManager_SaveCheckpoint_DisabledManager(t *testing.T) {
	// Create a checkpoint manager that's disabled (empty path)
	cm, _ := checkpoint.NewManager(checkpoint.ManagerConfig{Dir: ""})

	lm := NewLifecycleManager(nil, cm, nil)

	run := &Run{
		ID:         "test-run",
		WorkflowID: "test-workflow",
		Progress:   &Progress{CurrentStep: "step1"},
	}

	// Should return nil without error
	err := lm.SaveCheckpoint(context.Background(), run, 0, nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLifecycleManager_ResumeInterrupted_WithManager(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{Dir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create checkpoint manager: %v", err)
	}

	// Save a checkpoint first
	cp := &checkpoint.Checkpoint{
		RunID:      "interrupted-run",
		WorkflowID: "test-workflow",
		StepID:     "step1",
		StepIndex:  1,
	}
	_ = cm.Save(context.Background(), cp)

	lm := NewLifecycleManager(nil, cm, nil)

	// Should not error - just lists and tries to resume
	err = lm.ResumeInterrupted(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLifecycleManager_CleanupCheckpoint_WithManager(t *testing.T) {
	tmpDir := t.TempDir()
	cm, err := checkpoint.NewManager(checkpoint.ManagerConfig{Dir: tmpDir})
	if err != nil {
		t.Fatalf("failed to create checkpoint manager: %v", err)
	}

	// Save a checkpoint first
	cp := &checkpoint.Checkpoint{
		RunID:      "cleanup-run",
		WorkflowID: "test-workflow",
		StepID:     "step1",
	}
	_ = cm.Save(context.Background(), cp)

	lm := NewLifecycleManager(nil, cm, nil)

	err = lm.CleanupCheckpoint(context.Background(), "cleanup-run")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify checkpoint was deleted
	loaded, _ := cm.Load(context.Background(), "cleanup-run")
	if loaded != nil {
		t.Error("expected checkpoint to be deleted")
	}
}
