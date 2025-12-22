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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTLSConfigBuilder(t *testing.T) {
	builder := NewTLSConfigBuilder()
	cfg := builder.Build()

	// Should have secure defaults
	assert.NotNil(t, cfg)
	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	assert.NotEmpty(t, cfg.CipherSuites)
}

func TestTLSConfigBuilder_WithMinVersion(t *testing.T) {
	builder := NewTLSConfigBuilder()
	cfg := builder.WithMinVersion(tls.VersionTLS13).Build()

	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
}

func TestTLSConfigBuilder_WithMinVersion_ForceTLS12(t *testing.T) {
	// Should force TLS 1.2 as minimum even if lower version specified
	builder := NewTLSConfigBuilder()
	cfg := builder.WithMinVersion(tls.VersionTLS10).Build()

	assert.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
}

func TestTLSConfigBuilder_WithServerName(t *testing.T) {
	builder := NewTLSConfigBuilder()
	cfg := builder.WithServerName("api.example.com").Build()

	assert.Equal(t, "api.example.com", cfg.ServerName)
}

func TestTLSConfigBuilder_WithInsecureSkipVerify(t *testing.T) {
	builder := NewTLSConfigBuilder()
	cfg := builder.WithInsecureSkipVerify(true).Build()

	assert.True(t, cfg.InsecureSkipVerify)
}

func TestTLSConfigBuilder_WithSystemCertPool(t *testing.T) {
	builder := NewTLSConfigBuilder()
	err := builder.WithSystemCertPool()
	require.NoError(t, err)

	cfg := builder.Build()
	assert.NotNil(t, cfg.RootCAs)
}

func TestValidateTLSConfig_Valid(t *testing.T) {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	err := ValidateTLSConfig(cfg)
	assert.NoError(t, err)
}

func TestValidateTLSConfig_Nil(t *testing.T) {
	err := ValidateTLSConfig(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestValidateTLSConfig_MinVersionTooLow(t *testing.T) {
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS10,
	}

	err := ValidateTLSConfig(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "minimum TLS version")
}

func TestValidateTLSConfig_InsecureSkipVerify(t *testing.T) {
	cfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true,
	}

	// Should not error, but it's a warning condition
	err := ValidateTLSConfig(cfg)
	assert.NoError(t, err)
}

func TestTLSConfigBuilder_Chaining(t *testing.T) {
	// Test method chaining
	builder := NewTLSConfigBuilder()
	cfg := builder.
		WithMinVersion(tls.VersionTLS13).
		WithServerName("api.example.com").
		WithInsecureSkipVerify(false).
		Build()

	assert.Equal(t, uint16(tls.VersionTLS13), cfg.MinVersion)
	assert.Equal(t, "api.example.com", cfg.ServerName)
	assert.False(t, cfg.InsecureSkipVerify)
}
