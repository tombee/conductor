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
	"time"
)

// Config holds observability configuration.
type Config struct {
	// Enabled controls whether tracing is active.
	Enabled bool

	// ServiceName identifies this service in traces.
	ServiceName string

	// ServiceVersion is the application version.
	ServiceVersion string

	// Sampling configures trace sampling.
	Sampling SamplingConfig

	// Storage configures trace storage.
	Storage StorageConfig

	// Exporters configures OTLP export destinations.
	Exporters []ExporterConfig

	// BatchSize is the maximum number of spans per export batch (default: 512).
	BatchSize int

	// BatchInterval is how often to flush spans (default: 5s).
	BatchInterval time.Duration

	// Redaction configures sensitive data handling.
	Redaction RedactionConfig
}

// SamplingConfig controls which traces are recorded.
type SamplingConfig struct {
	// Enabled activates sampling (default: false - sample all).
	Enabled bool

	// Type is the sampling strategy: "head" or "tail".
	Type string

	// Rate is the fraction of traces to sample (0.0 - 1.0).
	// Rate of 1.0 means sample all traces.
	Rate float64

	// AlwaysSampleErrors samples all traces with errors.
	AlwaysSampleErrors bool
}

// StorageConfig controls local trace storage.
type StorageConfig struct {
	// Backend is the storage type: "sqlite" or "memory".
	Backend string

	// Path is the SQLite database path (for backend=sqlite).
	Path string

	// Retention defines how long to keep traces.
	Retention RetentionConfig
}

// RetentionConfig defines data retention policies.
type RetentionConfig struct {
	// Traces is how long to keep trace data.
	Traces time.Duration

	// Events is how long to keep event data.
	Events time.Duration

	// Aggregates is how long to keep aggregated metrics.
	Aggregates time.Duration
}

// ExporterConfig defines an OTLP export destination.
type ExporterConfig struct {
	// Type is the exporter type: "otlp", "otlp-http", or "console".
	Type string

	// Endpoint is the OTLP receiver URL.
	Endpoint string

	// Headers are additional HTTP headers for authentication.
	Headers map[string]string

	// TLS configures secure connections.
	TLS TLSConfig

	// Timeout is the export timeout.
	Timeout time.Duration
}

// TLSConfig configures TLS for exporters.
type TLSConfig struct {
	// Enabled activates TLS.
	Enabled bool

	// VerifyCertificate controls certificate validation.
	VerifyCertificate bool

	// CACertPath is the path to the CA certificate.
	CACertPath string
}

// RedactionConfig controls sensitive data redaction.
type RedactionConfig struct {
	// Level is the redaction mode: "none", "standard", or "strict".
	Level string

	// Patterns are custom redaction patterns.
	Patterns []RedactionPattern
}

// RedactionPattern defines a sensitive data pattern.
type RedactionPattern struct {
	// Name identifies this pattern.
	Name string

	// Regex is the pattern to match.
	Regex string

	// Replacement is the string to substitute.
	Replacement string
}

// DefaultConfig returns configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:        false, // Opt-in
		ServiceName:    "conductor",
		ServiceVersion: "unknown",
		Sampling: SamplingConfig{
			Enabled:            false,
			Type:               "head",
			Rate:               1.0, // Sample all by default
			AlwaysSampleErrors: true,
		},
		Storage: StorageConfig{
			Backend: "sqlite",
			Path:    "", // Will be set to DataDir/traces.db
			Retention: RetentionConfig{
				Traces:     7 * 24 * time.Hour,  // 7 days
				Events:     30 * 24 * time.Hour, // 30 days
				Aggregates: 90 * 24 * time.Hour, // 90 days
			},
		},
		Exporters:     nil,             // No exporters by default
		BatchSize:     512,             // OTLP default batch size
		BatchInterval: 5 * time.Second, // OTLP default batch interval
		Redaction: RedactionConfig{
			Level:    "strict", // Strict by default for safety
			Patterns: nil,      // No custom patterns
		},
	}
}
