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

// Package export provides trace exporters for external observability platforms.
package export

import (
	"context"
	"crypto/tls"
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// OTLPConfig holds configuration for OTLP gRPC exporter.
type OTLPConfig struct {
	// Endpoint is the gRPC endpoint (e.g., "localhost:4317").
	Endpoint string

	// Insecure disables TLS (for development only).
	Insecure bool

	// TLSConfig provides custom TLS configuration.
	TLSConfig *tls.Config

	// Headers contains custom headers to send with each request.
	Headers map[string]string
}

// NewOTLPExporter creates a new OTLP gRPC trace exporter.
func NewOTLPExporter(ctx context.Context, cfg OTLPConfig) (trace.SpanExporter, error) {
	var opts []otlptracegrpc.Option

	// Set endpoint
	opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))

	// Configure TLS
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	} else if cfg.TLSConfig != nil {
		// Validate custom TLS configuration
		if err := ValidateTLSConfig(cfg.TLSConfig); err != nil {
			return nil, fmt.Errorf("invalid TLS config: %w", err)
		}
		creds := credentials.NewTLS(cfg.TLSConfig)
		opts = append(opts, otlptracegrpc.WithTLSCredentials(creds))
	} else {
		// Default TLS (system cert pool with TLS 1.2+)
		creds := credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})
		opts = append(opts, otlptracegrpc.WithTLSCredentials(creds))
	}

	// Add custom headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	// Create exporter
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
	}

	return exporter, nil
}

// NewOTLPExporterWithDialOption creates a new OTLP gRPC exporter with custom dial options.
// This is useful for advanced gRPC configuration.
func NewOTLPExporterWithDialOption(ctx context.Context, cfg OTLPConfig, dialOpts ...grpc.DialOption) (trace.SpanExporter, error) {
	var opts []otlptracegrpc.Option

	// Set endpoint
	opts = append(opts, otlptracegrpc.WithEndpoint(cfg.Endpoint))

	// Configure TLS if not using custom dial options
	if len(dialOpts) == 0 {
		if cfg.Insecure {
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else if cfg.TLSConfig != nil {
			creds := credentials.NewTLS(cfg.TLSConfig)
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		} else {
			creds := credentials.NewTLS(&tls.Config{
				MinVersion: tls.VersionTLS12,
			})
			dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
		}
	}

	opts = append(opts, otlptracegrpc.WithDialOption(dialOpts...))

	// Add custom headers
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	// Create exporter
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP gRPC exporter: %w", err)
	}

	return exporter, nil
}
