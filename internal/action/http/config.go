package http

import (
	"time"

	"github.com/tombee/conductor/pkg/security"
)

// Config holds configuration for the HTTP action.
type Config struct {
	// Timeout is the default timeout for requests (default: 30s)
	Timeout time.Duration

	// AllowedHosts restricts which hosts can be contacted (empty = allow all)
	AllowedHosts []string

	// RequireHTTPS requires all requests to use HTTPS
	RequireHTTPS bool

	// BlockPrivateIPs blocks RFC1918, link-local, localhost
	BlockPrivateIPs bool

	// MaxResponseSize limits response body size (default: 10MB)
	MaxResponseSize int64

	// MaxRedirects limits redirect following (default: 10)
	MaxRedirects int

	// DNSMonitor provides DNS query monitoring for exfiltration prevention
	DNSMonitor *security.DNSQueryMonitor

	// SecurityConfig provides HTTP security validation
	SecurityConfig *security.HTTPSecurityConfig
}

// DefaultConfig returns a config with secure defaults.
func DefaultConfig() *Config {
	return &Config{
		Timeout:         30 * time.Second,
		AllowedHosts:    []string{},
		RequireHTTPS:    false,
		BlockPrivateIPs: true,
		MaxResponseSize: 10 * 1024 * 1024, // 10MB
		MaxRedirects:    10,
	}
}
