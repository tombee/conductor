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

package mcp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestDebugFormatter_FormatRequest(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: false,
	})

	params := map[string]interface{}{
		"arg1": "value1",
		"arg2": 42,
	}

	err := formatter.FormatRequest("tools/call", params)
	if err != nil {
		t.Fatalf("FormatRequest failed: %v", err)
	}

	output := buf.String()

	// Verify output contains expected components
	if !strings.Contains(output, "[test-server]") {
		t.Error("output should contain server name")
	}
	if !strings.Contains(output, "REQUEST") {
		t.Error("output should contain REQUEST")
	}
	if !strings.Contains(output, "tools/call") {
		t.Error("output should contain method name")
	}
	if !strings.Contains(output, "value1") {
		t.Error("output should contain param value")
	}
}

func TestDebugFormatter_FormatResponse(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: false,
	})

	result := map[string]interface{}{
		"status": "success",
		"data":   "result data",
	}

	err := formatter.FormatResponse("tools/call", result)
	if err != nil {
		t.Fatalf("FormatResponse failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "RESPONSE") {
		t.Error("output should contain RESPONSE")
	}
	if !strings.Contains(output, "success") {
		t.Error("output should contain result data")
	}
}

func TestDebugFormatter_FormatError(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: false,
	})

	err := formatter.FormatError("tools/call", errors.New("something went wrong"))
	if err != nil {
		t.Fatalf("FormatError failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "ERROR") {
		t.Error("output should contain ERROR")
	}
	if !strings.Contains(output, "something went wrong") {
		t.Error("output should contain error message")
	}
}

func TestDebugFormatter_WithTimestamps(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: true,
	})

	err := formatter.FormatRequest("test/method", nil)
	if err != nil {
		t.Fatalf("FormatRequest failed: %v", err)
	}

	output := buf.String()

	// Should contain a timestamp in HH:MM:SS.mmm format
	if !strings.Contains(output, ":") {
		t.Error("output should contain timestamp")
	}
}

func TestDebugFormatter_WithoutServerName(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ShowTimestamps: false,
	})

	err := formatter.FormatRequest("test/method", nil)
	if err != nil {
		t.Fatalf("FormatRequest failed: %v", err)
	}

	output := buf.String()

	// Should not contain brackets
	if strings.Contains(output, "[") {
		t.Error("output should not contain server name brackets")
	}
}

func TestDebugFormatter_LogRawMessage(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: false,
	})

	err := formatter.LogRawMessage("SEND", `{"jsonrpc":"2.0","id":1,"method":"test"}`)
	if err != nil {
		t.Fatalf("LogRawMessage failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "RAW") {
		t.Error("output should contain RAW")
	}
	if !strings.Contains(output, "SEND") {
		t.Error("output should contain direction")
	}
	if !strings.Contains(output, "jsonrpc") {
		t.Error("output should contain raw message")
	}
}

func TestDebugFormatter_ParseAndFormat_Request(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: false,
	})

	jsonMsg := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arg":"value"}}`

	err := formatter.ParseAndFormat(jsonMsg, "SEND")
	if err != nil {
		t.Fatalf("ParseAndFormat failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "REQUEST") {
		t.Error("output should contain REQUEST")
	}
	if !strings.Contains(output, "tools/call") {
		t.Error("output should contain method")
	}
}

func TestDebugFormatter_ParseAndFormat_Response(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: false,
	})

	jsonMsg := `{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}`

	err := formatter.ParseAndFormat(jsonMsg, "RECV")
	if err != nil {
		t.Fatalf("ParseAndFormat failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "RESPONSE") {
		t.Error("output should contain RESPONSE")
	}
	if !strings.Contains(output, "ok") {
		t.Error("output should contain result")
	}
}

func TestDebugFormatter_ParseAndFormat_Error(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: false,
	})

	jsonMsg := `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}`

	err := formatter.ParseAndFormat(jsonMsg, "RECV")
	if err != nil {
		t.Fatalf("ParseAndFormat failed: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "ERROR") {
		t.Error("output should contain ERROR")
	}
	if !strings.Contains(output, "Method not found") {
		t.Error("output should contain error message")
	}
}

func TestDebugFormatter_ParseAndFormat_InvalidJSON(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewDebugFormatter(DebugFormatterConfig{
		Writer:         &buf,
		ServerName:     "test-server",
		ShowTimestamps: false,
	})

	invalidJSON := `{invalid json}`

	err := formatter.ParseAndFormat(invalidJSON, "RECV")
	if err != nil {
		t.Fatalf("ParseAndFormat failed: %v", err)
	}

	output := buf.String()

	// Should fall back to raw message logging
	if !strings.Contains(output, "RAW") {
		t.Error("output should contain RAW for invalid JSON")
	}
	if !strings.Contains(output, "invalid json") {
		t.Error("output should contain the raw message")
	}
}
