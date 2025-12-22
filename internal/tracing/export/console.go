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
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

// ConsoleConfig holds configuration for console exporter.
type ConsoleConfig struct {
	// Writer is the output destination (default: os.Stdout).
	Writer io.Writer

	// PrettyPrint enables human-readable formatted output.
	PrettyPrint bool
}

// NewConsoleExporter creates a new console trace exporter for development.
// This exporter prints traces to stdout for debugging purposes.
func NewConsoleExporter(cfg ConsoleConfig) (trace.SpanExporter, error) {
	var opts []stdouttrace.Option

	// Set output writer
	writer := cfg.Writer
	if writer == nil {
		writer = os.Stdout
	}
	opts = append(opts, stdouttrace.WithWriter(writer))

	// Enable pretty printing if requested
	if cfg.PrettyPrint {
		opts = append(opts, stdouttrace.WithPrettyPrint())
	}

	// Create exporter
	exporter, err := stdouttrace.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create console exporter: %w", err)
	}

	return exporter, nil
}

// NewDefaultConsoleExporter creates a console exporter with pretty printing to stdout.
func NewDefaultConsoleExporter() (trace.SpanExporter, error) {
	return NewConsoleExporter(ConsoleConfig{
		Writer:      os.Stdout,
		PrettyPrint: true,
	})
}
