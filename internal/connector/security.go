package connector

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateURL checks if a URL is safe to access based on SSRF protection rules.
// Blocks private IP ranges, loopback addresses, and cloud metadata endpoints by default.
func ValidateURL(rawURL string, allowedHosts, blockedHosts []string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	host := parsedURL.Hostname()
	if host == "" {
		return fmt.Errorf("URL missing host: %s", rawURL)
	}

	// Check if host is explicitly blocked
	if isHostBlocked(host, blockedHosts) {
		return NewSSRFError(host)
	}

	// If allowed_hosts is specified, host must be in the list
	if len(allowedHosts) > 0 {
		if !isHostAllowed(host, allowedHosts) {
			return NewSSRFError(host)
		}
		// If explicitly allowed, skip IP validation
		return nil
	}

	// Resolve hostname to IP and check against private ranges
	if err := validateHostIP(host, blockedHosts); err != nil {
		return err
	}

	return nil
}

// isHostBlocked checks if a host matches any blocked pattern.
// Supports exact matches and wildcard patterns (*.example.com).
func isHostBlocked(host string, blockedHosts []string) bool {
	lowerHost := strings.ToLower(host)

	for _, blocked := range blockedHosts {
		blocked = strings.ToLower(blocked)

		// Exact match
		if blocked == lowerHost {
			return true
		}

		// Wildcard match (*.example.com)
		if strings.HasPrefix(blocked, "*.") {
			suffix := blocked[1:] // Remove *
			if strings.HasSuffix(lowerHost, suffix) {
				return true
			}
		}

		// CIDR notation - will be checked in validateHostIP
	}

	return false
}

// isHostAllowed checks if a host matches any allowed pattern.
func isHostAllowed(host string, allowedHosts []string) bool {
	lowerHost := strings.ToLower(host)

	for _, allowed := range allowedHosts {
		allowed = strings.ToLower(allowed)

		// Exact match
		if allowed == lowerHost {
			return true
		}

		// Wildcard match (*.example.com)
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:]
			if strings.HasSuffix(lowerHost, suffix) {
				return true
			}
		}
	}

	return false
}

// validateHostIP resolves the hostname and checks if it's in blocked ranges.
func validateHostIP(host string, blockedHosts []string) error {
	// Try to parse as IP first
	ip := net.ParseIP(host)
	if ip == nil {
		// Not a direct IP, resolve it
		ips, err := net.LookupIP(host)
		if err != nil {
			return NewConnectionError(fmt.Errorf("failed to resolve %s: %w", host, err))
		}
		if len(ips) == 0 {
			return NewConnectionError(fmt.Errorf("no IP addresses found for %s", host))
		}
		// Check the first resolved IP
		ip = ips[0]
	}

	// Check against blocked CIDR ranges
	for _, blocked := range blockedHosts {
		// Skip non-CIDR patterns
		if !strings.Contains(blocked, "/") && !strings.Contains(blocked, ":") {
			continue
		}

		// Try parsing as CIDR
		_, cidr, err := net.ParseCIDR(blocked)
		if err != nil {
			// Not a valid CIDR, skip
			continue
		}

		if cidr.Contains(ip) {
			return NewSSRFError(fmt.Sprintf("%s (resolved to %s, blocked by %s)", host, ip.String(), blocked))
		}
	}

	// Check against default blocked ranges
	if isPrivateIP(ip) || isLoopbackIP(ip) || isLinkLocalIP(ip) {
		return NewSSRFError(fmt.Sprintf("%s (resolved to private/loopback IP %s)", host, ip.String()))
	}

	return nil
}

// isPrivateIP checks if an IP is in private ranges.
func isPrivateIP(ip net.IP) bool {
	// RFC 1918 private ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}

	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// isLoopbackIP checks if an IP is a loopback address.
func isLoopbackIP(ip net.IP) bool {
	// 127.0.0.0/8 for IPv4, ::1 for IPv6
	loopbackRanges := []string{
		"127.0.0.0/8",
		"::1/128",
	}

	for _, cidr := range loopbackRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// isLinkLocalIP checks if an IP is a link-local address.
func isLinkLocalIP(ip net.IP) bool {
	// 169.254.0.0/16 for IPv4, fe80::/10 for IPv6
	linkLocalRanges := []string{
		"169.254.0.0/16",
		"fe80::/10",
	}

	for _, cidr := range linkLocalRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}

	// Special check for cloud metadata endpoint
	if ip.String() == "169.254.169.254" {
		return true
	}

	return false
}

// ValidatePathParameter checks if a path parameter is safe to use.
// Rejects parameters containing path traversal sequences.
func ValidatePathParameter(name, value string) error {
	// Check for path traversal sequences
	dangerous := []string{
		"../",
		"..\\",
		"%2e%2e/",
		"%2e%2e\\",
		"%2e%2e%2f",
		"%2e%2e%5c",
		"..%2f",
		"..%5c",
	}

	lowerValue := strings.ToLower(value)
	for _, pattern := range dangerous {
		if strings.Contains(lowerValue, pattern) {
			return NewPathInjectionError(name, value)
		}
	}

	// Check for null bytes
	if strings.Contains(value, "\x00") || strings.Contains(value, "%00") {
		return NewPathInjectionError(name, value)
	}

	return nil
}

// MaskSensitiveValue masks sensitive values in logs and error messages.
// Returns [REDACTED] for values that match sensitive patterns.
func MaskSensitiveValue(key, value string) string {
	lowerKey := strings.ToLower(key)

	// Patterns that indicate sensitive data
	sensitivePatterns := []string{
		"token",
		"secret",
		"key",
		"password",
		"credential",
		"api_key",
		"apikey",
		"auth",
		"authorization",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerKey, pattern) {
			return "[REDACTED]"
		}
	}

	return value
}

// MaskSensitiveHeaders masks sensitive HTTP headers for logging.
func MaskSensitiveHeaders(headers map[string][]string) map[string][]string {
	masked := make(map[string][]string)

	for key, values := range headers {
		lowerKey := strings.ToLower(key)

		// Headers that should always be masked
		if lowerKey == "authorization" ||
			lowerKey == "x-api-key" ||
			lowerKey == "x-auth-token" ||
			strings.Contains(lowerKey, "secret") ||
			strings.Contains(lowerKey, "credential") {
			masked[key] = []string{"[REDACTED]"}
		} else {
			masked[key] = values
		}
	}

	return masked
}
