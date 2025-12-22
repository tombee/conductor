// Package listener provides Unix socket and TCP listener abstractions.
package listener

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/tombee/conductor/internal/config"
)

func TestNew_UnixSocket(t *testing.T) {
	// Create temp directory for socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "test.sock")

	cfg := config.DaemonListenConfig{
		SocketPath: socketPath,
	}

	ln, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer ln.Close()

	// Verify socket was created
	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Socket file not created: %v", err)
	}

	// Check permissions (0600)
	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("Socket permissions = %o, want 0600", mode)
	}

	// Verify we can connect
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to socket: %v", err)
	}
	conn.Close()
}

func TestNew_TCP_Localhost(t *testing.T) {
	cfg := config.DaemonListenConfig{
		TCPAddr: "127.0.0.1:0", // Use port 0 to get a random available port
	}

	ln, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer ln.Close()

	// Verify we can connect
	addr := ln.Addr().String()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to TCP listener: %v", err)
	}
	conn.Close()
}

func TestNew_TCP_BlocksRemote(t *testing.T) {
	tests := []struct {
		name    string
		addr    string
		wantErr bool
	}{
		{
			name:    "localhost allowed",
			addr:    "127.0.0.1:0",
			wantErr: false,
		},
		{
			name:    "localhost by name allowed",
			addr:    "localhost:0",
			wantErr: false,
		},
		{
			name:    "::1 allowed",
			addr:    "[::1]:0",
			wantErr: false,
		},
		{
			name:    "empty host blocked",
			addr:    ":0",
			wantErr: true,
		},
		{
			name:    "0.0.0.0 blocked",
			addr:    "0.0.0.0:0",
			wantErr: true,
		},
		{
			name:    "any other address blocked",
			addr:    "192.168.1.1:0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.DaemonListenConfig{
				TCPAddr:     tt.addr,
				AllowRemote: false,
			}

			ln, err := New(cfg)
			if tt.wantErr {
				if err == nil {
					ln.Close()
					t.Error("New() should have failed for remote address")
				}
			} else {
				if err != nil {
					t.Errorf("New() error = %v", err)
				} else {
					ln.Close()
				}
			}
		})
	}
}

func TestNew_TCP_AllowRemote(t *testing.T) {
	cfg := config.DaemonListenConfig{
		TCPAddr:     "0.0.0.0:0",
		AllowRemote: true,
	}

	ln, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v, should be allowed with AllowRemote", err)
	}
	ln.Close()
}

func TestNew_UnixSocket_CreatesDirectory(t *testing.T) {
	// Use /tmp for shorter paths (macOS has 104-char limit for Unix socket paths)
	tmpDir, err := os.MkdirTemp("/tmp", "conductor-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "n", "s.sock")

	cfg := config.DaemonListenConfig{
		SocketPath: socketPath,
	}

	ln, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer ln.Close()

	// Verify directory was created
	dir := filepath.Dir(socketPath)
	_, err = os.Stat(dir)
	if err != nil {
		t.Errorf("Directory not created: %v", err)
	}
}

func TestNew_UnixSocket_RemovesExisting(t *testing.T) {
	// Use /tmp for shorter paths (macOS has 104-char limit for Unix socket paths)
	tmpDir, err := os.MkdirTemp("/tmp", "conductor-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "s.sock")

	// Create a regular file at the socket path
	if err := os.WriteFile(socketPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	cfg := config.DaemonListenConfig{
		SocketPath: socketPath,
	}

	ln, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer ln.Close()

	// Verify it's now a socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to connect to socket: %v", err)
	}
	conn.Close()
}

func TestIsRemoteAddr(t *testing.T) {
	tests := []struct {
		addr string
		want bool
	}{
		// Local addresses
		{"127.0.0.1:8080", false},
		{"localhost:8080", false},
		{"[::1]:8080", false}, // IPv6 localhost with brackets

		// Remote/wildcard addresses
		{":8080", true},           // Just port
		{"0.0.0.0:8080", true},    // All interfaces
		{"::", true},              // All IPv6 interfaces
		{"192.168.1.1:8080", true}, // Specific remote IP
		{"10.0.0.1:8080", true},
		{"example.com:8080", true}, // Hostname
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got := isRemoteAddr(tt.addr)
			if got != tt.want {
				t.Errorf("isRemoteAddr(%q) = %v, want %v", tt.addr, got, tt.want)
			}
		})
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
			name:       "empty string",
			host:       "",
			wantSocket: "",
			wantTCP:    "",
			wantErr:    false,
		},
		{
			name:       "unix socket",
			host:       "unix:///var/run/conductor.sock",
			wantSocket: "/var/run/conductor.sock",
			wantErr:    false,
		},
		{
			name:       "tcp",
			host:       "tcp://localhost:9000",
			wantTCP:    "localhost:9000",
			wantErr:    false,
		},
		{
			name:       "https",
			host:       "https://api.example.com:443",
			wantTCP:    "api.example.com:443",
			wantErr:    false,
		},
		{
			name:    "invalid format",
			host:    "invalid://something",
			wantErr: true,
		},
		{
			name:    "no scheme",
			host:    "localhost:9000",
			wantErr: true,
		},
		{
			name:    "http not supported",
			host:    "http://localhost:9000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseConductorHost(tt.host)

			if tt.wantErr {
				if err == nil {
					t.Error("ParseConductorHost() should have failed")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseConductorHost() error = %v", err)
			}

			if tt.host == "" {
				if cfg != nil {
					t.Error("ParseConductorHost(\"\") should return nil config")
				}
				return
			}

			if cfg.SocketPath != tt.wantSocket {
				t.Errorf("SocketPath = %v, want %v", cfg.SocketPath, tt.wantSocket)
			}
			if cfg.TCPAddr != tt.wantTCP {
				t.Errorf("TCPAddr = %v, want %v", cfg.TCPAddr, tt.wantTCP)
			}
		})
	}
}

func TestNew_Preference(t *testing.T) {
	t.Run("TCP takes precedence over socket", func(t *testing.T) {
		tmpDir := t.TempDir()
		socketPath := filepath.Join(tmpDir, "test.sock")

		cfg := config.DaemonListenConfig{
			SocketPath: socketPath,
			TCPAddr:    "127.0.0.1:0",
		}

		ln, err := New(cfg)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		defer ln.Close()

		// Should be a TCP listener
		if ln.Addr().Network() != "tcp" {
			t.Errorf("Network = %v, want tcp", ln.Addr().Network())
		}

		// Socket should NOT be created
		_, err = os.Stat(socketPath)
		if !os.IsNotExist(err) {
			t.Error("Socket file should not be created when TCP is configured")
		}
	})
}
