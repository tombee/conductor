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
	"time"

	"github.com/tombee/conductor/pkg/llm"
	"github.com/tombee/conductor/pkg/observability"
)

// TracedProvider wraps an LLM provider to add tracing spans for all operations.
// Each Complete() and Stream() call generates a span with token usage and cost attributes.
type TracedProvider struct {
	provider llm.Provider
	tracer   observability.Tracer
	metrics  *MetricsCollector // Optional metrics collector
}

// WrapProvider wraps an LLM provider with tracing instrumentation.
func WrapProvider(provider llm.Provider, tracer observability.Tracer) llm.Provider {
	return &TracedProvider{
		provider: provider,
		tracer:   tracer,
	}
}

// WrapProviderWithMetrics wraps an LLM provider with both tracing and metrics.
func WrapProviderWithMetrics(provider llm.Provider, tracer observability.Tracer, metrics *MetricsCollector) llm.Provider {
	return &TracedProvider{
		provider: provider,
		tracer:   tracer,
		metrics:  metrics,
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
	startTime := time.Now()

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
	latency := time.Since(startTime)

	if err != nil {
		span.RecordError(err)
		// Record failed request metrics
		if t.metrics != nil {
			t.metrics.RecordLLMRequest(ctx, t.provider.Name(), req.Model, "error", 0, 0, 0, latency)
		}
		return nil, err
	}

	// If usage is missing but provider implements UsageTrackable, get usage from GetLastUsage
	if resp.Usage.TotalTokens == 0 {
		if trackable, ok := t.provider.(llm.UsageTrackable); ok {
			if lastUsage := trackable.GetLastUsage(); lastUsage != nil {
				resp.Usage = *lastUsage
			}
		}
	}

	// Record token usage
	span.SetAttributes(map[string]any{
		"llm.response.model":              resp.Model,
		"llm.response.finish_reason":      string(resp.FinishReason),
		"llm.response.request_id":         resp.RequestID,
		"llm.usage.input_tokens":          resp.Usage.InputTokens,
		"llm.usage.output_tokens":         resp.Usage.OutputTokens,
		"llm.usage.total_tokens":          resp.Usage.TotalTokens,
		"llm.usage.cache_creation_tokens": resp.Usage.CacheCreationTokens,
		"llm.usage.cache_read_tokens":     resp.Usage.CacheReadTokens,
		"llm.response.tool_calls_count":   len(resp.ToolCalls),
		"llm.response.content_length":     len(resp.Content),
	})

	// Record successful request metrics
	if t.metrics != nil {
		t.metrics.RecordLLMRequest(ctx, t.provider.Name(), resp.Model, "success",
			resp.Usage.InputTokens, resp.Usage.OutputTokens, 0, latency)
	}

	span.SetStatus(observability.StatusCodeOK, "")
	return resp, nil
}

// Stream creates a span for the streaming request and records token usage from final chunk.
func (t *TracedProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	startTime := time.Now()

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
		// Record failed request metrics
		if t.metrics != nil {
			t.metrics.RecordLLMRequest(ctx, t.provider.Name(), req.Model, "error", 0, 0, 0, time.Since(startTime))
		}
		return nil, err
	}

	// Create a new channel to intercept the final chunk for span completion
	interceptedChunks := make(chan llm.StreamChunk)

	go func() {
		defer close(interceptedChunks)
		defer span.End()

		var contentLength int
		var toolCallsCount int

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
				// Record failed request metrics
				if t.metrics != nil {
					t.metrics.RecordLLMRequest(ctx, t.provider.Name(), req.Model, "error", 0, 0, 0, time.Since(startTime))
				}
				return
			}

			// Final chunk contains usage
			if chunk.Usage != nil {
				usage := *chunk.Usage

				// If usage is missing but provider implements UsageTrackable, get usage from GetLastUsage
				if usage.TotalTokens == 0 {
					if trackable, ok := t.provider.(llm.UsageTrackable); ok {
						if lastUsage := trackable.GetLastUsage(); lastUsage != nil {
							usage = *lastUsage
						}
					}
				}

				span.SetAttributes(map[string]any{
					"llm.response.finish_reason":      string(chunk.FinishReason),
					"llm.response.request_id":         chunk.RequestID,
					"llm.usage.input_tokens":          usage.InputTokens,
					"llm.usage.output_tokens":         usage.OutputTokens,
					"llm.usage.total_tokens":          usage.TotalTokens,
					"llm.usage.cache_creation_tokens": usage.CacheCreationTokens,
					"llm.usage.cache_read_tokens":     usage.CacheReadTokens,
					"llm.response.tool_calls_count":   toolCallsCount,
					"llm.response.content_length":     contentLength,
				})

				// Record successful request metrics
				if t.metrics != nil {
					t.metrics.RecordLLMRequest(ctx, t.provider.Name(), req.Model, "success",
						usage.InputTokens, usage.OutputTokens, 0, time.Since(startTime))
				}

				span.SetStatus(observability.StatusCodeOK, "")
			}
		}
	}()

	return interceptedChunks, nil
}
