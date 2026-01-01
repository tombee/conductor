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

package testing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tombee/conductor/internal/mcp"
)

// MockClient implements mcp.ClientProvider for testing.
type MockClient struct {
	serverName string
	tools      []mcp.ToolDefinition
	callFunc   func(ctx context.Context, req mcp.ToolCallRequest) (*mcp.ToolCallResponse, error)
	pingFunc   func(ctx context.Context) error
	closeFunc  func() error
	callDelay  time.Duration
	mu         sync.RWMutex
}

// NewMockClient creates a new mock MCP client.
func NewMockClient(serverName string, tools []mcp.ToolDefinition) *MockClient {
	return &MockClient{
		serverName: serverName,
		tools:      tools,
	}
}

// ListTools returns the configured list of tools.
func (c *MockClient) ListTools(ctx context.Context) ([]mcp.ToolDefinition, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Make a copy to prevent mutation
	toolsCopy := make([]mcp.ToolDefinition, len(c.tools))
	copy(toolsCopy, c.tools)
	return toolsCopy, nil
}

// CallTool executes a tool call using the configured handler.
func (c *MockClient) CallTool(ctx context.Context, req mcp.ToolCallRequest) (*mcp.ToolCallResponse, error) {
	c.mu.RLock()
	delay := c.callDelay
	callFunc := c.callFunc
	c.mu.RUnlock()

	// Simulate delay if configured
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Use custom handler if configured
	if callFunc != nil {
		return callFunc(ctx, req)
	}

	// Default behavior: echo back the request
	return &mcp.ToolCallResponse{
		Content: []mcp.ContentItem{
			{
				Type: "text",
				Text: fmt.Sprintf("Mock response for %s", req.Name),
			},
		},
	}, nil
}

// Close is a no-op for mock clients.
func (c *MockClient) Close() error {
	c.mu.RLock()
	closeFunc := c.closeFunc
	c.mu.RUnlock()

	if closeFunc != nil {
		return closeFunc()
	}
	return nil
}

// Ping returns success unless a custom ping function is configured.
func (c *MockClient) Ping(ctx context.Context) error {
	c.mu.RLock()
	pingFunc := c.pingFunc
	c.mu.RUnlock()

	if pingFunc != nil {
		return pingFunc(ctx)
	}
	return nil
}

// ServerName returns the mock server name.
func (c *MockClient) ServerName() string {
	return c.serverName
}

// Capabilities returns the mock server capabilities.
func (c *MockClient) Capabilities() *mcp.ServerCapabilities {
	// Return a basic capabilities structure for testing
	return &mcp.ServerCapabilities{
		Tools: &mcp.ToolsCapability{},
	}
}

// SetCallHandler sets a custom call handler for this client.
func (c *MockClient) SetCallHandler(f func(ctx context.Context, req mcp.ToolCallRequest) (*mcp.ToolCallResponse, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callFunc = f
}

// SetCallDelay sets a delay for all tool calls.
func (c *MockClient) SetCallDelay(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callDelay = d
}

// SetPingFunc sets a custom ping function.
func (c *MockClient) SetPingFunc(f func(ctx context.Context) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pingFunc = f
}

// SetCloseFunc sets a custom close function.
func (c *MockClient) SetCloseFunc(f func() error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeFunc = f
}

// MockServerConfig configures a mock MCP server.
type MockServerConfig struct {
	Name        string
	Tools       []mcp.ToolDefinition
	CallHandler func(ctx context.Context, req mcp.ToolCallRequest) (*mcp.ToolCallResponse, error)
	CallDelay   time.Duration
	StartError  error
	StartDelay  time.Duration
	OnStart     func(config mcp.ServerConfig) error
	OnStop      func(name string) error
	OnGetClient func(name string) error
}

// MockManager implements mcp.MCPManagerProvider for testing.
type MockManager struct {
	clients       map[string]*MockClient
	configs       map[string]MockServerConfig
	mu            sync.RWMutex
	onStart       func(config mcp.ServerConfig) error
	onStop        func(name string) error
	callDelay     time.Duration
	startDelay    time.Duration
	getClientHook func(name string) error
}

// NewMockManager creates a new mock MCP manager.
func NewMockManager() *MockManager {
	return &MockManager{
		clients: make(map[string]*MockClient),
		configs: make(map[string]MockServerConfig),
	}
}

// AddServer pre-configures a mock server with the given configuration.
func (m *MockManager) AddServer(config MockServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.configs[config.Name] = config
}

// Start starts a mock MCP server.
func (m *MockManager) Start(config mcp.ServerConfig) error {
	// Check if we have a pre-configured mock for this server (read lock)
	m.mu.RLock()
	mockConfig, exists := m.configs[config.Name]
	m.mu.RUnlock()

	if !exists {
		// Create a default mock config
		mockConfig = MockServerConfig{
			Name:  config.Name,
			Tools: []mcp.ToolDefinition{},
		}
	}

	// Apply start delay if configured (outside of lock)
	if mockConfig.StartDelay > 0 {
		time.Sleep(mockConfig.StartDelay)
	} else {
		m.mu.RLock()
		delay := m.startDelay
		m.mu.RUnlock()
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	// Create mock client
	client := NewMockClient(config.Name, mockConfig.Tools)
	if mockConfig.CallHandler != nil {
		client.SetCallHandler(mockConfig.CallHandler)
	}
	if mockConfig.CallDelay > 0 {
		client.SetCallDelay(mockConfig.CallDelay)
	} else {
		m.mu.RLock()
		delay := m.callDelay
		m.mu.RUnlock()
		if delay > 0 {
			client.SetCallDelay(delay)
		}
	}

	// Add client to map atomically
	m.mu.Lock()
	m.clients[config.Name] = client
	m.mu.Unlock()

	// Call custom onStart hook if configured (mock-specific)
	if mockConfig.OnStart != nil {
		if err := mockConfig.OnStart(config); err != nil {
			// Remove client if hook fails
			m.mu.Lock()
			delete(m.clients, config.Name)
			m.mu.Unlock()
			return err
		}
	}

	// Call global onStart hook if configured
	m.mu.RLock()
	onStart := m.onStart
	m.mu.RUnlock()

	if onStart != nil {
		if err := onStart(config); err != nil {
			// Remove client if hook fails
			m.mu.Lock()
			delete(m.clients, config.Name)
			m.mu.Unlock()
			return err
		}
	}

	// Return configured error if set
	if mockConfig.StartError != nil {
		// Remove client if error is configured
		m.mu.Lock()
		delete(m.clients, config.Name)
		m.mu.Unlock()
		return mockConfig.StartError
	}

	return nil
}

// Stop stops a mock MCP server.
func (m *MockManager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mockConfig, exists := m.configs[name]
	if exists && mockConfig.OnStop != nil {
		if err := mockConfig.OnStop(name); err != nil {
			return err
		}
	}

	if m.onStop != nil {
		if err := m.onStop(name); err != nil {
			return err
		}
	}

	client, exists := m.clients[name]
	if !exists {
		return fmt.Errorf("server not found: %s", name)
	}

	_ = client.Close()
	delete(m.clients, name)
	return nil
}

// GetClient returns the mock client for a server.
func (m *MockManager) GetClient(name string) (mcp.ClientProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Call custom hook if configured
	if m.getClientHook != nil {
		if err := m.getClientHook(name); err != nil {
			return nil, err
		}
	}

	// Check for mock-specific hook
	if mockConfig, exists := m.configs[name]; exists && mockConfig.OnGetClient != nil {
		if err := mockConfig.OnGetClient(name); err != nil {
			return nil, err
		}
	}

	client, exists := m.clients[name]
	if !exists {
		return nil, fmt.Errorf("server not found: %s", name)
	}

	return client, nil
}

// ListServers returns the names of all mock servers.
func (m *MockManager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.clients))
	for name := range m.clients {
		names = append(names, name)
	}
	return names
}

// IsRunning returns true if the mock server is running.
func (m *MockManager) IsRunning(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.clients[name]
	return exists
}

// WithOnStart sets a global start hook for all servers.
func (m *MockManager) WithOnStart(f func(config mcp.ServerConfig) error) *MockManager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStart = f
	return m
}

// WithOnStop sets a global stop hook for all servers.
func (m *MockManager) WithOnStop(f func(name string) error) *MockManager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStop = f
	return m
}

// WithCallDelay sets a global call delay for all clients.
func (m *MockManager) WithCallDelay(d time.Duration) *MockManager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callDelay = d
	return m
}

// WithStartDelay sets a global start delay for all servers.
func (m *MockManager) WithStartDelay(d time.Duration) *MockManager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startDelay = d
	return m
}

// WithGetClientHook sets a hook that's called when GetClient is invoked.
func (m *MockManager) WithGetClientHook(f func(name string) error) *MockManager {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getClientHook = f
	return m
}
