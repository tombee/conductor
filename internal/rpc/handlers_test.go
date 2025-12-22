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
	"context"
	"errors"
	"testing"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()

	if registry == nil {
		t.Fatal("expected registry, got nil")
	}

	if registry.handlers == nil {
		t.Error("expected handlers map, got nil")
	}

	if registry.streams == nil {
		t.Error("expected streams map, got nil")
	}
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	handler := func(ctx context.Context, req *Message) (*Message, error) {
		return NewResponse(req.CorrelationID, map[string]string{"result": "ok"})
	}

	registry.Register("test.method", handler)

	if !registry.HasMethod("test.method") {
		t.Error("expected method to be registered")
	}
}

func TestRegistry_RegisterStream(t *testing.T) {
	registry := NewRegistry()

	streamHandler := func(ctx context.Context, req *Message, writer *StreamWriter) error {
		return writer.Send(map[string]string{"chunk": "data"})
	}

	registry.RegisterStream("test.stream", streamHandler)

	if !registry.HasMethod("test.stream") {
		t.Error("expected stream method to be registered")
	}
}

func TestRegistry_Handle(t *testing.T) {
	registry := NewRegistry()

	// Register a handler
	registry.Register("echo", func(ctx context.Context, req *Message) (*Message, error) {
		var params map[string]string
		if err := req.UnmarshalParams(&params); err != nil {
			return nil, err
		}
		return NewResponse(req.CorrelationID, params)
	})

	tests := []struct {
		name    string
		method  string
		wantErr error
	}{
		{
			name:    "registered method",
			method:  "echo",
			wantErr: nil,
		},
		{
			name:    "unregistered method",
			method:  "unknown",
			wantErr: ErrMethodNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewRequest(tt.method, map[string]string{"test": "value"})
			if err != nil {
				t.Fatalf("NewRequest() failed: %v", err)
			}

			ctx := context.Background()
			resp, err := registry.Handle(ctx, req)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("Handle() expected error, got nil")
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("Handle() error = %v, wantErr %v", err, tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("Handle() unexpected error: %v", err)
				}
				if resp == nil {
					t.Error("Handle() returned nil response")
				}
			}
		})
	}
}

func TestRegistry_HandleStream_NotFound(t *testing.T) {
	registry := NewRegistry()

	req, err := NewRequest("unknown.stream", nil)
	if err != nil {
		t.Fatalf("NewRequest() failed: %v", err)
	}

	ctx := context.Background()
	writer := &StreamWriter{}

	err = registry.HandleStream(ctx, req, writer)
	if err == nil {
		t.Error("HandleStream() expected error for unknown method")
	}

	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("HandleStream() error = %v, want %v", err, ErrMethodNotFound)
	}
}

func TestRegistry_HasMethod(t *testing.T) {
	registry := NewRegistry()

	registry.Register("regular", func(ctx context.Context, req *Message) (*Message, error) {
		return nil, nil
	})

	registry.RegisterStream("stream", func(ctx context.Context, req *Message, writer *StreamWriter) error {
		return nil
	})

	tests := []struct {
		method string
		want   bool
	}{
		{"regular", true},
		{"stream", true},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := registry.HasMethod(tt.method); got != tt.want {
				t.Errorf("HasMethod(%s) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestNewSession(t *testing.T) {
	registry := NewRegistry()
	session := NewSession("test-session-123", nil, registry)

	if session == nil {
		t.Fatal("expected session, got nil")
	}

	if session.ID() != "test-session-123" {
		t.Errorf("expected session ID test-session-123, got %s", session.ID())
	}

	if session.metadata == nil {
		t.Error("expected metadata map, got nil")
	}
}

func TestSession_ProtocolVersion(t *testing.T) {
	session := NewSession("test", nil, nil)

	// Initially empty
	if session.ProtocolVersion() != "" {
		t.Errorf("expected empty protocol version, got %s", session.ProtocolVersion())
	}

	// Set version
	session.SetProtocolVersion("1.0")
	if session.ProtocolVersion() != "1.0" {
		t.Errorf("expected protocol version 1.0, got %s", session.ProtocolVersion())
	}
}

func TestSession_Metadata(t *testing.T) {
	session := NewSession("test", nil, nil)

	// Get non-existent key
	val, ok := session.GetMetadata("key")
	if ok {
		t.Error("expected ok=false for non-existent key")
	}
	if val != nil {
		t.Errorf("expected nil value, got %v", val)
	}

	// Set metadata
	session.SetMetadata("user", "test-user")
	session.SetMetadata("count", 42)

	// Get metadata
	userVal, ok := session.GetMetadata("user")
	if !ok {
		t.Error("expected ok=true for existing key")
	}
	if userVal != "test-user" {
		t.Errorf("expected user=test-user, got %v", userVal)
	}

	countVal, ok := session.GetMetadata("count")
	if !ok {
		t.Error("expected ok=true for existing key")
	}
	if countVal != 42 {
		t.Errorf("expected count=42, got %v", countVal)
	}
}
