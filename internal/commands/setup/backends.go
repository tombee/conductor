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

package setup

import (
	"github.com/tombee/conductor/internal/secrets"
)

// BackendInfo holds information about a secrets backend.
type BackendInfo struct {
	// Name is the backend identifier (e.g., "keychain", "env", "file")
	Name string

	// DisplayName is the human-readable name
	DisplayName string

	// Description explains what this backend does
	Description string

	// Available indicates if this backend is usable in the current environment
	Available bool

	// Recommended indicates if this is the recommended default for this platform
	Recommended bool
}

// GetAvailableBackends returns information about all available secrets backends.
// Backends are tested for availability in the current environment.
func GetAvailableBackends() []BackendInfo {
	backends := []BackendInfo{
		{
			Name:        "keychain",
			DisplayName: "System Keychain",
			Description: "Uses system keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service)",
			Available:   isKeychainAvailable(),
			Recommended: true, // Recommended on platforms where available
		},
		{
			Name:        "env",
			DisplayName: "Environment Variables",
			Description: "Reads from environment variables (read-only, useful for CI/CD)",
			Available:   true, // Always available
			Recommended: false,
		},
		{
			Name:        "file",
			DisplayName: "Encrypted File",
			Description: "Stores secrets in an encrypted file (portable, but less secure)",
			Available:   true, // Always available
			Recommended: false,
		},
	}

	return backends
}

// GetRecommendedBackend returns the recommended backend for the current platform.
// Returns "keychain" if available, otherwise "file".
func GetRecommendedBackend() string {
	if isKeychainAvailable() {
		return "keychain"
	}
	return "file"
}

// isKeychainAvailable checks if the system keychain backend is available.
func isKeychainAvailable() bool {
	backend := secrets.NewKeychainBackend()
	return backend.Available()
}
