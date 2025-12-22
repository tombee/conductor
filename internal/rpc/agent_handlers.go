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

	"github.com/tombee/conductor/pkg/agent"
	"github.com/tombee/conductor/pkg/tools"
)

// AgentHandlers provides RPC handlers for agent operations.
type AgentHandlers struct {
	llmProvider agent.LLMProvider
	registry    *tools.Registry
}

// NewAgentHandlers creates a new set of agent RPC handlers.
func NewAgentHandlers(llmProvider agent.LLMProvider, registry *tools.Registry) *AgentHandlers {
	return &AgentHandlers{
		llmProvider: llmProvider,
		registry:    registry,
	}
}

// Register registers all agent handlers with the registry.
func (h *AgentHandlers) Register(rpcRegistry *Registry) {
	rpcRegistry.Register("agent.run", h.handleRun)
	rpcRegistry.RegisterStream("agent.stream", h.handleStream)
}

// RunRequest is the request payload for agent.run.
type RunRequest struct {
	SystemPrompt   string `json:"system_prompt"`
	UserPrompt     string `json:"user_prompt"`
	MaxIterations  int    `json:"max_iterations,omitempty"`
}

// handleRun executes an agent synchronously and returns the final result.
func (h *AgentHandlers) handleRun(ctx context.Context, req *Message) (*Message, error) {
	var runReq RunRequest
	if err := req.UnmarshalParams(&runReq); err != nil {
		return nil, fmt.Errorf("invalid run request: %w", err)
	}

	if runReq.SystemPrompt == "" {
		return nil, fmt.Errorf("system_prompt is required")
	}
	if runReq.UserPrompt == "" {
		return nil, fmt.Errorf("user_prompt is required")
	}

	// Create agent
	ag := agent.NewAgent(h.llmProvider, h.registry)
	if runReq.MaxIterations > 0 {
		ag = ag.WithMaxIterations(runReq.MaxIterations)
	}

	// Run agent
	result, err := ag.Run(ctx, runReq.SystemPrompt, runReq.UserPrompt)
	if err != nil {
		return nil, fmt.Errorf("agent execution failed: %w", err)
	}

	return NewResponse(req.CorrelationID, result)
}

// StreamRequest is the request payload for agent.stream.
type StreamRequest struct {
	SystemPrompt  string `json:"system_prompt"`
	UserPrompt    string `json:"user_prompt"`
	MaxIterations int    `json:"max_iterations,omitempty"`
}

// handleStream executes an agent and streams progress events.
func (h *AgentHandlers) handleStream(ctx context.Context, req *Message, writer *StreamWriter) error {
	var streamReq StreamRequest
	if err := req.UnmarshalParams(&streamReq); err != nil {
		return fmt.Errorf("invalid stream request: %w", err)
	}

	if streamReq.SystemPrompt == "" {
		return fmt.Errorf("system_prompt is required")
	}
	if streamReq.UserPrompt == "" {
		return fmt.Errorf("user_prompt is required")
	}

	// Create agent with stream handler
	ag := agent.NewAgent(h.llmProvider, h.registry)
	if streamReq.MaxIterations > 0 {
		ag = ag.WithMaxIterations(streamReq.MaxIterations)
	}

	// Set up streaming handler
	ag = ag.WithStreamHandler(func(event agent.StreamEvent) {
		// Send each event to the client
		// Errors are logged but don't stop execution
		if err := writer.Send(map[string]interface{}{
			"type":    event.Type,
			"content": event.Content,
		}); err != nil {
			// Log error but continue
			// In production, this would use proper logging
			fmt.Printf("Failed to send stream event: %v\n", err)
		}
	})

	// Run agent
	result, err := ag.Run(ctx, streamReq.SystemPrompt, streamReq.UserPrompt)

	// Send final result
	if err != nil {
		// Send error event
		if sendErr := writer.Send(map[string]interface{}{
			"type":  "error",
			"error": err.Error(),
		}); sendErr != nil {
			return fmt.Errorf("failed to send error event: %w", sendErr)
		}
		return err
	}

	// Send final result event
	if err := writer.Send(map[string]interface{}{
		"type":   "result",
		"result": result,
	}); err != nil {
		return fmt.Errorf("failed to send final result: %w", err)
	}

	// Signal stream completion
	return writer.Done()
}
