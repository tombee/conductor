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

// Server monitoring and restart logic.
// Manages the lifecycle of individual MCP server instances including health monitoring,
// automatic restart with exponential backoff, and graceful shutdown.
package mcp

import (
	"context"
	"fmt"
	"time"
)

// monitorServer monitors a server's health and handles restarts.
func (m *Manager) monitorServer(state *serverState) {
	defer m.wg.Done()

	serverName := state.config.Name

	for {
		// Update state to starting
		state.mu.Lock()
		state.state = ServerStateStarting
		state.mu.Unlock()

		// Start the server
		if err := m.startServerClient(state); err != nil {
			m.logger.Error("failed to start mcp server",
				"server", serverName,
				"error", err,
			)

			// Update state to error
			state.mu.Lock()
			state.state = ServerStateError
			state.lastError = err.Error()
			state.mu.Unlock()

			// Apply backoff before retry
			backoff := m.calculateBackoff(state)
			m.logger.Info("mcp server will retry after backoff",
				"server", serverName,
				"backoff", backoff,
				"failures", state.failureCount,
			)

			select {
			case <-time.After(backoff):
				continue
			case <-state.stopCh:
				return
			case <-m.ctx.Done():
				return
			}
		}

		// Reset failure count and update state on successful start
		state.mu.Lock()
		state.failureCount = 0
		state.state = ServerStateRunning
		state.startedAt = time.Now()
		state.lastError = ""
		state.mu.Unlock()

		// Monitor for restart or stop signals
		select {
		case <-state.restartCh:
			m.logger.Info("restarting mcp server", "server", serverName)
			state.mu.Lock()
			state.state = ServerStateRestarting
			if state.client != nil {
				if err := state.client.Close(); err != nil {
					m.logger.Warn("failed to close MCP client during restart", "server", serverName, "error", err)
				}
				state.client = nil
			}
			state.mu.Unlock()
			continue

		case <-state.stopCh:
			m.logger.Info("stopping mcp server monitor", "server", serverName)
			state.mu.Lock()
			state.state = ServerStateStopped
			state.mu.Unlock()
			return

		case <-m.ctx.Done():
			m.logger.Info("manager shutting down, stopping mcp server", "server", serverName)
			state.mu.Lock()
			state.state = ServerStateStopped
			state.mu.Unlock()
			return
		}
	}
}

// startServerClient starts the MCP client for a server.
func (m *Manager) startServerClient(state *serverState) error {
	state.mu.Lock()
	defer state.mu.Unlock()

	// Create client config
	clientConfig := ClientConfig{
		ServerName: state.config.Name,
		Command:    state.config.Command,
		Args:       state.config.Args,
		Env:        state.config.Env,
		Timeout:    state.config.Timeout,
	}

	// Start client with timeout
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	client, err := NewClient(ctx, clientConfig)
	if err != nil {
		// Record failure
		state.failureCount++
		state.lastFailure = time.Now()
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Verify server is responsive with a ping
	if err := client.Ping(ctx); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			m.logger.Warn("failed to close MCP client after ping failure", "server", state.config.Name, "error", closeErr)
		}
		state.failureCount++
		state.lastFailure = time.Now()
		return fmt.Errorf("server ping failed: %w", err)
	}

	state.client = client
	return nil
}

// calculateBackoff calculates the backoff duration based on failure count.
// Uses exponential backoff: 1s, 2s, 4s, 8s, 16s, max 30s.
func (m *Manager) calculateBackoff(state *serverState) time.Duration {
	state.mu.RLock()
	failures := state.failureCount
	state.mu.RUnlock()

	if failures == 0 {
		return time.Second
	}

	// Exponential backoff: 2^failures seconds
	backoff := time.Duration(1<<uint(failures-1)) * time.Second

	// Cap at 30 seconds
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}

	return backoff
}
