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
	"log/slog"
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

			// Update state to error and increment restart count
			state.mu.Lock()
			state.state = ServerStateError
			state.lastError = err.Error()
			state.restartCount++
			currentRestartCount := state.restartCount
			restartPolicy := state.config.RestartPolicy
			maxAttempts := state.config.MaxRestartAttempts
			state.mu.Unlock()

			// Check restart policy
			if !m.shouldRestart(restartPolicy, maxAttempts, currentRestartCount) {
				m.logger.Info("restart policy prevents restart",
					"server", serverName,
					"policy", restartPolicy,
					"restart_count", currentRestartCount,
					"max_attempts", maxAttempts,
				)
				return
			}

			// Apply backoff before retry
			backoff := m.calculateBackoff(state)
			m.logger.Info("mcp server will retry after backoff",
				"server", serverName,
				"backoff", backoff,
				"failures", state.failureCount,
				"restart_count", currentRestartCount,
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

		// Reset failure count, restart count, and update state on successful start
		state.mu.Lock()
		state.failureCount = 0
		state.restartCount = 0
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
				_ = state.client.Close()
				state.client = nil
			}
			// Reset tool count on restart - will be re-queried on next successful connection
			state.toolCount = nil
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
		_ = client.Close()
		state.failureCount++
		state.lastFailure = time.Now()
		return fmt.Errorf("server ping failed: %w", err)
	}

	// Query tools to populate tool count
	// Use a short timeout to avoid blocking startup
	toolCtx, toolCancel := context.WithTimeout(m.ctx, 2*time.Second)
	defer toolCancel()

	tools, err := client.ListTools(toolCtx)
	if err != nil {
		// Log warning but don't fail startup - tool count will remain nil
		slog.Warn("Failed to query tool count", slog.String("server", state.config.Name), slog.String("error", err.Error()))
	} else {
		count := len(tools)
		state.toolCount = &count
	}

	// Store source and version from config
	state.source = state.config.Source
	state.version = state.config.Version

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

// shouldRestart checks if the server should be restarted based on restart policy.
func (m *Manager) shouldRestart(policy string, maxAttempts, currentCount int) bool {
	// Check restart policy
	switch policy {
	case "never":
		return false
	case "on-failure":
		// For now, we always restart on failure since we don't track exit codes
		// This can be enhanced later to check exit code from process
		// For the initial implementation, treat as "always"
		// Future: distinguish between clean shutdown (exit 0) vs failure
	case "always", "":
		// Default is always
	default:
		// Unknown policy, default to always
		slog.Warn("Unknown restart policy, defaulting to always", slog.String("policy", policy))
	}

	// Check max restart attempts (0 means unlimited)
	if maxAttempts > 0 && currentCount >= maxAttempts {
		return false
	}

	return true
}
