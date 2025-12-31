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

package validation

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateURL validates a URL for use as a provider or integration base URL.
// Requirements:
//   - Must be a valid URL
//   - Must use HTTPS (except for localhost)
//   - No path traversal attempts
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return fmt.Errorf("URL is required")
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return fmt.Errorf("URL must use HTTP or HTTPS scheme")
	}

	// Require HTTPS except for localhost
	if parsed.Scheme == "http" && !isLocalhost(parsed.Host) {
		return fmt.Errorf("HTTPS required for security (HTTP only allowed for localhost)")
	}

	// Block path traversal
	if strings.Contains(parsed.Path, "..") {
		return fmt.Errorf("path traversal not allowed")
	}

	return nil
}

// isLocalhost checks if a host is localhost
func isLocalhost(host string) bool {
	// Remove port if present
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	return host == "localhost" ||
		host == "127.0.0.1" ||
		host == "::1" ||
		host == "0.0.0.0"
}

// ValidateHTTPSURL validates that a URL uses HTTPS (stricter than ValidateURL).
// Use this for production/external APIs that must use HTTPS.
func ValidateHTTPSURL(urlStr string) error {
	if err := ValidateURL(urlStr); err != nil {
		return err
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return err
	}

	if parsed.Scheme != "https" {
		return fmt.Errorf("HTTPS required")
	}

	return nil
}

// NormalizeURL normalizes a URL by:
//   - Adding https:// if no scheme provided
//   - Removing trailing slashes
func NormalizeURL(urlStr string) string {
	// Add https:// if no scheme
	if !strings.Contains(urlStr, "://") {
		urlStr = "https://" + urlStr
	}

	// Remove trailing slash
	urlStr = strings.TrimSuffix(urlStr, "/")

	return urlStr
}
