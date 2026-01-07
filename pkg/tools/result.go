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

// ToolResult wraps tool execution results.
// This type is currently unused and exists for future extensibility.
type ToolResult struct {
	// Output contains the tool output data
	Output map[string]interface{}

	// Text contains extracted text result (if applicable)
	Text string

	// Error contains error information (if applicable)
	Error string
}

// NewToolResult creates a ToolResult from a map output.
func NewToolResult(rawOutput map[string]interface{}) ToolResult {
	result := ToolResult{
		Output: rawOutput,
	}

	// Extract common fields from raw output
	if text, ok := rawOutput["text"].(string); ok {
		result.Text = text
	} else if textResult, ok := rawOutput["result"].(string); ok {
		result.Text = textResult
	} else if response, ok := rawOutput["response"].(string); ok {
		result.Text = response
	}

	if err, ok := rawOutput["error"].(string); ok {
		result.Error = err
	}

	return result
}
