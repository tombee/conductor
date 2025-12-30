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
	"encoding/json"
	"fmt"
	"strings"
)

// parseToolCalls parses Claude's JSON response and extracts tool calls and text content
func (p *Provider) parseToolCalls(response []byte) ([]ToolCall, string, error) {
	var resp ClaudeResponse
	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, "", fmt.Errorf("failed to parse Claude response as JSON: %w", err)
	}

	var toolCalls []ToolCall
	var textContent strings.Builder

	for _, block := range resp.Content {
		switch block.Type {
		case "tool_use":
			toolCalls = append(toolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		case "text":
			if textContent.Len() > 0 {
				textContent.WriteString("\n")
			}
			textContent.WriteString(block.Text)
		}
	}

	return toolCalls, textContent.String(), nil
}
