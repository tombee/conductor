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
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

// Handler is a function that handles an RPC request.
type Handler func(ctx context.Context, req *Message) (*Message, error)

// StreamHandler is a function that handles a streaming RPC request.
// It should send messages to the StreamWriter and return when complete.
type StreamHandler func(ctx context.Context, req *Message, writer *StreamWriter) error

// Registry manages RPC method handlers.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string]Handler
	streams  map[string]StreamHandler
}

// NewRegistry creates a new handler registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
		streams:  make(map[string]StreamHandler),
	}
}

// Register registers a handler for the given method.
func (r *Registry) Register(method string, handler Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handlers[method] = handler
}

// RegisterStream registers a streaming handler for the given method.
func (r *Registry) RegisterStream(method string, handler StreamHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams[method] = handler
}

// Handle dispatches a request to the appropriate handler.
func (r *Registry) Handle(ctx context.Context, req *Message) (*Message, error) {
	r.mu.RLock()
	handler, hasHandler := r.handlers[req.Method]
	_, hasStream := r.streams[req.Method]
	r.mu.RUnlock()

	if !hasHandler && !hasStream {
		return nil, fmt.Errorf("%w: %s", ErrMethodNotFound, req.Method)
	}

	if hasStream {
		// Streaming methods cannot be called via Handle
		return nil, fmt.Errorf("method %s requires streaming", req.Method)
	}

	return handler(ctx, req)
}

// HandleStream dispatches a streaming request to the appropriate handler.
func (r *Registry) HandleStream(ctx context.Context, req *Message, writer *StreamWriter) error {
	r.mu.RLock()
	handler, ok := r.streams[req.Method]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrMethodNotFound, req.Method)
	}

	return handler(ctx, req, writer)
}

// HasMethod checks if a method is registered (either regular or streaming).
func (r *Registry) HasMethod(method string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, hasHandler := r.handlers[method]
	_, hasStream := r.streams[method]
	return hasHandler || hasStream
}

// StreamWriter provides methods to send streaming messages to the client.
type StreamWriter struct {
	conn          *websocket.Conn
	correlationID string
	streamID      string
	mu            sync.Mutex
}

// NewStreamWriter creates a new StreamWriter.
func NewStreamWriter(conn *websocket.Conn, correlationID, streamID string) *StreamWriter {
	return &StreamWriter{
		conn:          conn,
		correlationID: correlationID,
		streamID:      streamID,
	}
}

// Send sends a streaming message to the client.
func (w *StreamWriter) Send(data interface{}) error {
	msg, err := NewStreamMessage(w.correlationID, w.streamID, data, false)
	if err != nil {
		return fmt.Errorf("failed to create stream message: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.conn.WriteJSON(msg)
}

// Done sends a final streaming message indicating the stream is complete.
func (w *StreamWriter) Done() error {
	msg, err := NewStreamMessage(w.correlationID, w.streamID, nil, true)
	if err != nil {
		return fmt.Errorf("failed to create stream done message: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.conn.WriteJSON(msg)
}

// Session represents a connection session with state management.
type Session struct {
	mu             sync.RWMutex
	id             string
	conn           *websocket.Conn
	registry       *Registry
	authenticated  bool
	protocolVersion string
	metadata       map[string]interface{}
}

// NewSession creates a new session for a connection.
func NewSession(id string, conn *websocket.Conn, registry *Registry) *Session {
	return &Session{
		id:       id,
		conn:     conn,
		registry: registry,
		metadata: make(map[string]interface{}),
	}
}

// ID returns the session ID.
func (s *Session) ID() string {
	return s.id
}

// SetProtocolVersion sets the negotiated protocol version.
func (s *Session) SetProtocolVersion(version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.protocolVersion = version
}

// ProtocolVersion returns the negotiated protocol version.
func (s *Session) ProtocolVersion() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.protocolVersion
}

// SetMetadata sets a metadata value for the session.
func (s *Session) SetMetadata(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[key] = value
}

// GetMetadata retrieves a metadata value from the session.
func (s *Session) GetMetadata(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.metadata[key]
	return val, ok
}
