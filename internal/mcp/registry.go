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

// Registry manages global MCP server configuration and runtime state.
// It combines the persistent configuration from mcp.yaml with the runtime
// manager to provide a unified interface for MCP server management.
type Registry struct {
	// config is the global MCP configuration
	config *MCPGlobalConfig

	// manager handles runtime server lifecycle
	manager *Manager

	// state manages runtime state persistence
	state *StateManager

	// logger is used for structured logging
	logger *slog.Logger

	// mu protects config
	mu sync.RWMutex
}

// RegistryConfig configures the MCP registry.
type RegistryConfig struct {
	// Logger is used for structured logging (optional)
	Logger *slog.Logger

	// LogCapture captures MCP server logs (optional)
	LogCapture *LogCapture
}

// NewRegistry creates a new MCP registry.
func NewRegistry(cfg RegistryConfig) (*Registry, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Load global configuration
	config, err := LoadMCPConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load MCP config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid MCP config: %w", err)
	}

	// Create runtime manager
	manager := NewManager(ManagerConfig{
		Logger:     logger,
		LogCapture: cfg.LogCapture,
	})

	// Create state manager
	state, err := NewStateManager()
	if err != nil {
		logger.Warn("failed to create state manager, state will not be persisted", "error", err)
	}

	return &Registry{
		config:  config,
		manager: manager,
		state:   state,
		logger:  logger,
	}, nil
}

// Start starts the registry, including auto-starting any servers marked for auto-start.
func (r *Registry) Start(ctx context.Context) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Track which servers to start
	serversToStart := make(map[string]*MCPServerEntry)

	// First, add auto-start servers
	for name, entry := range r.config.Servers {
		if entry.AutoStart {
			serversToStart[name] = entry
		}
	}

	// Then, add servers that were running before restart
	if r.state != nil {
		for _, name := range r.state.GetServersToResume() {
			// Only resume if it exists in config and wasn't already added
			if entry, exists := r.config.Servers[name]; exists {
				if _, alreadyAdded := serversToStart[name]; !alreadyAdded {
					r.logger.Info("resuming MCP server from previous session", "server", name)
					serversToStart[name] = entry
				}
			}
		}
	}

	// Start all servers
	for name, entry := range serversToStart {
		r.logger.Info("starting MCP server", "server", name)
		if err := r.manager.Start(entry.ToServerConfig(name)); err != nil {
			r.logger.Error("failed to start MCP server",
				"server", name,
				"error", err,
			)
			// Update state to reflect failure
			if r.state != nil {
				r.state.UpdateServerState(name, false, 1, err.Error(), nil)
			}
			// Continue with other servers even if one fails
		} else {
			// Update state to reflect running
			if r.state != nil {
				now := time.Now()
				r.state.UpdateServerState(name, true, 0, "", &now)
			}
		}
	}

	// Save state after startup
	if r.state != nil {
		if err := r.state.Save(); err != nil {
			r.logger.Warn("failed to save MCP state", "error", err)
		}
	}

	return nil
}

// Stop stops all running servers and shuts down the registry.
func (r *Registry) Stop() error {
	// Mark all servers as stopped in state (graceful shutdown - don't resume)
	if r.state != nil {
		r.state.MarkAllStopped()
		if err := r.state.Save(); err != nil {
			r.logger.Warn("failed to save MCP state on shutdown", "error", err)
		}
	}

	return r.manager.Close()
}

// Manager returns the underlying runtime manager.
func (r *Registry) Manager() *Manager {
	return r.manager
}

// Config returns the current global configuration.
func (r *Registry) Config() *MCPGlobalConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// RegisterGlobal registers a new global MCP server.
// The server is added to the configuration file and optionally started.
func (r *Registry) RegisterGlobal(name string, entry *MCPServerEntry, start bool) error {
	// Validate server name
	if err := ValidateServerName(name); err != nil {
		return ErrInvalidServerName(name)
	}

	// Validate entry
	if err := entry.Validate(); err != nil {
		return ErrInvalidConfig(err.Error())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already exists
	if _, exists := r.config.Servers[name]; exists {
		return ErrServerAlreadyExists(name)
	}

	// Add to configuration
	r.config.Servers[name] = entry

	// Save configuration
	if err := SaveMCPConfig(r.config); err != nil {
		// Rollback
		delete(r.config.Servers, name)
		return fmt.Errorf("failed to save config: %w", err)
	}

	r.logger.Info("registered global MCP server", "server", name)

	// Start if requested
	if start {
		if err := r.manager.Start(entry.ToServerConfig(name)); err != nil {
			return ErrStartFailed(name, err)
		}
	}

	return nil
}

// UnregisterGlobal removes a global MCP server.
// If the server is running, it is stopped first.
func (r *Registry) UnregisterGlobal(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if exists
	if _, exists := r.config.Servers[name]; !exists {
		return ErrServerNotFound(name)
	}

	// Stop if running
	if r.manager.IsRunning(name) {
		if err := r.manager.Stop(name); err != nil {
			r.logger.Warn("failed to stop server during unregister",
				"server", name,
				"error", err,
			)
		}
	}

	// Remove from configuration
	delete(r.config.Servers, name)

	// Save configuration
	if err := SaveMCPConfig(r.config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	r.logger.Info("unregistered global MCP server", "server", name)

	return nil
}

// GetGlobal returns the configuration for a global server.
func (r *Registry) GetGlobal(name string) (*MCPServerEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.config.Servers[name]
	if !exists {
		return nil, ErrServerNotFound(name)
	}

	return entry, nil
}

// ListGlobal returns the names of all registered global servers.
func (r *Registry) ListGlobal() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.config.Servers))
	for name := range r.config.Servers {
		names = append(names, name)
	}
	return names
}

// StartServer starts a global server by name.
func (r *Registry) StartServer(name string) error {
	r.mu.RLock()
	entry, exists := r.config.Servers[name]
	r.mu.RUnlock()

	if !exists {
		return ErrServerNotFound(name)
	}

	// Check if already running
	if r.manager.IsRunning(name) {
		return ErrServerAlreadyRunning(name)
	}

	if err := r.manager.Start(entry.ToServerConfig(name)); err != nil {
		// Update state to reflect failure
		if r.state != nil {
			r.state.UpdateServerState(name, false, 1, err.Error(), nil)
			r.state.Save()
		}
		return ErrStartFailed(name, err)
	}

	// Update state to reflect running
	if r.state != nil {
		now := time.Now()
		r.state.UpdateServerState(name, true, 0, "", &now)
		r.state.Save()
	}

	return nil
}

// StopServer stops a running server by name.
func (r *Registry) StopServer(name string) error {
	// Check if server exists in global config
	r.mu.RLock()
	_, exists := r.config.Servers[name]
	r.mu.RUnlock()

	if !exists {
		return ErrServerNotFound(name)
	}

	// Check if running
	if !r.manager.IsRunning(name) {
		return ErrServerNotRunning(name)
	}

	err := r.manager.Stop(name)

	// Update state to reflect stopped (explicit stop - don't resume on restart)
	if r.state != nil {
		r.state.UpdateServerState(name, false, 0, "", nil)
		r.state.Save()
	}

	return err
}

// RestartServer restarts a server by name.
func (r *Registry) RestartServer(name string) error {
	// Check if server exists in global config
	r.mu.RLock()
	_, exists := r.config.Servers[name]
	r.mu.RUnlock()

	if !exists {
		return ErrServerNotFound(name)
	}

	return r.manager.Restart(name)
}

// GetServerStatus returns the status of a server.
func (r *Registry) GetServerStatus(name string) (*ServerStatus, error) {
	// First check if it's a global server
	r.mu.RLock()
	entry, isGlobal := r.config.Servers[name]
	r.mu.RUnlock()

	// Try to get runtime status
	status, err := r.manager.GetStatus(name)
	if err != nil {
		// If it's a global server that's not running, return stopped status
		if isGlobal {
			return &ServerStatus{
				Name:    name,
				State:   ServerStateStopped,
				Running: false,
				Config:  &ServerConfig{Name: name, Command: entry.Command, Args: entry.Args, Env: entry.Env},
			}, nil
		}
		return nil, ErrServerNotFound(name)
	}

	return status, nil
}

// ListAllServers returns status for all servers (global + workflow-scoped).
func (r *Registry) ListAllServers() []*ServerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Start with runtime statuses
	statuses := r.manager.ListAllStatus()
	statusMap := make(map[string]*ServerStatus)
	for _, s := range statuses {
		statusMap[s.Name] = s
	}

	// Add stopped global servers
	for name, entry := range r.config.Servers {
		if _, exists := statusMap[name]; !exists {
			statuses = append(statuses, &ServerStatus{
				Name:    name,
				State:   ServerStateStopped,
				Running: false,
				Config:  &ServerConfig{Name: name, Command: entry.Command, Args: entry.Args, Env: entry.Env},
			})
		}
	}

	return statuses
}

// GetClient returns the MCP client for a running server.
func (r *Registry) GetClient(name string) (ClientProvider, error) {
	return r.manager.GetClient(name)
}

// Reload reloads the configuration from disk.
func (r *Registry) Reload() error {
	config, err := LoadMCPConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	r.mu.Lock()
	r.config = config
	r.mu.Unlock()

	r.logger.Info("reloaded MCP configuration")

	return nil
}

// UpdateGlobal updates an existing global server configuration.
func (r *Registry) UpdateGlobal(name string, entry *MCPServerEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if exists
	if _, exists := r.config.Servers[name]; !exists {
		return ErrServerNotFound(name)
	}

	// Validate entry
	if err := entry.Validate(); err != nil {
		return ErrInvalidConfig(err.Error())
	}

	// Update configuration
	r.config.Servers[name] = entry

	// Save configuration
	if err := SaveMCPConfig(r.config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	r.logger.Info("updated global MCP server", "server", name)

	return nil
}

// IsGlobal returns true if the named server is a global server.
func (r *Registry) IsGlobal(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.config.Servers[name]
	return exists
}

// ServerSummary provides a summary of MCP server status.
type ServerSummary struct {
	Total   int
	Running int
	Stopped int
	Error   int
}

// GetSummary returns a summary of all server statuses.
func (r *Registry) GetSummary() ServerSummary {
	statuses := r.ListAllServers()

	summary := ServerSummary{
		Total: len(statuses),
	}

	for _, s := range statuses {
		switch s.State {
		case ServerStateRunning:
			summary.Running++
		case ServerStateStopped:
			summary.Stopped++
		case ServerStateError:
			summary.Error++
		}
	}

	return summary
}
