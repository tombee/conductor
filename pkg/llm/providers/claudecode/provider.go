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
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/llm"
)

// Provider implements the llm.Provider interface using the Claude Code CLI
type Provider struct {
	cliCommand string // The CLI command to use ("claude" or "claude-code")
	cliPath    string // Full path to the CLI binary
	models     llm.ModelTierMap
}

// cliResponse represents the JSON output from Claude CLI with --output-format json
type cliResponse struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

// New creates a new Claude Code CLI provider
func New() *Provider {
	return &Provider{
		models: defaultModelTiers(),
	}
}

// NewWithCredentials creates a new Claude Code CLI provider from credentials.
// This is the factory function used by the registry for two-phase initialization.
// For CLI-based providers, credentials are optional (CLI handles its own auth).
func NewWithCredentials(creds llm.Credentials) (llm.Provider, error) {
	p := &Provider{
		models: defaultModelTiers(),
	}

	// If CLI credentials provided, use the specified path
	if cliCreds, ok := creds.(llm.CLIAuthCredentials); ok && cliCreds.CLIPath != "" {
		p.cliPath = cliCreds.CLIPath
		p.cliCommand = cliCreds.CLIPath
	}

	return p, nil
}

// NewWithModels creates a new Claude Code CLI provider with custom model tier mappings.
// If models is empty, falls back to default tier mappings.
func NewWithModels(models llm.ModelTierMap) *Provider {
	if models.IsEmpty() {
		models = defaultModelTiers()
	}
	return &Provider{
		models: models,
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

	// Use simple execution path - Claude CLI handles tool execution internally via MCP
	return p.executeSimple(ctx, req)
}

// executeSimple executes a simple (non-tool) completion request
func (p *Provider) executeSimple(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	// Build the CLI command with JSON output for usage stats
	args := p.buildCLIArgs(req, true)

	cmd := exec.CommandContext(ctx, p.cliCommand, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startTime := time.Now()
	err := cmd.Run()

	if err != nil {
		return nil, fmt.Errorf("claude CLI failed: %w (stderr: %s)", err, stderr.String())
	}

	// Parse JSON response from Claude CLI
	var cliResp cliResponse
	if err := json.Unmarshal(stdout.Bytes(), &cliResp); err != nil {
		// Fallback to treating output as plain text if JSON parsing fails
		return &llm.CompletionResponse{
			Content:      strings.TrimSpace(stdout.String()),
			FinishReason: llm.FinishReasonStop,
			Model:        req.Model,
			Created:      startTime,
			Usage:        llm.TokenUsage{},
		}, nil
	}

	// Check for error response
	if cliResp.IsError {
		return nil, fmt.Errorf("claude CLI error: %s", cliResp.Result)
	}

	// Build response with usage stats
	usage := llm.TokenUsage{
		InputTokens:        cliResp.Usage.InputTokens,
		OutputTokens:       cliResp.Usage.OutputTokens,
		TotalTokens:        cliResp.Usage.InputTokens + cliResp.Usage.OutputTokens,
		CacheCreationTokens: cliResp.Usage.CacheCreationInputTokens,
		CacheReadTokens:     cliResp.Usage.CacheReadInputTokens,
	}

	response := &llm.CompletionResponse{
		Content:      cliResp.Result,
		FinishReason: llm.FinishReasonStop,
		Model:        req.Model,
		Created:      startTime,
		Usage:        usage,
	}

	return response, nil
}

// Stream sends a streaming completion request via the Claude CLI
func (p *Provider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	// Ensure CLI is detected
	if p.cliCommand == "" {
		if found, err := p.Detect(); !found || err != nil {
			return nil, fmt.Errorf("claude CLI not available: %w", err)
		}
	}

	// Build the CLI command (no JSON for streaming - uses plain text)
	args := p.buildCLIArgs(req, false)

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
			select {
			case <-ctx.Done():
				// Context cancelled, stop reading
				chunks <- llm.StreamChunk{
					Error:        ctx.Err(),
					FinishReason: llm.FinishReasonError,
				}
				return
			default:
			}

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
				if _, copyErr := io.Copy(&stderrBuf, stderr); copyErr == nil && stderrBuf.Len() > 0 {
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
func (p *Provider) buildCLIArgs(req llm.CompletionRequest, useJSON bool) []string {
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

	// When tools are specified, configure Conductor MCP server for tool execution
	if len(req.Tools) > 0 {
		mcpConfig := p.buildMCPConfig()
		args = append(args, "--mcp-config", mcpConfig)
	}

	// Request JSON output for usage stats (sync mode) or stream-json (streaming mode)
	if useJSON {
		args = append(args, "--output-format", "json")
	}

	// Build the prompt from messages
	prompt := p.buildPrompt(req.Messages, nil)
	args = append(args, "-p", prompt)

	return args
}

// buildMCPConfig returns the MCP configuration JSON for Conductor tools
func (p *Provider) buildMCPConfig() string {
	// Configure Conductor MCP server to expose Conductor tools
	// Claude CLI will start this as a subprocess and connect via stdio
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"conductor": map[string]interface{}{
				"command": "conductor",
				"args":    []string{"mcp-server"},
			},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		// Fallback to hardcoded JSON if marshal fails (shouldn't happen)
		return `{"mcpServers":{"conductor":{"command":"conductor","args":["mcp-server"]}}}`
	}

	return string(data)
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
func (p *Provider) buildPrompt(messages []llm.Message, _ []llm.Tool) string {
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
func defaultModelTiers() llm.ModelTierMap {
	return llm.ModelTierMap{
		Fast:      "claude-3-5-haiku-20241022",
		Balanced:  "claude-sonnet-4-20250514",
		Strategic: "claude-opus-4-20250514",
	}
}

// DiscoverModels returns the list of models available through Claude Code.
// This returns models from the provider's capabilities.
func (p *Provider) DiscoverModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return p.Capabilities().Models, nil
}

// Ensure Provider implements ModelDiscoverer
var _ llm.ModelDiscoverer = (*Provider)(nil)
