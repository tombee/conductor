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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tombee/conductor/pkg/observability"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestOTelProvider_BasicSpan(t *testing.T) {
	// Create a test exporter to capture spans
	exporter := tracetest.NewInMemoryExporter()

	// Create provider with in-memory exporter
	provider, err := NewOTelProvider(
		"test-service",
		"1.0.0",
		sdktrace.WithSyncer(exporter),
	)
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	// Create a tracer
	tracer := provider.Tracer("test")

	// Start a span
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "test-operation",
		observability.WithSpanKind(observability.SpanKindInternal),
		observability.WithAttributes(map[string]any{
			"test.key": "test-value",
			"test.num": 42,
		}),
	)

	// Add an event
	span.AddEvent("test-event", map[string]any{
		"event.detail": "some-detail",
	})

	// Set status and end
	span.SetStatus(observability.StatusCodeOK, "")
	span.End()

	// Force flush to ensure span is exported
	err = provider.ForceFlush(context.Background())
	require.NoError(t, err)

	// Verify the span was captured
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	capturedSpan := spans[0]
	assert.Equal(t, "test-operation", capturedSpan.Name)

	// Check attributes
	attrs := capturedSpan.Attributes
	assert.Len(t, attrs, 2)

	// Find and verify attributes
	var foundTestKey, foundTestNum bool
	for _, attr := range attrs {
		if attr.Key == "test.key" {
			assert.Equal(t, "test-value", attr.Value.AsString())
			foundTestKey = true
		}
		if attr.Key == "test.num" {
			assert.Equal(t, int64(42), attr.Value.AsInt64())
			foundTestNum = true
		}
	}
	assert.True(t, foundTestKey, "test.key attribute not found")
	assert.True(t, foundTestNum, "test.num attribute not found")

	// Check events
	require.Len(t, capturedSpan.Events, 1)
	assert.Equal(t, "test-event", capturedSpan.Events[0].Name)
}

func TestOTelProvider_NestedSpans(t *testing.T) {
	// Create a test exporter
	exporter := tracetest.NewInMemoryExporter()

	provider, err := NewOTelProvider(
		"test-service",
		"1.0.0",
		sdktrace.WithSyncer(exporter),
	)
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer("test")

	// Create parent span
	ctx := context.Background()
	ctx, parentSpan := tracer.Start(ctx, "parent")

	// Create child span
	_, childSpan := tracer.Start(ctx, "child")
	childSpan.End()

	parentSpan.End()

	// Force flush
	err = provider.ForceFlush(context.Background())
	require.NoError(t, err)

	// Verify hierarchy
	spans := exporter.GetSpans()
	require.Len(t, spans, 2)

	// Find parent and child
	var parent, child *tracetest.SpanStub
	for i := range spans {
		if spans[i].Name == "parent" {
			parent = &spans[i]
		} else if spans[i].Name == "child" {
			child = &spans[i]
		}
	}

	require.NotNil(t, parent)
	require.NotNil(t, child)

	// Child should have parent's span ID as parent
	assert.Equal(t, parent.SpanContext.SpanID(), child.Parent.SpanID())
	assert.Equal(t, parent.SpanContext.TraceID(), child.Parent.TraceID())
}

func TestOTelProvider_ErrorRecording(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()

	provider, err := NewOTelProvider(
		"test-service",
		"1.0.0",
		sdktrace.WithSyncer(exporter),
	)
	require.NoError(t, err)
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer("test")

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "error-operation")

	// Record an error
	testErr := assert.AnError
	span.RecordError(testErr)
	span.End()

	err = provider.ForceFlush(context.Background())
	require.NoError(t, err)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	capturedSpan := spans[0]

	// Check that error was recorded as event
	require.Greater(t, len(capturedSpan.Events), 0)

	// Span status should be error
	assert.Equal(t, "Error", capturedSpan.Status.Code.String())
}
