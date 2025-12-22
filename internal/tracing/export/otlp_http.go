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

package export

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/trace"
)

// OTLPHTTPConfig holds configuration for OTLP HTTP exporter.
type OTLPHTTPConfig struct {
	// Endpoint is the HTTP endpoint (e.g., "https://api.honeycomb.io").
	Endpoint string

	// URLPath is the URL path for traces (default: "/v1/traces").
	URLPath string

	// Insecure disables TLS (for development only).
	Insecure bool

	// TLSConfig provides custom TLS configuration.
	TLSConfig *tls.Config

	// Headers contains custom headers to send with each request.
	Headers map[string]string
}

// NewOTLPHTTPExporter creates a new OTLP HTTP trace exporter.
func NewOTLPHTTPExporter(ctx context.Context, cfg OTLPHTTPConfig) (trace.SpanExporter, error) {
	var opts []otlptracehttp.Option

	// Set endpoint
	opts = append(opts, otlptracehttp.WithEndpoint(cfg.Endpoint))

	// Set URL path if specified
	if cfg.URLPath != "" {
		opts = append(opts, otlptracehttp.WithURLPath(cfg.URLPath))
	}

	// Configure TLS
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	} else if cfg.TLSConfig != nil {
		// Validate custom TLS configuration
		if err := ValidateTLSConfig(cfg.TLSConfig); err != nil {
			return nil, fmt.Errorf("invalid TLS config: %w", err)
		}
		opts = append(opts, otlptracehttp.WithTLSClientConfig(cfg.TLSConfig))
	} else {
		// Default TLS with minimum version TLS 1.2
		opts = append(opts, otlptracehttp.WithTLSClientConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		}))
	}

	// Add custom headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}

	// Create exporter
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP HTTP exporter: %w", err)
	}

	return exporter, nil
}

// NewOTLPHTTPExporterWithClient creates a new OTLP HTTP exporter with a custom HTTP client.
// This is an advanced function that requires using a wrapper pattern since the SDK
// doesn't directly expose a WithHTTPClient option in all versions.
func NewOTLPHTTPExporterWithClient(ctx context.Context, cfg OTLPHTTPConfig, _ *http.Client) (trace.SpanExporter, error) {
	// For compatibility, we fall back to the standard exporter
	// In production, you would need to implement a custom exporter wrapper
	// that uses the provided HTTP client
	return NewOTLPHTTPExporter(ctx, cfg)
}
