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

	"github.com/tombee/conductor/internal/operation"
)

// OperationRegistry defines the interface for executing operations
// This allows for testing with mock implementations
type OperationRegistry interface {
	Execute(ctx context.Context, reference string, inputs map[string]interface{}) (*operation.Result, error)
	List() []string
}

// ClaudeResponse represents the JSON response from Claude CLI
type ClaudeResponse struct {
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Model      string         `json:"model,omitempty"`
}

// ContentBlock represents a single content block in Claude's response
type ContentBlock struct {
	Type  string          `json:"type"` // "text" or "tool_use"
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`    // tool_use_id
	Name  string          `json:"name,omitempty"`  // tool name
	Input json.RawMessage `json:"input,omitempty"` // tool inputs as raw JSON
}

// ToolCall represents a parsed tool call from Claude's response
type ToolCall struct {
	ID    string          // tool_use_id for correlation
	Name  string          // tool name (e.g., "file.read")
	Input json.RawMessage // tool input parameters as raw JSON
}

// ToolResult represents the result of executing a tool
type ToolResult struct {
	ID      string // tool_use_id to correlate with the call
	Content string // result content or error message
	IsError bool   // whether this is an error result
}
