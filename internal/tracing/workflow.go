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

	"github.com/tombee/conductor/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// WorkflowSpan wraps an OpenTelemetry span with workflow-specific helpers.
type WorkflowSpan struct {
	span trace.Span
}

// StartWorkflowRun creates a root span for a workflow run.
// This should be called at the start of workflow execution.
func StartWorkflowRun(ctx context.Context, tracer trace.Tracer, runID, workflowName string) (context.Context, *WorkflowSpan) {
	ctx, span := tracer.Start(ctx, fmt.Sprintf("workflow.run: %s", workflowName),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("workflow.name", workflowName),
			attribute.String("workflow.run_id", runID),
			attribute.String("span.type", "workflow.run"),
		),
	)

	return ctx, &WorkflowSpan{span: span}
}

// StartStep creates a span for a workflow step execution.
// This should be called for each step in the workflow.
func StartStep(ctx context.Context, tracer trace.Tracer, stepID, stepType string) (context.Context, *WorkflowSpan) {
	ctx, span := tracer.Start(ctx, fmt.Sprintf("step: %s", stepID),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("step.id", stepID),
			attribute.String("step.type", stepType),
			attribute.String("span.type", "workflow.step"),
		),
	)

	return ctx, &WorkflowSpan{span: span}
}

// SetAttributes adds key-value attributes to the span.
func (w *WorkflowSpan) SetAttributes(attrs map[string]any) {
	if w == nil || w.span == nil {
		return
	}

	var otelAttrs []attribute.KeyValue
	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			otelAttrs = append(otelAttrs, attribute.String(k, val))
		case int:
			otelAttrs = append(otelAttrs, attribute.Int(k, val))
		case int64:
			otelAttrs = append(otelAttrs, attribute.Int64(k, val))
		case float64:
			otelAttrs = append(otelAttrs, attribute.Float64(k, val))
		case bool:
			otelAttrs = append(otelAttrs, attribute.Bool(k, val))
		default:
			otelAttrs = append(otelAttrs, attribute.String(k, fmt.Sprintf("%v", val)))
		}
	}

	w.span.SetAttributes(otelAttrs...)
}

// AddEvent records a timestamped event within the span.
func (w *WorkflowSpan) AddEvent(name string, attrs map[string]any) {
	if w == nil || w.span == nil {
		return
	}

	var otelAttrs []attribute.KeyValue
	for k, v := range attrs {
		switch val := v.(type) {
		case string:
			otelAttrs = append(otelAttrs, attribute.String(k, val))
		case int:
			otelAttrs = append(otelAttrs, attribute.Int(k, val))
		case int64:
			otelAttrs = append(otelAttrs, attribute.Int64(k, val))
		case float64:
			otelAttrs = append(otelAttrs, attribute.Float64(k, val))
		case bool:
			otelAttrs = append(otelAttrs, attribute.Bool(k, val))
		default:
			otelAttrs = append(otelAttrs, attribute.String(k, fmt.Sprintf("%v", val)))
		}
	}

	w.span.AddEvent(name, trace.WithAttributes(otelAttrs...))
}

// RecordError records an error that occurred during execution.
func (w *WorkflowSpan) RecordError(err error) {
	if w == nil || w.span == nil || err == nil {
		return
	}

	w.span.RecordError(err)
	w.span.SetStatus(codes.Error, err.Error())
}

// SetStatus sets the span's final status.
func (w *WorkflowSpan) SetStatus(code observability.StatusCode, message string) {
	if w == nil || w.span == nil {
		return
	}

	var otelCode codes.Code
	switch code {
	case observability.StatusCodeOK:
		otelCode = codes.Ok
	case observability.StatusCodeError:
		otelCode = codes.Error
	default:
		otelCode = codes.Unset
	}

	w.span.SetStatus(otelCode, message)
}

// End marks the span as complete.
func (w *WorkflowSpan) End() {
	if w == nil || w.span == nil {
		return
	}

	w.span.End()
}

// SpanContext returns the span's trace context for propagation.
func (w *WorkflowSpan) SpanContext() trace.SpanContext {
	if w == nil || w.span == nil {
		return trace.SpanContext{}
	}

	return w.span.SpanContext()
}

// TraceID returns the trace ID as a string.
func (w *WorkflowSpan) TraceID() string {
	if w == nil || w.span == nil {
		return ""
	}

	return w.span.SpanContext().TraceID().String()
}

// SpanID returns the span ID as a string.
func (w *WorkflowSpan) SpanID() string {
	if w == nil || w.span == nil {
		return ""
	}

	return w.span.SpanContext().SpanID().String()
}
