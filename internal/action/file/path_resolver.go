package file

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathResolver handles path resolution, expansion, and security validation.
type PathResolver struct {
	workflowDir   string
	outputDir     string
	tempDir       string
	allowedRoots  []string
	allowSymlinks bool
	allowAbsolute bool
}

// PathResolverConfig holds configuration for the PathResolver.
type PathResolverConfig struct {
	WorkflowDir   string
	OutputDir     string
	TempDir       string
	AllowedRoots  []string
	AllowSymlinks bool
	AllowAbsolute bool
}

// NewPathResolver creates a new PathResolver with the given configuration.
func NewPathResolver(config *PathResolverConfig) *PathResolver {
	return &PathResolver{
		workflowDir:   config.WorkflowDir,
		outputDir:     config.OutputDir,
		tempDir:       config.TempDir,
		allowedRoots:  config.AllowedRoots,
		allowSymlinks: config.AllowSymlinks,
		allowAbsolute: config.AllowAbsolute,
	}
}

// Resolve expands path prefixes and validates the path against security rules.
// Returns the absolute, canonical path or an error if the path is invalid or unsafe.
func (r *PathResolver) Resolve(path string) (string, error) {
	// Step 1: Expand prefixes
	expanded, err := r.expandPrefixes(path)
	if err != nil {
		return "", err
	}

	// Step 2: Convert to absolute path
	var absPath string
	if filepath.IsAbs(expanded) {
		absPath = expanded
	} else {
		// Relative paths are resolved against workflow directory
		if r.workflowDir == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return "", &OperationError{
					Message:   fmt.Sprintf("failed to get current directory: %v", err),
					ErrorType: ErrorTypeInternal,
				}
			}
			absPath = filepath.Join(cwd, expanded)
		} else {
			absPath = filepath.Join(r.workflowDir, expanded)
		}
	}

	// Step 3: Canonicalize path (resolve .., ., symlinks)
	canonical, err := r.canonicalizePath(absPath)
	if err != nil {
		return "", err
	}

	// Step 4: Validate against security rules
	if err := r.validatePath(canonical); err != nil {
		return "", err
	}

	return canonical, nil
}

// expandPrefixes expands special path prefixes.
func (r *PathResolver) expandPrefixes(path string) (string, error) {
	// Handle escaped prefixes ($$out -> $out literal)
	if strings.HasPrefix(path, "$$") {
		return "." + path[1:], nil // $$out -> .$out
	}

	// $out/ prefix
	if strings.HasPrefix(path, "$out/") {
		if r.outputDir == "" {
			return "", &OperationError{
				Message:   "$out directory not configured",
				ErrorType: ErrorTypeConfiguration,
			}
		}
		return filepath.Join(r.outputDir, path[5:]), nil
	}

	// $temp/ prefix
	if strings.HasPrefix(path, "$temp/") {
		if r.tempDir == "" {
			return "", &OperationError{
				Message:   "$temp directory not configured",
				ErrorType: ErrorTypeConfiguration,
			}
		}
		return filepath.Join(r.tempDir, path[6:]), nil
	}

	// ~/ prefix (home directory)
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", &OperationError{
				Message:   fmt.Sprintf("failed to get home directory: %v", err),
				ErrorType: ErrorTypeInternal,
			}
		}
		return filepath.Join(home, path[2:]), nil
	}

	// ./ prefix or relative path
	return path, nil
}

// canonicalizePath resolves the canonical absolute path.
func (r *PathResolver) canonicalizePath(path string) (string, error) {
	// Clean the path to resolve . and ..
	cleaned := filepath.Clean(path)

	// Check if path exists to resolve symlinks
	info, err := os.Lstat(cleaned)
	if err != nil {
		// Path doesn't exist yet - that's OK for write operations
		if os.IsNotExist(err) {
			return cleaned, nil
		}
		return "", &OperationError{
			Message:   fmt.Sprintf("failed to stat path: %v", err),
			ErrorType: ErrorTypeInternal,
		}
	}

	// If it's a symlink, evaluate it
	if info.Mode()&os.ModeSymlink != 0 {
		if !r.allowSymlinks {
			return "", &OperationError{
				Message:   "symlinks are not allowed",
				ErrorType: ErrorTypeSymlinkDenied,
			}
		}

		// Evaluate symlink
		target, err := filepath.EvalSymlinks(cleaned)
		if err != nil {
			return "", &OperationError{
				Message:   fmt.Sprintf("failed to evaluate symlink: %v", err),
				ErrorType: ErrorTypeInternal,
			}
		}

		// Validate symlink target - use special validation that allows absolute paths under configured dirs
		if err := r.validateSymlinkTarget(target); err != nil {
			return "", &OperationError{
				Message:   fmt.Sprintf("symlink target violates security policy: %v", err),
				ErrorType: ErrorTypeSymlinkDenied,
			}
		}

		return target, nil
	}

	return cleaned, nil
}

// validatePath checks if a path is allowed by security rules.
func (r *PathResolver) validatePath(path string) error {
	// If absolute paths are globally allowed, skip this check
	if r.allowAbsolute {
		return r.validateAllowedRoots(path)
	}

	// For relative paths, just check allowed roots
	if !filepath.IsAbs(path) {
		return r.validateAllowedRoots(path)
	}

	// Absolute paths need to be under configured directories
	// Use Rel to check if path is under a directory
	// For paths that might have been resolved through symlinks, we need to check
	// both the original configured dir and its canonical form
	isAllowed := false

	checkUnderDir := func(dir string) bool {
		if dir == "" {
			return false
		}

		// Try with original dir path
		if rel, err := filepath.Rel(dir, path); err == nil && !strings.HasPrefix(rel, "..") {
			return true
		}

		// Try with canonicalized dir path (for cases where dir contains symlinks)
		if canonicalDir, err := filepath.EvalSymlinks(dir); err == nil && canonicalDir != "" {
			if rel, err := filepath.Rel(canonicalDir, path); err == nil && !strings.HasPrefix(rel, "..") {
				return true
			}
		}

		return false
	}

	if checkUnderDir(r.workflowDir) {
		isAllowed = true
	}

	if !isAllowed && checkUnderDir(r.outputDir) {
		isAllowed = true
	}

	if !isAllowed && checkUnderDir(r.tempDir) {
		isAllowed = true
	}

	if !isAllowed {
		return &OperationError{
			Message:   "absolute paths are not allowed",
			ErrorType: ErrorTypePathTraversal,
		}
	}

	return r.validateAllowedRoots(path)
}

// validateAllowedRoots checks if path is within allowed roots (if configured).
func (r *PathResolver) validateAllowedRoots(path string) error {
	// If no allowed roots specified, allow the path
	if len(r.allowedRoots) == 0 {
		return nil
	}

	// Check if path is under any allowed root
	for _, root := range r.allowedRoots {
		absRoot, err := filepath.Abs(root)
		if err != nil {
			continue
		}

		if rel, err := filepath.Rel(absRoot, path); err == nil && !strings.HasPrefix(rel, "..") {
			return nil
		}
	}

	return &OperationError{
		Message:   "path is outside allowed directories",
		ErrorType: ErrorTypePathTraversal,
	}
}

// validateSymlinkTarget validates a symlink target path.
// Symlink targets use the same validation as regular paths.
func (r *PathResolver) validateSymlinkTarget(path string) error {
	// Use the same validation logic
	err := r.validatePath(path)
	if err != nil {
		// Convert error type to symlink-specific
		if opErr, ok := err.(*OperationError); ok {
			opErr.ErrorType = ErrorTypeSymlinkDenied
			opErr.Message = "symlink target violates security policy: " + opErr.Message
		}
	}
	return err
}
