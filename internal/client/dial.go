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
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Environment variable names for client configuration.
const (
	ConductorHostEnv   = "CONDUCTOR_HOST"
	ConductorAPIKeyEnv = "CONDUCTOR_API_KEY"
)

// DefaultSocketPath returns the default Unix socket path for the daemon.
func DefaultSocketPath() (string, error) {
	// Use XDG_RUNTIME_DIR if available (Linux)
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "conductor", "conductor.sock"), nil
	}

	// Fall back to ~/.conductor/conductor.sock
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	return filepath.Join(homeDir, ".conductor", "conductor.sock"), nil
}

// ParseConductorHost parses a CONDUCTOR_HOST value into a transport.
// Supports:
//   - unix:///path/to/socket
//   - tcp://host:port
//   - https://host:port
//
// If host is empty, returns a transport for the default socket path.
func ParseConductorHost(host string) (*Transport, error) {
	if host == "" {
		return DefaultTransport()
	}

	switch {
	case strings.HasPrefix(host, "unix://"):
		socketPath := strings.TrimPrefix(host, "unix://")
		return NewUnixTransport(socketPath), nil

	case strings.HasPrefix(host, "tcp://"):
		addr := strings.TrimPrefix(host, "tcp://")
		return NewTCPTransport(addr), nil

	case strings.HasPrefix(host, "https://"):
		addr := strings.TrimPrefix(host, "https://")
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		return NewTLSTransport(addr, tlsConfig), nil

	default:
		return nil, fmt.Errorf("invalid CONDUCTOR_HOST format: %s (must start with unix://, tcp://, or https://)", host)
	}
}

// FromEnvironment creates a client configured from environment variables.
func FromEnvironment() (*Client, error) {
	host := os.Getenv(ConductorHostEnv)
	transport, err := ParseConductorHost(host)
	if err != nil {
		return nil, err
	}

	opts := []Option{WithTransport(transport)}

	// Add API key if configured
	if apiKey := os.Getenv(ConductorAPIKeyEnv); apiKey != "" {
		opts = append(opts, WithAPIKey(apiKey))
	}

	return New(opts...)
}

// DaemonNotRunningError indicates the daemon is not running.
type DaemonNotRunningError struct {
	SocketPath string
	Err        error
}

func (e *DaemonNotRunningError) Error() string {
	return fmt.Sprintf("conductor daemon is not running (socket: %s)", e.SocketPath)
}

func (e *DaemonNotRunningError) Unwrap() error {
	return e.Err
}

// Guidance returns user-friendly guidance for starting the daemon.
func (e *DaemonNotRunningError) Guidance() string {
	return `Conductor daemon is not running.

Start the daemon with:
  conductord                    # Foreground (for development)
  conductord &                  # Background
  brew services start conductor # macOS service (if installed via Homebrew)

Or enable auto-start:
  conductor config set daemon.auto_start true`
}

// IsDaemonNotRunning checks if an error indicates the daemon is not running.
func IsDaemonNotRunning(err error) bool {
	if err == nil {
		return false
	}

	// Check for our specific error type
	var dnr *DaemonNotRunningError
	if ok := errorAs(err, &dnr); ok {
		return true
	}

	// Check for common connection errors
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no such file or directory") ||
		strings.Contains(errStr, "socket") && strings.Contains(errStr, "not found")
}

// errorAs is a helper that wraps errors.As to avoid import cycle issues.
func errorAs(err error, target any) bool {
	// Simple type assertion - for full errors.As behavior, import errors package
	switch t := target.(type) {
	case **DaemonNotRunningError:
		if e, ok := err.(*DaemonNotRunningError); ok {
			*t = e
			return true
		}
	}
	return false
}
