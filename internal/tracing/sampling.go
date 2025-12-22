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
	"math/rand"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// SamplerConfig configures trace sampling behavior
type SamplerConfig struct {
	// Enabled controls whether sampling is active
	Enabled bool

	// Rate is the sampling rate (0.0 - 1.0)
	// 1.0 = 100% sampling (all traces)
	// 0.1 = 10% sampling
	Rate float64

	// AlwaysSampleErrors ensures error traces are always sampled
	AlwaysSampleErrors bool
}

// NewSampler creates an OpenTelemetry sampler based on the configuration
func NewSampler(cfg SamplerConfig) sdktrace.Sampler {
	if !cfg.Enabled || cfg.Rate >= 1.0 {
		// No sampling - always sample
		return sdktrace.AlwaysSample()
	}

	if cfg.Rate <= 0.0 {
		// Never sample (unless error)
		if cfg.AlwaysSampleErrors {
			return &errorAwareSampler{
				baseSampler: sdktrace.NeverSample(),
			}
		}
		return sdktrace.NeverSample()
	}

	// Rate-based sampling
	baseSampler := sdktrace.TraceIDRatioBased(cfg.Rate)

	if cfg.AlwaysSampleErrors {
		return &errorAwareSampler{
			baseSampler: baseSampler,
		}
	}

	return baseSampler
}

// errorAwareSampler wraps a base sampler to always sample error traces
type errorAwareSampler struct {
	baseSampler sdktrace.Sampler
}

// ShouldSample implements the Sampler interface
func (s *errorAwareSampler) ShouldSample(params sdktrace.SamplingParameters) sdktrace.SamplingResult {
	// Check if this span represents an error by looking at attributes
	for _, attr := range params.Attributes {
		// Check for error status attributes
		if attr.Key == "error" && attr.Value.AsBool() {
			return sdktrace.SamplingResult{
				Decision:   sdktrace.RecordAndSample,
				Tracestate: trace.SpanContextFromContext(params.ParentContext).TraceState(),
			}
		}
		if attr.Key == "conductor.status" && attr.Value.AsString() == "error" {
			return sdktrace.SamplingResult{
				Decision:   sdktrace.RecordAndSample,
				Tracestate: trace.SpanContextFromContext(params.ParentContext).TraceState(),
			}
		}
	}

	// Defer to base sampler for non-error traces
	return s.baseSampler.ShouldSample(params)
}

// Description returns a description of the sampler
func (s *errorAwareSampler) Description() string {
	return "ErrorAwareSampler{base=" + s.baseSampler.Description() + "}"
}

// deterministicSampler implements deterministic sampling based on trace ID
// This ensures that the same trace ID always gets the same sampling decision
type deterministicSampler struct {
	rate float64
}

// NewDeterministicSampler creates a sampler that makes consistent decisions
// based on trace ID. This is useful for distributed tracing where multiple
// services should agree on whether to sample a trace.
func NewDeterministicSampler(rate float64) sdktrace.Sampler {
	if rate >= 1.0 {
		return sdktrace.AlwaysSample()
	}
	if rate <= 0.0 {
		return sdktrace.NeverSample()
	}
	return &deterministicSampler{rate: rate}
}

// ShouldSample implements the Sampler interface
func (s *deterministicSampler) ShouldSample(params sdktrace.SamplingParameters) sdktrace.SamplingResult {
	// Use trace ID for deterministic decision
	traceID := params.TraceID

	// Hash the trace ID to get a value between 0 and 1
	// We use the last 8 bytes of the trace ID as a pseudo-random value
	var hash uint64
	for i := 8; i < 16; i++ {
		hash = hash*31 + uint64(traceID[i])
	}

	// Normalize to 0.0 - 1.0
	normalized := float64(hash) / float64(^uint64(0))

	decision := sdktrace.Drop
	if normalized < s.rate {
		decision = sdktrace.RecordAndSample
	}

	return sdktrace.SamplingResult{
		Decision:   decision,
		Tracestate: trace.SpanContextFromContext(params.ParentContext).TraceState(),
	}
}

// Description returns a description of the sampler
func (s *deterministicSampler) Description() string {
	return "DeterministicSampler{rate=" + formatFloat(s.rate) + "}"
}

// randomSampler implements random sampling (non-deterministic)
// Each sampling decision is independent
type randomSampler struct {
	rate float64
}

// NewRandomSampler creates a sampler that makes random sampling decisions
func NewRandomSampler(rate float64) sdktrace.Sampler {
	if rate >= 1.0 {
		return sdktrace.AlwaysSample()
	}
	if rate <= 0.0 {
		return sdktrace.NeverSample()
	}
	return &randomSampler{rate: rate}
}

// ShouldSample implements the Sampler interface
func (s *randomSampler) ShouldSample(params sdktrace.SamplingParameters) sdktrace.SamplingResult {
	decision := sdktrace.Drop
	if rand.Float64() < s.rate {
		decision = sdktrace.RecordAndSample
	}

	return sdktrace.SamplingResult{
		Decision:   decision,
		Tracestate: trace.SpanContextFromContext(params.ParentContext).TraceState(),
	}
}

// Description returns a description of the sampler
func (s *randomSampler) Description() string {
	return "RandomSampler{rate=" + formatFloat(s.rate) + "}"
}

// Helper function to format float for description
func formatFloat(f float64) string {
	if f == 0.0 {
		return "0.0"
	}
	if f == 1.0 {
		return "1.0"
	}
	// Format to 2 decimal places
	return string(rune(int(f*100))) + "%"
}
