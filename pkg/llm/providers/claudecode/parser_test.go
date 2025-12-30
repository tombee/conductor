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
	"testing"
)

func TestParseToolCalls_SingleTool(t *testing.T) {
	p := New()

	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_123",
				"name": "file.read",
				"input": {"path": "/tmp/test.txt"}
			}
		],
		"stop_reason": "tool_use"
	}`

	toolCalls, textContent, err := p.parseToolCalls([]byte(response))
	if err != nil {
		t.Fatalf("parseToolCalls failed: %v", err)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].ID != "toolu_123" {
		t.Errorf("expected tool call ID 'toolu_123', got %q", toolCalls[0].ID)
	}

	if toolCalls[0].Name != "file.read" {
		t.Errorf("expected tool name 'file.read', got %q", toolCalls[0].Name)
	}

	var input map[string]interface{}
	if err := json.Unmarshal(toolCalls[0].Input, &input); err != nil {
		t.Fatalf("failed to parse tool input: %v", err)
	}

	if input["path"] != "/tmp/test.txt" {
		t.Errorf("expected path '/tmp/test.txt', got %v", input["path"])
	}

	if textContent != "" {
		t.Errorf("expected empty text content, got %q", textContent)
	}
}

func TestParseToolCalls_MultipleTools(t *testing.T) {
	p := New()

	response := `{
		"content": [
			{
				"type": "tool_use",
				"id": "toolu_123",
				"name": "file.read",
				"input": {"path": "/tmp/test.txt"}
			},
			{
				"type": "tool_use",
				"id": "toolu_456",
				"name": "shell.run",
				"input": {"command": "ls -la"}
			}
		],
		"stop_reason": "tool_use"
	}`

	toolCalls, textContent, err := p.parseToolCalls([]byte(response))
	if err != nil {
		t.Fatalf("parseToolCalls failed: %v", err)
	}

	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "file.read" {
		t.Errorf("expected first tool 'file.read', got %q", toolCalls[0].Name)
	}

	if toolCalls[1].Name != "shell.run" {
		t.Errorf("expected second tool 'shell.run', got %q", toolCalls[1].Name)
	}

	if textContent != "" {
		t.Errorf("expected empty text content, got %q", textContent)
	}
}

func TestParseToolCalls_NoTools(t *testing.T) {
	p := New()

	response := `{
		"content": [
			{
				"type": "text",
				"text": "This is a text response without any tool calls."
			}
		],
		"stop_reason": "end_turn"
	}`

	toolCalls, textContent, err := p.parseToolCalls([]byte(response))
	if err != nil {
		t.Fatalf("parseToolCalls failed: %v", err)
	}

	if len(toolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
	}

	expectedText := "This is a text response without any tool calls."
	if textContent != expectedText {
		t.Errorf("expected text %q, got %q", expectedText, textContent)
	}
}

func TestParseToolCalls_MixedContent(t *testing.T) {
	p := New()

	response := `{
		"content": [
			{
				"type": "text",
				"text": "Let me check that file for you."
			},
			{
				"type": "tool_use",
				"id": "toolu_789",
				"name": "file.read",
				"input": {"path": "/etc/hosts"}
			}
		],
		"stop_reason": "tool_use"
	}`

	toolCalls, textContent, err := p.parseToolCalls([]byte(response))
	if err != nil {
		t.Fatalf("parseToolCalls failed: %v", err)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}

	if toolCalls[0].Name != "file.read" {
		t.Errorf("expected tool 'file.read', got %q", toolCalls[0].Name)
	}

	expectedText := "Let me check that file for you."
	if textContent != expectedText {
		t.Errorf("expected text %q, got %q", expectedText, textContent)
	}
}

func TestParseToolCalls_MultipleTextBlocks(t *testing.T) {
	p := New()

	response := `{
		"content": [
			{
				"type": "text",
				"text": "First line."
			},
			{
				"type": "text",
				"text": "Second line."
			}
		],
		"stop_reason": "end_turn"
	}`

	toolCalls, textContent, err := p.parseToolCalls([]byte(response))
	if err != nil {
		t.Fatalf("parseToolCalls failed: %v", err)
	}

	if len(toolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
	}

	expectedText := "First line.\nSecond line."
	if textContent != expectedText {
		t.Errorf("expected text %q, got %q", expectedText, textContent)
	}
}

func TestParseToolCalls_InvalidJSON(t *testing.T) {
	p := New()

	response := `{invalid json`

	_, _, err := p.parseToolCalls([]byte(response))
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}

	if err != nil && !contains(err.Error(), "parse") {
		t.Errorf("expected parse error message, got: %v", err)
	}
}

func TestParseToolCalls_EmptyResponse(t *testing.T) {
	p := New()

	response := `{
		"content": [],
		"stop_reason": "end_turn"
	}`

	toolCalls, textContent, err := p.parseToolCalls([]byte(response))
	if err != nil {
		t.Fatalf("parseToolCalls failed: %v", err)
	}

	if len(toolCalls) != 0 {
		t.Errorf("expected 0 tool calls, got %d", len(toolCalls))
	}

	if textContent != "" {
		t.Errorf("expected empty text content, got %q", textContent)
	}
}
