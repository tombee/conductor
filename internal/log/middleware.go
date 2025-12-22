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

package log

import (
	"log/slog"
	"time"
)

// RPCRequest represents an RPC request for logging purposes.
type RPCRequest struct {
	// MessageType is the type of RPC message (e.g., "execute_tool", "llm_request").
	MessageType string

	// CorrelationID is the correlation ID for tracing the request.
	CorrelationID string

	// RequestID is the unique ID for this specific request.
	RequestID string

	// RemoteAddr is the remote address of the client.
	RemoteAddr string

	// Metadata contains additional request metadata.
	Metadata map[string]interface{}
}

// RPCResponse represents an RPC response for logging purposes.
type RPCResponse struct {
	// Success indicates whether the request was successful.
	Success bool

	// Error is the error message if the request failed.
	Error string

	// DurationMs is the duration of the request in milliseconds.
	DurationMs int64

	// Metadata contains additional response metadata.
	Metadata map[string]interface{}
}

// LogRPCRequest logs an incoming RPC request.
func LogRPCRequest(logger *slog.Logger, req *RPCRequest) {
	attrs := []any{
		"event", "rpc_request",
		"message_type", req.MessageType,
		"remote", req.RemoteAddr,
	}

	if req.CorrelationID != "" {
		attrs = append(attrs, "correlation_id", req.CorrelationID)
	}

	if req.RequestID != "" {
		attrs = append(attrs, "request_id", req.RequestID)
	}

	for k, v := range req.Metadata {
		attrs = append(attrs, k, v)
	}

	logger.Info("rpc request received", attrs...)
}

// LogRPCResponse logs an RPC response.
func LogRPCResponse(logger *slog.Logger, req *RPCRequest, resp *RPCResponse) {
	attrs := []any{
		"event", "rpc_response",
		"message_type", req.MessageType,
		"success", resp.Success,
		"duration_ms", resp.DurationMs,
		"remote", req.RemoteAddr,
	}

	if req.CorrelationID != "" {
		attrs = append(attrs, "correlation_id", req.CorrelationID)
	}

	if req.RequestID != "" {
		attrs = append(attrs, "request_id", req.RequestID)
	}

	if resp.Error != "" {
		attrs = append(attrs, "error", resp.Error)
	}

	for k, v := range resp.Metadata {
		attrs = append(attrs, k, v)
	}

	level := slog.LevelInfo
	message := "rpc request completed"

	if !resp.Success {
		level = slog.LevelError
		message = "rpc request failed"
	}

	logger.Log(nil, level, message, attrs...)
}

// RPCMiddleware wraps an RPC handler function with logging.
// It logs the request when it arrives and the response when it completes.
type RPCMiddleware struct {
	logger *slog.Logger
}

// NewRPCMiddleware creates a new RPC logging middleware.
func NewRPCMiddleware(logger *slog.Logger) *RPCMiddleware {
	return &RPCMiddleware{
		logger: logger,
	}
}

// Handler wraps a function that processes an RPC request.
// It logs the request and response automatically.
func (m *RPCMiddleware) Handler(req *RPCRequest, handler func() error) error {
	start := time.Now()

	// Log incoming request
	LogRPCRequest(m.logger, req)

	// Execute handler
	err := handler()

	// Calculate duration
	duration := time.Since(start).Milliseconds()

	// Log response
	resp := &RPCResponse{
		Success:    err == nil,
		DurationMs: duration,
	}

	if err != nil {
		resp.Error = err.Error()
	}

	LogRPCResponse(m.logger, req, resp)

	return err
}

// HandlerWithMetadata wraps a function that processes an RPC request and returns metadata.
// It logs the request and response with the returned metadata.
func (m *RPCMiddleware) HandlerWithMetadata(req *RPCRequest, handler func() (map[string]interface{}, error)) (map[string]interface{}, error) {
	start := time.Now()

	// Log incoming request
	LogRPCRequest(m.logger, req)

	// Execute handler
	metadata, err := handler()

	// Calculate duration
	duration := time.Since(start).Milliseconds()

	// Log response
	resp := &RPCResponse{
		Success:    err == nil,
		DurationMs: duration,
		Metadata:   metadata,
	}

	if err != nil {
		resp.Error = err.Error()
	}

	LogRPCResponse(m.logger, req, resp)

	return metadata, err
}
