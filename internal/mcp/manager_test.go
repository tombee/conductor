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
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.servers == nil {
		t.Error("servers map should be initialized")
	}
	if mgr.logger == nil {
		t.Error("logger should be initialized")
	}
	defer mgr.Close()
}

func TestNewManager_WithLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	mgr := NewManager(ManagerConfig{
		Logger: logger,
	})
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.logger != logger {
		t.Error("logger should match provided logger")
	}
	defer mgr.Close()
}

func TestManager_Start_Validation(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	tests := []struct {
		name      string
		config    ServerConfig
		wantError bool
		errorMsg  string
	}{
		{
			name: "missing server name",
			config: ServerConfig{
				Command: "echo",
			},
			wantError: true,
			errorMsg:  "server name is required",
		},
		{
			name: "missing command",
			config: ServerConfig{
				Name: "test-server",
			},
			wantError: true,
			errorMsg:  "command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.Start(tt.config)

			if tt.wantError {
				if err == nil {
					t.Errorf("Start() expected error containing %q, got nil", tt.errorMsg)
				} else if err.Error() != tt.errorMsg {
					t.Errorf("Start() error = %v, want %v", err, tt.errorMsg)
				}
			} else if err != nil {
				t.Errorf("Start() unexpected error: %v", err)
			}
		})
	}
}

func TestManager_Start_DuplicateServer(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	config := ServerConfig{
		Name:    "test-server",
		Command: "echo",
		Args:    []string{"hello"},
	}

	// First start - will fail to connect but should register
	_ = mgr.Start(config)

	// Wait for server to register
	require.Eventually(t, func() bool {
		return mgr.ServerCount() >= 1
	}, 5*time.Second, 10*time.Millisecond, "server should register")

	// Second start - should fail with duplicate error
	err := mgr.Start(config)
	if err == nil {
		t.Error("Start() should fail with duplicate server")
	}
	if err != nil && err.Error() != "server test-server is already running" {
		t.Errorf("Start() error = %v, want duplicate server error", err)
	}
}

func TestManager_Stop_NotFound(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	err := mgr.Stop("nonexistent")
	if err == nil {
		t.Error("Stop() should fail for nonexistent server")
	}
	if err != nil && err.Error() != "server not found: nonexistent" {
		t.Errorf("Stop() error = %v, want not found error", err)
	}
}

func TestManager_GetClient_NotFound(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	_, err := mgr.GetClient("nonexistent")
	if err == nil {
		t.Error("GetClient() should fail for nonexistent server")
	}
	if err != nil && err.Error() != "server not found: nonexistent" {
		t.Errorf("GetClient() error = %v, want not found error", err)
	}
}

func TestManager_GetClient_NotReady(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	config := ServerConfig{
		Name:    "test-server",
		Command: "echo", // Not an MCP server, will fail to connect
		Args:    []string{"hello"},
		Timeout: 500 * time.Millisecond, // Short timeout for test
	}

	_ = mgr.Start(config)

	// Wait for server to register
	require.Eventually(t, func() bool {
		return mgr.ServerCount() >= 1
	}, 5*time.Second, 10*time.Millisecond, "server should register")

	// GetClient should fail because server isn't connected yet (still trying or failed)
	_, err := mgr.GetClient("test-server")
	if err == nil {
		t.Error("GetClient() should fail for server that's not ready")
	}
}

func TestManager_ListServers(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	// Initially empty
	servers := mgr.ListServers()
	if len(servers) != 0 {
		t.Errorf("ListServers() = %v, want empty", servers)
	}

	// Start a server (will fail to connect but should register)
	config := ServerConfig{
		Name:    "test-server",
		Command: "echo",
	}
	_ = mgr.Start(config)

	// Wait for server to register
	require.Eventually(t, func() bool {
		return mgr.ServerCount() >= 1
	}, 5*time.Second, 10*time.Millisecond, "server should register")

	servers = mgr.ListServers()
	if len(servers) != 1 {
		t.Errorf("ListServers() count = %d, want 1", len(servers))
	}
	if len(servers) > 0 && servers[0] != "test-server" {
		t.Errorf("ListServers()[0] = %v, want test-server", servers[0])
	}
}

func TestManager_GetStatus_NotFound(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	_, err := mgr.GetStatus("nonexistent")
	if err == nil {
		t.Error("GetStatus() should fail for nonexistent server")
	}
}

func TestManager_GetStatus(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	config := ServerConfig{
		Name:    "test-server",
		Command: "echo",
		Timeout: 500 * time.Millisecond, // Short timeout for test
	}
	_ = mgr.Start(config)

	// Wait for server to register
	require.Eventually(t, func() bool {
		return mgr.ServerCount() >= 1
	}, 5*time.Second, 10*time.Millisecond, "server should register")

	status, err := mgr.GetStatus("test-server")
	if err != nil {
		t.Errorf("GetStatus() unexpected error: %v", err)
	}
	if status == nil {
		t.Fatal("GetStatus() returned nil status")
	}
	if status.Name != "test-server" {
		t.Errorf("GetStatus().Name = %v, want test-server", status.Name)
	}

	// Eventually Running should become false (echo is not an MCP server)
	require.Eventually(t, func() bool {
		status, err := mgr.GetStatus("test-server")
		return err == nil && status != nil && !status.Running
	}, 15*time.Second, 100*time.Millisecond, "server should fail to connect")
}

func TestManager_StopAll(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	// Start multiple servers
	for i := 1; i <= 3; i++ {
		config := ServerConfig{
			Name:    "test-server-" + string(rune('0'+i)),
			Command: "echo",
			Timeout: 500 * time.Millisecond, // Short timeout for test
		}
		_ = mgr.Start(config)
	}

	// Wait for all servers to register
	require.Eventually(t, func() bool {
		return mgr.ServerCount() >= 3
	}, 5*time.Second, 10*time.Millisecond, "all servers should register")

	// Stop all
	if err := mgr.StopAll(); err != nil {
		t.Errorf("StopAll() error = %v", err)
	}

	// Verify all stopped
	servers := mgr.ListServers()
	if len(servers) != 0 {
		t.Errorf("ListServers() after StopAll = %v, want empty", servers)
	}
}

func TestManager_Restart_NotFound(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	err := mgr.Restart("nonexistent")
	if err == nil {
		t.Error("Restart() should fail for nonexistent server")
	}
}

func TestManager_CalculateBackoff(t *testing.T) {
	mgr := NewManager(ManagerConfig{})
	defer mgr.Close()

	state := &serverState{
		config: ServerConfig{Name: "test"},
	}

	tests := []struct {
		name         string
		failureCount int
		wantBackoff  time.Duration
	}{
		{
			name:         "no failures",
			failureCount: 0,
			wantBackoff:  1 * time.Second,
		},
		{
			name:         "first failure",
			failureCount: 1,
			wantBackoff:  1 * time.Second,
		},
		{
			name:         "second failure",
			failureCount: 2,
			wantBackoff:  2 * time.Second,
		},
		{
			name:         "third failure",
			failureCount: 3,
			wantBackoff:  4 * time.Second,
		},
		{
			name:         "fourth failure",
			failureCount: 4,
			wantBackoff:  8 * time.Second,
		},
		{
			name:         "fifth failure",
			failureCount: 5,
			wantBackoff:  16 * time.Second,
		},
		{
			name:         "sixth failure (capped)",
			failureCount: 6,
			wantBackoff:  30 * time.Second, // Max cap
		},
		{
			name:         "many failures (capped)",
			failureCount: 10,
			wantBackoff:  30 * time.Second, // Max cap
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state.failureCount = tt.failureCount
			backoff := mgr.calculateBackoff(state)
			if backoff != tt.wantBackoff {
				t.Errorf("calculateBackoff() = %v, want %v", backoff, tt.wantBackoff)
			}
		})
	}
}

func TestServerConfig_Structure(t *testing.T) {
	config := ServerConfig{
		Name:    "test-server",
		Command: "/usr/bin/python",
		Args:    []string{"-m", "mcp_server"},
		Env:     []string{"API_KEY=secret"},
		Timeout: 45 * time.Second,
	}

	if config.Name != "test-server" {
		t.Errorf("ServerConfig.Name = %v, want test-server", config.Name)
	}
	if config.Command != "/usr/bin/python" {
		t.Errorf("ServerConfig.Command = %v, want /usr/bin/python", config.Command)
	}
	if len(config.Args) != 2 {
		t.Errorf("ServerConfig.Args length = %d, want 2", len(config.Args))
	}
	if len(config.Env) != 1 {
		t.Errorf("ServerConfig.Env length = %d, want 1", len(config.Env))
	}
	if config.Timeout != 45*time.Second {
		t.Errorf("ServerConfig.Timeout = %v, want 45s", config.Timeout)
	}
}

func TestServerStatus_Structure(t *testing.T) {
	now := time.Now()
	status := ServerStatus{
		Name:         "test-server",
		Running:      true,
		FailureCount: 3,
		LastFailure:  &now,
	}

	if status.Name != "test-server" {
		t.Errorf("ServerStatus.Name = %v, want test-server", status.Name)
	}
	if !status.Running {
		t.Error("ServerStatus.Running should be true")
	}
	if status.FailureCount != 3 {
		t.Errorf("ServerStatus.FailureCount = %d, want 3", status.FailureCount)
	}
	if status.LastFailure == nil {
		t.Error("ServerStatus.LastFailure should not be nil")
	}
}

func TestManager_Close(t *testing.T) {
	mgr := NewManager(ManagerConfig{})

	// Start a server
	config := ServerConfig{
		Name:    "test-server",
		Command: "echo",
	}
	_ = mgr.Start(config)

	// Close manager
	err := mgr.Close()
	if err != nil {
		t.Errorf("Close() unexpected error: %v", err)
	}

	// Verify all servers are stopped
	servers := mgr.ListServers()
	if len(servers) != 0 {
		t.Errorf("ListServers() after Close = %v, want empty", servers)
	}
}
