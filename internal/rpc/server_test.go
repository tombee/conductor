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

package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// waitForServerReady polls the health endpoint until the server is ready or timeout.
func waitForServerReady(t *testing.T, port int) {
	t.Helper()
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	require.Eventually(t, func() bool {
		resp, err := http.Get(url)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK
	}, 5*time.Second, 10*time.Millisecond, "server should become ready")
}

func TestServerConfig_Defaults(t *testing.T) {
	config := DefaultConfig()

	if config.PortRange[0] != 9876 {
		t.Errorf("expected default start port 9876, got %d", config.PortRange[0])
	}

	if config.PortRange[1] != 9899 {
		t.Errorf("expected default end port 9899, got %d", config.PortRange[1])
	}

	if config.ShutdownTimeout != 5*time.Second {
		t.Errorf("expected default shutdown timeout 5s, got %v", config.ShutdownTimeout)
	}

	if config.Logger == nil {
		t.Error("expected default logger, got nil")
	}
}

func TestNewServer(t *testing.T) {
	tests := []struct {
		name   string
		config *ServerConfig
	}{
		{
			name:   "with nil config",
			config: nil,
		},
		{
			name:   "with custom config",
			config: &ServerConfig{PortRange: [2]int{10000, 10010}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.config)
			if server == nil {
				t.Fatal("expected server, got nil")
			}

			if server.config == nil {
				t.Error("expected config, got nil")
			}

			if server.logger == nil {
				t.Error("expected logger, got nil")
			}

			if server.connections == nil {
				t.Error("expected connections map, got nil")
			}
		})
	}
}

func TestServer_StartAndPort(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &ServerConfig{
		PortRange: [2]int{19876, 19899}, // Use high port range for testing
		Logger:    logger,
	}

	server := NewServer(config)
	defer server.Close()

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	if port < config.PortRange[0] || port > config.PortRange[1] {
		t.Errorf("port %d outside configured range [%d, %d]",
			port, config.PortRange[0], config.PortRange[1])
	}

	if server.Port() != port {
		t.Errorf("Port() returned %d, expected %d", server.Port(), port)
	}

	// Starting again should return same port
	port2, err := server.Start(ctx)
	if err != nil {
		t.Errorf("second start failed: %v", err)
	}

	if port2 != port {
		t.Errorf("second start returned different port: %d vs %d", port2, port)
	}
}

func TestServer_NoPortAvailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &ServerConfig{
		PortRange: [2]int{1, 1}, // Port 1 requires root, will fail
		Logger:    logger,
	}

	server := NewServer(config)
	defer server.Close()

	ctx := context.Background()
	_, err := server.Start(ctx)
	if err == nil {
		t.Fatal("expected error when no ports available")
	}

	if err != ErrNoPortAvailable {
		t.Errorf("expected ErrNoPortAvailable, got %v", err)
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &ServerConfig{
		PortRange: [2]int{19900, 19920},
		Logger:    logger,
	}

	server := NewServer(config)
	defer server.Close()

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	waitForServerReady(t, port)

	// Test health endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		t.Fatalf("health check request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var health map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if health["status"] != "ready" {
		t.Errorf("expected status 'ready', got %q", health["status"])
	}

	if health["version"] == "" {
		t.Error("expected version in health response")
	}

	if health["message"] == "" {
		t.Error("expected message in health response")
	}
}

func TestServer_HealthEndpoint_AfterShutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &ServerConfig{
		PortRange:       [2]int{19921, 19940},
		ShutdownTimeout: 1 * time.Second,
		Logger:          logger,
	}

	server := NewServer(config)

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	waitForServerReady(t, port)

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}

	// Health check should fail or return error status
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
	if err != nil {
		// Connection refused is expected after shutdown
		if !strings.Contains(err.Error(), "connection refused") {
			t.Errorf("unexpected error: %v", err)
		}
		return
	}
	defer resp.Body.Close()

	// If we get a response, it should be unavailable
	if resp.StatusCode == http.StatusOK {
		t.Error("expected non-OK status after shutdown")
	}
}

func TestServer_WebSocketUpgrade(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &ServerConfig{
		PortRange: [2]int{19941, 19960},
		Logger:    logger,
	}

	server := NewServer(config)
	defer server.Close()

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	waitForServerReady(t, port)

	// Connect via WebSocket
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	// Connection should be established
	if conn == nil {
		t.Fatal("expected connection, got nil")
	}
}

func TestServer_WebSocketAuth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authToken := "test-secret-token-12345"
	config := &ServerConfig{
		PortRange: [2]int{19961, 19980},
		AuthToken: authToken,
		Logger:    logger,
	}

	server := NewServer(config)
	defer server.Close()

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	waitForServerReady(t, port)

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", port)

	t.Run("without token", func(t *testing.T) {
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			t.Fatal("expected dial to fail without auth token")
		}

		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("with wrong token", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-Auth-Token", "wrong-token")

		_, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
		if err == nil {
			t.Fatal("expected dial to fail with wrong token")
		}

		if resp != nil && resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})

	t.Run("with correct token", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-Auth-Token", authToken)

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
		if err != nil {
			t.Fatalf("dial with correct token failed: %v", err)
		}
		defer conn.Close()

		if conn == nil {
			t.Fatal("expected connection, got nil")
		}
	})
}

func TestServer_RateLimiting(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	authToken := "test-secret-token-rate-limit"
	config := &ServerConfig{
		PortRange: [2]int{20021, 20040},
		AuthToken: authToken,
		Logger:    logger,
	}

	server := NewServer(config)
	defer server.Close()

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	waitForServerReady(t, port)

	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", port)

	t.Run("rate limit after max failed attempts", func(t *testing.T) {
		headers := http.Header{}
		headers.Set("X-Auth-Token", "wrong-token")

		// Make MaxFailedAttempts failed attempts
		for i := 0; i < MaxFailedAttempts; i++ {
			_, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
			if err == nil {
				t.Fatal("expected dial to fail with wrong token")
			}

			if resp == nil {
				t.Fatal("expected response, got nil")
			}

			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("attempt %d: expected status 401, got %d", i, resp.StatusCode)
			}
		}

		// Next attempt should be rate limited
		_, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
		if err == nil {
			t.Fatal("expected dial to fail due to rate limit")
		}

		if resp == nil {
			t.Fatal("expected response, got nil")
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			t.Errorf("expected status 429, got %d", resp.StatusCode)
		}
	})
}

func TestServer_Shutdown(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &ServerConfig{
		PortRange:       [2]int{19981, 20000},
		ShutdownTimeout: 2 * time.Second,
		Logger:          logger,
	}

	server := NewServer(config)

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	waitForServerReady(t, port)

	// Shutdown should complete within timeout
	shutdownErr := server.Shutdown(ctx)
	if shutdownErr != nil {
		t.Errorf("shutdown failed: %v", shutdownErr)
	}

	// Second shutdown should return error
	if err := server.Shutdown(ctx); err != ErrServerClosed {
		t.Errorf("expected ErrServerClosed on second shutdown, got %v", err)
	}

	// Starting after shutdown should fail
	if _, err := server.Start(ctx); err != ErrServerClosed {
		t.Errorf("expected ErrServerClosed after shutdown, got %v", err)
	}
}

func TestServer_ShutdownWithConnections(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &ServerConfig{
		PortRange:       [2]int{20001, 20020},
		ShutdownTimeout: 2 * time.Second,
		Logger:          logger,
	}

	server := NewServer(config)

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	waitForServerReady(t, port)

	// Establish WebSocket connection
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/ws", port)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	// Shutdown server
	shutdownErr := server.Shutdown(ctx)
	if shutdownErr != nil {
		t.Errorf("shutdown with connections failed: %v", shutdownErr)
	}

	// Connection should receive close message
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected read error after shutdown")
	}
}

func TestServer_PortDiscovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	config := &ServerConfig{
		PortRange: [2]int{20021, 20040},
		Logger:    logger,
	}

	server := NewServer(config)
	defer server.Close()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ctx := context.Background()
	port, err := server.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var output strings.Builder
	io.Copy(&output, r)

	// Check port discovery output
	expectedOutput := fmt.Sprintf("CONDUCTOR_BACKEND_PORT=%d\n", port)
	if !strings.Contains(output.String(), expectedOutput) {
		t.Errorf("expected port output %q, got %q", expectedOutput, output.String())
	}
}
