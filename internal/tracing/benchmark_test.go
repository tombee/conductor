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

	"github.com/tombee/conductor/pkg/observability"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// BenchmarkSpanCreation measures the overhead of creating and ending a span.
func BenchmarkSpanCreation(b *testing.B) {
	exporter := tracetest.NewInMemoryExporter()
	provider, err := NewOTelProvider(
		"benchmark-service",
		"1.0.0",
		sdktrace.WithSyncer(exporter),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer("benchmark")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "test-operation")
		span.End()
	}
}

// BenchmarkSpanWithAttributes measures overhead with attributes.
func BenchmarkSpanWithAttributes(b *testing.B) {
	exporter := tracetest.NewInMemoryExporter()
	provider, err := NewOTelProvider(
		"benchmark-service",
		"1.0.0",
		sdktrace.WithSyncer(exporter),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer("benchmark")
	ctx := context.Background()

	attrs := map[string]any{
		"key1": "value1",
		"key2": 42,
		"key3": true,
		"key4": 3.14,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "test-operation",
			observability.WithAttributes(attrs),
		)
		span.End()
	}
}

// BenchmarkSpanWithEvents measures overhead with events.
func BenchmarkSpanWithEvents(b *testing.B) {
	exporter := tracetest.NewInMemoryExporter()
	provider, err := NewOTelProvider(
		"benchmark-service",
		"1.0.0",
		sdktrace.WithSyncer(exporter),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer("benchmark")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "test-operation")
		span.AddEvent("event1", map[string]any{"detail": "value"})
		span.AddEvent("event2", map[string]any{"detail": "value"})
		span.End()
	}
}

// BenchmarkNestedSpans measures overhead of nested spans.
func BenchmarkNestedSpans(b *testing.B) {
	exporter := tracetest.NewInMemoryExporter()
	provider, err := NewOTelProvider(
		"benchmark-service",
		"1.0.0",
		sdktrace.WithSyncer(exporter),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer("benchmark")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx1, span1 := tracer.Start(ctx, "parent")
		ctx2, span2 := tracer.Start(ctx1, "child1")
		_, span3 := tracer.Start(ctx2, "child2")
		span3.End()
		span2.End()
		span1.End()
	}
}

// BenchmarkNoOpTracing measures baseline with no tracing.
func BenchmarkNoOpTracing(b *testing.B) {
	ctx := context.Background()

	// Simulate work without tracing
	doWork := func(ctx context.Context) {
		_ = ctx
		// Minimal work
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doWork(ctx)
	}
}

// BenchmarkWithTracing measures overhead with tracing enabled.
func BenchmarkWithTracing(b *testing.B) {
	exporter := tracetest.NewInMemoryExporter()
	provider, err := NewOTelProvider(
		"benchmark-service",
		"1.0.0",
		sdktrace.WithSyncer(exporter),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer("benchmark")
	ctx := context.Background()

	// Simulate work with tracing
	doWork := func(ctx context.Context) {
		_, span := tracer.Start(ctx, "work")
		defer span.End()
		// Minimal work
		_ = ctx
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doWork(ctx)
	}
}

// BenchmarkBatchExport measures export performance.
func BenchmarkBatchExport(b *testing.B) {
	exporter := tracetest.NewInMemoryExporter()
	provider, err := NewOTelProvider(
		"benchmark-service",
		"1.0.0",
		sdktrace.WithBatcher(exporter),
	)
	if err != nil {
		b.Fatal(err)
	}
	defer provider.Shutdown(context.Background())

	tracer := provider.Tracer("benchmark")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "test-operation")
		span.End()

		if i%1000 == 0 {
			// Force flush periodically
			provider.ForceFlush(ctx)
		}
	}
}
