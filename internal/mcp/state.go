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
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/pkg/security"
)

// MCPRuntimeState represents the persisted runtime state of MCP servers.
// This is stored at ~/.config/conductor/mcp-state.json
type MCPRuntimeState struct {
	// Version is the state file format version.
	Version int `json:"version"`

	// Servers contains the runtime state of each server.
	Servers map[string]*ServerRuntimeState `json:"servers"`

	// LastUpdated is when the state was last persisted.
	LastUpdated time.Time `json:"last_updated"`
}

// ServerRuntimeState represents the persisted runtime state of a single server.
type ServerRuntimeState struct {
	// WasRunning indicates whether the server was running when state was saved.
	WasRunning bool `json:"was_running"`

	// FailureCount is the number of consecutive failures.
	FailureCount int `json:"failure_count"`

	// LastFailure is when the last failure occurred.
	LastFailure *time.Time `json:"last_failure,omitempty"`

	// LastError is the last error message.
	LastError string `json:"last_error,omitempty"`

	// StartedAt is when the server was started (if running).
	StartedAt *time.Time `json:"started_at,omitempty"`
}

const (
	// StateFileVersion is the current version of the state file format.
	StateFileVersion = 1
)

// StateManager manages MCP runtime state persistence.
type StateManager struct {
	mu       sync.RWMutex
	state    *MCPRuntimeState
	filePath string
	dirty    bool
}

// NewStateManager creates a new state manager.
func NewStateManager() (*StateManager, error) {
	// Get state file path
	configDir, err := config.ConfigDir()
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(configDir, "mcp-state.json")

	sm := &StateManager{
		filePath: filePath,
		state: &MCPRuntimeState{
			Version: StateFileVersion,
			Servers: make(map[string]*ServerRuntimeState),
		},
	}

	// Try to load existing state
	if err := sm.load(); err != nil && !os.IsNotExist(err) {
		// Log warning but continue with empty state
		// The error is not fatal - we can start fresh
		sm.state = &MCPRuntimeState{
			Version: StateFileVersion,
			Servers: make(map[string]*ServerRuntimeState),
		}
	}

	return sm, nil
}

// load loads state from the file.
func (sm *StateManager) load() error {
	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		return err
	}

	var state MCPRuntimeState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}

	// Validate version
	if state.Version != StateFileVersion {
		// For now, just start fresh if version mismatches
		return nil
	}

	if state.Servers == nil {
		state.Servers = make(map[string]*ServerRuntimeState)
	}

	sm.state = &state
	return nil
}

// Save persists the current state to disk.
func (sm *StateManager) Save() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.dirty {
		return nil
	}

	return sm.saveLocked()
}

// saveLocked saves without acquiring lock (caller must hold lock).
func (sm *StateManager) saveLocked() error {
	sm.state.LastUpdated = time.Now()

	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists with appropriate permissions
	dir := filepath.Dir(sm.filePath)
	_, dirMode := security.DeterminePermissions(sm.filePath)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return err
	}

	// Write to temp file first, then rename for atomicity
	fileMode, _ := security.DeterminePermissions(sm.filePath)
	tmpFile := sm.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, fileMode); err != nil {
		return err
	}

	if err := os.Rename(tmpFile, sm.filePath); err != nil {
		os.Remove(tmpFile) // Clean up temp file on failure
		return err
	}

	sm.dirty = false
	return nil
}

// UpdateServerState updates the runtime state for a server.
func (sm *StateManager) UpdateServerState(name string, running bool, failureCount int, lastError string, startedAt *time.Time) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	state := sm.state.Servers[name]
	if state == nil {
		state = &ServerRuntimeState{}
		sm.state.Servers[name] = state
	}

	state.WasRunning = running
	state.FailureCount = failureCount
	state.LastError = lastError
	state.StartedAt = startedAt

	if lastError != "" {
		now := time.Now()
		state.LastFailure = &now
	}

	sm.dirty = true
}

// GetServerState returns the persisted state for a server.
func (sm *StateManager) GetServerState(name string) *ServerRuntimeState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.state.Servers[name]
}

// GetServersToResume returns the names of servers that should be resumed.
func (sm *StateManager) GetServersToResume() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var servers []string
	for name, state := range sm.state.Servers {
		if state.WasRunning {
			servers = append(servers, name)
		}
	}
	return servers
}

// RemoveServer removes a server from the state.
func (sm *StateManager) RemoveServer(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.state.Servers, name)
	sm.dirty = true
}

// Clear clears all state.
func (sm *StateManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.state.Servers = make(map[string]*ServerRuntimeState)
	sm.dirty = true
}

// MarkAllStopped marks all servers as stopped.
// Called during graceful shutdown to not resume servers on next start.
func (sm *StateManager) MarkAllStopped() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, state := range sm.state.Servers {
		state.WasRunning = false
	}
	sm.dirty = true
}

// FilePath returns the path to the state file.
func (sm *StateManager) FilePath() string {
	return sm.filePath
}
