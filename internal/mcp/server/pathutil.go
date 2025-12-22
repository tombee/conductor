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

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath validates a file path for security issues
// Rejects directory traversal attempts and validates against allowed directories
func ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}

	// Reject paths containing directory traversal sequences
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains directory traversal sequence (..)")
	}

	// Clean and resolve the path
	cleanPath := filepath.Clean(path)
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve symlinks
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If file doesn't exist yet, that's okay - just use absPath
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to resolve symlinks: %w", err)
		}
		resolvedPath = absPath
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check if path is within current directory
	if !isPathWithinDir(resolvedPath, cwd) {
		// Check CONDUCTOR_ALLOWED_PATHS environment variable
		allowedPaths := os.Getenv("CONDUCTOR_ALLOWED_PATHS")
		if allowedPaths == "" {
			return fmt.Errorf("path is outside current directory and CONDUCTOR_ALLOWED_PATHS is not set")
		}

		// Check each allowed path
		allowed := false
		for _, allowedDir := range filepath.SplitList(allowedPaths) {
			absAllowedDir, err := filepath.Abs(allowedDir)
			if err != nil {
				continue
			}
			if isPathWithinDir(resolvedPath, absAllowedDir) {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("path is not within current directory or CONDUCTOR_ALLOWED_PATHS")
		}
	}

	return nil
}

// isPathWithinDir checks if path is within or equal to dir
func isPathWithinDir(path, dir string) bool {
	// Normalize both paths
	path = filepath.Clean(path)
	dir = filepath.Clean(dir)

	// Make both absolute if not already
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return false
		}
		path = absPath
	}
	if !filepath.IsAbs(dir) {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return false
		}
		dir = absDir
	}

	// Check if path starts with dir
	// Add separator to avoid false matches like /foo matching /foobar
	dirWithSep := dir + string(filepath.Separator)
	pathWithSep := path + string(filepath.Separator)

	return path == dir || strings.HasPrefix(pathWithSep, dirWithSep)
}
