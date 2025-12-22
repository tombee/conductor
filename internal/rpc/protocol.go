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
	"fmt"

	"github.com/google/uuid"
)

const (
	// Protocol version for version negotiation
	ProtocolVersion = "1.0"

	// Minimum supported protocol version
	MinProtocolVersion = "1.0"
)

var (
	// ErrInvalidMessage is returned when a message cannot be parsed.
	ErrInvalidMessage = errors.New("rpc: invalid message format")

	// ErrMissingCorrelationID is returned when a message lacks a correlation ID.
	ErrMissingCorrelationID = errors.New("rpc: missing correlation ID")

	// ErrUnsupportedVersion is returned when protocol version negotiation fails.
	ErrUnsupportedVersion = errors.New("rpc: unsupported protocol version")

	// ErrMethodNotFound is returned when the requested method doesn't exist.
	ErrMethodNotFound = errors.New("rpc: method not found")
)

// MessageType identifies the type of RPC message.
type MessageType string

const (
	// MessageTypeRequest is a request from client to server.
	MessageTypeRequest MessageType = "request"

	// MessageTypeResponse is a response from server to client.
	MessageTypeResponse MessageType = "response"

	// MessageTypeStream is a streaming message (partial response).
	MessageTypeStream MessageType = "stream"

	// MessageTypeError is an error response.
	MessageTypeError MessageType = "error"

	// MessageTypeHandshake is a protocol version handshake message.
	MessageTypeHandshake MessageType = "handshake"
)

// Message is the base structure for all RPC messages.
type Message struct {
	// Type identifies the message type
	Type MessageType `json:"type"`

	// CorrelationID links requests with responses
	CorrelationID string `json:"correlationId"`

	// Version is the protocol version (used in handshake)
	Version string `json:"version,omitempty"`

	// Method is the RPC method to invoke (request only)
	Method string `json:"method,omitempty"`

	// Params contains method parameters (request only)
	Params json.RawMessage `json:"params,omitempty"`

	// Result contains the response data (response only)
	Result json.RawMessage `json:"result,omitempty"`

	// Error contains error information (error only)
	Error *ErrorResponse `json:"error,omitempty"`

	// StreamID identifies a stream session (stream only)
	StreamID string `json:"streamId,omitempty"`

	// StreamDone indicates the end of a stream (stream only)
	StreamDone bool `json:"streamDone,omitempty"`
}

// ErrorResponse contains structured error information.
type ErrorResponse struct {
	// Code is a machine-readable error code
	Code string `json:"code"`

	// Message is a human-readable error message
	Message string `json:"message"`

	// Details contains additional error context
	Details map[string]interface{} `json:"details,omitempty"`
}

// NewRequest creates a new request message with a generated correlation ID.
func NewRequest(method string, params interface{}) (*Message, error) {
	correlationID := uuid.New().String()

	var paramsJSON json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal params: %w", err)
		}
		paramsJSON = data
	}

	return &Message{
		Type:          MessageTypeRequest,
		CorrelationID: correlationID,
		Method:        method,
		Params:        paramsJSON,
	}, nil
}

// NewResponse creates a response message for the given request.
func NewResponse(correlationID string, result interface{}) (*Message, error) {
	var resultJSON json.RawMessage
	if result != nil {
		data, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
		resultJSON = data
	}

	return &Message{
		Type:          MessageTypeResponse,
		CorrelationID: correlationID,
		Result:        resultJSON,
	}, nil
}

// NewErrorResponse creates an error response message.
func NewErrorResponse(correlationID, code, message string, details map[string]interface{}) *Message {
	return &Message{
		Type:          MessageTypeError,
		CorrelationID: correlationID,
		Error: &ErrorResponse{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// NewStreamMessage creates a stream message.
func NewStreamMessage(correlationID, streamID string, data interface{}, done bool) (*Message, error) {
	var resultJSON json.RawMessage
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal stream data: %w", err)
		}
		resultJSON = bytes
	}

	return &Message{
		Type:          MessageTypeStream,
		CorrelationID: correlationID,
		StreamID:      streamID,
		Result:        resultJSON,
		StreamDone:    done,
	}, nil
}

// NewHandshake creates a handshake message for protocol version negotiation.
func NewHandshake() *Message {
	return &Message{
		Type:          MessageTypeHandshake,
		CorrelationID: uuid.New().String(),
		Version:       ProtocolVersion,
	}
}

// Validate checks if the message is well-formed.
func (m *Message) Validate() error {
	if m.CorrelationID == "" {
		return ErrMissingCorrelationID
	}

	switch m.Type {
	case MessageTypeRequest:
		if m.Method == "" {
			return fmt.Errorf("%w: missing method", ErrInvalidMessage)
		}
	case MessageTypeHandshake:
		if m.Version == "" {
			return fmt.Errorf("%w: missing version", ErrInvalidMessage)
		}
	case MessageTypeStream:
		if m.StreamID == "" {
			return fmt.Errorf("%w: missing stream ID", ErrInvalidMessage)
		}
	case MessageTypeResponse, MessageTypeError:
		// Valid as-is
	default:
		return fmt.Errorf("%w: unknown message type %q", ErrInvalidMessage, m.Type)
	}

	return nil
}

// UnmarshalParams unmarshals the params field into the given value.
func (m *Message) UnmarshalParams(v interface{}) error {
	if m.Params == nil {
		return nil
	}
	return json.Unmarshal(m.Params, v)
}

// UnmarshalResult unmarshals the result field into the given value.
func (m *Message) UnmarshalResult(v interface{}) error {
	if m.Result == nil {
		return nil
	}
	return json.Unmarshal(m.Result, v)
}

// Marshal encodes the message to JSON.
func (m *Message) Marshal() ([]byte, error) {
	return json.Marshal(m)
}

// ParseMessage parses a JSON message.
func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMessage, err)
	}

	if err := msg.Validate(); err != nil {
		return nil, err
	}

	return &msg, nil
}

// IsVersionSupported checks if a protocol version is supported.
func IsVersionSupported(version string) bool {
	// For now, only support exact version match
	// Future: Implement proper version comparison
	return version == ProtocolVersion || version == MinProtocolVersion
}
