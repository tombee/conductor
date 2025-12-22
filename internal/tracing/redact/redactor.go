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

// Package redact provides sensitive data redaction for observability data.
package redact

import (
	"context"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// RedactionMode determines the level of redaction applied to spans.
type RedactionMode string

const (
	// ModeNone disables redaction (not recommended for production).
	ModeNone RedactionMode = "none"

	// ModeStandard applies pattern-based redaction for common secrets.
	ModeStandard RedactionMode = "standard"

	// ModeStrict redacts all attribute values (only keys preserved).
	ModeStrict RedactionMode = "strict"
)

// Pattern defines a redaction pattern with a name and regular expression.
type Pattern struct {
	Name        string
	Regex       *regexp.Regexp
	Replacement string
}

// StandardPatterns returns the default set of redaction patterns.
func StandardPatterns() []Pattern {
	return []Pattern{
		{
			Name:        "api_key",
			Regex:       regexp.MustCompile(`(?i)(api[_-]?key|apikey)["\s:=]+([a-zA-Z0-9_\-]{16,})`),
			Replacement: "$1=[REDACTED]",
		},
		{
			Name:        "bearer_token",
			Regex:       regexp.MustCompile(`(?i)(bearer\s+)([a-zA-Z0-9_\-\.]{20,})`),
			Replacement: "$1[REDACTED]",
		},
		{
			Name:        "password",
			Regex:       regexp.MustCompile(`(?i)(password|passwd|pwd)["\s:=]+([^\s"]+)`),
			Replacement: "$1=[REDACTED]",
		},
		{
			Name:        "aws_key",
			Regex:       regexp.MustCompile(`(AKIA[0-9A-Z]{16})`),
			Replacement: "[REDACTED-AWS-KEY]",
		},
		{
			Name:        "private_key",
			Regex:       regexp.MustCompile(`(?s)(-----BEGIN (RSA |EC |DSA )?PRIVATE KEY-----).*?(-----END (RSA |EC |DSA )?PRIVATE KEY-----)`),
			Replacement: "$1[REDACTED]$3",
		},
		{
			Name:        "email",
			Regex:       regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
			Replacement: "[REDACTED-EMAIL]",
		},
		{
			Name:        "ssn",
			Regex:       regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			Replacement: "[REDACTED-SSN]",
		},
		{
			Name:        "credit_card",
			Regex:       regexp.MustCompile(`\b\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}\b`),
			Replacement: "[REDACTED-CC]",
		},
		{
			Name:        "jwt",
			Regex:       regexp.MustCompile(`eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*`),
			Replacement: "[REDACTED-JWT]",
		},
		{
			Name:        "generic_secret",
			Regex:       regexp.MustCompile(`(?i)(secret|token)["\s:=]+([a-zA-Z0-9_\-]{16,})`),
			Replacement: "$1=[REDACTED]",
		},
	}
}

// Redactor applies redaction rules to sensitive data in spans.
type Redactor struct {
	mode     RedactionMode
	patterns []Pattern
}

// NewRedactor creates a new redactor with the specified mode.
func NewRedactor(mode RedactionMode) *Redactor {
	return &Redactor{
		mode:     mode,
		patterns: StandardPatterns(),
	}
}

// NewRedactorWithPatterns creates a redactor with custom patterns.
func NewRedactorWithPatterns(mode RedactionMode, patterns []Pattern) *Redactor {
	return &Redactor{
		mode:     mode,
		patterns: patterns,
	}
}

// RedactString applies redaction patterns to a string value.
func (r *Redactor) RedactString(s string) string {
	if r.mode == ModeNone {
		return s
	}

	if r.mode == ModeStrict {
		return "[REDACTED]"
	}

	// Apply pattern-based redaction
	result := s
	for _, pattern := range r.patterns {
		result = pattern.Regex.ReplaceAllString(result, pattern.Replacement)
	}
	return result
}

// RedactAttributes applies redaction to span attributes.
func (r *Redactor) RedactAttributes(attrs []attribute.KeyValue) []attribute.KeyValue {
	if r.mode == ModeNone {
		return attrs
	}

	redacted := make([]attribute.KeyValue, len(attrs))
	for i, attr := range attrs {
		key := string(attr.Key)
		value := attr.Value.AsInterface()

		// Check if this attribute should be redacted based on key name
		if r.shouldRedactKey(key) {
			redacted[i] = attribute.String(key, "[REDACTED]")
			continue
		}

		// Redact string values
		if strVal, ok := value.(string); ok {
			redacted[i] = attribute.String(key, r.RedactString(strVal))
		} else if r.mode == ModeStrict {
			// In strict mode, redact all values
			redacted[i] = attribute.String(key, "[REDACTED]")
		} else {
			redacted[i] = attr
		}
	}
	return redacted
}

// shouldRedactKey checks if an attribute key indicates sensitive data.
func (r *Redactor) shouldRedactKey(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitiveKeys := []string{
		"password", "passwd", "pwd",
		"secret", "token",
		"api_key", "apikey",
		"private_key", "private",
		"authorization", "auth",
		"cookie", "session",
	}

	for _, sensitive := range sensitiveKeys {
		if strings.Contains(lowerKey, sensitive) {
			return true
		}
	}
	return false
}

// RedactorSpanProcessor is an OpenTelemetry SpanProcessor that redacts sensitive data.
type RedactorSpanProcessor struct {
	redactor *Redactor
	next     sdktrace.SpanProcessor
}

// NewRedactorSpanProcessor creates a span processor that applies redaction.
func NewRedactorSpanProcessor(redactor *Redactor, next sdktrace.SpanProcessor) *RedactorSpanProcessor {
	return &RedactorSpanProcessor{
		redactor: redactor,
		next:     next,
	}
}

// OnStart is called when a span starts.
func (p *RedactorSpanProcessor) OnStart(ctx context.Context, span sdktrace.ReadWriteSpan) {
	if p.next != nil {
		p.next.OnStart(ctx, span)
	}
}

// OnEnd is called when a span ends. This is where we apply redaction.
func (p *RedactorSpanProcessor) OnEnd(span sdktrace.ReadOnlySpan) {
	// Note: OpenTelemetry's ReadOnlySpan interface doesn't support mutation.
	// Redaction must be implemented as part of the exporter or storage layer.
	// This processor serves as a placeholder for future redaction implementation.
	// For now, we pass through to the next processor.
	if p.next != nil {
		p.next.OnEnd(span)
	}
}

// Shutdown shuts down the processor.
func (p *RedactorSpanProcessor) Shutdown(ctx context.Context) error {
	if p.next != nil {
		return p.next.Shutdown(ctx)
	}
	return nil
}

// ForceFlush forces the processor to flush.
func (p *RedactorSpanProcessor) ForceFlush(ctx context.Context) error {
	if p.next != nil {
		return p.next.ForceFlush(ctx)
	}
	return nil
}
