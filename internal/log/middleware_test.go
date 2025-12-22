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
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestLogRPCRequest(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)

	req := &RPCRequest{
		MessageType:   "execute_tool",
		CorrelationID: "correlation-123",
		RequestID:     "request-456",
		RemoteAddr:    "127.0.0.1:54321",
		Metadata: map[string]interface{}{
			"tool": "bash",
		},
	}

	LogRPCRequest(logger, req)

	output := buf.String()

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	// Check for expected fields
	if logEntry["event"] != "rpc_request" {
		t.Errorf("expected event to be 'rpc_request', got: %v", logEntry["event"])
	}

	if logEntry["message_type"] != "execute_tool" {
		t.Errorf("expected message_type to be 'execute_tool', got: %v", logEntry["message_type"])
	}

	if logEntry["correlation_id"] != "correlation-123" {
		t.Errorf("expected correlation_id to be 'correlation-123', got: %v", logEntry["correlation_id"])
	}

	if logEntry["request_id"] != "request-456" {
		t.Errorf("expected request_id to be 'request-456', got: %v", logEntry["request_id"])
	}

	if logEntry["remote"] != "127.0.0.1:54321" {
		t.Errorf("expected remote to be '127.0.0.1:54321', got: %v", logEntry["remote"])
	}

	if logEntry["tool"] != "bash" {
		t.Errorf("expected tool to be 'bash', got: %v", logEntry["tool"])
	}
}

func TestLogRPCRequest_MinimalFields(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)

	req := &RPCRequest{
		MessageType: "ping",
		RemoteAddr:  "127.0.0.1:54321",
	}

	LogRPCRequest(logger, req)

	output := buf.String()

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	// Should not have correlation_id or request_id
	if _, ok := logEntry["correlation_id"]; ok {
		t.Errorf("expected no correlation_id field for minimal request")
	}

	if _, ok := logEntry["request_id"]; ok {
		t.Errorf("expected no request_id field for minimal request")
	}
}

func TestLogRPCResponse_Success(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)

	req := &RPCRequest{
		MessageType:   "execute_tool",
		CorrelationID: "correlation-123",
		RequestID:     "request-456",
		RemoteAddr:    "127.0.0.1:54321",
	}

	resp := &RPCResponse{
		Success:    true,
		DurationMs: 150,
		Metadata: map[string]interface{}{
			"exit_code": 0,
		},
	}

	LogRPCResponse(logger, req, resp)

	output := buf.String()

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	// Check for expected fields
	if logEntry["event"] != "rpc_response" {
		t.Errorf("expected event to be 'rpc_response', got: %v", logEntry["event"])
	}

	if logEntry["success"] != true {
		t.Errorf("expected success to be true, got: %v", logEntry["success"])
	}

	if logEntry["duration_ms"] != float64(150) {
		t.Errorf("expected duration_ms to be 150, got: %v", logEntry["duration_ms"])
	}

	if logEntry["level"] != "INFO" {
		t.Errorf("expected level to be 'INFO', got: %v", logEntry["level"])
	}

	if logEntry["msg"] != "rpc request completed" {
		t.Errorf("expected msg to be 'rpc request completed', got: %v", logEntry["msg"])
	}

	if logEntry["exit_code"] != float64(0) {
		t.Errorf("expected exit_code to be 0, got: %v", logEntry["exit_code"])
	}

	// Should not have error field for successful response
	if _, ok := logEntry["error"]; ok {
		t.Errorf("expected no error field for successful response")
	}
}

func TestLogRPCResponse_Error(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)

	req := &RPCRequest{
		MessageType:   "execute_tool",
		CorrelationID: "correlation-123",
		RequestID:     "request-456",
		RemoteAddr:    "127.0.0.1:54321",
	}

	resp := &RPCResponse{
		Success:    false,
		Error:      "command failed",
		DurationMs: 50,
	}

	LogRPCResponse(logger, req, resp)

	output := buf.String()

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal([]byte(output), &logEntry); err != nil {
		t.Fatalf("expected valid JSON output: %v", err)
	}

	// Check for expected fields
	if logEntry["success"] != false {
		t.Errorf("expected success to be false, got: %v", logEntry["success"])
	}

	if logEntry["error"] != "command failed" {
		t.Errorf("expected error to be 'command failed', got: %v", logEntry["error"])
	}

	if logEntry["level"] != "ERROR" {
		t.Errorf("expected level to be 'ERROR', got: %v", logEntry["level"])
	}

	if logEntry["msg"] != "rpc request failed" {
		t.Errorf("expected msg to be 'rpc request failed', got: %v", logEntry["msg"])
	}
}

func TestRPCMiddleware_Handler_Success(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	middleware := NewRPCMiddleware(logger)

	req := &RPCRequest{
		MessageType:   "ping",
		CorrelationID: "correlation-123",
		RemoteAddr:    "127.0.0.1:54321",
	}

	handlerCalled := false
	err := middleware.Handler(req, func() error {
		handlerCalled = true
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if !handlerCalled {
		t.Errorf("expected handler to be called")
	}

	output := buf.String()

	// Should have two log entries: request and response
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d: %s", len(lines), output)
	}

	// Check request log
	var requestLog map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &requestLog); err != nil {
		t.Fatalf("expected valid JSON for request log: %v", err)
	}

	if requestLog["event"] != "rpc_request" {
		t.Errorf("expected first log to be rpc_request, got: %v", requestLog["event"])
	}

	// Check response log
	var responseLog map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &responseLog); err != nil {
		t.Fatalf("expected valid JSON for response log: %v", err)
	}

	if responseLog["event"] != "rpc_response" {
		t.Errorf("expected second log to be rpc_response, got: %v", responseLog["event"])
	}

	if responseLog["success"] != true {
		t.Errorf("expected success to be true, got: %v", responseLog["success"])
	}

	// Should have duration_ms
	if _, ok := responseLog["duration_ms"]; !ok {
		t.Errorf("expected duration_ms to be present")
	}
}

func TestRPCMiddleware_Handler_Error(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	middleware := NewRPCMiddleware(logger)

	req := &RPCRequest{
		MessageType: "execute_tool",
		RemoteAddr:  "127.0.0.1:54321",
	}

	testErr := errors.New("handler error")
	err := middleware.Handler(req, func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("expected error to be returned, got: %v", err)
	}

	output := buf.String()

	// Should have two log entries: request and response
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}

	// Check response log
	var responseLog map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &responseLog); err != nil {
		t.Fatalf("expected valid JSON for response log: %v", err)
	}

	if responseLog["success"] != false {
		t.Errorf("expected success to be false, got: %v", responseLog["success"])
	}

	if responseLog["error"] != "handler error" {
		t.Errorf("expected error to be 'handler error', got: %v", responseLog["error"])
	}

	if responseLog["level"] != "ERROR" {
		t.Errorf("expected level to be ERROR, got: %v", responseLog["level"])
	}
}

func TestRPCMiddleware_HandlerWithMetadata_Success(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	middleware := NewRPCMiddleware(logger)

	req := &RPCRequest{
		MessageType: "execute_tool",
		RemoteAddr:  "127.0.0.1:54321",
	}

	expectedMetadata := map[string]interface{}{
		"exit_code": 0,
		"output":    "success",
	}

	metadata, err := middleware.HandlerWithMetadata(req, func() (map[string]interface{}, error) {
		return expectedMetadata, nil
	})

	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}

	if metadata["exit_code"] != 0 {
		t.Errorf("expected exit_code to be 0, got: %v", metadata["exit_code"])
	}

	output := buf.String()

	// Should have two log entries: request and response
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}

	// Check response log contains metadata
	var responseLog map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &responseLog); err != nil {
		t.Fatalf("expected valid JSON for response log: %v", err)
	}

	if responseLog["exit_code"] != float64(0) {
		t.Errorf("expected exit_code in log to be 0, got: %v", responseLog["exit_code"])
	}

	if responseLog["output"] != "success" {
		t.Errorf("expected output in log to be 'success', got: %v", responseLog["output"])
	}
}

func TestRPCMiddleware_HandlerWithMetadata_Error(t *testing.T) {
	var buf bytes.Buffer

	cfg := &Config{
		Level:  "info",
		Format: FormatJSON,
		Output: &buf,
	}

	logger := New(cfg)
	middleware := NewRPCMiddleware(logger)

	req := &RPCRequest{
		MessageType: "execute_tool",
		RemoteAddr:  "127.0.0.1:54321",
	}

	partialMetadata := map[string]interface{}{
		"exit_code": 1,
	}

	testErr := errors.New("command failed")

	metadata, err := middleware.HandlerWithMetadata(req, func() (map[string]interface{}, error) {
		return partialMetadata, testErr
	})

	if err != testErr {
		t.Errorf("expected error to be returned, got: %v", err)
	}

	if metadata["exit_code"] != 1 {
		t.Errorf("expected exit_code to be 1, got: %v", metadata["exit_code"])
	}

	output := buf.String()

	// Should have two log entries: request and response
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 log lines, got %d", len(lines))
	}

	// Check response log
	var responseLog map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &responseLog); err != nil {
		t.Fatalf("expected valid JSON for response log: %v", err)
	}

	if responseLog["success"] != false {
		t.Errorf("expected success to be false, got: %v", responseLog["success"])
	}

	if responseLog["error"] != "command failed" {
		t.Errorf("expected error to be 'command failed', got: %v", responseLog["error"])
	}

	// Should still have metadata
	if responseLog["exit_code"] != float64(1) {
		t.Errorf("expected exit_code in log to be 1, got: %v", responseLog["exit_code"])
	}
}

func TestNewRPCMiddleware(t *testing.T) {
	logger := New(nil)
	middleware := NewRPCMiddleware(logger)

	if middleware == nil {
		t.Errorf("expected non-nil middleware")
	}

	if middleware.logger != logger {
		t.Errorf("expected middleware to use provided logger")
	}
}
