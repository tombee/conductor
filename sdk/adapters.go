package sdk

import (
	"context"
	"fmt"

	pkgAgent "github.com/tombee/conductor/pkg/agent"
	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/tools"
	pkgWorkflow "github.com/tombee/conductor/pkg/workflow"
)

// sdkLLMProviderAdapter adapts SDK LLM providers to pkg/workflow LLMProvider interface
type sdkLLMProviderAdapter struct {
	sdk *SDK
}

// Complete makes a synchronous LLM call
func (a *sdkLLMProviderAdapter) Complete(ctx context.Context, prompt string, options map[string]interface{}) (*pkgWorkflow.CompletionResult, error) {
	// Extract model from options
	model, ok := options["model"].(string)
	if !ok || model == "" {
		model = "claude-sonnet-4-20250514" // Default model
	}

	// Get provider for model
	provider, err := a.sdk.providers.Get(model)
	if err != nil {
		return nil, &ProviderError{
			Provider:  model,
			Cause:     err,
			Retryable: false,
		}
	}

	// Create completion request
	req := llm.CompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{
				Role:    llm.MessageRoleUser,
				Content: prompt,
			},
		},
	}

	// Map options if provided
	if temp, ok := options["temperature"].(float64); ok {
		req.Temperature = &temp
	}
	if maxTokens, ok := options["max_tokens"].(int); ok {
		req.MaxTokens = &maxTokens
	}

	response, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, &ProviderError{
			Provider:  model,
			Cause:     err,
			Retryable: true,
		}
	}

	// Build the result with token usage
	result := &pkgWorkflow.CompletionResult{
		Content: response.Content,
		Model:   response.Model,
	}

	// Copy usage data if available
	if response.Usage.TotalTokens > 0 {
		result.Usage = &llm.TokenUsage{
			InputTokens:         response.Usage.InputTokens,
			OutputTokens:        response.Usage.OutputTokens,
			TotalTokens:         response.Usage.TotalTokens,
			CacheCreationTokens: response.Usage.CacheCreationTokens,
			CacheReadTokens:     response.Usage.CacheReadTokens,
		}
	}

	return result, nil
}

// sdkToolRegistryAdapter adapts SDK tool registry to pkg/workflow ToolRegistry interface
type sdkToolRegistryAdapter struct {
	sdk *SDK
}

// Get retrieves a tool by name
func (a *sdkToolRegistryAdapter) Get(name string) (pkgWorkflow.Tool, error) {
	tool, err := a.sdk.toolRegistry.Get(name)
	if err != nil {
		return nil, err
	}

	// Wrap the tools.Tool in a workflow.Tool adapter
	return &toolAdapter{tool: tool}, nil
}

// Execute executes a tool with the given inputs
func (a *sdkToolRegistryAdapter) Execute(ctx context.Context, name string, inputs map[string]interface{}) (map[string]interface{}, error) {
	return a.sdk.toolRegistry.Execute(ctx, name, inputs)
}

// ListTools returns all registered tools
func (a *sdkToolRegistryAdapter) ListTools() []pkgWorkflow.Tool {
	pkgTools := a.sdk.toolRegistry.ListTools()
	result := make([]pkgWorkflow.Tool, len(pkgTools))
	for i, t := range pkgTools {
		result[i] = &toolAdapter{tool: t}
	}
	return result
}

// toolAdapter adapts a tools.Tool to workflow.Tool
type toolAdapter struct {
	tool tools.Tool
}

func (t *toolAdapter) Name() string {
	return t.tool.Name()
}

func (t *toolAdapter) Description() string {
	return t.tool.Description()
}

func (t *toolAdapter) Execute(ctx context.Context, inputs map[string]interface{}) (map[string]interface{}, error) {
	return t.tool.Execute(ctx, inputs)
}

// sdkAgentLLMProviderAdapter adapts SDK LLM providers to pkg/agent LLMProvider interface
type sdkAgentLLMProviderAdapter struct {
	sdk *SDK
}

// Complete makes a synchronous LLM call for agent
func (a *sdkAgentLLMProviderAdapter) Complete(ctx context.Context, messages []pkgAgent.Message) (*pkgAgent.Response, error) {
	// For now, we'll use a simple implementation that concatenates messages
	// TODO: Properly handle multi-turn conversation with tool calls

	// Find a provider to use
	// For now, use the first available provider
	providerNames := a.sdk.providers.List()
	if len(providerNames) == 0 {
		return nil, fmt.Errorf("no LLM provider configured")
	}

	providerName := providerNames[0]
	provider, err := a.sdk.providers.Get(providerName)
	if err != nil {
		return nil, err
	}

	// Convert pkg/agent messages to pkg/llm messages
	llmMessages := make([]llm.Message, len(messages))
	for i, msg := range messages {
		var role llm.MessageRole
		switch msg.Role {
		case "system":
			role = llm.MessageRoleSystem
		case "user":
			role = llm.MessageRoleUser
		case "assistant":
			role = llm.MessageRoleAssistant
		case "tool":
			role = llm.MessageRoleTool
		default:
			role = llm.MessageRoleUser
		}
		llmMessages[i] = llm.Message{
			Role:    role,
			Content: msg.Content,
		}
	}

	// Create request
	req := llm.CompletionRequest{
		Model:    providerName,
		Messages: llmMessages,
	}

	// Call provider
	response, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert response
	return &pkgAgent.Response{
		Content:      response.Content,
		ToolCalls:    []pkgAgent.ToolCall{},
		FinishReason: "stop",
		Usage: pkgAgent.TokenUsage{
			InputTokens:  response.Usage.InputTokens,
			OutputTokens: response.Usage.OutputTokens,
			TotalTokens:  response.Usage.TotalTokens,
		},
	}, nil
}

// Stream makes a streaming LLM call for agent
func (a *sdkAgentLLMProviderAdapter) Stream(ctx context.Context, messages []pkgAgent.Message) (<-chan pkgAgent.StreamEvent, error) {
	// TODO: Implement streaming
	return nil, fmt.Errorf("streaming not yet implemented")
}
