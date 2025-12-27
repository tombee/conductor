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

/*
Package tracing provides distributed tracing and observability for Conductor.

This package implements OpenTelemetry-based tracing for workflow execution,
LLM calls, and HTTP requests. It also provides Prometheus metrics collection
and correlation ID propagation for distributed debugging.

# Overview

The tracing package supports:

  - Distributed tracing via OpenTelemetry
  - Prometheus metrics export
  - Correlation ID propagation across services
  - LLM call tracing with token counts
  - Workflow and step span creation

# Quick Start

Create an OTel provider:

	cfg := tracing.Config{
	    Enabled:        true,
	    ServiceName:    "conductor",
	    ServiceVersion: "1.0.0",
	    Sampling: tracing.SamplingConfig{
	        Rate: 0.1, // 10% sampling
	    },
	}

	provider, err := tracing.NewOTelProviderWithConfig(cfg)

Get a tracer and create spans:

	tracer := provider.Tracer("workflow")

	ctx, span := tracer.Start(ctx, "execute-step",
	    trace.WithAttributes(
	        attribute.String("step.id", stepID),
	    ),
	)
	defer span.End()

# Correlation IDs

Correlation IDs link requests across service boundaries:

	// In HTTP middleware
	correlationID := tracing.FromContext(ctx)

	// Add to outbound requests
	req.Header.Set("X-Correlation-ID", string(correlationID))

	// Middleware extracts and injects
	handler = tracing.CorrelationMiddleware(handler)

# Metrics Collection

Prometheus metrics are collected:

	// Get metrics collector
	collector := provider.MetricsCollector()

	// Record events
	collector.RecordRunStart(ctx, runID, workflowID)
	collector.RecordRunComplete(ctx, runID, workflowID, "completed", "api", duration)

Metrics exposed at /metrics:

  - conductor_runs_total{workflow,status,trigger}
  - conductor_run_duration_seconds{workflow,status,trigger}
  - conductor_steps_total{workflow,step,status}
  - conductor_llm_requests_total{provider,model,status}
  - conductor_tokens_total{provider,model,type}

# Configuration

Full configuration options:

	daemon:
	  observability:
	    enabled: true
	    service_name: conductor
	    sampling:
	      type: ratio
	      rate: 0.1
	      always_sample_errors: true
	    exporters:
	      - type: otlp
	        endpoint: localhost:4317
	    redaction:
	      level: standard
	      patterns:
	        - name: api_key
	          regex: "sk-[a-zA-Z0-9]+"
	          replacement: "[REDACTED]"

# Key Components

  - OTelProvider: OpenTelemetry SDK wrapper
  - MetricsCollector: Prometheus metrics recording
  - CorrelationID: Request correlation across services
  - Sampler: Configurable trace sampling
  - Exporter: Trace export to backends (OTLP, etc.)

# Subpackages

  - storage: SQLite-based span storage
  - audit: Security audit logging
*/
package tracing
