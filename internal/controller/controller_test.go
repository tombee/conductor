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

package controller

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/config"
)

func TestControllerStartStop(t *testing.T) {
	// Create temp directory for socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := config.Default()
	cfg.Controller.Listen.SocketPath = socketPath
	cfg.Controller.ShutdownTimeout = 5 * time.Second

	d, err := New(cfg, Options{
		Version:   "test",
		Commit:    "test",
		BuildDate: "test",
	})
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	// Start controller in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(ctx)
	}()

	// Wait for socket to be available
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatal("Socket file was not created")
	}

	// Test health endpoint
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	resp, err := client.Get("http://localhost/v1/health")
	if err != nil {
		t.Fatalf("Failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var health map[string]any
	if err := json.Unmarshal(body, &health); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", health["status"])
	}

	// Shutdown
	cancel()
	if err := d.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown failed: %v", err)
	}

	// Verify socket is cleaned up
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("Socket file was not cleaned up after shutdown")
	}
}

func TestControllerPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")
	pidFile := filepath.Join(tmpDir, "test.pid")

	cfg := config.Default()
	cfg.Controller.Listen.SocketPath = socketPath
	cfg.Controller.PIDFile = pidFile
	cfg.Controller.ShutdownTimeout = 5 * time.Second

	d, err := New(cfg, Options{Version: "test"})
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go d.Start(ctx)

	// Wait for PID file
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(pidFile); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Verify PID file exists and contains valid PID
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("Failed to read PID file: %v", err)
	}

	if len(data) == 0 {
		t.Error("PID file is empty")
	}

	// Shutdown and verify cleanup
	cancel()
	d.Shutdown(context.Background())

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Error("PID file was not cleaned up after shutdown")
	}
}

func TestControllerVersionEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := config.Default()
	cfg.Controller.Listen.SocketPath = socketPath
	cfg.Controller.ShutdownTimeout = 5 * time.Second

	d, err := New(cfg, Options{
		Version:   "1.2.3",
		Commit:    "abc123",
		BuildDate: "2025-01-01",
	})
	if err != nil {
		t.Fatalf("Failed to create controller: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go d.Start(ctx)

	// Wait for socket
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(socketPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	resp, err := client.Get("http://localhost/v1/version")
	if err != nil {
		t.Fatalf("Failed to call version endpoint: %v", err)
	}
	defer resp.Body.Close()

	var version map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		t.Fatalf("Failed to parse version response: %v", err)
	}

	if version["version"] != "1.2.3" {
		t.Errorf("Expected version '1.2.3', got %s", version["version"])
	}
	if version["commit"] != "abc123" {
		t.Errorf("Expected commit 'abc123', got %s", version["commit"])
	}
	if version["build_date"] != "2025-01-01" {
		t.Errorf("Expected build_date '2025-01-01', got %s", version["build_date"])
	}

	cancel()
	d.Shutdown(context.Background())
}
