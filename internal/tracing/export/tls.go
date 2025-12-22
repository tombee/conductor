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

package export

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TLSConfigBuilder helps build secure TLS configurations for OTLP exporters.
type TLSConfigBuilder struct {
	config *tls.Config
}

// NewTLSConfigBuilder creates a new TLS config builder with secure defaults.
func NewTLSConfigBuilder() *TLSConfigBuilder {
	return &TLSConfigBuilder{
		config: &tls.Config{
			MinVersion: tls.VersionTLS12,
			// Prefer modern cipher suites
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			},
		},
	}
}

// WithMinVersion sets the minimum TLS version (must be >= TLS 1.2).
func (b *TLSConfigBuilder) WithMinVersion(version uint16) *TLSConfigBuilder {
	if version < tls.VersionTLS12 {
		// Force minimum TLS 1.2 for security
		version = tls.VersionTLS12
	}
	b.config.MinVersion = version
	return b
}

// WithServerName sets the expected server name for SNI and certificate validation.
func (b *TLSConfigBuilder) WithServerName(serverName string) *TLSConfigBuilder {
	b.config.ServerName = serverName
	return b
}

// WithInsecureSkipVerify disables certificate verification (DO NOT USE IN PRODUCTION).
// This should only be used for testing or development environments.
func (b *TLSConfigBuilder) WithInsecureSkipVerify(skip bool) *TLSConfigBuilder {
	b.config.InsecureSkipVerify = skip
	return b
}

// WithClientCert loads a client certificate for mutual TLS authentication.
func (b *TLSConfigBuilder) WithClientCert(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("failed to load client certificate: %w", err)
	}
	b.config.Certificates = []tls.Certificate{cert}
	return nil
}

// WithCustomCA loads a custom CA certificate for server verification.
func (b *TLSConfigBuilder) WithCustomCA(caFile string) error {
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return fmt.Errorf("failed to parse CA certificate")
	}

	b.config.RootCAs = caCertPool
	return nil
}

// WithSystemCertPool uses the system certificate pool for verification.
// This is the default and most secure option for production.
func (b *TLSConfigBuilder) WithSystemCertPool() error {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		return fmt.Errorf("failed to load system cert pool: %w", err)
	}
	b.config.RootCAs = certPool
	return nil
}

// Build returns the configured TLS config.
func (b *TLSConfigBuilder) Build() *tls.Config {
	return b.config
}

// ValidateTLSConfig validates that a TLS config meets security requirements.
func ValidateTLSConfig(cfg *tls.Config) error {
	if cfg == nil {
		return fmt.Errorf("TLS config is nil")
	}

	if cfg.MinVersion < tls.VersionTLS12 {
		return fmt.Errorf("minimum TLS version must be 1.2 or higher, got %d", cfg.MinVersion)
	}

	if cfg.InsecureSkipVerify {
		// Warning: this is a security risk but we allow it for development
		// In production, consider returning an error here
	}

	return nil
}

// TLSConfigInput provides configuration for building a TLS config.
type TLSConfigInput struct {
	Enabled           bool
	VerifyCertificate bool
	CACertPath        string
}

// BuildTLSConfig creates a TLS configuration from input parameters.
// Returns nil if TLS is not enabled.
func BuildTLSConfig(input TLSConfigInput) (*tls.Config, error) {
	if !input.Enabled {
		return nil, nil
	}

	builder := NewTLSConfigBuilder()

	// Configure certificate verification
	if !input.VerifyCertificate {
		builder.WithInsecureSkipVerify(true)
	}

	// Load custom CA if provided
	if input.CACertPath != "" {
		if err := builder.WithCustomCA(input.CACertPath); err != nil {
			return nil, fmt.Errorf("failed to load CA certificate: %w", err)
		}
	} else if input.VerifyCertificate {
		// Use system cert pool for verification
		if err := builder.WithSystemCertPool(); err != nil {
			return nil, fmt.Errorf("failed to load system cert pool: %w", err)
		}
	}

	return builder.Build(), nil
}
