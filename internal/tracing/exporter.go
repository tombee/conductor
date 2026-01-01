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
	"log/slog"

	"github.com/tombee/conductor/internal/tracing/export"
	"github.com/tombee/conductor/internal/tracing/storage"
	"github.com/tombee/conductor/pkg/observability"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// StorageExporter exports OpenTelemetry spans to our SQLite storage.
// This implements the trace.SpanExporter interface.
type StorageExporter struct {
	store *storage.SQLiteStore
}

// NewStorageExporter creates a new storage exporter.
func NewStorageExporter(store *storage.SQLiteStore) *StorageExporter {
	return &StorageExporter{store: store}
}

// ExportSpans exports a batch of spans to storage.
func (e *StorageExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	for _, otelSpan := range spans {
		span := convertOTelSpan(otelSpan)
		if err := e.store.StoreSpan(ctx, span); err != nil {
			// Log error but don't fail the batch
			// This prevents one bad span from blocking all spans
			continue
		}
	}

	return nil
}

// Shutdown flushes any remaining spans and releases resources.
func (e *StorageExporter) Shutdown(ctx context.Context) error {
	// No buffering, so nothing to flush
	return nil
}

// convertOTelSpan converts an OpenTelemetry span to our observability.Span type.
func convertOTelSpan(otelSpan sdktrace.ReadOnlySpan) *observability.Span {
	span := &observability.Span{
		TraceID:   otelSpan.SpanContext().TraceID().String(),
		SpanID:    otelSpan.SpanContext().SpanID().String(),
		Name:      otelSpan.Name(),
		StartTime: otelSpan.StartTime(),
		EndTime:   otelSpan.EndTime(),
	}

	// Set parent ID
	if otelSpan.Parent().IsValid() {
		span.ParentID = otelSpan.Parent().SpanID().String()
	}

	// Convert span kind
	switch otelSpan.SpanKind() {
	case trace.SpanKindInternal:
		span.Kind = observability.SpanKindInternal
	case trace.SpanKindClient:
		span.Kind = observability.SpanKindClient
	case trace.SpanKindServer:
		span.Kind = observability.SpanKindServer
	case trace.SpanKindProducer:
		span.Kind = observability.SpanKindProducer
	case trace.SpanKindConsumer:
		span.Kind = observability.SpanKindConsumer
	default:
		span.Kind = observability.SpanKindInternal
	}

	// Convert status
	status := otelSpan.Status()
	switch status.Code {
	case 1: // OK
		span.Status.Code = observability.StatusCodeOK
	case 2: // Error
		span.Status.Code = observability.StatusCodeError
		span.Status.Message = status.Description
	default: // Unset
		span.Status.Code = observability.StatusCodeUnset
	}

	// Convert attributes
	span.Attributes = make(map[string]any)
	for _, attr := range otelSpan.Attributes() {
		span.Attributes[string(attr.Key)] = attr.Value.AsInterface()
	}

	// Convert events
	span.Events = make([]observability.Event, 0, len(otelSpan.Events()))
	for _, otelEvent := range otelSpan.Events() {
		event := observability.Event{
			Name:       otelEvent.Name,
			Timestamp:  otelEvent.Time,
			Attributes: make(map[string]any),
		}

		for _, attr := range otelEvent.Attributes {
			event.Attributes[string(attr.Key)] = attr.Value.AsInterface()
		}

		span.Events = append(span.Events, event)
	}

	return span
}

// Compile-time check that StorageExporter implements sdktrace.SpanExporter
var _ sdktrace.SpanExporter = (*StorageExporter)(nil)

// CreateExporter creates a span exporter from configuration.
// This factory function supports multiple exporter types and handles creation errors gracefully.
func CreateExporter(ctx context.Context, cfg ExporterConfig) (sdktrace.SpanExporter, error) {
	switch cfg.Type {
	case "console":
		return export.NewConsoleExporter(export.ConsoleConfig{
			Writer:      nil, // Use default stdout
			PrettyPrint: true,
		})

	case "otlp":
		tlsConfig, err := export.BuildTLSConfig(export.TLSConfigInput{
			Enabled:           cfg.TLS.Enabled,
			VerifyCertificate: cfg.TLS.VerifyCertificate,
			CACertPath:        cfg.TLS.CACertPath,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config for OTLP exporter: %w", err)
		}

		return export.NewOTLPExporter(ctx, export.OTLPConfig{
			Endpoint:  cfg.Endpoint,
			Insecure:  !cfg.TLS.Enabled,
			TLSConfig: tlsConfig,
			Headers:   cfg.Headers,
		})

	case "otlp_http", "otlp-http":
		tlsConfig, err := export.BuildTLSConfig(export.TLSConfigInput{
			Enabled:           cfg.TLS.Enabled,
			VerifyCertificate: cfg.TLS.VerifyCertificate,
			CACertPath:        cfg.TLS.CACertPath,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config for OTLP HTTP exporter: %w", err)
		}

		return export.NewOTLPHTTPExporter(ctx, export.OTLPHTTPConfig{
			Endpoint:  cfg.Endpoint,
			URLPath:   "", // Use default /v1/traces
			Insecure:  !cfg.TLS.Enabled,
			TLSConfig: tlsConfig,
			Headers:   cfg.Headers,
		})

	case "none", "":
		// No exporter - tracing disabled
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown exporter type: %s", cfg.Type)
	}
}

// CreateExportersFromConfig creates batch span processors for all configured exporters.
// Exporter creation failures are logged but don't block startup.
func CreateExportersFromConfig(ctx context.Context, cfg Config) ([]sdktrace.SpanProcessor, error) {
	var processors []sdktrace.SpanProcessor

	for i, exporterCfg := range cfg.Exporters {
		exporter, err := CreateExporter(ctx, exporterCfg)
		if err != nil {
			// Log warning but continue - partial export is better than no export
			slog.Warn("failed to create exporter, skipping",
				"index", i,
				"type", exporterCfg.Type,
				"endpoint", exporterCfg.Endpoint,
				"error", err)
			continue
		}

		if exporter == nil {
			// Type was "none" - skip
			continue
		}

		// Wrap in batch processor with configured batch size and interval
		batchOpts := []sdktrace.BatchSpanProcessorOption{}

		// Set batch size from config (default is 512 if not configured)
		if cfg.BatchSize > 0 {
			batchOpts = append(batchOpts, sdktrace.WithMaxExportBatchSize(cfg.BatchSize))
		}

		// Set batch interval from config (default is 5s if not configured)
		if cfg.BatchInterval > 0 {
			batchOpts = append(batchOpts, sdktrace.WithBatchTimeout(cfg.BatchInterval))
		}

		processor := sdktrace.NewBatchSpanProcessor(exporter, batchOpts...)
		processors = append(processors, processor)

		slog.Info("created exporter",
			"type", exporterCfg.Type,
			"endpoint", exporterCfg.Endpoint)
	}

	return processors, nil
}
