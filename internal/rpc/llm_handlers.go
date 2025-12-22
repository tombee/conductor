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
	"fmt"
	"time"

	"github.com/tombee/conductor/pkg/llm"
)

// LLMHandlers contains handlers for LLM-related RPC methods.
type LLMHandlers struct {
	registry    *llm.Registry
	costTracker *llm.CostTracker
}

// NewLLMHandlers creates a new LLM handlers instance.
func NewLLMHandlers(registry *llm.Registry, costTracker *llm.CostTracker) *LLMHandlers {
	return &LLMHandlers{
		registry:    registry,
		costTracker: costTracker,
	}
}

// RegisterHandlers registers all LLM RPC handlers.
func (h *LLMHandlers) RegisterHandlers(reg *Registry) {
	reg.Register("llm.complete", h.HandleComplete)
	reg.RegisterStream("llm.stream", h.HandleStream)
	reg.Register("llm.listProviders", h.HandleListProviders)
	reg.Register("llm.getProvider", h.HandleGetProvider)
	reg.Register("llm.getCostRecords", h.HandleGetCostRecords)
}

// CompleteRequest is the RPC request payload for llm.complete.
type CompleteRequest struct {
	Provider    string                 `json:"provider"`     // Provider name (or "default")
	Model       string                 `json:"model"`        // Model ID or tier
	Messages    []llm.Message          `json:"messages"`     // Conversation messages
	Temperature *float64               `json:"temperature"`  // Optional temperature
	MaxTokens   *int                   `json:"maxTokens"`    // Optional max tokens
	Tools       []llm.Tool             `json:"tools"`        // Optional tools
	Metadata    map[string]string      `json:"metadata"`     // Optional metadata for tracking
}

// CompleteResponse is the RPC response payload for llm.complete.
type CompleteResponse struct {
	Content      string           `json:"content"`
	ToolCalls    []llm.ToolCall   `json:"toolCalls,omitempty"`
	FinishReason string           `json:"finishReason"`
	Usage        llm.TokenUsage   `json:"usage"`
	Model        string           `json:"model"`
	RequestID    string           `json:"requestId"`
	Cost         *llm.CostInfo    `json:"cost,omitempty"`
	Created      time.Time        `json:"created"`
}

// HandleComplete handles synchronous LLM completion requests.
func (h *LLMHandlers) HandleComplete(ctx context.Context, req *Message) (*Message, error) {
	var completionReq CompleteRequest
	if err := json.Unmarshal(req.Params, &completionReq); err != nil {
		return nil, fmt.Errorf("invalid request params: %w", err)
	}

	// Get provider
	var provider llm.Provider
	var err error

	if completionReq.Provider == "" || completionReq.Provider == "default" {
		provider, err = h.registry.GetDefault()
	} else {
		provider, err = h.registry.Get(completionReq.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// Create completion request
	llmReq := llm.CompletionRequest{
		Messages:      completionReq.Messages,
		Model:         completionReq.Model,
		Temperature:   completionReq.Temperature,
		MaxTokens:     completionReq.MaxTokens,
		Tools:         completionReq.Tools,
		Metadata:      completionReq.Metadata,
	}

	// Execute completion
	resp, err := provider.Complete(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	// Calculate cost
	var costInfo *llm.CostInfo
	caps := provider.Capabilities()
	if modelInfo := llm.GetModelByID(caps.Models, resp.Model); modelInfo != nil {
		costInfo = modelInfo.CalculateCost(resp.Usage)

		// Track cost
		if h.costTracker != nil {
			h.costTracker.Track(llm.CostRecord{
				RequestID: resp.RequestID,
				Provider:  provider.Name(),
				Model:     resp.Model,
				Timestamp: resp.Created,
				Usage:     resp.Usage,
				Cost:      costInfo,
				Metadata:  completionReq.Metadata,
			})
		}
	}

	// Build response
	result := CompleteResponse{
		Content:      resp.Content,
		ToolCalls:    resp.ToolCalls,
		FinishReason: string(resp.FinishReason),
		Usage:        resp.Usage,
		Model:        resp.Model,
		RequestID:    resp.RequestID,
		Cost:         costInfo,
		Created:      resp.Created,
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &Message{
		Type:          MessageTypeResponse,
		CorrelationID: req.CorrelationID,
		Result:        resultBytes,
	}, nil
}

// StreamChunkData is the data sent in each stream chunk.
type StreamChunkData struct {
	Delta        llm.StreamDelta    `json:"delta"`
	FinishReason string             `json:"finishReason,omitempty"`
	Usage        *llm.TokenUsage    `json:"usage,omitempty"`
	RequestID    string             `json:"requestId"`
	Cost         *llm.CostInfo      `json:"cost,omitempty"`
}

// HandleStream handles streaming LLM completion requests.
func (h *LLMHandlers) HandleStream(ctx context.Context, req *Message, writer *StreamWriter) error {
	var completionReq CompleteRequest
	if err := json.Unmarshal(req.Params, &completionReq); err != nil {
		return fmt.Errorf("invalid request params: %w", err)
	}

	// Get provider
	var provider llm.Provider
	var err error

	if completionReq.Provider == "" || completionReq.Provider == "default" {
		provider, err = h.registry.GetDefault()
	} else {
		provider, err = h.registry.Get(completionReq.Provider)
	}

	if err != nil {
		return fmt.Errorf("failed to get provider: %w", err)
	}

	// Create completion request
	llmReq := llm.CompletionRequest{
		Messages:      completionReq.Messages,
		Model:         completionReq.Model,
		Temperature:   completionReq.Temperature,
		MaxTokens:     completionReq.MaxTokens,
		Tools:         completionReq.Tools,
		Metadata:      completionReq.Metadata,
	}

	// Start streaming
	chunks, err := provider.Stream(ctx, llmReq)
	if err != nil {
		return fmt.Errorf("stream failed to start: %w", err)
	}

	// Stream chunks to client
	var finalUsage *llm.TokenUsage
	var requestID string
	var modelID string

	for chunk := range chunks {
		if chunk.Error != nil {
			// Send error chunk
			errorData := map[string]interface{}{
				"error": chunk.Error.Error(),
			}
			if err := writer.Send(errorData); err != nil {
				return fmt.Errorf("failed to send error chunk: %w", err)
			}
			return chunk.Error
		}

		// Capture metadata
		if chunk.RequestID != "" {
			requestID = chunk.RequestID
		}
		if chunk.Usage != nil {
			finalUsage = chunk.Usage
		}

		// Send chunk
		chunkData := StreamChunkData{
			Delta:        chunk.Delta,
			FinishReason: string(chunk.FinishReason),
			Usage:        chunk.Usage,
			RequestID:    chunk.RequestID,
		}

		if err := writer.Send(chunkData); err != nil {
			return fmt.Errorf("failed to send chunk: %w", err)
		}

		// If this is the final chunk, calculate cost
		if chunk.FinishReason != "" && finalUsage != nil {
			// Get model from request or use resolved model
			modelID = completionReq.Model
			if modelID == "" {
				caps := provider.Capabilities()
				if len(caps.Models) > 0 {
					modelID = caps.Models[0].ID
				}
			}

			// Calculate cost
			caps := provider.Capabilities()
			if modelInfo := llm.GetModelByID(caps.Models, modelID); modelInfo != nil {
				costInfo := modelInfo.CalculateCost(*finalUsage)

				// Track cost
				if h.costTracker != nil {
					h.costTracker.Track(llm.CostRecord{
						RequestID: requestID,
						Provider:  provider.Name(),
						Model:     modelID,
						Timestamp: time.Now(),
						Usage:     *finalUsage,
						Cost:      costInfo,
						Metadata:  completionReq.Metadata,
					})
				}

				// Send final cost chunk
				costData := StreamChunkData{
					RequestID: requestID,
					Cost:      costInfo,
				}
				if err := writer.Send(costData); err != nil {
					return fmt.Errorf("failed to send cost chunk: %w", err)
				}
			}
		}
	}

	return writer.Done()
}

// HandleListProviders lists all registered providers.
func (h *LLMHandlers) HandleListProviders(ctx context.Context, req *Message) (*Message, error) {
	providers := h.registry.List()

	result := map[string]interface{}{
		"providers": providers,
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &Message{
		Type:          MessageTypeResponse,
		CorrelationID: req.CorrelationID,
		Result:        resultBytes,
	}, nil
}

// GetProviderRequest is the request for getting provider details.
type GetProviderRequest struct {
	Provider string `json:"provider"`
}

// GetProviderResponse contains provider details.
type GetProviderResponse struct {
	Name         string            `json:"name"`
	Capabilities llm.Capabilities  `json:"capabilities"`
}

// HandleGetProvider gets details about a specific provider.
func (h *LLMHandlers) HandleGetProvider(ctx context.Context, req *Message) (*Message, error) {
	var getReq GetProviderRequest
	if err := json.Unmarshal(req.Params, &getReq); err != nil {
		return nil, fmt.Errorf("invalid request params: %w", err)
	}

	var provider llm.Provider
	var err error

	if getReq.Provider == "" || getReq.Provider == "default" {
		provider, err = h.registry.GetDefault()
	} else {
		provider, err = h.registry.Get(getReq.Provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	result := GetProviderResponse{
		Name:         provider.Name(),
		Capabilities: provider.Capabilities(),
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &Message{
		Type:          MessageTypeResponse,
		CorrelationID: req.CorrelationID,
		Result:        resultBytes,
	}, nil
}

// GetCostRecordsRequest is the request for getting cost records.
type GetCostRecordsRequest struct {
	Provider string     `json:"provider,omitempty"` // Filter by provider
	Model    string     `json:"model,omitempty"`    // Filter by model
	Start    *time.Time `json:"start,omitempty"`    // Start time filter
	End      *time.Time `json:"end,omitempty"`      // End time filter
}

// GetCostRecordsResponse contains cost records and aggregates.
type GetCostRecordsResponse struct {
	Records    []llm.CostRecord              `json:"records"`
	Aggregates map[string]llm.CostAggregate  `json:"aggregates"`
}

// HandleGetCostRecords retrieves cost tracking data.
func (h *LLMHandlers) HandleGetCostRecords(ctx context.Context, req *Message) (*Message, error) {
	if h.costTracker == nil {
		return nil, fmt.Errorf("cost tracking not enabled")
	}

	var costReq GetCostRecordsRequest
	if err := json.Unmarshal(req.Params, &costReq); err != nil {
		return nil, fmt.Errorf("invalid request params: %w", err)
	}

	var records []llm.CostRecord

	// Apply filters
	if costReq.Provider != "" {
		records = h.costTracker.GetRecordsByProvider(costReq.Provider)
	} else if costReq.Model != "" {
		records = h.costTracker.GetRecordsByModel(costReq.Model)
	} else if costReq.Start != nil && costReq.End != nil {
		records = h.costTracker.GetRecordsByTimeRange(*costReq.Start, *costReq.End)
	} else {
		records = h.costTracker.GetRecords()
	}

	// Get aggregates
	aggregates := map[string]llm.CostAggregate{
		"byProvider": llm.CostAggregate{}, // Placeholder - would aggregate filtered records
	}

	result := GetCostRecordsResponse{
		Records:    records,
		Aggregates: aggregates,
	}

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return &Message{
		Type:          MessageTypeResponse,
		CorrelationID: req.CorrelationID,
		Result:        resultBytes,
	}, nil
}
