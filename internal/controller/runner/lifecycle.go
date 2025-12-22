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
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/controller/checkpoint"
	"github.com/tombee/conductor/internal/mcp"
	"github.com/tombee/conductor/pkg/tools"
	"github.com/tombee/conductor/pkg/workflow"
)

// LifecycleManager handles MCP server lifecycle and checkpoint management.
type LifecycleManager struct {
	mcpManager   mcp.MCPManagerProvider
	checkpoints  *checkpoint.Manager
	toolRegistry *tools.Registry
}

// NewLifecycleManager creates a new LifecycleManager.
func NewLifecycleManager(mcpManager mcp.MCPManagerProvider, cm *checkpoint.Manager, tr *tools.Registry) *LifecycleManager {
	if mcpManager == nil {
		mcpManager = mcp.NewManager(mcp.ManagerConfig{})
	}
	if tr == nil {
		tr = tools.NewRegistry()
	}
	return &LifecycleManager{
		mcpManager:   mcpManager,
		checkpoints:  cm,
		toolRegistry: tr,
	}
}

// ToolRegistry returns the lifecycle manager's tool registry.
func (l *LifecycleManager) ToolRegistry() *tools.Registry {
	return l.toolRegistry
}

// MCPManager returns the MCP manager.
func (l *LifecycleManager) MCPManager() mcp.MCPManagerProvider {
	return l.mcpManager
}

// LogFunc is a callback for logging during lifecycle operations.
type LogFunc func(level, message, stepID string)

// StartMCPServers starts all MCP servers defined in the workflow.
// Returns the list of started server names for later cleanup.
func (l *LifecycleManager) StartMCPServers(ctx context.Context, def *workflow.Definition, logFn LogFunc) ([]string, error) {
	if len(def.MCPServers) == 0 {
		return nil, nil
	}

	if logFn != nil {
		logFn("info", fmt.Sprintf("Starting %d MCP server(s)", len(def.MCPServers)), "")
	}

	var serverNames []string
	for _, mcpServerDef := range def.MCPServers {
		if logFn != nil {
			logFn("info", fmt.Sprintf("Starting MCP server: %s", mcpServerDef.Name), "")
		}

		// Convert workflow MCPServerConfig to mcp.ServerConfig
		serverConfig := mcp.ServerConfig{
			Name:    mcpServerDef.Name,
			Command: mcpServerDef.Command,
			Args:    mcpServerDef.Args,
			Env:     mcpServerDef.Env,
			Timeout: time.Duration(mcpServerDef.Timeout) * time.Second,
		}

		// Start the server
		if err := l.mcpManager.Start(serverConfig); err != nil {
			return serverNames, fmt.Errorf("failed to start MCP server %s: %w", mcpServerDef.Name, err)
		}

		serverNames = append(serverNames, mcpServerDef.Name)

		// Wait for the server to be ready and register its tools
		if err := l.registerMCPTools(ctx, mcpServerDef.Name, logFn); err != nil {
			return serverNames, fmt.Errorf("failed to register tools for MCP server %s: %w", mcpServerDef.Name, err)
		}

		if logFn != nil {
			logFn("info", fmt.Sprintf("MCP server started: %s", mcpServerDef.Name), "")
		}
	}

	return serverNames, nil
}

// StopMCPServers stops the specified MCP servers.
func (l *LifecycleManager) StopMCPServers(serverNames []string, logFn LogFunc) {
	for _, serverName := range serverNames {
		if err := l.mcpManager.Stop(serverName); err != nil {
			if logFn != nil {
				logFn("warn", fmt.Sprintf("Failed to stop MCP server %s: %v", serverName, err), "")
			}
		}
	}
}

// registerMCPTools discovers and registers tools from an MCP server.
func (l *LifecycleManager) registerMCPTools(ctx context.Context, serverName string, logFn LogFunc) error {
	// Get the MCP client for this server
	// We need to wait a bit for the server to initialize
	var client mcp.ClientProvider
	var err error

	// Retry for up to 10 seconds
	maxAttempts := 20
	for attempt := 0; attempt < maxAttempts; attempt++ {
		client, err = l.mcpManager.GetClient(serverName)
		if err == nil {
			break
		}
		if attempt < maxAttempts-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to get client for server %s: %w", serverName, err)
	}

	// List available tools from the MCP server
	listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	toolDefs, err := client.ListTools(listCtx)
	if err != nil {
		return fmt.Errorf("failed to list tools from server %s: %w", serverName, err)
	}

	if logFn != nil {
		logFn("info", fmt.Sprintf("Registering %d tool(s) from MCP server %s", len(toolDefs), serverName), "")
	}

	// Register each tool in the registry
	for _, toolDef := range toolDefs {
		mcpTool := mcp.NewMCPTool(serverName, toolDef, client)
		l.toolRegistry.Register(mcpTool)
		if logFn != nil {
			logFn("debug", fmt.Sprintf("Registered MCP tool: %s", mcpTool.Name()), "")
		}
	}

	return nil
}

// SaveCheckpoint saves a checkpoint for the current execution state.
func (l *LifecycleManager) SaveCheckpoint(ctx context.Context, run *Run, stepIndex int, workflowCtx map[string]any) error {
	if l.checkpoints == nil || !l.checkpoints.Enabled() {
		return nil
	}

	cp := &checkpoint.Checkpoint{
		RunID:       run.ID,
		WorkflowID:  run.WorkflowID,
		StepID:      run.Progress.CurrentStep,
		StepIndex:   stepIndex,
		Context:     workflowCtx,
		StepOutputs: make(map[string]any),
	}

	// Copy step outputs
	for k, v := range workflowCtx {
		if k != "inputs" {
			cp.StepOutputs[k] = v
		}
	}

	return l.checkpoints.Save(ctx, cp)
}

// CleanupCheckpoint removes checkpoint on successful completion.
func (l *LifecycleManager) CleanupCheckpoint(ctx context.Context, runID string) error {
	if l.checkpoints == nil || !l.checkpoints.Enabled() {
		return nil
	}
	return l.checkpoints.Delete(ctx, runID)
}

// ResumeInterrupted attempts to resume any interrupted runs from checkpoints.
func (l *LifecycleManager) ResumeInterrupted(ctx context.Context) error {
	if l.checkpoints == nil || !l.checkpoints.Enabled() {
		return nil
	}

	runIDs, err := l.checkpoints.ListInterrupted(ctx)
	if err != nil {
		return fmt.Errorf("failed to list interrupted runs: %w", err)
	}

	for _, runID := range runIDs {
		cp, err := l.checkpoints.Load(ctx, runID)
		if err != nil {
			// Log and continue
			continue
		}
		if cp == nil {
			continue
		}

		// Resume from checkpoint not yet implemented.
		// Future: reload workflow definition and continue execution from saved state.
		_ = cp
	}

	return nil
}
