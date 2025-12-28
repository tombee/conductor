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

package secrets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tombee/conductor/pkg/profile"
)

const (
	// MaxFileSize is the maximum allowed secret file size (64KB).
	MaxFileSize = 64 * 1024
)

// FileProviderConfig controls file provider security settings.
type FileProviderConfig struct {
	// Enabled controls whether file provider is available.
	// Default: false (security-first, must be explicitly enabled)
	Enabled bool

	// Allowlist specifies which file paths can be accessed.
	// Required when Enabled is true. Empty allowlist denies all access.
	// Paths must be absolute. Supports prefix matching.
	Allowlist []string

	// FollowSymlinks controls whether symlinks are followed.
	// Default: false (prevents path traversal attacks)
	FollowSymlinks bool

	// MaxSize is the maximum file size in bytes.
	// Default: 64KB
	MaxSize int64
}

// FileProvider implements secret resolution from files.
// This provider reads secrets from filesystem files with strict security controls.
//
// Security features:
//   - Disabled by default (must be explicitly enabled)
//   - Requires absolute paths (rejects relative paths)
//   - Requires explicit allowlist (no paths allowed without configuration)
//   - Symlink resolution and optional rejection
//   - File size limits (default 64KB)
//   - Path traversal detection
//
// Reference format:
//   - file:/etc/secrets/github-token
//   - file:/Users/user/.config/conductor/secrets/api-key
type FileProvider struct {
	config FileProviderConfig
}

// NewFileProvider creates a new file secret provider.
func NewFileProvider(config FileProviderConfig) *FileProvider {
	// Set default max size if not specified
	if config.MaxSize == 0 {
		config.MaxSize = MaxFileSize
	}

	return &FileProvider{
		config: config,
	}
}

// Scheme returns the provider's URI scheme identifier.
func (f *FileProvider) Scheme() string {
	return "file"
}

// Resolve retrieves a secret value from a file.
//
// The reference should be an absolute file path.
// Example: "/etc/secrets/github-token" not "file:/etc/secrets/github-token"
//
// Security checks:
//   - Provider must be enabled
//   - Path must be absolute
//   - Path must be in allowlist
//   - Path must not be symlink (if FollowSymlinks is false)
//   - File size must be within limits
//
// Returns the file contents with trailing whitespace trimmed.
func (f *FileProvider) Resolve(ctx context.Context, reference string) (string, error) {
	// Check if file provider is enabled
	if !f.config.Enabled {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryAccessDenied,
			"file:"+reference,
			"file",
			"file provider is disabled",
			nil,
		)
	}

	// Validate path is absolute
	if !filepath.IsAbs(reference) {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryInvalidSyntax,
			"file:"+reference,
			"file",
			"path must be absolute",
			nil,
		)
	}

	// Resolve symlinks and clean path to detect path traversal
	resolvedPath, err := f.resolvePath(reference)
	if err != nil {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryAccessDenied,
			"file:"+reference,
			"file",
			"path resolution failed",
			err,
		)
	}

	// Check allowlist against both original and resolved paths
	// This handles cases where the reference path or allowlist might contain symlinks
	if !f.isAllowed(reference) && !f.isAllowed(resolvedPath) {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryAccessDenied,
			"file:"+reference,
			"file",
			"path not in allowlist",
			nil,
		)
	}

	// Check if path is a symlink when symlinks are disabled
	if !f.config.FollowSymlinks {
		if isSymlink, err := f.isSymlink(reference); err != nil {
			// If file doesn't exist, skip symlink check - will fail later with NOT_FOUND
			if !os.IsNotExist(err) {
				return "", profile.NewSecretResolutionError(
					profile.ErrorCategoryAccessDenied,
					"file:"+reference,
					"file",
					"symlink check failed",
					err,
				)
			}
		} else if isSymlink {
			return "", profile.NewSecretResolutionError(
				profile.ErrorCategoryAccessDenied,
				"file:"+reference,
				"file",
				"symlinks not allowed",
				nil,
			)
		}
	}

	// Check file size
	stat, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", profile.NewSecretResolutionError(
				profile.ErrorCategoryNotFound,
				"file:"+reference,
				"file",
				"file not found",
				err,
			)
		}
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryAccessDenied,
			"file:"+reference,
			"file",
			"file stat failed",
			err,
		)
	}

	if stat.Size() > f.config.MaxSize {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryInvalidSyntax,
			"file:"+reference,
			"file",
			fmt.Sprintf("file too large (max %d bytes)", f.config.MaxSize),
			nil,
		)
	}

	// Read file contents
	contents, err := os.ReadFile(resolvedPath)
	if err != nil {
		if os.IsPermission(err) {
			return "", profile.NewSecretResolutionError(
				profile.ErrorCategoryAccessDenied,
				"file:"+reference,
				"file",
				"permission denied",
				err,
			)
		}
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryNotFound,
			"file:"+reference,
			"file",
			"failed to read file",
			err,
		)
	}

	// Trim trailing whitespace (common in secret files)
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryNotFound,
			"file:"+reference,
			"file",
			"file is empty",
			nil,
		)
	}

	return value, nil
}

// resolvePath resolves symlinks and cleans the path.
// This is platform-specific to handle Unix realpath vs Windows GetFullPathName semantics.
func (f *FileProvider) resolvePath(path string) (string, error) {
	// Use filepath.EvalSymlinks for cross-platform symlink resolution
	// This resolves all symlinks in the path and returns the absolute path
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If file doesn't exist, EvalSymlinks fails - use Abs instead
		if os.IsNotExist(err) {
			// For non-existent files, just clean and make absolute
			return filepath.Abs(filepath.Clean(path))
		}
		return "", err
	}

	// Clean the path to remove . and .. elements
	return filepath.Clean(resolved), nil
}

// isSymlink checks if the given path is a symlink.
func (f *FileProvider) isSymlink(path string) (bool, error) {
	// Use Lstat to get info about the link itself, not the target
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}

	return info.Mode()&os.ModeSymlink != 0, nil
}

// isAllowed checks if a path is in the allowlist.
// Paths are matched by prefix to support directory-level allowlisting.
func (f *FileProvider) isAllowed(path string) bool {
	// Empty allowlist denies all access
	if len(f.config.Allowlist) == 0 {
		return false
	}

	// Normalize path for comparison
	normalizedPath := f.normalizePath(path)

	for _, allowed := range f.config.Allowlist {
		normalizedAllowed := f.normalizePath(allowed)

		// Support exact match and prefix match (directory allowlisting)
		if normalizedPath == normalizedAllowed {
			return true
		}

		// Check if path is under allowed directory
		// Example: /etc/secrets/ allows /etc/secrets/token
		if strings.HasPrefix(normalizedPath, normalizedAllowed+string(filepath.Separator)) {
			return true
		}

		// Check if allowed path has trailing separator (explicit directory)
		if strings.HasSuffix(normalizedAllowed, string(filepath.Separator)) {
			if strings.HasPrefix(normalizedPath, normalizedAllowed) {
				return true
			}
		}
	}

	return false
}

// normalizePath normalizes a path for consistent comparison across platforms.
func (f *FileProvider) normalizePath(path string) string {
	// Clean the path to remove . and .. elements
	cleaned := filepath.Clean(path)

	// On Windows, normalize to use forward slashes for comparison
	// This makes allowlist rules more portable
	if runtime.GOOS == "windows" {
		cleaned = filepath.ToSlash(cleaned)
	}

	return cleaned
}
