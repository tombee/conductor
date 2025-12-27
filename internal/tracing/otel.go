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
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tombee/conductor/pkg/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// OTelProvider wraps the OpenTelemetry SDK to implement our TracerProvider interface.
type OTelProvider struct {
	tp             *sdktrace.TracerProvider
	mp             *metric.MeterProvider
	promExporter   *prometheus.Exporter
	metricsCollector *MetricsCollector
}

// NewOTelProviderWithConfig creates a new OpenTelemetry-based tracer provider with full configuration.
func NewOTelProviderWithConfig(cfg Config, opts ...sdktrace.TracerProviderOption) (*OTelProvider, error) {
	// Create sampler from config
	sampler := NewSampler(SamplerConfig{
		Enabled:            cfg.Sampling.Enabled,
		Rate:               cfg.Sampling.Rate,
		AlwaysSampleErrors: cfg.Sampling.AlwaysSampleErrors,
	})

	// Prepend sampler option
	allOpts := append([]sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sampler),
	}, opts...)

	return NewOTelProvider(cfg.ServiceName, cfg.ServiceVersion, allOpts...)
}

// NewOTelProvider creates a new OpenTelemetry-based tracer provider.
func NewOTelProvider(serviceName, version string, opts ...sdktrace.TracerProviderOption) (*OTelProvider, error) {
	// Create resource with service information
	// Note: We don't set SchemaURL to avoid conflicts when merging with default resource
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			"",  // Empty schema URL to avoid conflicts
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Prepend resource option
	allOpts := append([]sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
	}, opts...)

	tp := sdktrace.NewTracerProvider(allOpts...)

	// Set as global tracer provider (for libraries that use otel.Tracer)
	otel.SetTracerProvider(tp)

	// Create Prometheus exporter for metrics
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// Create meter provider with Prometheus exporter
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(promExporter),
	)

	// Create metrics collector
	metricsCollector, err := NewMetricsCollector(mp)
	if err != nil {
		return nil, fmt.Errorf("failed to create metrics collector: %w", err)
	}

	return &OTelProvider{
		tp:               tp,
		mp:               mp,
		promExporter:     promExporter,
		metricsCollector: metricsCollector,
	}, nil
}

// Tracer returns a tracer for the given instrumentation scope.
func (p *OTelProvider) Tracer(name string) observability.Tracer {
	return &otelTracer{
		tracer: p.tp.Tracer(name),
	}
}

// Shutdown flushes any pending spans and releases resources.
func (p *OTelProvider) Shutdown(ctx context.Context) error {
	if err := p.tp.Shutdown(ctx); err != nil {
		return err
	}
	if p.mp != nil {
		return p.mp.Shutdown(ctx)
	}
	return nil
}

// ForceFlush exports all pending spans synchronously.
func (p *OTelProvider) ForceFlush(ctx context.Context) error {
	if err := p.tp.ForceFlush(ctx); err != nil {
		return err
	}
	if p.mp != nil {
		return p.mp.ForceFlush(ctx)
	}
	return nil
}

// MetricsCollector returns the metrics collector for recording workflow metrics.
func (p *OTelProvider) MetricsCollector() *MetricsCollector {
	return p.metricsCollector
}

// MetricsHandler returns an HTTP handler for Prometheus metrics endpoint.
// The OpenTelemetry prometheus exporter registers metrics with the default Prometheus registry,
// so we use promhttp.Handler() to expose them.
func (p *OTelProvider) MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// otelTracer wraps an OpenTelemetry tracer.
type otelTracer struct {
	tracer trace.Tracer
}

// Start begins a new span.
func (t *otelTracer) Start(ctx context.Context, name string, opts ...observability.SpanOption) (context.Context, observability.SpanHandle) {
	// Build span config from options
	cfg := &observability.SpanConfig{}
	for _, opt := range opts {
		opt.ApplySpanOption(cfg)
	}

	// Convert to OpenTelemetry options
	var otelOpts []trace.SpanStartOption

	// Set span kind
	switch cfg.SpanKind {
	case observability.SpanKindClient:
		otelOpts = append(otelOpts, trace.WithSpanKind(trace.SpanKindClient))
	case observability.SpanKindServer:
		otelOpts = append(otelOpts, trace.WithSpanKind(trace.SpanKindServer))
	case observability.SpanKindProducer:
		otelOpts = append(otelOpts, trace.WithSpanKind(trace.SpanKindProducer))
	case observability.SpanKindConsumer:
		otelOpts = append(otelOpts, trace.WithSpanKind(trace.SpanKindConsumer))
	default:
		otelOpts = append(otelOpts, trace.WithSpanKind(trace.SpanKindInternal))
	}

	// Set attributes
	if len(cfg.Attributes) > 0 {
		attrs := make([]attribute.KeyValue, 0, len(cfg.Attributes))
		for k, v := range cfg.Attributes {
			attrs = append(attrs, toAttribute(k, v))
		}
		otelOpts = append(otelOpts, trace.WithAttributes(attrs...))
	}

	// Set custom timestamp if provided
	if cfg.Timestamp != nil {
		// OTel expects time.Time, so we convert from nanos
		// This will be used in the span config
		otelOpts = append(otelOpts, trace.WithTimestamp(timeFromNanos(*cfg.Timestamp)))
	}

	ctx, span := t.tracer.Start(ctx, name, otelOpts...)

	return ctx, &otelSpan{span: span}
}

// otelSpan wraps an OpenTelemetry span.
type otelSpan struct {
	span trace.Span
}

// End marks the span as complete.
func (s *otelSpan) End(opts ...observability.SpanEndOption) {
	cfg := &observability.SpanEndConfig{}
	for _, opt := range opts {
		opt.ApplySpanEndOption(cfg)
	}

	var otelOpts []trace.SpanEndOption
	if cfg.Timestamp != nil {
		otelOpts = append(otelOpts, trace.WithTimestamp(timeFromNanos(*cfg.Timestamp)))
	}

	s.span.End(otelOpts...)
}

// SetStatus sets the span's final status.
func (s *otelSpan) SetStatus(code observability.StatusCode, message string) {
	var otelCode codes.Code
	switch code {
	case observability.StatusCodeOK:
		otelCode = codes.Ok
	case observability.StatusCodeError:
		otelCode = codes.Error
	default:
		otelCode = codes.Unset
	}
	s.span.SetStatus(otelCode, message)
}

// SetAttributes adds key-value metadata to the span.
func (s *otelSpan) SetAttributes(attrs map[string]any) {
	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		otelAttrs = append(otelAttrs, toAttribute(k, v))
	}
	s.span.SetAttributes(otelAttrs...)
}

// AddEvent records a timestamped event within the span.
func (s *otelSpan) AddEvent(name string, attrs map[string]any) {
	otelAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		otelAttrs = append(otelAttrs, toAttribute(k, v))
	}
	s.span.AddEvent(name, trace.WithAttributes(otelAttrs...))
}

// SpanContext returns the span's trace context.
func (s *otelSpan) SpanContext() observability.TraceContext {
	sc := s.span.SpanContext()
	return observability.TraceContext{
		TraceID:    sc.TraceID().String(),
		SpanID:     sc.SpanID().String(),
		TraceFlags: byte(sc.TraceFlags()),
		TraceState: sc.TraceState().String(),
	}
}

// RecordError records an error that occurred during span execution.
func (s *otelSpan) RecordError(err error) {
	s.span.RecordError(err)
	s.span.SetStatus(codes.Error, err.Error())
}
