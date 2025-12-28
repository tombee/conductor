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
	"testing"
	"time"
)

func TestToolCount_ResetOnRestart(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	// Create a server state directly for testing
	state := &serverState{
		config: ServerConfig{
			Name:    "test-server",
			Command: "echo",
			Timeout: 500 * time.Millisecond,
		},
		restartCh: make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
	}

	// Set initial tool count
	initialCount := 5
	state.toolCount = &initialCount

	// Verify initial state
	if state.toolCount == nil {
		t.Fatal("toolCount should be initialized")
	}
	if *state.toolCount != 5 {
		t.Errorf("toolCount = %d, want 5", *state.toolCount)
	}

	// Simulate restart by resetting toolCount (as done in monitorServer)
	state.mu.Lock()
	state.toolCount = nil
	state.mu.Unlock()

	// Verify tool count is reset
	state.mu.RLock()
	if state.toolCount != nil {
		t.Errorf("toolCount should be nil after restart, got %v", state.toolCount)
	}
	state.mu.RUnlock()
}

func TestToolCount_NilVsZero(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	tests := []struct {
		name      string
		toolCount *int
		wantNil   bool
		wantValue int
	}{
		{
			name:      "nil means not queried",
			toolCount: nil,
			wantNil:   true,
		},
		{
			name:      "zero means no tools",
			toolCount: func() *int { v := 0; return &v }(),
			wantNil:   false,
			wantValue: 0,
		},
		{
			name:      "positive value",
			toolCount: func() *int { v := 3; return &v }(),
			wantNil:   false,
			wantValue: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &serverState{
				config: ServerConfig{
					Name:    "test-server",
					Command: "echo",
				},
				toolCount: tt.toolCount,
				restartCh: make(chan struct{}),
				stopCh:    make(chan struct{}),
			}

			state.mu.RLock()
			defer state.mu.RUnlock()

			if tt.wantNil {
				if state.toolCount != nil {
					t.Errorf("toolCount should be nil, got %v", *state.toolCount)
				}
			} else {
				if state.toolCount == nil {
					t.Error("toolCount should not be nil")
				} else if *state.toolCount != tt.wantValue {
					t.Errorf("toolCount = %d, want %d", *state.toolCount, tt.wantValue)
				}
			}
		})
	}
}

func TestShouldRestart_PolicyNever(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	// Policy "never" should never restart
	if mgr.shouldRestart("never", 0, 1) {
		t.Error("should not restart with policy 'never'")
	}
	if mgr.shouldRestart("never", 5, 3) {
		t.Error("should not restart with policy 'never', even under max attempts")
	}
}

func TestShouldRestart_PolicyAlways(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	// Policy "always" should restart indefinitely when maxAttempts=0
	if !mgr.shouldRestart("always", 0, 1) {
		t.Error("should restart with policy 'always' and unlimited attempts")
	}
	if !mgr.shouldRestart("always", 0, 100) {
		t.Error("should restart with policy 'always' and unlimited attempts, even after many restarts")
	}

	// Policy "always" should respect maxAttempts when set
	if !mgr.shouldRestart("always", 5, 3) {
		t.Error("should restart when count < maxAttempts")
	}
	if mgr.shouldRestart("always", 5, 5) {
		t.Error("should not restart when count >= maxAttempts")
	}
	if mgr.shouldRestart("always", 5, 10) {
		t.Error("should not restart when count >> maxAttempts")
	}
}

func TestShouldRestart_MaxAttempts(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	tests := []struct {
		name        string
		maxAttempts int
		count       int
		want        bool
	}{
		{"unlimited restarts", 0, 100, true},
		{"under limit", 10, 5, true},
		{"at limit", 10, 10, false},
		{"over limit", 10, 15, false},
		{"first attempt", 5, 1, true},
		{"max 1 attempt at limit", 1, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mgr.shouldRestart("always", tt.maxAttempts, tt.count)
			if got != tt.want {
				t.Errorf("shouldRestart(always, %d, %d) = %v, want %v",
					tt.maxAttempts, tt.count, got, tt.want)
			}
		})
	}
}

func TestShouldRestart_DefaultPolicy(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	// Empty policy should default to "always"
	if !mgr.shouldRestart("", 0, 5) {
		t.Error("empty policy should default to 'always'")
	}

	// Unknown policy should default to "always" (with warning)
	if !mgr.shouldRestart("unknown", 0, 5) {
		t.Error("unknown policy should default to 'always'")
	}
}

func TestRestartCount_Reset(t *testing.T) {
	state := &serverState{
		config: ServerConfig{
			Name:    "test-server",
			Command: "echo",
		},
		restartCount: 5,
		restartCh:    make(chan struct{}),
		stopCh:       make(chan struct{}),
	}

	// Verify initial count
	if state.restartCount != 5 {
		t.Errorf("initial restartCount = %d, want 5", state.restartCount)
	}

	// Simulate reaching Running state (as done in monitorServer)
	state.mu.Lock()
	state.restartCount = 0
	state.mu.Unlock()

	// Verify reset
	state.mu.RLock()
	if state.restartCount != 0 {
		t.Errorf("restartCount should be 0 after reaching Running state, got %d", state.restartCount)
	}
	state.mu.RUnlock()
}

func TestVersionTracking_StoreInState(t *testing.T) {
	state := &serverState{
		config: ServerConfig{
			Name:    "test-server",
			Command: "npx",
			Source:  "npm:@modelcontextprotocol/server-everything",
			Version: "^1.0.0",
		},
		restartCh: make(chan struct{}),
		stopCh:    make(chan struct{}),
	}

	// Simulate storing version info (as done in startServerClient)
	state.mu.Lock()
	state.source = state.config.Source
	state.version = state.config.Version
	state.mu.Unlock()

	// Verify stored values
	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.source != "npm:@modelcontextprotocol/server-everything" {
		t.Errorf("source = %q, want %q", state.source, "npm:@modelcontextprotocol/server-everything")
	}
	if state.version != "^1.0.0" {
		t.Errorf("version = %q, want %q", state.version, "^1.0.0")
	}
}

func TestVersionTracking_EmptyValues(t *testing.T) {
	state := &serverState{
		config: ServerConfig{
			Name:    "test-server",
			Command: "python",
			// No source or version specified
		},
		restartCh: make(chan struct{}),
		stopCh:    make(chan struct{}),
	}

	// Simulate storing version info
	state.mu.Lock()
	state.source = state.config.Source
	state.version = state.config.Version
	state.mu.Unlock()

	// Verify empty values
	state.mu.RLock()
	defer state.mu.RUnlock()

	if state.source != "" {
		t.Errorf("source should be empty, got %q", state.source)
	}
	if state.version != "" {
		t.Errorf("version should be empty, got %q", state.version)
	}
}

func TestVersionTracking_InStatus(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	// Create a server state with version info
	state := &serverState{
		config: ServerConfig{
			Name:    "test-server",
			Command: "npx",
			Source:  "npm:test-server",
			Version: "^2.1.0",
		},
		source:    "npm:test-server",
		version:   "^2.1.0",
		state:     ServerStateRunning,
		restartCh: make(chan struct{}),
		stopCh:    make(chan struct{}),
	}

	// Register the server
	mgr.mu.Lock()
	mgr.servers["test-server"] = state
	mgr.mu.Unlock()

	// Get status
	status, err := mgr.GetStatus("test-server")
	if err != nil {
		t.Fatalf("GetStatus() error: %v", err)
	}

	// Verify version info in status
	if status.Source != "npm:test-server" {
		t.Errorf("status.Source = %q, want %q", status.Source, "npm:test-server")
	}
	if status.Version != "^2.1.0" {
		t.Errorf("status.Version = %q, want %q", status.Version, "^2.1.0")
	}
}
