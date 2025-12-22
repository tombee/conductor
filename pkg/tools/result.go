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

package tools

import "github.com/tombee/conductor/pkg/workflow"

// ToolResult wraps tool execution results with both typed and legacy formats.
// This struct facilitates the migration from map[string]interface{} to StepOutput (SPEC-40).
type ToolResult struct {
	// Output contains the typed tool output (new format)
	Output workflow.StepOutput

	// RawOutput contains the legacy map format for backward compatibility
	RawOutput map[string]interface{}
}

// NewToolResult creates a ToolResult from a legacy map output.
// This helper converts old-style tool results to the new typed format.
func NewToolResult(rawOutput map[string]interface{}) ToolResult {
	output := workflow.StepOutput{
		Data: rawOutput,
	}

	// Extract common fields from raw output
	if text, ok := rawOutput["text"].(string); ok {
		output.Text = text
	} else if result, ok := rawOutput["result"].(string); ok {
		output.Text = result
	} else if response, ok := rawOutput["response"].(string); ok {
		output.Text = response
	}

	if err, ok := rawOutput["error"].(string); ok {
		output.Error = err
	}

	return ToolResult{
		Output:    output,
		RawOutput: rawOutput,
	}
}

// ToMap returns the legacy map format for backward compatibility.
func (r ToolResult) ToMap() map[string]interface{} {
	if r.RawOutput != nil {
		return r.RawOutput
	}

	// Convert StepOutput back to map if RawOutput is not available
	result := make(map[string]interface{})
	if r.Output.Text != "" {
		result["text"] = r.Output.Text
	}
	if r.Output.Error != "" {
		result["error"] = r.Output.Error
	}
	if dataMap, ok := r.Output.Data.(map[string]interface{}); ok {
		for k, v := range dataMap {
			result[k] = v
		}
	} else if r.Output.Data != nil {
		result["data"] = r.Output.Data
	}

	return result
}
