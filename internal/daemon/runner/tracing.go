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

package runner

import (
	"context"
	"log/slog"

	"github.com/tombee/conductor/internal/tracing"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// safeStartSpan safely starts a new span with panic recovery.
// Returns nil span if tracer is nil or if panic occurs.
func safeStartSpan(ctx context.Context, tracer trace.Tracer, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if tracer == nil {
		return ctx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during span start", "error", r, "span_name", name)
		}
	}()

	return tracer.Start(ctx, name, opts...)
}

// safeEndSpan safely ends a span with panic recovery.
func safeEndSpan(span trace.Span) {
	if span == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during span end", "error", r)
		}
	}()

	span.End()
}

// safeSetAttributes safely sets attributes on a span with panic recovery.
func safeSetAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	if span == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during set attributes", "error", r)
		}
	}()

	span.SetAttributes(attrs...)
}

// safeRecordError safely records an error on a span with panic recovery.
func safeRecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during record error", "error", r)
		}
	}()

	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// safeSetStatus safely sets the status on a span with panic recovery.
func safeSetStatus(span trace.Span, code codes.Code, message string) {
	if span == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during set status", "error", r)
		}
	}()

	span.SetStatus(code, message)
}

// safeStartWorkflowRun safely starts a workflow run span with panic recovery.
func safeStartWorkflowRun(ctx context.Context, tracer trace.Tracer, runID, workflowName string) (context.Context, *tracing.WorkflowSpan) {
	if tracer == nil {
		return ctx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during workflow span start", "error", r, "workflow", workflowName)
		}
	}()

	return tracing.StartWorkflowRun(ctx, tracer, runID, workflowName)
}

// safeStartStep safely starts a step span with panic recovery.
func safeStartStep(ctx context.Context, tracer trace.Tracer, stepID, stepType string) (context.Context, *tracing.WorkflowSpan) {
	if tracer == nil {
		return ctx, nil
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during step span start", "error", r, "step_id", stepID)
		}
	}()

	return tracing.StartStep(ctx, tracer, stepID, stepType)
}

// safeEndWorkflowSpan safely ends a workflow span with panic recovery.
func safeEndWorkflowSpan(span *tracing.WorkflowSpan) {
	if span == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during workflow span end", "error", r)
		}
	}()

	span.End()
}

// safeSetWorkflowSpanAttributes safely sets attributes on a workflow span with panic recovery.
func safeSetWorkflowSpanAttributes(span *tracing.WorkflowSpan, attrs map[string]any) {
	if span == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during workflow span set attributes", "error", r)
		}
	}()

	span.SetAttributes(attrs)
}

// safeRecordWorkflowSpanError safely records an error on a workflow span with panic recovery.
func safeRecordWorkflowSpanError(span *tracing.WorkflowSpan, err error) {
	if span == nil || err == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Warn("panic during workflow span record error", "error", r)
		}
	}()

	span.RecordError(err)
}
