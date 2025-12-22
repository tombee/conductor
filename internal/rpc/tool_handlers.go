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

package rpc

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/pkg/tools"
)

// ToolHandlers provides RPC handlers for tool operations.
type ToolHandlers struct {
	registry *tools.Registry
}

// NewToolHandlers creates a new set of tool RPC handlers.
func NewToolHandlers(registry *tools.Registry) *ToolHandlers {
	return &ToolHandlers{
		registry: registry,
	}
}

// Register registers all tool handlers with the registry.
func (h *ToolHandlers) Register(rpcRegistry *Registry) {
	rpcRegistry.Register("tool.list", h.handleList)
	rpcRegistry.Register("tool.execute", h.handleExecute)
	rpcRegistry.Register("tool.get", h.handleGet)
}

// handleList lists all available tools.
func (h *ToolHandlers) handleList(ctx context.Context, req *Message) (*Message, error) {
	descriptors := h.registry.GetToolDescriptors()

	return NewResponse(req.CorrelationID, map[string]interface{}{
		"tools": descriptors,
		"count": len(descriptors),
	})
}

// GetRequest is the request payload for tool.get.
type GetToolRequest struct {
	Name string `json:"name"`
}

// handleGet retrieves information about a specific tool.
func (h *ToolHandlers) handleGet(ctx context.Context, req *Message) (*Message, error) {
	var getReq GetToolRequest
	if err := req.UnmarshalParams(&getReq); err != nil {
		return nil, fmt.Errorf("invalid get request: %w", err)
	}

	if getReq.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	tool, err := h.registry.Get(getReq.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool: %w", err)
	}

	return NewResponse(req.CorrelationID, map[string]interface{}{
		"name":        tool.Name(),
		"description": tool.Description(),
		"schema":      tool.Schema(),
	})
}

// ExecuteRequest is the request payload for tool.execute.
type ExecuteRequest struct {
	Name   string                 `json:"name"`
	Inputs map[string]interface{} `json:"inputs"`
}

// handleExecute executes a tool with the given inputs.
func (h *ToolHandlers) handleExecute(ctx context.Context, req *Message) (*Message, error) {
	var execReq ExecuteRequest
	if err := req.UnmarshalParams(&execReq); err != nil {
		return nil, fmt.Errorf("invalid execute request: %w", err)
	}

	if execReq.Name == "" {
		return nil, fmt.Errorf("tool name is required")
	}
	if execReq.Inputs == nil {
		execReq.Inputs = make(map[string]interface{})
	}

	outputs, err := h.registry.Execute(ctx, execReq.Name, execReq.Inputs)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	return NewResponse(req.CorrelationID, map[string]interface{}{
		"tool":    execReq.Name,
		"outputs": outputs,
		"success": true,
	})
}
