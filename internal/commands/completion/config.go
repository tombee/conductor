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

package completion

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
)

// CheckFilePermissions verifies that a file has secure permissions (mode <= 0600).
// Returns true if permissions are acceptable, false if too permissive.
func CheckFilePermissions(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		// If file doesn't exist or can't be accessed, consider it secure
		// (completion will fail gracefully anyway)
		return true
	}

	// Check if file mode is more permissive than 0600
	// We only care about the permission bits (last 9 bits)
	mode := info.Mode().Perm()
	return mode <= 0600
}

// LoadConfigForCompletion loads the conductor configuration with permission validation.
// Returns nil config and error if permissions are too permissive or config cannot be loaded.
// This function is designed to fail silently for completion contexts.
func LoadConfigForCompletion() (*config.Config, error) {
	configPath := shared.GetConfigPath()
	if configPath == "" {
		// Check CONDUCTOR_CONFIG environment variable
		configPath = os.Getenv("CONDUCTOR_CONFIG")
	}
	if configPath == "" {
		// Use default config path
		var err error
		configPath, err = config.ConfigPath()
		if err != nil {
			return nil, err
		}
	}

	// Validate permissions before loading
	if !CheckFilePermissions(configPath) {
		// Silent failure - return nil to indicate completion should be skipped
		return nil, nil
	}

	// Load config - errors are expected and should be handled by caller
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// SafeCompletionWrapper wraps a completion function with panic recovery.
// Returns empty completion list on panic or error.
func SafeCompletionWrapper(fn func() ([]string, cobra.ShellCompDirective)) (results []string, directive cobra.ShellCompDirective) {
	// Set defaults for panic recovery
	results = []string{}
	directive = cobra.ShellCompDirectiveNoFileComp

	defer func() {
		if r := recover(); r != nil {
			// Panic recovery - return empty completion (already set above)
			results = []string{}
			directive = cobra.ShellCompDirectiveNoFileComp
		}
	}()

	// Execute the completion function
	results, directive = fn()
	if results == nil {
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	}
	return results, directive
}
