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

package tracing

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/observability"
)

// TracedProvider wraps an LLM provider to add tracing spans for all operations.
// Each Complete() and Stream() call generates a span with token usage and cost attributes.
type TracedProvider struct {
	provider llm.Provider
	tracer   observability.Tracer
}

// WrapProvider wraps an LLM provider with tracing instrumentation.
func WrapProvider(provider llm.Provider, tracer observability.Tracer) llm.Provider {
	return &TracedProvider{
		provider: provider,
		tracer:   tracer,
	}
}

// Name returns the underlying provider's name.
func (t *TracedProvider) Name() string {
	return t.provider.Name()
}

// Capabilities returns the underlying provider's capabilities.
func (t *TracedProvider) Capabilities() llm.Capabilities {
	return t.provider.Capabilities()
}

// Complete creates a span for the completion request and records token usage.
func (t *TracedProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	ctx, span := t.tracer.Start(ctx, "llm.complete",
		observability.WithSpanKind(observability.SpanKindClient),
		observability.WithAttributes(map[string]any{
			"llm.provider":    t.provider.Name(),
			"llm.model":       req.Model,
			"llm.temperature": req.Temperature,
			"llm.max_tokens":  req.MaxTokens,
		}),
	)
	defer span.End()

	// Add request metadata as attributes
	if len(req.Metadata) > 0 {
		metadataAttrs := make(map[string]any, len(req.Metadata))
		for k, v := range req.Metadata {
			metadataAttrs[fmt.Sprintf("llm.metadata.%s", k)] = v
		}
		span.SetAttributes(metadataAttrs)
	}

	// Execute the completion
	resp, err := t.provider.Complete(ctx, req)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	// Record token usage
	span.SetAttributes(map[string]any{
		"llm.response.model":                   resp.Model,
		"llm.response.finish_reason":           string(resp.FinishReason),
		"llm.response.request_id":              resp.RequestID,
		"llm.usage.prompt_tokens":              resp.Usage.PromptTokens,
		"llm.usage.completion_tokens":          resp.Usage.CompletionTokens,
		"llm.usage.total_tokens":               resp.Usage.TotalTokens,
		"llm.usage.cache_creation_tokens":      resp.Usage.CacheCreationTokens,
		"llm.usage.cache_read_tokens":          resp.Usage.CacheReadTokens,
		"llm.response.tool_calls_count":        len(resp.ToolCalls),
		"llm.response.content_length":          len(resp.Content),
	})

	// Attempt to retrieve and record cost information
	if costRecord := llm.GetCostRecordByRequestID(resp.RequestID); costRecord != nil && costRecord.Cost != nil {
		span.SetAttributes(map[string]any{
			"llm.cost.amount":   costRecord.Cost.Amount,
			"llm.cost.currency": costRecord.Cost.Currency,
			"llm.cost.accuracy": string(costRecord.Cost.Accuracy),
			"llm.cost.source":   costRecord.Cost.Source,
		})
	}

	span.SetStatus(observability.StatusCodeOK, "")
	return resp, nil
}

// Stream creates a span for the streaming request and records token usage from final chunk.
func (t *TracedProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	ctx, span := t.tracer.Start(ctx, "llm.stream",
		observability.WithSpanKind(observability.SpanKindClient),
		observability.WithAttributes(map[string]any{
			"llm.provider":    t.provider.Name(),
			"llm.model":       req.Model,
			"llm.temperature": req.Temperature,
			"llm.max_tokens":  req.MaxTokens,
		}),
	)

	// Add request metadata as attributes
	if len(req.Metadata) > 0 {
		metadataAttrs := make(map[string]any, len(req.Metadata))
		for k, v := range req.Metadata {
			metadataAttrs[fmt.Sprintf("llm.metadata.%s", k)] = v
		}
		span.SetAttributes(metadataAttrs)
	}

	// Execute the streaming request
	chunks, err := t.provider.Stream(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.End()
		return nil, err
	}

	// Create a new channel to intercept the final chunk for span completion
	interceptedChunks := make(chan llm.StreamChunk)

	go func() {
		defer close(interceptedChunks)
		defer span.End()

		var contentLength int
		var toolCallsCount int
		var lastRequestID string

		for chunk := range chunks {
			// Forward the chunk
			interceptedChunks <- chunk

			// Track metrics
			contentLength += len(chunk.Delta.Content)
			if chunk.Delta.ToolCallDelta != nil {
				toolCallsCount++
			}

			// Capture error
			if chunk.Error != nil {
				span.RecordError(chunk.Error)
				return
			}

			// Record request ID
			if chunk.RequestID != "" {
				lastRequestID = chunk.RequestID
			}

			// Final chunk contains usage
			if chunk.Usage != nil {
				span.SetAttributes(map[string]any{
					"llm.response.finish_reason":      string(chunk.FinishReason),
					"llm.response.request_id":         chunk.RequestID,
					"llm.usage.prompt_tokens":         chunk.Usage.PromptTokens,
					"llm.usage.completion_tokens":     chunk.Usage.CompletionTokens,
					"llm.usage.total_tokens":          chunk.Usage.TotalTokens,
					"llm.usage.cache_creation_tokens": chunk.Usage.CacheCreationTokens,
					"llm.usage.cache_read_tokens":     chunk.Usage.CacheReadTokens,
					"llm.response.tool_calls_count":   toolCallsCount,
					"llm.response.content_length":     contentLength,
				})

				// Attempt to retrieve and record cost information
				if lastRequestID != "" {
					if costRecord := llm.GetCostRecordByRequestID(lastRequestID); costRecord != nil && costRecord.Cost != nil {
						span.SetAttributes(map[string]any{
							"llm.cost.amount":   costRecord.Cost.Amount,
							"llm.cost.currency": costRecord.Cost.Currency,
							"llm.cost.accuracy": string(costRecord.Cost.Accuracy),
							"llm.cost.source":   costRecord.Cost.Source,
						})
					}
				}

				span.SetStatus(observability.StatusCodeOK, "")
			}
		}
	}()

	return interceptedChunks, nil
}
