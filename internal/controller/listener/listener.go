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

// Package listener provides Unix socket and TCP listener abstractions.
package listener

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/tombee/conductor/internal/config"
)

// New creates a new listener based on configuration.
// Priority: TCP (if configured) > Unix socket (default)
func New(cfg config.ControllerListenConfig) (net.Listener, error) {
	// If TCP address is configured, use TCP
	if cfg.TCPAddr != "" {
		return newTCPListener(cfg)
	}

	// Default to Unix socket
	return newUnixListener(cfg.SocketPath)
}

// newUnixListener creates a Unix socket listener.
func newUnixListener(socketPath string) (net.Listener, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Remove existing socket file if present
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix socket
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on Unix socket: %w", err)
	}

	// Set socket permissions (owner only)
	if err := os.Chmod(socketPath, 0600); err != nil {
		ln.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return ln, nil
}

// newTCPListener creates a TCP listener, with optional TLS.
func newTCPListener(cfg config.ControllerListenConfig) (net.Listener, error) {
	// Security check: block non-localhost bindings unless explicitly allowed
	if !cfg.AllowRemote && isRemoteAddr(cfg.TCPAddr) {
		return nil, fmt.Errorf(
			"binding to %s exposes the daemon to the network.\n"+
				"This allows anyone with network access to execute workflows.\n\n"+
				"If you understand the risks, use: --allow-remote\n"+
				"For production, use HTTPS with authentication: --listen https://...",
			cfg.TCPAddr,
		)
	}

	// Create base TCP listener
	ln, err := net.Listen("tcp", cfg.TCPAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on TCP: %w", err)
	}

	// If TLS is configured, wrap with TLS
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			ln.Close()
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		return tls.NewListener(ln, tlsConfig), nil
	}

	return ln, nil
}

// isRemoteAddr returns true if the address binds to non-localhost interfaces.
func isRemoteAddr(addr string) bool {
	// Parse host from addr
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// addr might be just a port like ":9000"
		host = addr
		if strings.HasPrefix(addr, ":") {
			host = ""
		}
	}

	// Empty host or 0.0.0.0 means all interfaces
	if host == "" || host == "0.0.0.0" || host == "::" {
		return true
	}

	// Check if it's localhost
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}

	// Any other address is considered remote
	return true
}

// ParseConductorHost parses a CONDUCTOR_HOST value into listener config.
// Supports:
//   - unix:///path/to/socket
//   - tcp://host:port
//   - https://host:port
func ParseConductorHost(host string) (*config.ControllerListenConfig, error) {
	if host == "" {
		return nil, nil
	}

	cfg := &config.ControllerListenConfig{}

	switch {
	case strings.HasPrefix(host, "unix://"):
		cfg.SocketPath = strings.TrimPrefix(host, "unix://")

	case strings.HasPrefix(host, "tcp://"):
		cfg.TCPAddr = strings.TrimPrefix(host, "tcp://")

	case strings.HasPrefix(host, "https://"):
		cfg.TCPAddr = strings.TrimPrefix(host, "https://")
		// Note: TLS cert/key must be configured separately

	default:
		return nil, fmt.Errorf("invalid CONDUCTOR_HOST format: %s (must start with unix://, tcp://, or https://)", host)
	}

	return cfg, nil
}
