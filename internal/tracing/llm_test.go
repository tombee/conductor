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
	"errors"
	"testing"
	"time"

	"github.com/tombee/conductor/pkg/llm"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// mockProvider implements llm.Provider for testing.
type mockProvider struct {
	name         string
	completeFunc func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error)
	streamFunc   func(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error)
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		Streaming: true,
		Tools:     true,
	}
}

func (m *mockProvider) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func (m *mockProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func TestTracedProvider_Complete(t *testing.T) {
	// Setup in-memory span exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shutdown tracer provider: %v", err)
		}
	}()

	otelProvider := &OTelProvider{tp: tp}
	tracer := otelProvider.Tracer("test")

	// Create mock provider
	mock := &mockProvider{
		name: "test-provider",
		completeFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			return &llm.CompletionResponse{
				Content:      "test response",
				FinishReason: llm.FinishReasonStop,
				Usage: llm.TokenUsage{
					InputTokens:         10,
					OutputTokens:        20,
					TotalTokens:         30,
					CacheCreationTokens: 5,
					CacheReadTokens:     15,
				},
				Model:     "test-model",
				RequestID: "req-123",
				Created:   time.Now(),
			}, nil
		},
	}

	// Wrap with tracing
	traced := WrapProvider(mock, tracer)

	// Execute completion
	ctx := context.Background()
	resp, err := traced.Complete(ctx, llm.CompletionRequest{
		Model: "test-model",
		Messages: []llm.Message{
			{Role: llm.MessageRoleUser, Content: "test"},
		},
	})

	// Verify response
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "test response" {
		t.Errorf("expected content 'test response', got %q", resp.Content)
	}

	// Force flush to ensure span is exported
	if err := tp.ForceFlush(ctx); err != nil {
		t.Fatalf("failed to flush spans: %v", err)
	}

	// Verify span was created
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "llm.complete" {
		t.Errorf("expected span name 'llm.complete', got %q", span.Name)
	}

	// Verify attributes
	attrs := span.Attributes
	expectedAttrs := map[string]any{
		"llm.provider":                    "test-provider",
		"llm.model":                       "test-model",
		"llm.response.model":              "test-model",
		"llm.response.request_id":         "req-123",
		"llm.usage.input_tokens":          int64(10),
		"llm.usage.output_tokens":         int64(20),
		"llm.usage.total_tokens":          int64(30),
		"llm.usage.cache_creation_tokens": int64(5),
		"llm.usage.cache_read_tokens":     int64(15),
	}

	for key, expectedValue := range expectedAttrs {
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == key {
				found = true
				if attr.Value.AsInterface() != expectedValue {
					t.Errorf("attribute %q: expected %v, got %v", key, expectedValue, attr.Value.AsInterface())
				}
				break
			}
		}
		if !found {
			t.Errorf("missing attribute: %q", key)
		}
	}
}

func TestTracedProvider_Complete_Error(t *testing.T) {
	// Setup in-memory span exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shutdown tracer provider: %v", err)
		}
	}()

	otelProvider := &OTelProvider{tp: tp}
	tracer := otelProvider.Tracer("test")

	// Create mock provider that returns error
	expectedErr := errors.New("test error")
	mock := &mockProvider{
		name: "test-provider",
		completeFunc: func(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
			return nil, expectedErr
		},
	}

	// Wrap with tracing
	traced := WrapProvider(mock, tracer)

	// Execute completion
	ctx := context.Background()
	_, err := traced.Complete(ctx, llm.CompletionRequest{
		Model: "test-model",
	})

	// Verify error
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}

	// Force flush
	if err := tp.ForceFlush(ctx); err != nil {
		t.Fatalf("failed to flush spans: %v", err)
	}

	// Verify span recorded error
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Status.Code.String() != "Error" {
		t.Errorf("expected error status, got %v", span.Status.Code)
	}
}

func TestTracedProvider_Stream(t *testing.T) {
	// Setup in-memory span exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shutdown tracer provider: %v", err)
		}
	}()

	otelProvider := &OTelProvider{tp: tp}
	tracer := otelProvider.Tracer("test")

	// Create mock provider
	mock := &mockProvider{
		name: "test-provider",
		streamFunc: func(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
			ch := make(chan llm.StreamChunk, 3)
			go func() {
				defer close(ch)
				ch <- llm.StreamChunk{
					Delta: llm.StreamDelta{Content: "Hello"},
				}
				ch <- llm.StreamChunk{
					Delta: llm.StreamDelta{Content: " World"},
				}
				ch <- llm.StreamChunk{
					FinishReason: llm.FinishReasonStop,
					RequestID:    "req-456",
					Usage: &llm.TokenUsage{
						InputTokens:  15,
						OutputTokens: 25,
						TotalTokens:  40,
					},
				}
			}()
			return ch, nil
		},
	}

	// Wrap with tracing
	traced := WrapProvider(mock, tracer)

	// Execute streaming
	ctx := context.Background()
	chunks, err := traced.Stream(ctx, llm.CompletionRequest{
		Model: "test-model",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Consume all chunks
	var content string
	for chunk := range chunks {
		content += chunk.Delta.Content
	}

	if content != "Hello World" {
		t.Errorf("expected content 'Hello World', got %q", content)
	}

	// Force flush
	if err := tp.ForceFlush(ctx); err != nil {
		t.Fatalf("failed to flush spans: %v", err)
	}

	// Verify span was created
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name != "llm.stream" {
		t.Errorf("expected span name 'llm.stream', got %q", span.Name)
	}

	// Verify usage attributes were set
	attrs := span.Attributes
	expectedAttrs := map[string]any{
		"llm.provider":             "test-provider",
		"llm.model":                "test-model",
		"llm.response.request_id":  "req-456",
		"llm.usage.input_tokens":   int64(15),
		"llm.usage.output_tokens":  int64(25),
		"llm.usage.total_tokens":   int64(40),
	}

	for key, expectedValue := range expectedAttrs {
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == key {
				found = true
				if attr.Value.AsInterface() != expectedValue {
					t.Errorf("attribute %q: expected %v, got %v", key, expectedValue, attr.Value.AsInterface())
				}
				break
			}
		}
		if !found {
			t.Errorf("missing attribute: %q", key)
		}
	}
}

func TestTracedProvider_Stream_Error(t *testing.T) {
	// Setup in-memory span exporter
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSyncer(exporter),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			t.Errorf("failed to shutdown tracer provider: %v", err)
		}
	}()

	otelProvider := &OTelProvider{tp: tp}
	tracer := otelProvider.Tracer("test")

	// Create mock provider that returns error in stream
	expectedErr := errors.New("stream error")
	mock := &mockProvider{
		name: "test-provider",
		streamFunc: func(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
			ch := make(chan llm.StreamChunk, 1)
			go func() {
				defer close(ch)
				ch <- llm.StreamChunk{Error: expectedErr}
			}()
			return ch, nil
		},
	}

	// Wrap with tracing
	traced := WrapProvider(mock, tracer)

	// Execute streaming
	ctx := context.Background()
	chunks, err := traced.Stream(ctx, llm.CompletionRequest{
		Model: "test-model",
	})

	if err != nil {
		t.Fatalf("unexpected error from Stream(): %v", err)
	}

	// Consume chunks (error should be in chunk)
	var gotErr error
	for chunk := range chunks {
		if chunk.Error != nil {
			gotErr = chunk.Error
		}
	}

	if gotErr != expectedErr {
		t.Fatalf("expected error %v in chunk, got %v", expectedErr, gotErr)
	}

	// Force flush
	if err := tp.ForceFlush(ctx); err != nil {
		t.Fatalf("failed to flush spans: %v", err)
	}

	// Verify span recorded error
	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Status.Code.String() != "Error" {
		t.Errorf("expected error status, got %v", span.Status.Code)
	}
}
