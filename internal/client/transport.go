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
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

// Transport creates an HTTP transport for connecting to the controller.
type Transport struct {
	// SocketPath is the Unix socket path for local connections.
	SocketPath string

	// TCPAddr is the TCP address for remote connections.
	TCPAddr string

	// TLSConfig is the TLS configuration for HTTPS connections.
	TLSConfig *tls.Config
}

// RoundTrip implements http.RoundTripper.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := t.httpTransport()
	return transport.RoundTrip(req)
}

// httpTransport creates the underlying HTTP transport.
func (t *Transport) httpTransport() *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	if t.SocketPath != "" {
		// Unix socket connection
		transport.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", t.SocketPath)
		}
	} else if t.TCPAddr != "" {
		// TCP connection
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			return d.DialContext(ctx, "tcp", t.TCPAddr)
		}

		if t.TLSConfig != nil {
			transport.TLSClientConfig = t.TLSConfig
		}
	}

	return transport
}

// DefaultTransport creates a transport using the default socket path.
func DefaultTransport() (*Transport, error) {
	socketPath, err := DefaultSocketPath()
	if err != nil {
		return nil, err
	}

	return &Transport{
		SocketPath: socketPath,
	}, nil
}

// NewUnixTransport creates a transport for a Unix socket.
func NewUnixTransport(socketPath string) *Transport {
	return &Transport{
		SocketPath: socketPath,
	}
}

// NewTCPTransport creates a transport for a TCP connection.
func NewTCPTransport(addr string) *Transport {
	return &Transport{
		TCPAddr: addr,
	}
}

// NewTLSTransport creates a transport for a TLS/HTTPS connection.
func NewTLSTransport(addr string, tlsConfig *tls.Config) *Transport {
	if tlsConfig == nil {
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	return &Transport{
		TCPAddr:   addr,
		TLSConfig: tlsConfig,
	}
}
