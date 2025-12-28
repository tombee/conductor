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

package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir returns the XDG config directory for Conductor
// On Unix: ~/.config/conductor
// On macOS: ~/.config/conductor (follows XDG even though Library/Application Support is more common)
// Respects XDG_CONFIG_HOME environment variable
func ConfigDir() (string, error) {
	var base string

	// Check XDG_CONFIG_HOME first
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		base = xdg
	} else {
		// Get user home directory
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		// Use platform-specific defaults
		if runtime.GOOS == "darwin" {
			// On macOS, we still use ~/.config to follow XDG spec
			// even though ~/Library/Application Support is more idiomatic
			base = filepath.Join(home, ".config")
		} else {
			// On Linux and other Unix systems
			base = filepath.Join(home, ".config")
		}
	}

	configDir := filepath.Join(base, "conductor")

	// Create directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return "", err
	}

	return configDir, nil
}

// ConfigPath returns the full path to the config file
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}
