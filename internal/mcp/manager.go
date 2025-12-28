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

// MCP server lifecycle and configuration management.
// Manages starting, stopping, monitoring, and health checks for MCP servers,
// with automatic restart logic and exponential backoff on failures.
package mcp

import (
	"context"
	"errors"
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

	// stopTimeout is the maximum time to wait for servers to stop gracefully
	stopTimeout time.Duration
}

// ManagerConfig configures the MCP manager.
type ManagerConfig struct {
	// Logger is used for structured logging (optional)
	Logger *slog.Logger

	// StopTimeout is the maximum time to wait for all servers to stop gracefully.
	// If zero, defaults to 30 seconds.
	StopTimeout time.Duration
}

// NewManager creates a new MCP server manager.
func NewManager(cfg ManagerConfig) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	stopTimeout := cfg.StopTimeout
	if stopTimeout == 0 {
		stopTimeout = 30 * time.Second
	}

	return &Manager{
		servers:     make(map[string]*serverState),
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		stopTimeout: stopTimeout,
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
		if err := state.client.Close(); err != nil {
			m.logger.Warn("failed to close MCP client during stop", "server", name, "error", err)
		}
		state.client = nil
	}
	state.mu.Unlock()

	m.logger.Info("mcp server stopped", "server", name)

	return nil
}

// StopAll stops all managed MCP servers.
// It waits for all server monitors to complete, up to the configured stopTimeout.
// Returns an error containing all stop failures, if any.
func (m *Manager) StopAll() error {
	m.mu.Lock()
	serverNames := make([]string, 0, len(m.servers))
	for name := range m.servers {
		serverNames = append(serverNames, name)
	}
	m.mu.Unlock()

	// Stop each server and collect errors
	var errs []error
	for _, name := range serverNames {
		if err := m.Stop(name); err != nil {
			m.logger.Warn("failed to stop MCP server", "server", name, "error", err)
			errs = append(errs, fmt.Errorf("server %s: %w", name, err))
		}
	}

	// Wait for all monitors to finish with timeout
	if !waitGroupTimeout(&m.wg, m.stopTimeout) {
		m.logger.Warn("timeout waiting for MCP servers to stop",
			"timeout", m.stopTimeout,
			"servers", serverNames,
		)
	}

	// Return aggregated errors if any
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// waitGroupTimeout waits for a WaitGroup with a timeout.
// Returns true if the wait completed successfully, false if it timed out.
func waitGroupTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
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


// Close shuts down the manager and all managed servers.
func (m *Manager) Close() error {
	m.cancel()
	return m.StopAll()
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
