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

package security

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HTTPSecurityConfig defines security controls for HTTP requests.
type HTTPSecurityConfig struct {
	// AllowedHosts lists hosts that can be contacted
	AllowedHosts []string `yaml:"allowed_hosts,omitempty" json:"allowed_hosts,omitempty"`

	// AllowedSchemes restricts URL schemes (http, https)
	// Default: https only
	AllowedSchemes []string `yaml:"allowed_schemes,omitempty" json:"allowed_schemes,omitempty"`

	// DenyPrivateIPs blocks RFC1918, link-local, localhost
	DenyPrivateIPs bool `yaml:"deny_private_ips" json:"deny_private_ips"`

	// DenyMetadata blocks cloud metadata endpoints (169.254.169.254)
	DenyMetadata bool `yaml:"deny_metadata" json:"deny_metadata"`

	// MaxRequestSize limits request body size (bytes)
	MaxRequestSize int64 `yaml:"max_request_size,omitempty" json:"max_request_size,omitempty"`

	// MaxResponseSize limits response body size (bytes)
	MaxResponseSize int64 `yaml:"max_response_size,omitempty" json:"max_response_size,omitempty"`

	// ValidateTLS requires valid TLS certificates
	ValidateTLS bool `yaml:"validate_tls" json:"validate_tls"`

	// AllowedMethods restricts HTTP methods
	AllowedMethods []string `yaml:"allowed_methods,omitempty" json:"allowed_methods,omitempty"`

	// ForbiddenHeaders lists headers that cannot be set
	ForbiddenHeaders []string `yaml:"forbidden_headers,omitempty" json:"forbidden_headers,omitempty"`

	// ResolveBeforeValidation resolves DNS before host allowlist check
	// Prevents DNS rebinding attacks
	ResolveBeforeValidation bool `yaml:"resolve_before_validation" json:"resolve_before_validation"`

	// MaxRedirects limits redirect following (default: 0 = no redirects)
	MaxRedirects int `yaml:"max_redirects" json:"max_redirects"`

	// ValidateRedirects re-validates host allowlist on each redirect
	ValidateRedirects bool `yaml:"validate_redirects" json:"validate_redirects"`

	// DNSCacheTimeout caches DNS resolutions (default: 30s)
	DNSCacheTimeout time.Duration `yaml:"dns_cache_timeout,omitempty" json:"dns_cache_timeout,omitempty"`
}

// DefaultHTTPSecurityConfig returns a secure default configuration.
func DefaultHTTPSecurityConfig() *HTTPSecurityConfig {
	return &HTTPSecurityConfig{
		AllowedHosts:            []string{},
		AllowedSchemes:          []string{"https"},
		DenyPrivateIPs:          true,
		DenyMetadata:            true,
		MaxRequestSize:          1024 * 1024,      // 1 MB
		MaxResponseSize:         10 * 1024 * 1024, // 10 MB
		ValidateTLS:             true,
		AllowedMethods:          []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
		ForbiddenHeaders:        []string{},
		ResolveBeforeValidation: true,
		MaxRedirects:            0,
		ValidateRedirects:       true,
		DNSCacheTimeout:         30 * time.Second,
	}
}

// ValidateURL validates a URL against the security configuration.
func (c *HTTPSecurityConfig) ValidateURL(rawURL string) error {
	// Parse URL
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Check scheme
	if len(c.AllowedSchemes) > 0 {
		allowed := false
		for _, scheme := range c.AllowedSchemes {
			if parsedURL.Scheme == scheme {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("URL scheme not allowed: %s (allowed: %v)", parsedURL.Scheme, c.AllowedSchemes)
		}
	}

	// Extract host (without port)
	host := parsedURL.Hostname()
	if host == "" {
		return fmt.Errorf("URL missing host")
	}

	// Validate host against allowlist
	if len(c.AllowedHosts) > 0 {
		if err := c.validateHost(host); err != nil {
			return err
		}
	}

	// Resolve and validate IP if required
	if c.ResolveBeforeValidation || c.DenyPrivateIPs || c.DenyMetadata {
		if err := c.validateResolvedHost(host); err != nil {
			return err
		}
	}

	return nil
}

// ValidateMethod validates an HTTP method.
func (c *HTTPSecurityConfig) ValidateMethod(method string) error {
	if len(c.AllowedMethods) == 0 {
		return nil // No restrictions
	}

	method = strings.ToUpper(method)
	for _, allowedMethod := range c.AllowedMethods {
		if strings.ToUpper(allowedMethod) == method {
			return nil
		}
	}

	return fmt.Errorf("HTTP method not allowed: %s (allowed: %v)", method, c.AllowedMethods)
}

// ValidateHeaders validates HTTP headers.
func (c *HTTPSecurityConfig) ValidateHeaders(headers http.Header) error {
	for _, forbidden := range c.ForbiddenHeaders {
		if headers.Get(forbidden) != "" {
			return fmt.Errorf("forbidden header: %s", forbidden)
		}
	}
	return nil
}

// validateHost checks if a host is in the allowlist.
func (c *HTTPSecurityConfig) validateHost(host string) error {
	for _, allowedHost := range c.AllowedHosts {
		if matchesHost(host, allowedHost) {
			return nil
		}
	}
	return fmt.Errorf("host not in allowlist: %s", host)
}

// validateResolvedHost resolves DNS and validates the IP address.
func (c *HTTPSecurityConfig) validateResolvedHost(host string) error {
	// Resolve hostname to IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve host: %w", err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("no IP addresses found for host: %s", host)
	}

	// Validate each resolved IP
	for _, ip := range ips {
		if err := c.validateIP(ip); err != nil {
			return fmt.Errorf("host %s resolves to blocked IP %s: %w", host, ip, err)
		}
	}

	return nil
}

// validateIP checks if an IP address is allowed.
func (c *HTTPSecurityConfig) validateIP(ip net.IP) error {
	// Check for private IPs if configured
	if c.DenyPrivateIPs {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("private/local IP addresses are blocked: %s", ip)
		}
	}

	// Check for metadata service IPs
	if c.DenyMetadata {
		if isMetadataIP(ip) {
			return fmt.Errorf("metadata service IP blocked: %s", ip)
		}
	}

	return nil
}

// isPrivateOrLocalIP checks if an IP is private, loopback, or link-local.
func isPrivateOrLocalIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate()
}

// isMetadataIP checks if an IP is a cloud metadata service.
func isMetadataIP(ip net.IP) bool {
	// AWS, Azure, GCP metadata service
	metadataIPs := []string{
		"169.254.169.254",
		"fd00:ec2::254", // AWS IPv6 metadata
	}

	ipStr := ip.String()
	for _, metaIP := range metadataIPs {
		if ipStr == metaIP {
			return true
		}
	}

	return false
}

// DNSCache provides DNS resolution caching to prevent rebinding attacks.
type DNSCache struct {
	mu      sync.RWMutex
	cache   map[string]*dnsCacheEntry
	timeout time.Duration
}

type dnsCacheEntry struct {
	ips       []net.IP
	timestamp time.Time
}

// NewDNSCache creates a new DNS cache with the given timeout.
func NewDNSCache(timeout time.Duration) *DNSCache {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &DNSCache{
		cache:   make(map[string]*dnsCacheEntry),
		timeout: timeout,
	}
}

// Lookup resolves a hostname and caches the result.
func (d *DNSCache) Lookup(host string) ([]net.IP, error) {
	d.mu.RLock()
	entry, exists := d.cache[host]
	d.mu.RUnlock()

	// Check cache
	if exists && time.Since(entry.timestamp) < d.timeout {
		return entry.ips, nil
	}

	// Resolve
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}

	// Cache result
	d.mu.Lock()
	d.cache[host] = &dnsCacheEntry{
		ips:       ips,
		timestamp: time.Now(),
	}
	d.mu.Unlock()

	return ips, nil
}

// Validate checks if cached IPs match current resolution (prevents rebinding).
func (d *DNSCache) Validate(host string) error {
	d.mu.RLock()
	cached, exists := d.cache[host]
	d.mu.RUnlock()

	if !exists {
		return fmt.Errorf("host not in DNS cache: %s", host)
	}

	// Re-resolve
	current, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to re-resolve host: %w", err)
	}

	// Compare IPs
	if !ipsEqual(cached.ips, current) {
		return fmt.Errorf("DNS rebinding detected: host %s changed from %v to %v",
			host, cached.ips, current)
	}

	return nil
}

// ipsEqual checks if two IP slices contain the same addresses.
func ipsEqual(a, b []net.IP) bool {
	if len(a) != len(b) {
		return false
	}

	// Create sets for comparison
	aSet := make(map[string]bool)
	for _, ip := range a {
		aSet[ip.String()] = true
	}

	for _, ip := range b {
		if !aSet[ip.String()] {
			return false
		}
	}

	return true
}

// SecureDialContext creates a DialContext function that validates IPs.
func (c *HTTPSecurityConfig) SecureDialContext(dnsCache *DNSCache) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		// Extract host from address
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address: %w", err)
		}

		// Resolve and validate
		var ips []net.IP
		if dnsCache != nil {
			ips, err = dnsCache.Lookup(host)
		} else {
			ips, err = net.LookupIP(host)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to resolve host: %w", err)
		}

		// Validate each IP
		for _, ip := range ips {
			if err := c.validateIP(ip); err != nil {
				return nil, err
			}
		}

		// Use first IP to dial
		ipAddr := net.JoinHostPort(ips[0].String(), port)
		return (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext(ctx, network, ipAddr)
	}
}
