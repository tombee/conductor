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

// Package publicapi implements the public-facing API server for webhooks and triggers.
// The public API is separate from the control plane to enable secure deployments
// where management APIs remain private while webhooks are publicly accessible.
package publicapi

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/tombee/conductor/internal/config"
	internallog "github.com/tombee/conductor/internal/log"
)

// Server manages the lifecycle of the public API HTTP server.
type Server struct {
	cfg    config.PublicAPIConfig
	logger *slog.Logger
	server *http.Server

	mu sync.RWMutex
	ln net.Listener
}

// New creates a new public API server.
func New(cfg config.PublicAPIConfig, handler http.Handler, logger *slog.Logger) *Server {
	if logger == nil {
		logger = internallog.WithComponent(internallog.New(internallog.FromEnv()), "public-api")
	}

	return &Server{
		cfg:    cfg,
		logger: logger,
		server: &http.Server{
			Handler:      handler,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 0, // Disabled for SSE streaming - LLM calls can take 10+ minutes
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Start starts the public API server and blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	// Create TCP listener
	ln, err := net.Listen("tcp", s.cfg.TCP)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.cfg.TCP, err)
	}
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()

	s.logger.Info("public API server starting",
		slog.String("listen_addr", ln.Addr().String()))

	// Start serving in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully shuts down the public API server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("public API server shutting down")

	// Disable keep-alive to stop accepting new connections
	s.server.SetKeepAlivesEnabled(false)

	// Gracefully shutdown with context timeout
	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Warn("public API server shutdown error",
			internallog.Error(err))
		return err
	}

	s.logger.Info("public API server stopped")
	return nil
}

// Addr returns the listener address, or empty string if not started.
func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}
