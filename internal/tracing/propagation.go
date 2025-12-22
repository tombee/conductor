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
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// W3CPropagator returns a TextMapPropagator that implements W3C Trace Context.
func W3CPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// InjectHTTPHeaders injects the trace context into HTTP request headers.
// This enables distributed tracing across service boundaries.
func InjectHTTPHeaders(ctx context.Context, req *http.Request) {
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// ExtractHTTPHeaders extracts the trace context from HTTP request headers.
// Returns a new context with the extracted trace context.
func ExtractHTTPHeaders(ctx context.Context, req *http.Request) context.Context {
	propagator := otel.GetTextMapPropagator()
	return propagator.Extract(ctx, propagation.HeaderCarrier(req.Header))
}

// HTTPMiddleware returns an HTTP middleware that extracts trace context from incoming requests.
// It should be used in the HTTP router to enable trace context propagation.
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from headers
		ctx := ExtractHTTPHeaders(r.Context(), r)

		// Update request with new context
		r = r.WithContext(ctx)

		// Call next handler
		next.ServeHTTP(w, r)
	})
}

// TracingMiddleware returns an HTTP middleware that creates a span for each HTTP request.
// It should be used after HTTPMiddleware in the middleware chain.
func TracingMiddleware(next http.Handler) http.Handler {
	tracer := otel.Tracer("conductor.http")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start a new span for this request
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()

		// Add HTTP attributes
		span.SetAttributes(
			// Use semantic convention attribute constructors if available
			// For now, using simple string keys
		)

		// Update request with span context
		r = r.WithContext(ctx)

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(wrapped, r)

		// Record response status
		span.SetAttributes()
		if wrapped.statusCode >= 400 {
			span.SetStatus(1, "") // Error status
		} else {
			span.SetStatus(0, "") // OK status
		}
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
