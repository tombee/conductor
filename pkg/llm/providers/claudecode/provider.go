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

// Package claudecode provides a provider implementation that wraps the Claude Code CLI.
// This enables zero-configuration usage for users who have Claude Code installed.
package claudecode

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/llm"
)

// Provider implements the llm.Provider interface using the Claude Code CLI
type Provider struct {
	cliCommand string // The CLI command to use ("claude" or "claude-code")
	cliPath    string // Full path to the CLI binary
	models     config.ModelTierMap
	registry   OperationRegistry // Operation registry for tool execution
}

// New creates a new Claude Code CLI provider
func New() *Provider {
	return &Provider{
		models: defaultModelTiers(),
	}
}

// NewWithModels creates a new Claude Code CLI provider with custom model tier mappings
func NewWithModels(models config.ModelTierMap) *Provider {
	return &Provider{
		models: models,
	}
}

// NewWithRegistry creates a new Claude Code CLI provider with an operation registry for tool execution
func NewWithRegistry(registry OperationRegistry) *Provider {
	return &Provider{
		models:   defaultModelTiers(),
		registry: registry,
	}
}

// Name returns the provider identifier
func (p *Provider) Name() string {
	return "claude-code"
}

// Capabilities returns the provider's supported features
func (p *Provider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		Streaming: true,
		Tools:     true,
		Models: []llm.ModelInfo{
			{
				ID:                    "claude-opus-4-20250514",
				Name:                  "Claude Opus 4",
				Tier:                  llm.ModelTierStrategic,
				MaxTokens:             200000,
				MaxOutputTokens:       8192,
				InputPricePerMillion:  15.00,
				OutputPricePerMillion: 75.00,
				SupportsTools:         true,
				SupportsVision:        true,
			},
			{
				ID:                    "claude-sonnet-4-20250514",
				Name:                  "Claude Sonnet 4",
				Tier:                  llm.ModelTierBalanced,
				MaxTokens:             200000,
				MaxOutputTokens:       8192,
				InputPricePerMillion:  3.00,
				OutputPricePerMillion: 15.00,
				SupportsTools:         true,
				SupportsVision:        true,
			},
			{
				ID:                    "claude-3-5-haiku-20241022",
				Name:                  "Claude 3.5 Haiku",
				Tier:                  llm.ModelTierFast,
				MaxTokens:             200000,
				MaxOutputTokens:       8192,
				InputPricePerMillion:  0.80,
				OutputPricePerMillion: 4.00,
				SupportsTools:         true,
				SupportsVision:        true,
			},
		},
	}
}

// Complete sends a synchronous completion request via the Claude CLI
func (p *Provider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	// Ensure CLI is detected
	if p.cliCommand == "" {
		if found, err := p.Detect(); !found || err != nil {
			return nil, fmt.Errorf("claude CLI not available: %w", err)
		}
	}

	// If tools are specified, use the tool-enabled execution path
	if len(req.Tools) > 0 {
		return p.executeWithTools(ctx, req)
	}

	// No tools - use simple execution path (backward compatible)
	return p.executeSimple(ctx, req)
}

// executeSimple executes a simple (non-tool) completion request
func (p *Provider) executeSimple(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	// Build the CLI command
	args := p.buildCLIArgs(req)

	cmd := exec.CommandContext(ctx, p.cliCommand, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()
	duration := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("claude CLI failed: %w (stderr: %s)", err, stderr.String())
	}

	// Parse the response
	response := &llm.CompletionResponse{
		Content:      strings.TrimSpace(stdout.String()),
		FinishReason: llm.FinishReasonStop,
		Model:        req.Model,
		Created:      startTime,
		// Note: Claude CLI doesn't provide token usage information
		Usage: llm.TokenUsage{},
	}

	_ = duration // Could be used for logging/metrics

	return response, nil
}

// executeWithTools executes a completion request with tool support
// This involves a multi-turn conversation loop until Claude returns a final response
func (p *Provider) executeWithTools(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	const maxIterations = 10

	messages := req.Messages
	startTime := time.Now()

	for i := 0; i < maxIterations; i++ {
		// Build request with current messages
		currentReq := req
		currentReq.Messages = messages

		// Call Claude CLI
		args := p.buildCLIArgs(currentReq)
		cmd := exec.CommandContext(ctx, p.cliCommand, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("claude CLI failed: %w (stderr: %s)", err, stderr.String())
		}

		// Parse response for tool calls
		toolCalls, textContent, err := p.parseToolCalls(stdout.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to parse Claude response: %w", err)
		}

		// If no tool calls, we're done - return the final response
		if len(toolCalls) == 0 {
			return &llm.CompletionResponse{
				Content:      textContent,
				FinishReason: llm.FinishReasonStop,
				Model:        req.Model,
				Created:      startTime,
				Usage:        llm.TokenUsage{},
			}, nil
		}

		// Execute tools
		toolResults := p.executeTools(ctx, toolCalls)

		// Build messages for next turn
		// Add assistant message with tool calls
		messages = append(messages, llm.Message{
			Role:    llm.MessageRoleAssistant,
			Content: textContent,
		})

		// Add tool result messages
		for _, result := range toolResults {
			messages = append(messages, llm.Message{
				Role:       llm.MessageRoleTool,
				Content:    result.Content,
				ToolCallID: result.ID,
			})
		}
	}

	return nil, fmt.Errorf("tool loop exceeded maximum iterations (%d)", maxIterations)
}

// Stream sends a streaming completion request via the Claude CLI
func (p *Provider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	// Ensure CLI is detected
	if p.cliCommand == "" {
		if found, err := p.Detect(); !found || err != nil {
			return nil, fmt.Errorf("claude CLI not available: %w", err)
		}
	}

	// Build the CLI command
	args := p.buildCLIArgs(req)

	cmd := exec.CommandContext(ctx, p.cliCommand, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude CLI: %w", err)
	}

	// Create channel for streaming chunks
	chunks := make(chan llm.StreamChunk, 10)

	// Start goroutine to read and stream output
	go func() {
		defer close(chunks)
		defer cmd.Wait()

		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				chunks <- llm.StreamChunk{
					Delta: llm.StreamDelta{
						Content: string(buf[:n]),
					},
				}
			}
			if err != nil {
				// Check for errors from stderr
				var stderrBuf bytes.Buffer
				stderrBuf.ReadFrom(stderr)
				if stderrBuf.Len() > 0 {
					chunks <- llm.StreamChunk{
						Error:        fmt.Errorf("claude CLI error: %s", stderrBuf.String()),
						FinishReason: llm.FinishReasonError,
					}
				} else {
					chunks <- llm.StreamChunk{
						FinishReason: llm.FinishReasonStop,
					}
				}
				return
			}
		}
	}()

	return chunks, nil
}

// buildCLIArgs constructs the command-line arguments for the Claude CLI
func (p *Provider) buildCLIArgs(req llm.CompletionRequest) []string {
	args := []string{}

	// Add model if specified
	if req.Model != "" {
		// Resolve model tier to specific model if needed
		model := p.resolveModel(req.Model)
		args = append(args, "--model", model)
	}

	// Add temperature if specified
	if req.Temperature != nil {
		args = append(args, "--temperature", fmt.Sprintf("%.2f", *req.Temperature))
	}

	// Add max tokens if specified
	if req.MaxTokens != nil {
		args = append(args, "--max-tokens", fmt.Sprintf("%d", *req.MaxTokens))
	}

	// When tools are specified, disable Claude's built-in tools and use JSON output
	if len(req.Tools) > 0 {
		args = append(args, "--tools", "")
		args = append(args, "--output-format", "json")
	}

	// Build the prompt from messages
	prompt := p.buildPrompt(req.Messages)
	args = append(args, "-p", prompt)

	return args
}

// resolveModel resolves a model tier to a specific model ID
func (p *Provider) resolveModel(model string) string {
	// Check if it's a tier name
	switch model {
	case "fast":
		if p.models.Fast != "" {
			return p.models.Fast
		}
		return "claude-3-5-haiku-20241022"
	case "balanced":
		if p.models.Balanced != "" {
			return p.models.Balanced
		}
		return "claude-sonnet-4-20250514"
	case "strategic":
		if p.models.Strategic != "" {
			return p.models.Strategic
		}
		return "claude-opus-4-20250514"
	default:
		// Assume it's already a model ID
		return model
	}
}

// buildPrompt constructs a prompt string from messages
func (p *Provider) buildPrompt(messages []llm.Message) string {
	var parts []string

	for _, msg := range messages {
		switch msg.Role {
		case llm.MessageRoleSystem:
			parts = append(parts, fmt.Sprintf("System: %s", msg.Content))
		case llm.MessageRoleUser:
			parts = append(parts, fmt.Sprintf("User: %s", msg.Content))
		case llm.MessageRoleAssistant:
			parts = append(parts, fmt.Sprintf("Assistant: %s", msg.Content))
		case llm.MessageRoleTool:
			parts = append(parts, fmt.Sprintf("Tool Result: %s", msg.Content))
		}
	}

	return strings.Join(parts, "\n\n")
}

// defaultModelTiers returns the default model tier mappings for Claude
func defaultModelTiers() config.ModelTierMap {
	return config.ModelTierMap{
		Fast:      "claude-3-5-haiku-20241022",
		Balanced:  "claude-sonnet-4-20250514",
		Strategic: "claude-opus-4-20250514",
	}
}
