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

package rpc

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		params  interface{}
		wantErr bool
	}{
		{
			name:    "simple request",
			method:  "test.method",
			params:  map[string]string{"key": "value"},
			wantErr: false,
		},
		{
			name:    "request with nil params",
			method:  "test.method",
			params:  nil,
			wantErr: false,
		},
		{
			name:    "request with complex params",
			method:  "complex.method",
			params:  map[string]interface{}{"nested": map[string]int{"count": 42}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewRequest(tt.method, tt.params)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if msg.Type != MessageTypeRequest {
					t.Errorf("expected type %s, got %s", MessageTypeRequest, msg.Type)
				}

				if msg.Method != tt.method {
					t.Errorf("expected method %s, got %s", tt.method, msg.Method)
				}

				if msg.CorrelationID == "" {
					t.Error("expected correlation ID, got empty string")
				}

				if tt.params != nil && msg.Params == nil {
					t.Error("expected params, got nil")
				}
			}
		})
	}
}

func TestNewResponse(t *testing.T) {
	correlationID := "test-correlation-123"

	tests := []struct {
		name    string
		result  interface{}
		wantErr bool
	}{
		{
			name:    "simple response",
			result:  map[string]string{"status": "ok"},
			wantErr: false,
		},
		{
			name:    "response with nil result",
			result:  nil,
			wantErr: false,
		},
		{
			name:    "response with complex result",
			result:  map[string]interface{}{"data": []int{1, 2, 3}},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewResponse(correlationID, tt.result)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if msg.Type != MessageTypeResponse {
					t.Errorf("expected type %s, got %s", MessageTypeResponse, msg.Type)
				}

				if msg.CorrelationID != correlationID {
					t.Errorf("expected correlationID %s, got %s", correlationID, msg.CorrelationID)
				}

				if tt.result != nil && msg.Result == nil {
					t.Error("expected result, got nil")
				}
			}
		})
	}
}

func TestNewErrorResponse(t *testing.T) {
	correlationID := "test-correlation-456"
	code := "TEST_ERROR"
	message := "Test error message"
	details := map[string]interface{}{"key": "value"}

	msg := NewErrorResponse(correlationID, code, message, details)

	if msg.Type != MessageTypeError {
		t.Errorf("expected type %s, got %s", MessageTypeError, msg.Type)
	}

	if msg.CorrelationID != correlationID {
		t.Errorf("expected correlationID %s, got %s", correlationID, msg.CorrelationID)
	}

	if msg.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if msg.Error.Code != code {
		t.Errorf("expected error code %s, got %s", code, msg.Error.Code)
	}

	if msg.Error.Message != message {
		t.Errorf("expected error message %s, got %s", message, msg.Error.Message)
	}

	if msg.Error.Details == nil {
		t.Error("expected error details, got nil")
	}
}

func TestNewStreamMessage(t *testing.T) {
	correlationID := "test-correlation-789"
	streamID := "stream-123"

	tests := []struct {
		name    string
		data    interface{}
		done    bool
		wantErr bool
	}{
		{
			name:    "stream message with data",
			data:    map[string]string{"chunk": "data"},
			done:    false,
			wantErr: false,
		},
		{
			name:    "stream message done",
			data:    nil,
			done:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewStreamMessage(correlationID, streamID, tt.data, tt.done)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewStreamMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if msg.Type != MessageTypeStream {
					t.Errorf("expected type %s, got %s", MessageTypeStream, msg.Type)
				}

				if msg.CorrelationID != correlationID {
					t.Errorf("expected correlationID %s, got %s", correlationID, msg.CorrelationID)
				}

				if msg.StreamID != streamID {
					t.Errorf("expected streamID %s, got %s", streamID, msg.StreamID)
				}

				if msg.StreamDone != tt.done {
					t.Errorf("expected done %v, got %v", tt.done, msg.StreamDone)
				}
			}
		})
	}
}

func TestNewHandshake(t *testing.T) {
	msg := NewHandshake()

	if msg.Type != MessageTypeHandshake {
		t.Errorf("expected type %s, got %s", MessageTypeHandshake, msg.Type)
	}

	if msg.Version != ProtocolVersion {
		t.Errorf("expected version %s, got %s", ProtocolVersion, msg.Version)
	}

	if msg.CorrelationID == "" {
		t.Error("expected correlation ID, got empty string")
	}
}

func TestMessage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		msg     *Message
		wantErr error
	}{
		{
			name: "valid request",
			msg: &Message{
				Type:          MessageTypeRequest,
				CorrelationID: "test-123",
				Method:        "test.method",
			},
			wantErr: nil,
		},
		{
			name: "missing correlation ID",
			msg: &Message{
				Type:   MessageTypeRequest,
				Method: "test.method",
			},
			wantErr: ErrMissingCorrelationID,
		},
		{
			name: "request missing method",
			msg: &Message{
				Type:          MessageTypeRequest,
				CorrelationID: "test-123",
			},
			wantErr: ErrInvalidMessage,
		},
		{
			name: "handshake missing version",
			msg: &Message{
				Type:          MessageTypeHandshake,
				CorrelationID: "test-123",
			},
			wantErr: ErrInvalidMessage,
		},
		{
			name: "stream missing stream ID",
			msg: &Message{
				Type:          MessageTypeStream,
				CorrelationID: "test-123",
			},
			wantErr: ErrInvalidMessage,
		},
		{
			name: "valid response",
			msg: &Message{
				Type:          MessageTypeResponse,
				CorrelationID: "test-123",
			},
			wantErr: nil,
		},
		{
			name: "valid error",
			msg: &Message{
				Type:          MessageTypeError,
				CorrelationID: "test-123",
			},
			wantErr: nil,
		},
		{
			name: "unknown message type",
			msg: &Message{
				Type:          "unknown",
				CorrelationID: "test-123",
			},
			wantErr: ErrInvalidMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() expected error %v, got nil", tt.wantErr)
				} else if err.Error() != tt.wantErr.Error() {
					// Check if error wraps expected error
					if !errors.Is(err, tt.wantErr) && !contains(err.Error(), tt.wantErr.Error()) {
						t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
					}
				}
			}
		})
	}
}

func TestMessage_UnmarshalParams(t *testing.T) {
	type testParams struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	params := testParams{Name: "test", Count: 42}
	msg, err := NewRequest("test.method", params)
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	var result testParams
	if err := msg.UnmarshalParams(&result); err != nil {
		t.Fatalf("UnmarshalParams() failed: %v", err)
	}

	if result.Name != params.Name {
		t.Errorf("expected name %s, got %s", params.Name, result.Name)
	}

	if result.Count != params.Count {
		t.Errorf("expected count %d, got %d", params.Count, result.Count)
	}
}

func TestMessage_UnmarshalResult(t *testing.T) {
	type testResult struct {
		Status string `json:"status"`
		Value  int    `json:"value"`
	}

	result := testResult{Status: "ok", Value: 100}
	msg, err := NewResponse("test-123", result)
	if err != nil {
		t.Fatalf("NewResponse() failed: %v", err)
	}

	var parsed testResult
	if err := msg.UnmarshalResult(&parsed); err != nil {
		t.Fatalf("UnmarshalResult() failed: %v", err)
	}

	if parsed.Status != result.Status {
		t.Errorf("expected status %s, got %s", result.Status, parsed.Status)
	}

	if parsed.Value != result.Value {
		t.Errorf("expected value %d, got %d", result.Value, parsed.Value)
	}
}

func TestMessage_Marshal(t *testing.T) {
	msg, err := NewRequest("test.method", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	data, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("expected marshaled data, got empty")
	}

	// Verify it's valid JSON
	var check map[string]interface{}
	if err := json.Unmarshal(data, &check); err != nil {
		t.Errorf("Marshal() produced invalid JSON: %v", err)
	}
}

func TestParseMessage(t *testing.T) {
	validMsg, _ := NewRequest("test.method", map[string]string{"key": "value"})
	validData, _ := validMsg.Marshal()

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "valid message",
			data:    validData,
			wantErr: false,
		},
		{
			name:    "invalid JSON",
			data:    []byte("not json"),
			wantErr: true,
		},
		{
			name:    "missing correlation ID",
			data:    []byte(`{"type":"request","method":"test"}`),
			wantErr: true,
		},
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := ParseMessage(tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && msg == nil {
				t.Error("ParseMessage() returned nil message")
			}
		})
	}
}

func TestIsVersionSupported(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{ProtocolVersion, true},
		{MinProtocolVersion, true},
		{"0.9", false},
		{"2.0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			if got := IsVersionSupported(tt.version); got != tt.want {
				t.Errorf("IsVersionSupported(%s) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}()))
}
