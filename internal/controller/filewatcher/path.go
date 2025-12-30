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

package filewatcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// blockedPaths contains filesystem paths that should never be watched.
// These are sensitive system directories that could expose security risks.
// On macOS, some paths are symlinks (e.g., /etc -> /private/etc), so we include both.
var blockedPaths = []string{
	"/etc",
	"/private/etc",
	"/sys",
	"/proc",
	"/dev",
	"/boot",
	"/root",
	"/.ssh",
	"/var/log",
	"/private/var/log",
	"/var/run",
	"/private/var/run",
	"/tmp/systemd-private",
}

// NormalizePath normalizes a file path by:
// - Expanding tilde (~) to home directory
// - Expanding environment variables
// - Converting to absolute path
// - Resolving symlinks
// - Validating against blocked paths
func NormalizePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Expand tilde
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(home, path[2:])
	} else if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = home
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Clean the path (remove .., ., etc.)
	absPath = filepath.Clean(absPath)

	// Resolve symlinks (TOCTOU-safe: we'll re-resolve on each event)
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// Path might not exist yet, which is okay for file watchers
		// that watch for creation events
		if os.IsNotExist(err) {
			resolvedPath = absPath
		} else {
			return "", fmt.Errorf("failed to resolve symlinks: %w", err)
		}
	}

	// Validate against blocked paths
	if err := validatePathNotBlocked(resolvedPath); err != nil {
		return "", err
	}

	return resolvedPath, nil
}

// validatePathNotBlocked checks if a path is in the blocked list.
func validatePathNotBlocked(path string) error {
	// Check if path matches or is a subdirectory of any blocked path
	for _, blocked := range blockedPaths {
		if path == blocked || strings.HasPrefix(path, blocked+string(filepath.Separator)) {
			return fmt.Errorf("path %s is blocked for security reasons (matches %s)", path, blocked)
		}
	}

	// Additional check for hidden SSH directories anywhere
	if strings.Contains(path, "/.ssh/") || strings.HasSuffix(path, "/.ssh") {
		return fmt.Errorf("path %s is blocked for security reasons (SSH directory)", path)
	}

	return nil
}

// ResolveSymlink resolves a symlink path safely.
// This should be called on every file event to prevent TOCTOU attacks.
func ResolveSymlink(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		if os.IsNotExist(err) {
			// For delete events, the file might not exist anymore
			return path, nil
		}
		return "", fmt.Errorf("failed to resolve symlink: %w", err)
	}

	// Re-validate the resolved path
	if err := validatePathNotBlocked(resolved); err != nil {
		return "", err
	}

	return resolved, nil
}

// WalkDirectory recursively walks a directory up to maxDepth levels.
// Returns a list of paths to watch (directories for recursive watching).
func WalkDirectory(root string, maxDepth int) ([]string, error) {
	paths := []string{root}

	if maxDepth <= 0 {
		return paths, nil
	}

	// Walk the directory tree
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't access
			return nil
		}

		// Skip non-directories
		if !info.IsDir() {
			return nil
		}

		// Calculate depth relative to root
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}

		depth := 0
		if rel != "." {
			depth = len(strings.Split(rel, string(filepath.Separator)))
		}

		if depth > maxDepth {
			return filepath.SkipDir
		}

		// Add to paths list if not already the root
		if path != root {
			paths = append(paths, path)
		}

		return nil
	})

	return paths, err
}
