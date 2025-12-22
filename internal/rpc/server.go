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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	// ErrServerClosed is returned when operations are attempted on a closed server.
	ErrServerClosed = errors.New("rpc: server closed")

	// ErrNoPortAvailable is returned when no port in the configured range is available.
	ErrNoPortAvailable = errors.New("rpc: no port available in range")

	// ErrShutdownTimeout is returned when graceful shutdown exceeds the timeout.
	ErrShutdownTimeout = errors.New("rpc: shutdown timeout exceeded")
)

// ServerConfig configures the RPC server.
type ServerConfig struct {
	// PortRange specifies the range of ports to try (inclusive).
	// Default: [9876, 9899]
	PortRange [2]int

	// ShutdownTimeout is the maximum duration to wait for graceful shutdown.
	// Default: 5 seconds
	ShutdownTimeout time.Duration

	// AuthToken is the required token for WebSocket connections.
	// If empty, authentication is disabled.
	AuthToken string

	// Logger is the structured logger for server events.
	// If nil, a default logger is used.
	Logger *slog.Logger
}

// DefaultConfig returns a ServerConfig with sensible defaults.
func DefaultConfig() *ServerConfig {
	return &ServerConfig{
		PortRange:       [2]int{9876, 9899},
		ShutdownTimeout: 5 * time.Second,
		Logger:          slog.Default(),
	}
}

// Server is an RPC server that handles WebSocket connections.
type Server struct {
	config   *ServerConfig
	logger   *slog.Logger
	upgrader websocket.Upgrader

	mu         sync.RWMutex
	httpServer *http.Server
	listener   net.Listener
	port       int
	closed     bool

	// Authentication
	tokenValidator *TokenValidator

	// Connection tracking
	connMu      sync.RWMutex
	connections map[*websocket.Conn]struct{}

	// Shutdown coordination
	shutdownOnce sync.Once
	shutdownCh   chan struct{}
}

// NewServer creates a new RPC server with the given configuration.
func NewServer(config *ServerConfig) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 5 * time.Second
	}

	if config.PortRange[0] == 0 {
		config.PortRange = [2]int{9876, 9899}
	}

	s := &Server{
		config: config,
		logger: config.Logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for localhost connections
				// TODO: Restrict in production
				return true
			},
		},
		connections: make(map[*websocket.Conn]struct{}),
		shutdownCh:  make(chan struct{}),
	}

	// Initialize token validator if auth is enabled
	if config.AuthToken != "" {
		s.tokenValidator = NewTokenValidator(config.AuthToken)
	}

	return s
}

// Start starts the RPC server and finds an available port in the configured range.
// It returns the port number on which the server is listening, or an error.
func (s *Server) Start(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return 0, ErrServerClosed
	}

	if s.httpServer != nil {
		return s.port, nil // Already started
	}

	// Find an available port
	port, listener, err := s.findAvailablePort()
	if err != nil {
		return 0, err
	}

	s.listener = listener
	s.port = port

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ws", s.handleWebSocket)

	s.httpServer = &http.Server{
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
		// WriteTimeout intentionally omitted to support long-lived WebSocket connections
	}

	// Start HTTP server in background
	go func() {
		s.logger.Info("rpc server starting",
			"port", port,
			"portRange", s.config.PortRange)

		if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("rpc server error", "error", err)
		}
	}()

	// Output port for Electron to discover
	fmt.Printf("CONDUCTOR_BACKEND_PORT=%d\n", port)

	s.logger.Info("rpc server started", "port", port)
	return port, nil
}

// findAvailablePort attempts to find an available port in the configured range.
func (s *Server) findAvailablePort() (int, net.Listener, error) {
	startPort := s.config.PortRange[0]
	endPort := s.config.PortRange[1]

	for port := startPort; port <= endPort; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			return port, listener, nil
		}
		s.logger.Debug("port unavailable", "port", port, "error", err)
	}

	return 0, nil, ErrNoPortAvailable
}

// Port returns the port the server is listening on, or 0 if not started.
func (s *Server) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}

// handleHealth handles health check requests.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()

	status := "ready"
	httpStatus := http.StatusOK

	if closed {
		status = "error"
		httpStatus = http.StatusServiceUnavailable
	}

	response := map[string]string{
		"status":  status,
		"version": "0.1.0", // TODO: Read from build metadata
		"message": "Conductor RPC server",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(response)
}

// handleWebSocket handles WebSocket upgrade requests.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()

	if closed {
		http.Error(w, "Server shutting down", http.StatusServiceUnavailable)
		return
	}

	// Check authentication token if configured
	if s.tokenValidator != nil {
		token := r.Header.Get("X-Auth-Token")
		if err := s.tokenValidator.Validate(token, r.RemoteAddr); err != nil {
			// Log auth failure without leaking the token
			if errors.Is(err, ErrRateLimitExceeded) {
				s.logger.Warn("authentication rate limit exceeded",
					"remote", r.RemoteAddr,
					"error", err)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			} else {
				s.logger.Warn("authentication failed",
					"remote", r.RemoteAddr,
					"hasToken", token != "",
					"error", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			return
		}
	}

	// Upgrade to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "error", err, "remote", r.RemoteAddr)
		return
	}

	s.logger.Info("websocket connection established", "remote", r.RemoteAddr)

	// Track connection
	s.connMu.Lock()
	s.connections[conn] = struct{}{}
	s.connMu.Unlock()

	// Handle connection in background
	go s.handleConnection(conn)
}

// handleConnection manages a WebSocket connection lifecycle.
func (s *Server) handleConnection(conn *websocket.Conn) {
	defer func() {
		// Remove from tracking
		s.connMu.Lock()
		delete(s.connections, conn)
		s.connMu.Unlock()

		conn.Close()
		s.logger.Info("websocket connection closed", "remote", conn.RemoteAddr())
	}()

	// Set up ping/pong for connection health
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Start ping ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Read messages (placeholder for T003)
	for {
		select {
		case <-s.shutdownCh:
			// Server shutting down
			return
		case <-pingTicker.C:
			// Send ping
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				s.logger.Debug("ping failed", "error", err)
				return
			}
		default:
			// Read next message
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					s.logger.Warn("websocket read error", "error", err)
				}
				return
			}

			// Handle message (placeholder - full implementation in T003)
			s.logger.Debug("received message", "type", messageType, "size", len(message))
		}
	}
}

// Shutdown gracefully shuts down the server, closing all connections.
// It waits up to the configured ShutdownTimeout for connections to close.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return ErrServerClosed
	}
	s.closed = true
	s.mu.Unlock()

	var shutdownErr error
	s.shutdownOnce.Do(func() {
		close(s.shutdownCh)

		s.logger.Info("rpc server shutting down")

		// Create shutdown context with timeout
		shutdownCtx, cancel := context.WithTimeout(ctx, s.config.ShutdownTimeout)
		defer cancel()

		// Close all WebSocket connections
		s.connMu.Lock()
		for conn := range s.connections {
			conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"),
				time.Now().Add(time.Second),
			)
			conn.Close()
		}
		s.connMu.Unlock()

		// Shutdown HTTP server
		if s.httpServer != nil {
			if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					shutdownErr = ErrShutdownTimeout
				} else {
					shutdownErr = err
				}
			}
		}

		// Clean up token validator
		if s.tokenValidator != nil {
			s.tokenValidator.Close()
		}

		s.logger.Info("rpc server shutdown complete")
	})

	return shutdownErr
}

// Close immediately closes the server without waiting for connections to close.
func (s *Server) Close() error {
	return s.Shutdown(context.Background())
}
