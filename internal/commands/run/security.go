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

package run

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/tombee/conductor/pkg/security"
)

// SecurityOptions encapsulates security parameters for workflow execution.
type SecurityOptions struct {
	// Mode is the security profile name (unrestricted, standard, strict, air-gapped)
	Mode string

	// AllowHosts are additional allowed network hosts
	AllowHosts []string

	// AllowPaths are additional allowed filesystem paths
	AllowPaths []string
}

// ValidateSecurityMode validates the security profile name.
// Returns an error with valid profile names if invalid.
func ValidateSecurityMode(mode string) error {
	if mode == "" {
		// Empty mode is valid (means no security restrictions)
		return nil
	}

	// Check if it's a built-in profile
	validProfiles := security.GetBuiltinProfiles()
	for _, valid := range validProfiles {
		if mode == valid {
			return nil
		}
	}

	// Invalid profile
	return fmt.Errorf("invalid security profile '%s', valid: %s",
		mode, strings.Join(validProfiles, ", "))
}

// ValidateHosts validates that all hosts are valid hostnames or IP addresses.
func ValidateHosts(hosts []string) error {
	for _, host := range hosts {
		if host == "" {
			return fmt.Errorf("host cannot be empty")
		}

		// Check if it's a valid hostname format
		// Allow wildcards for subdomain matching (e.g., "*.example.com")
		if strings.HasPrefix(host, "*.") {
			host = host[2:] // Remove wildcard prefix for validation
		}

		// Basic hostname validation - must not contain invalid characters
		// We allow hostnames, IPs, and ports (e.g., "example.com:443")
		if strings.Contains(host, "://") {
			return fmt.Errorf("host '%s' should not include protocol (e.g., use 'example.com' not 'https://example.com')", host)
		}

		// Parse as URL to validate format
		_, err := url.Parse("http://" + host)
		if err != nil {
			return fmt.Errorf("invalid host '%s': %w", host, err)
		}
	}

	return nil
}

// ValidatePaths validates that all paths are valid filesystem paths.
// Paths can be absolute or relative to CWD.
func ValidatePaths(paths []string) error {
	for _, path := range paths {
		if path == "" {
			return fmt.Errorf("path cannot be empty")
		}

		// Clean the path to normalize it
		cleaned := filepath.Clean(path)
		if cleaned == "" {
			return fmt.Errorf("path '%s' is invalid", path)
		}

		// Check for suspicious patterns
		if strings.Contains(path, "\x00") {
			return fmt.Errorf("path '%s' contains null byte", path)
		}
	}

	return nil
}

// buildSecurityProfile creates a SecurityProfile from SecurityOptions.
// If mode is empty, returns nil (no security restrictions).
// Merges AllowHosts and AllowPaths with the profile's defaults (additive semantics).
func buildSecurityProfile(opts SecurityOptions) (*security.SecurityProfile, error) {
	// If no security mode specified, return nil (unrestricted)
	if opts.Mode == "" {
		return nil, nil
	}

	// Validate the security mode
	if err := ValidateSecurityMode(opts.Mode); err != nil {
		return nil, err
	}

	// Validate hosts and paths
	if err := ValidateHosts(opts.AllowHosts); err != nil {
		return nil, err
	}
	if err := ValidatePaths(opts.AllowPaths); err != nil {
		return nil, err
	}

	// Load the profile (no custom profiles for now, pass nil)
	profile, err := security.LoadProfile(opts.Mode, nil)
	if err != nil {
		return nil, err
	}

	// Merge additional allowed hosts (additive semantics)
	if len(opts.AllowHosts) > 0 {
		profile.Network.Allow = append(profile.Network.Allow, opts.AllowHosts...)
	}

	// Merge additional allowed paths (additive semantics)
	if len(opts.AllowPaths) > 0 {
		// Add to both read and write lists
		profile.Filesystem.Read = append(profile.Filesystem.Read, opts.AllowPaths...)
		profile.Filesystem.Write = append(profile.Filesystem.Write, opts.AllowPaths...)
	}

	return profile, nil
}
