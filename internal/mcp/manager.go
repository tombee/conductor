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
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// ServerConfig defines the configuration for an MCP server.
type ServerConfig struct {
	// Name is the unique identifier for this server
	Name string

	// Command is the executable to run
	Command string

	// Args are the command-line arguments
	Args []string

	// Env are environment variables to pass to the server
	Env []string

	// Timeout is the default timeout for tool calls (defaults to 30s)
	Timeout time.Duration
}

// ServerState represents the lifecycle state of an MCP server.
type ServerState string

const (
	// ServerStateStopped indicates the server is not running.
	ServerStateStopped ServerState = "stopped"
	// ServerStateStarting indicates the server is starting up.
	ServerStateStarting ServerState = "starting"
	// ServerStateRunning indicates the server is running and healthy.
	ServerStateRunning ServerState = "running"
	// ServerStateRestarting indicates the server is being restarted.
	ServerStateRestarting ServerState = "restarting"
	// ServerStateError indicates the server failed to start or crashed.
	ServerStateError ServerState = "error"
)

// serverState tracks the runtime state of an MCP server.
type serverState struct {
	// config is the server configuration
	config ServerConfig

	// client is the active MCP client connection
	client *Client

	// startedAt is the timestamp when the server was last started
	startedAt time.Time

	// state is the current lifecycle state
	state ServerState

	// lastError is the most recent error message
	lastError string

	// failureCount tracks consecutive failures for backoff calculation
	failureCount int

	// lastFailure is the timestamp of the last failure
	lastFailure time.Time

	// restartCh signals when a restart is needed
	restartCh chan struct{}

	// stopCh signals when the server should be stopped
	stopCh chan struct{}

	// mu protects the state fields
	mu sync.RWMutex
}

// Manager manages the lifecycle of MCP servers.
// It handles starting, stopping, health monitoring, and restart logic with exponential backoff.
type Manager struct {
	// servers tracks all managed MCP servers by name
	servers map[string]*serverState

	// logger is used for structured logging
	logger *slog.Logger

	// mu protects the servers map
	mu sync.RWMutex

	// ctx is the manager's lifecycle context
	ctx context.Context

	// cancel stops all managed servers
	cancel context.CancelFunc

	// wg tracks active server monitors
	wg sync.WaitGroup
}

// ManagerConfig configures the MCP manager.
type ManagerConfig struct {
	// Logger is used for structured logging (optional)
	Logger *slog.Logger
}

// NewManager creates a new MCP server manager.
func NewManager(cfg ManagerConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Manager{
		servers: make(map[string]*serverState),
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start starts an MCP server with the given configuration.
// If a server with the same name is already running, it returns an error.
func (m *Manager) Start(config ServerConfig) error {
	if config.Name == "" {
		return fmt.Errorf("server name is required")
	}
	if config.Command == "" {
		return fmt.Errorf("command is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if server already exists
	if _, exists := m.servers[config.Name]; exists {
		return fmt.Errorf("server %s is already running", config.Name)
	}

	// Create server state
	state := &serverState{
		config:    config,
		restartCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}

	m.servers[config.Name] = state

	// Start the server in a goroutine
	m.wg.Add(1)
	go m.monitorServer(state)

	m.logger.Info("mcp server started",
		"server", config.Name,
		"command", config.Command,
	)

	return nil
}

// Stop stops an MCP server by name.
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	state, exists := m.servers[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("server not found: %s", name)
	}
	delete(m.servers, name)
	m.mu.Unlock()

	// Signal stop
	close(state.stopCh)

	// Close the client connection
	state.mu.Lock()
	if state.client != nil {
		_ = state.client.Close()
		state.client = nil
	}
	state.mu.Unlock()

	m.logger.Info("mcp server stopped", "server", name)

	return nil
}

// StopAll stops all managed MCP servers.
func (m *Manager) StopAll() {
	m.mu.Lock()
	serverNames := make([]string, 0, len(m.servers))
	for name := range m.servers {
		serverNames = append(serverNames, name)
	}
	m.mu.Unlock()

	// Stop each server
	for _, name := range serverNames {
		_ = m.Stop(name)
	}

	// Wait for all monitors to finish
	m.wg.Wait()
}

// GetClient returns the MCP client for a server by name.
// If the server is not running or not healthy, it returns an error.
func (m *Manager) GetClient(name string) (ClientProvider, error) {
	m.mu.RLock()
	state, exists := m.servers[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	state.mu.RLock()
	client := state.client
	state.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("server %s is not ready", name)
	}

	return client, nil
}

// Restart triggers a restart of the specified server.
func (m *Manager) Restart(name string) error {
	m.mu.RLock()
	state, exists := m.servers[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server not found: %s", name)
	}

	// Trigger restart
	select {
	case state.restartCh <- struct{}{}:
		m.logger.Info("mcp server restart triggered", "server", name)
		return nil
	default:
		return fmt.Errorf("restart already pending for server: %s", name)
	}
}

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
				_ = state.client.Close()
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
		_ = client.Close()
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

// Close shuts down the manager and all managed servers.
func (m *Manager) Close() error {
	m.cancel()
	m.StopAll()
	return nil
}

// ListServers returns the names of all managed servers.
func (m *Manager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

// ServerStatus represents the status of an MCP server.
type ServerStatus struct {
	Name         string
	State        ServerState
	Running      bool
	StartedAt    *time.Time
	Uptime       time.Duration
	FailureCount int
	LastFailure  *time.Time
	LastError    string
	ToolCount    int
	Config       *ServerConfig
}

// GetStatus returns the status of a server by name.
func (m *Manager) GetStatus(name string) (*ServerStatus, error) {
	m.mu.RLock()
	state, exists := m.servers[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	status := &ServerStatus{
		Name:         name,
		State:        state.state,
		Running:      state.client != nil,
		FailureCount: state.failureCount,
		LastError:    state.lastError,
		Config:       &state.config,
	}

	if !state.startedAt.IsZero() {
		status.StartedAt = &state.startedAt
		if state.client != nil {
			status.Uptime = time.Since(state.startedAt)
		}
	}

	if !state.lastFailure.IsZero() {
		status.LastFailure = &state.lastFailure
	}

	return status, nil
}

// GetConfig returns the configuration for a server by name.
func (m *Manager) GetConfig(name string) (*ServerConfig, error) {
	m.mu.RLock()
	state, exists := m.servers[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	// Return a copy to prevent mutation
	configCopy := state.config
	return &configCopy, nil
}

// ListAllStatus returns the status of all managed servers.
func (m *Manager) ListAllStatus() []*ServerStatus {
	m.mu.RLock()
	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	m.mu.RUnlock()

	statuses := make([]*ServerStatus, 0, len(names))
	for _, name := range names {
		status, err := m.GetStatus(name)
		if err == nil {
			statuses = append(statuses, status)
		}
	}

	return statuses
}

// IsRunning returns true if the named server is running.
func (m *Manager) IsRunning(name string) bool {
	m.mu.RLock()
	state, exists := m.servers[name]
	m.mu.RUnlock()

	if !exists {
		return false
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	return state.client != nil
}

// ServerCount returns the number of managed servers.
func (m *Manager) ServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.servers)
}

// RunningCount returns the number of running servers.
func (m *Manager) RunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, state := range m.servers {
		state.mu.RLock()
		if state.client != nil {
			count++
		}
		state.mu.RUnlock()
	}
	return count
}
