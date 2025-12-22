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
	"encoding/json"
	"testing"

	"github.com/tombee/conductor/pkg/agent"
	"github.com/tombee/conductor/pkg/tools"
)

// Helper function to create test messages
func agentTestMessage(method string, params interface{}) *Message {
	paramsJSON, _ := json.Marshal(params)
	return &Message{
		Type:          MessageTypeRequest,
		CorrelationID: "test-" + method,
		Method:        method,
		Params:        paramsJSON,
	}
}

// mockLLMProvider is a mock LLM provider for testing.
type mockLLMProvider struct {
	completeFunc func(ctx context.Context, messages []agent.Message) (*agent.Response, error)
	streamFunc   func(ctx context.Context, messages []agent.Message) (<-chan agent.StreamEvent, error)
}

func (m *mockLLMProvider) Complete(ctx context.Context, messages []agent.Message) (*agent.Response, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, messages)
	}
	return &agent.Response{
		Content:      "Mock response",
		FinishReason: "stop",
		Usage: agent.TokenUsage{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	}, nil
}

func (m *mockLLMProvider) Stream(ctx context.Context, messages []agent.Message) (<-chan agent.StreamEvent, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, messages)
	}
	ch := make(chan agent.StreamEvent)
	close(ch)
	return ch, nil
}

func TestAgentHandlers_Run(t *testing.T) {
	llm := &mockLLMProvider{}
	registry := tools.NewRegistry()
	handlers := NewAgentHandlers(llm, registry)

	tests := []struct {
		name      string
		params    interface{}
		wantError bool
	}{
		{
			name: "valid run",
			params: map[string]interface{}{
				"system_prompt": "You are a helpful assistant",
				"user_prompt":   "Hello, world!",
			},
			wantError: false,
		},
		{
			name: "with max iterations",
			params: map[string]interface{}{
				"system_prompt":  "You are a helpful assistant",
				"user_prompt":    "Hello, world!",
				"max_iterations": 10,
			},
			wantError: false,
		},
		{
			name: "missing system prompt",
			params: map[string]interface{}{
				"user_prompt": "Hello, world!",
			},
			wantError: true,
		},
		{
			name: "missing user prompt",
			params: map[string]interface{}{
				"system_prompt": "You are a helpful assistant",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := agentTestMessage("agent.run", tt.params)
			resp, err := handlers.handleRun(context.Background(), req)
			if (err != nil) != tt.wantError {
				t.Errorf("handleRun() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && resp == nil {
				t.Error("handleRun() returned nil response")
			}
		})
	}
}

func TestAgentHandlers_Register(t *testing.T) {
	llm := &mockLLMProvider{}
	registry := tools.NewRegistry()
	handlers := NewAgentHandlers(llm, registry)
	rpcRegistry := NewRegistry()

	handlers.Register(rpcRegistry)

	if !rpcRegistry.HasMethod("agent.run") {
		t.Error("agent.run not registered")
	}
	if !rpcRegistry.HasMethod("agent.stream") {
		t.Error("agent.stream not registered")
	}
}
