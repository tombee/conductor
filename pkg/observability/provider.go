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

package observability

import (
	"context"
)

// TracerProvider is the main interface for creating and managing traces.
// Implementations are responsible for span creation, storage, and export.
type TracerProvider interface {
	// Tracer returns a tracer for the given instrumentation scope.
	// The name should identify the instrumenting package (e.g., "conductor.workflow").
	Tracer(name string) Tracer

	// Shutdown flushes any pending spans and releases resources.
	// Calling Shutdown multiple times is safe.
	Shutdown(ctx context.Context) error

	// ForceFlush exports all pending spans synchronously.
	// This is useful before process termination or checkpointing.
	ForceFlush(ctx context.Context) error
}

// Tracer creates spans within a specific instrumentation scope.
type Tracer interface {
	// Start begins a new span as a child of the context's current span.
	// If the context contains no span, this creates a root span.
	// The returned context contains the new span for propagation.
	Start(ctx context.Context, name string, opts ...SpanOption) (context.Context, SpanHandle)
}

// Span represents an active span that can be modified.
// This is a handle to an in-flight span, not the final stored representation.
type SpanHandle interface {
	// End marks the span as complete and records it.
	// Calling End multiple times is safe (subsequent calls are no-ops).
	End(opts ...SpanEndOption)

	// SetStatus sets the span's final status.
	SetStatus(code StatusCode, message string)

	// SetAttributes adds key-value metadata to the span.
	// Later calls with the same key overwrite earlier values.
	SetAttributes(attrs map[string]any)

	// AddEvent records a timestamped event within the span.
	AddEvent(name string, attrs map[string]any)

	// SpanContext returns the span's trace context for propagation.
	SpanContext() TraceContext

	// RecordError records an error that occurred during span execution.
	// This is a convenience method that calls AddEvent with error details.
	RecordError(err error)
}

// SpanOption configures span creation.
type SpanOption interface {
	// ApplySpanOption applies this option to a span configuration.
	// This method is public to allow cross-package option handling.
	ApplySpanOption(*SpanConfig)
}

// SpanEndOption configures span completion.
type SpanEndOption interface {
	// ApplySpanEndOption applies this option to a span end configuration.
	// This method is public to allow cross-package option handling.
	ApplySpanEndOption(*SpanEndConfig)
}

// SpanConfig holds span creation options.
// This is exported to allow implementations in other packages.
type SpanConfig struct {
	SpanKind   SpanKind
	Attributes map[string]any
	Timestamp  *int64 // Unix nanoseconds
}

// SpanEndConfig holds span end options.
// This is exported to allow implementations in other packages.
type SpanEndConfig struct {
	Timestamp *int64 // Unix nanoseconds
}

// WithSpanKind sets the span kind.
func WithSpanKind(kind SpanKind) SpanOption {
	return spanKindOption(kind)
}

type spanKindOption SpanKind

func (o spanKindOption) ApplySpanOption(c *SpanConfig) {
	c.SpanKind = SpanKind(o)
}

// WithAttributes sets initial span attributes.
func WithAttributes(attrs map[string]any) SpanOption {
	return spanAttributesOption(attrs)
}

type spanAttributesOption map[string]any

func (o spanAttributesOption) ApplySpanOption(c *SpanConfig) {
	if c.Attributes == nil {
		c.Attributes = make(map[string]any)
	}
	for k, v := range o {
		c.Attributes[k] = v
	}
}

// WithTimestamp sets a custom start time for the span.
func WithTimestamp(timestampNanos int64) SpanOption {
	return spanTimestampOption(timestampNanos)
}

type spanTimestampOption int64

func (o spanTimestampOption) ApplySpanOption(c *SpanConfig) {
	ts := int64(o)
	c.Timestamp = &ts
}

// WithEndTimestamp sets a custom end time for the span.
func WithEndTimestamp(timestampNanos int64) SpanEndOption {
	return spanEndTimestampOption(timestampNanos)
}

type spanEndTimestampOption int64

func (o spanEndTimestampOption) ApplySpanEndOption(c *SpanEndConfig) {
	ts := int64(o)
	c.Timestamp = &ts
}
