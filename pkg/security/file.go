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
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// sensitivePatterns defines filename patterns that require restrictive permissions (0600/0700).
// These patterns are matched case-insensitively against the basename of the file path.
var sensitivePatterns = []string{
	// Config files
	"config", "settings", "conf", ".cfg", ".ini",
	// Secrets and credentials
	"secret", "credential", "password", "auth",
	// Keys and certificates
	"key", ".pem", ".p12", ".jks", "private",
	// Environment files
	".env",
	// Tokens
	"token", "bearer", "api_key",
}

// DeterminePermissions returns appropriate file and directory permissions based on the file path.
// Sensitive files (matching patterns) get 0600/0700, general files get 0640/0750.
// Pattern matching is case-insensitive and applies to the basename only.
func DeterminePermissions(path string) (fileMode, dirMode os.FileMode) {
	base := strings.ToLower(filepath.Base(path))

	for _, pattern := range sensitivePatterns {
		if strings.Contains(base, pattern) {
			return 0600, 0700
		}
	}

	return 0640, 0750
}

// VerifyPermissions verifies that a file has the expected permissions by checking via file descriptor.
// This prevents TOCTOU (time-of-check-time-of-use) race conditions.
func VerifyPermissions(fd *os.File, expected os.FileMode) error {
	info, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("failed to verify permissions: %w", err)
	}

	actual := info.Mode().Perm()
	if actual != expected {
		return fmt.Errorf("permissions mismatch: got %o, expected %o", actual, expected)
	}

	return nil
}

// FileSecurityConfig defines security controls for file operations.
type FileSecurityConfig struct {
	// AllowedReadPaths lists paths agent can read
	AllowedReadPaths []string `yaml:"allowed_read_paths,omitempty" json:"allowed_read_paths,omitempty"`

	// AllowedWritePaths lists paths agent can write
	AllowedWritePaths []string `yaml:"allowed_write_paths,omitempty" json:"allowed_write_paths,omitempty"`

	// DeniedPaths lists paths always denied (higher priority)
	DeniedPaths []string `yaml:"denied_paths,omitempty" json:"denied_paths,omitempty"`

	// MaxFileSize is the maximum file size to read/write (bytes)
	MaxFileSize int64 `yaml:"max_file_size,omitempty" json:"max_file_size,omitempty"`

	// FollowSymlinks allows symlink traversal
	// When false, symlinks are rejected
	FollowSymlinks bool `yaml:"follow_symlinks" json:"follow_symlinks"`

	// AllowedTypes restricts file types (file, dir)
	// Does not allow device, fifo, socket
	AllowedTypes []string `yaml:"allowed_types,omitempty" json:"allowed_types,omitempty"`

	// ValidateInode tracks inodes to prevent hardlink escapes
	ValidateInode bool `yaml:"validate_inode" json:"validate_inode"`

	// UseFileDescriptors uses O_NOFOLLOW and fstat on fd (not path)
	// Prevents TOCTOU attacks
	UseFileDescriptors bool `yaml:"use_file_descriptors" json:"use_file_descriptors"`

	// ResolveSymlinks resolves and validates symlink targets before access
	ResolveSymlinks bool `yaml:"resolve_symlinks" json:"resolve_symlinks"`

	// MaxSymlinkDepth limits symlink chain depth (default: 10)
	// Deprecated: filepath.EvalSymlinks has its own internal limit for symlink resolution
	MaxSymlinkDepth int `yaml:"max_symlink_depth,omitempty" json:"max_symlink_depth,omitempty"`

	// VerboseErrors includes path details in error messages for debugging
	// When false (default), returns generic "access denied" errors
	VerboseErrors bool `yaml:"verbose_errors" json:"verbose_errors"`
}

// DefaultFileSecurityConfig returns a secure default configuration.
func DefaultFileSecurityConfig() *FileSecurityConfig {
	return &FileSecurityConfig{
		AllowedReadPaths:   []string{},
		AllowedWritePaths:  []string{},
		DeniedPaths:        []string{},
		MaxFileSize:        10 * 1024 * 1024, // 10 MB
		FollowSymlinks:     false,            // Symlinks rejected at file type check
		AllowedTypes:       []string{"file", "dir", "symlink"},
		ValidateInode:      true,
		UseFileDescriptors: true, // Use file descriptors for TOCTOU prevention
		ResolveSymlinks:    true, // Resolve and validate symlink targets
		MaxSymlinkDepth:    10,
	}
}

// ValidatePath validates a file path against the security configuration.
func (c *FileSecurityConfig) ValidatePath(path string, action AccessAction) error {
	// Reject paths with explicit directory traversal as a sanity check
	// This catches malformed paths like "/tmp/../../../etc/passwd"
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		return fmt.Errorf("invalid path: directory traversal detected")
	}

	// Resolve to absolute path
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve symlinks if configured
	resolvedPath := absPath
	if c.ResolveSymlinks {
		resolved, err := c.resolveSymlinks(absPath)
		if err != nil {
			return fmt.Errorf("failed to resolve symlinks: %w", err)
		}
		resolvedPath = resolved
	}

	// Check deny list first (highest priority)
	expandedDenyPaths := expandHomePaths(c.DeniedPaths)
	for _, denyPath := range expandedDenyPaths {
		// Resolve symlinks in the deny path as well for proper comparison
		resolvedDenyPath, err := c.resolveSymlinks(denyPath)
		if err != nil {
			// If we can't resolve the deny path, try matching against the original
			resolvedDenyPath = denyPath
		}

		if matchesPath(resolvedPath, resolvedDenyPath) {
			if c.VerboseErrors {
				return fmt.Errorf("path explicitly denied: %s", path)
			}
			return fmt.Errorf("file access denied")
		}
	}

	// Determine which allowlist to check based on action
	var allowlist []string
	switch action {
	case ActionRead:
		allowlist = c.AllowedReadPaths
	case ActionWrite:
		allowlist = c.AllowedWritePaths
	default:
		return fmt.Errorf("unknown action for file access: %s", action)
	}

	// If allowlist is empty, allow all (unless in strict mode)
	if len(allowlist) == 0 {
		// Assume unrestricted if no allowlist configured
		return nil
	}

	// Check if resolved path is within allowlist
	expandedAllowPaths := expandHomePaths(allowlist)
	for _, allowedPath := range expandedAllowPaths {
		// Resolve symlinks in the allowed path as well for proper comparison
		resolvedAllowedPath, err := c.resolveSymlinks(allowedPath)
		if err != nil {
			// If we can't resolve the allowed path, try matching against the original
			resolvedAllowedPath = allowedPath
		}

		if matchesPath(resolvedPath, resolvedAllowedPath) {
			return nil
		}
	}

	if c.VerboseErrors {
		return fmt.Errorf("path not in allowlist: %s", path)
	}
	return fmt.Errorf("file access denied")
}

// ValidateFileInfo validates file metadata against security policy.
func (c *FileSecurityConfig) ValidateFileInfo(info os.FileInfo) error {
	// Check file type
	mode := info.Mode()

	// Determine file type
	var fileType string
	switch {
	case mode.IsDir():
		fileType = "dir"
	case mode.IsRegular():
		fileType = "file"
	case mode&os.ModeSymlink != 0:
		fileType = "symlink"
	case mode&os.ModeDevice != 0:
		fileType = "device"
	case mode&os.ModeNamedPipe != 0:
		fileType = "fifo"
	case mode&os.ModeSocket != 0:
		fileType = "socket"
	default:
		fileType = "unknown"
	}

	// Check if symlink is allowed
	if fileType == "symlink" && !c.FollowSymlinks {
		return fmt.Errorf("symlinks not allowed by security policy")
	}

	// Check if file type is allowed
	if len(c.AllowedTypes) > 0 {
		allowed := false
		for _, allowedType := range c.AllowedTypes {
			if fileType == allowedType {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("file type not allowed: %s (allowed: %v)", fileType, c.AllowedTypes)
		}
	}

	// Check file size for regular files
	if mode.IsRegular() && c.MaxFileSize > 0 {
		if info.Size() > c.MaxFileSize {
			return fmt.Errorf("file size (%d bytes) exceeds maximum allowed (%d bytes)",
				info.Size(), c.MaxFileSize)
		}
	}

	return nil
}

// resolveSymlinks resolves symlink chains up to MaxSymlinkDepth.
// It handles symlinks in all path components, not just the final component.
func (c *FileSecurityConfig) resolveSymlinks(path string) (string, error) {
	if !c.ResolveSymlinks {
		return path, nil
	}

	// Try to resolve the full path using filepath.EvalSymlinks
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If the file doesn't exist, try resolving the parent directory
		if os.IsNotExist(err) {
			dir := filepath.Dir(path)
			base := filepath.Base(path)

			// Try to resolve parent directory
			resolvedDir, err := filepath.EvalSymlinks(dir)
			if err != nil {
				// If parent also doesn't exist, recurse up the directory tree
				if os.IsNotExist(err) {
					// Try one level up
					parentResolved, err := c.resolveSymlinks(dir)
					if err != nil {
						// If we can't resolve any parent, return cleaned path
						if os.IsNotExist(err) {
							return filepath.Clean(path), nil
						}
						return "", err
					}
					return filepath.Join(parentResolved, base), nil
				}
				return "", fmt.Errorf("failed to resolve parent directory: %w", err)
			}

			return filepath.Join(resolvedDir, base), nil
		}
		return "", fmt.Errorf("failed to resolve symlinks: %w", err)
	}

	return resolved, nil
}

// OpenFileSecure opens a file with security checks.
// Uses O_NOFOLLOW to prevent symlink attacks and validates via file descriptor.
func (c *FileSecurityConfig) OpenFileSecure(path string, flag int, perm os.FileMode) (*os.File, error) {
	// Validate path first
	action := ActionRead
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_APPEND) != 0 {
		action = ActionWrite
	}

	if err := c.ValidatePath(path, action); err != nil {
		return nil, err
	}

	// Resolve the path for opening if ResolveSymlinks is enabled
	// This ensures we open the target, not the symlink itself
	openPath := path
	if c.ResolveSymlinks {
		resolved, err := c.resolveSymlinks(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to resolve symlinks: %w", err)
		}
		if err == nil {
			openPath = resolved
		}
	}

	// Add O_NOFOLLOW to prevent symlink attacks if not following symlinks
	if c.UseFileDescriptors && !c.FollowSymlinks {
		// O_NOFOLLOW is not portable across all systems, but works on Unix-like systems
		flag |= syscall.O_NOFOLLOW
	}

	// Open the file (using resolved path if symlink resolution is enabled)
	file, err := os.OpenFile(openPath, flag, perm)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	// Validate using file descriptor (prevents TOCTOU)
	if c.UseFileDescriptors {
		info, err := file.Stat()
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to stat file descriptor: %w", err)
		}

		if err := c.ValidateFileInfo(info); err != nil {
			file.Close()
			return nil, err
		}
	}

	return file, nil
}

// CheckConfigPermissions checks if a config file or directory has overly permissive permissions.
// Returns a list of warning messages for files that are world-readable or group-writable.
// This function is intended for startup validation to warn about insecure permissions on existing files.
func CheckConfigPermissions(path string) []string {
	var warnings []string

	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		// If file doesn't exist, no warnings needed
		if os.IsNotExist(err) {
			return warnings
		}
		// For other errors, add a warning
		warnings = append(warnings, fmt.Sprintf("unable to check permissions for %s: %v", path, err))
		return warnings
	}

	mode := info.Mode()
	perm := mode.Perm()

	// Check if it's a directory
	if mode.IsDir() {
		// Directories should not be world-readable or world-writable
		if perm&0004 != 0 {
			warnings = append(warnings, fmt.Sprintf("directory %s is world-readable (permissions: %o), recommend chmod 0700 or 0750", path, perm))
		}
		if perm&0002 != 0 {
			warnings = append(warnings, fmt.Sprintf("directory %s is world-writable (permissions: %o), recommend chmod 0700 or 0750", path, perm))
		}
		// Also warn about group-writable for directories
		if perm&0020 != 0 {
			warnings = append(warnings, fmt.Sprintf("directory %s is group-writable (permissions: %o), recommend chmod 0700", path, perm))
		}
	} else {
		// Regular files should not be world-readable or world-writable
		if perm&0004 != 0 {
			warnings = append(warnings, fmt.Sprintf("file %s is world-readable (permissions: %o), recommend chmod 0600 or 0640", path, perm))
		}
		if perm&0002 != 0 {
			warnings = append(warnings, fmt.Sprintf("file %s is world-writable (permissions: %o), recommend chmod 0600 or 0640", path, perm))
		}
		// Warn about group-writable for files (especially sensitive ones)
		if perm&0020 != 0 {
			base := strings.ToLower(filepath.Base(path))
			// Check if this looks like a sensitive file
			isSensitive := false
			for _, pattern := range sensitivePatterns {
				if strings.Contains(base, pattern) {
					isSensitive = true
					break
				}
			}
			if isSensitive {
				warnings = append(warnings, fmt.Sprintf("sensitive file %s is group-writable (permissions: %o), recommend chmod 0600", path, perm))
			}
		}
	}

	return warnings
}

// WriteFileAtomic writes content to a file atomically.
// Uses write-to-temp-then-rename pattern to prevent partial writes.
func (c *FileSecurityConfig) WriteFileAtomic(path string, content []byte, perm os.FileMode) error {
	// Validate path
	if err := c.ValidatePath(path, ActionWrite); err != nil {
		return err
	}

	// Resolve the path if ResolveSymlinks is enabled
	writePath := path
	if c.ResolveSymlinks {
		resolved, err := c.resolveSymlinks(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to resolve symlinks: %w", err)
		}
		if err == nil {
			writePath = resolved
		}
	}

	// Check content size
	if c.MaxFileSize > 0 && int64(len(content)) > c.MaxFileSize {
		return fmt.Errorf("content size (%d bytes) exceeds maximum allowed (%d bytes)",
			len(content), c.MaxFileSize)
	}

	// Create temp file in same directory
	dir := filepath.Dir(writePath)
	tmpFile, err := os.CreateTemp(dir, ".conductor-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Clean up temp file on error
	defer func() {
		if tmpFile != nil {
			tmpFile.Close()
			os.Remove(tmpPath)
		}
	}()

	// Set permissions on temp file BEFORE writing content (security best practice)
	if err := tmpFile.Chmod(0600); err != nil {
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// Verify permissions were set correctly via file descriptor
	if err := VerifyPermissions(tmpFile, 0600); err != nil {
		return fmt.Errorf("failed to verify temp file permissions: %w", err)
	}

	// Write content to temp file
	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Sync to disk
	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	tmpFile = nil // Prevent cleanup

	// Set permissions
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Atomic rename (using resolved path)
	if err := os.Rename(tmpPath, writePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Audit log: file written with permissions
	slog.Debug("file written with permissions",
		"path", path,
		"permissions", fmt.Sprintf("%o", perm),
		"size", len(content))

	return nil
}
