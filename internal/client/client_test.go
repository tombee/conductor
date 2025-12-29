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

package client

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestClientHealth(t *testing.T) {
	// Create a test server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/health" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"timestamp": "2025-01-01T00:00:00Z",
			"uptime":    "1h0m0s",
		})
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Create client with test server
	client, err := New(WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	client.baseURL = server.URL

	// Test Health
	ctx := context.Background()
	health, err := client.Health(ctx)
	if err != nil {
		t.Fatalf("Health failed: %v", err)
	}

	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", health.Status)
	}
}

func TestClientVersion(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/version" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"version":    "1.0.0",
			"commit":     "abc123",
			"build_date": "2025-01-01",
			"go_version": "go1.21",
			"os":         "linux",
			"arch":       "amd64",
		})
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client, err := New(WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	client.baseURL = server.URL

	ctx := context.Background()
	version, err := client.Version(ctx)
	if err != nil {
		t.Fatalf("Version failed: %v", err)
	}

	if version.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", version.Version)
	}
}

func TestClientPing(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	client, err := New(WithHTTPClient(server.Client()))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	client.baseURL = server.URL

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Errorf("Ping failed: %v", err)
	}
}

func TestClientWithUnixSocket(t *testing.T) {
	// Create temp directory for socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	// Create Unix socket listener
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create Unix socket: %v", err)
	}
	defer ln.Close()

	// Start simple HTTP server on socket
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		}),
	}
	go server.Serve(ln)
	defer server.Close()

	// Wait for server to be ready
	time.Sleep(50 * time.Millisecond)

	// Create client with Unix transport
	transport := NewUnixTransport(socketPath)
	client, err := New(WithTransport(transport))
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Errorf("Ping via Unix socket failed: %v", err)
	}
}

func TestParseConductorHost(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		wantSocket string
		wantTCP    string
		wantErr    bool
	}{
		{
			name:       "unix socket",
			host:       "unix:///var/run/conductor.sock",
			wantSocket: "/var/run/conductor.sock",
		},
		{
			name:    "tcp address",
			host:    "tcp://localhost:9000",
			wantTCP: "localhost:9000",
		},
		{
			name:    "https address",
			host:    "https://conductor.example.com:9000",
			wantTCP: "conductor.example.com:9000",
		},
		{
			name:    "invalid format",
			host:    "http://localhost:9000",
			wantErr: true,
		},
		{
			name:    "invalid scheme",
			host:    "ftp://localhost:9000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport, err := ParseConductorHost(tt.host)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.wantSocket != "" && transport.SocketPath != tt.wantSocket {
				t.Errorf("Expected socket path %s, got %s", tt.wantSocket, transport.SocketPath)
			}

			if tt.wantTCP != "" && transport.TCPAddr != tt.wantTCP {
				t.Errorf("Expected TCP addr %s, got %s", tt.wantTCP, transport.TCPAddr)
			}
		})
	}
}

func TestDefaultSocketPath(t *testing.T) {
	// Clear XDG_RUNTIME_DIR to test home directory fallback
	origXDG := os.Getenv("XDG_RUNTIME_DIR")
	os.Unsetenv("XDG_RUNTIME_DIR")
	defer os.Setenv("XDG_RUNTIME_DIR", origXDG)

	path, err := DefaultSocketPath()
	if err != nil {
		t.Fatalf("DefaultSocketPath failed: %v", err)
	}

	if path == "" {
		t.Error("DefaultSocketPath returned empty string")
	}

	// Should end with conductor.sock
	if filepath.Base(path) != "conductor.sock" {
		t.Errorf("Expected path to end with conductor.sock, got %s", path)
	}
}

func TestIsDaemonNotRunning(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "connection refused",
			err:  &net.OpError{Op: "dial", Err: &os.SyscallError{Err: os.ErrNotExist}},
			want: false, // This specific error doesn't match our string checks
		},
		{
			name: "daemon not running error",
			err:  &DaemonNotRunningError{SocketPath: "/tmp/test.sock"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDaemonNotRunning(tt.err)
			if got != tt.want {
				t.Errorf("IsDaemonNotRunning() = %v, want %v", got, tt.want)
			}
		})
	}
}
