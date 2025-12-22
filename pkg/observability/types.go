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

// Package observability provides types and interfaces for tracing and observability.
// This package is designed to be embeddable in other Go applications.
package observability

import (
	"time"
)

// Span represents a unit of work in a trace.
// Spans form a tree structure representing the execution hierarchy.
type Span struct {
	// TraceID uniquely identifies the entire trace.
	TraceID string

	// SpanID uniquely identifies this span within the trace.
	SpanID string

	// ParentID is the SpanID of the parent span. Empty for root spans.
	ParentID string

	// Name is a human-readable description of this span.
	Name string

	// Kind indicates the span's role in the trace.
	Kind SpanKind

	// StartTime is when this span began.
	StartTime time.Time

	// EndTime is when this span completed. Zero for active spans.
	EndTime time.Time

	// Status indicates the span's outcome.
	Status SpanStatus

	// Attributes contains key-value metadata about this span.
	Attributes map[string]any

	// Events are timestamped log entries within this span.
	Events []Event
}

// SpanKind categorizes the type of work represented by a span.
type SpanKind string

const (
	// SpanKindInternal represents work happening within the application.
	SpanKindInternal SpanKind = "internal"

	// SpanKindClient represents an outbound synchronous call.
	SpanKindClient SpanKind = "client"

	// SpanKindServer represents handling an inbound synchronous request.
	SpanKindServer SpanKind = "server"

	// SpanKindProducer represents sending a message to a queue/broker.
	SpanKindProducer SpanKind = "producer"

	// SpanKindConsumer represents receiving a message from a queue/broker.
	SpanKindConsumer SpanKind = "consumer"
)

// SpanStatus indicates whether a span completed successfully.
type SpanStatus struct {
	// Code is the status category.
	Code StatusCode

	// Message provides additional context for errors.
	Message string
}

// StatusCode represents the outcome of a span.
type StatusCode int

const (
	// StatusCodeUnset indicates no status was explicitly set.
	StatusCodeUnset StatusCode = 0

	// StatusCodeOK indicates successful completion.
	StatusCodeOK StatusCode = 1

	// StatusCodeError indicates an error occurred.
	StatusCodeError StatusCode = 2
)

// Event represents a timestamped occurrence within a span.
type Event struct {
	// Name identifies the event type.
	Name string

	// Timestamp is when this event occurred.
	Timestamp time.Time

	// Attributes contains event-specific metadata.
	Attributes map[string]any
}

// TraceContext contains the propagation information for distributed tracing.
// This follows the W3C Trace Context specification.
type TraceContext struct {
	// TraceID uniquely identifies the trace.
	TraceID string

	// SpanID identifies the current span.
	SpanID string

	// TraceFlags contains trace-level flags (sampled, debug, etc).
	TraceFlags byte

	// TraceState holds vendor-specific trace information.
	TraceState string
}

// Duration returns the span's execution time.
// Returns 0 for active spans (EndTime is zero).
func (s *Span) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return 0
	}
	return s.EndTime.Sub(s.StartTime)
}

// IsActive returns true if the span is still in progress.
func (s *Span) IsActive() bool {
	return s.EndTime.IsZero()
}

// Success returns true if the span completed successfully.
func (s *Span) Success() bool {
	return s.Status.Code == StatusCodeOK
}

// ToTraceContext extracts the trace context for propagation.
func (s *Span) ToTraceContext() TraceContext {
	return TraceContext{
		TraceID:    s.TraceID,
		SpanID:     s.SpanID,
		TraceFlags: 0, // Will be set by propagator
		TraceState: "",
	}
}
