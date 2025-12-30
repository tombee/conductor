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

package claudecode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// executeTools executes a list of tool calls via the operation registry
func (p *Provider) executeTools(ctx context.Context, calls []ToolCall) []ToolResult {
	if p.registry == nil {
		return p.makeErrorResults(calls, "tool execution not available: operation registry not configured")
	}

	results := make([]ToolResult, len(calls))

	for i, call := range calls {
		result, err := p.executeSingleTool(ctx, call)
		if err != nil {
			results[i] = ToolResult{
				ID:      call.ID,
				Content: sanitizeError(err.Error()),
				IsError: true,
			}
		} else {
			results[i] = *result
		}
	}

	return results
}

// executeSingleTool executes a single tool call
func (p *Provider) executeSingleTool(ctx context.Context, call ToolCall) (*ToolResult, error) {
	// Parse inputs from raw JSON
	var inputs map[string]interface{}
	if err := json.Unmarshal(call.Input, &inputs); err != nil {
		return nil, fmt.Errorf("invalid tool input format: %w", err)
	}

	// Execute via operation registry
	opResult, err := p.registry.Execute(ctx, call.Name, inputs)
	if err != nil {
		// Check if it's an unknown operation error
		if strings.Contains(err.Error(), "not found") {
			availableOps := p.registry.List()
			return nil, fmt.Errorf("unknown tool: %s. Available tools: %s", call.Name, strings.Join(availableOps, ", "))
		}
		return nil, err
	}

	// Format the result
	content := formatOperationResult(opResult.Response)
	return &ToolResult{
		ID:      call.ID,
		Content: content,
		IsError: false,
	}, nil
}

// makeErrorResults creates error results for all tool calls with the same error message
func (p *Provider) makeErrorResults(calls []ToolCall, errMsg string) []ToolResult {
	results := make([]ToolResult, len(calls))
	sanitized := sanitizeError(errMsg)
	for i, call := range calls {
		results[i] = ToolResult{
			ID:      call.ID,
			Content: sanitized,
			IsError: true,
		}
	}
	return results
}

// formatOperationResult formats an operation result as a string for Claude
func formatOperationResult(result interface{}) string {
	if result == nil {
		return "Operation completed successfully"
	}

	// If it's already a string, return it
	if str, ok := result.(*string); ok && str != nil {
		return *str
	}
	if str, ok := result.(string); ok {
		return str
	}

	// Otherwise, marshal to JSON for structured output
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Result: %v", result)
	}
	return string(data)
}
