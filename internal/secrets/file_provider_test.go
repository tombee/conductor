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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tombee/conductor/pkg/profile"
)

func TestFileProvider_Scheme(t *testing.T) {
	provider := NewFileProvider(FileProviderConfig{})
	if got := provider.Scheme(); got != "file" {
		t.Errorf("Scheme() = %q, want %q", got, "file")
	}
}

func TestFileProvider_Resolve_Disabled(t *testing.T) {
	provider := NewFileProvider(FileProviderConfig{
		Enabled: false,
	})

	ctx := context.Background()
	_, err := provider.Resolve(ctx, "/etc/secrets/token")

	if err == nil {
		t.Fatal("expected error when provider is disabled")
	}

	var resErr *profile.SecretResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected SecretResolutionError, got %T", err)
	}

	if resErr.Category != profile.ErrorCategoryAccessDenied {
		t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryAccessDenied)
	}
}

func TestFileProvider_Resolve_RelativePath(t *testing.T) {
	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{"/"},
	})

	ctx := context.Background()
	_, err := provider.Resolve(ctx, "../secrets/token")

	if err == nil {
		t.Fatal("expected error for relative path")
	}

	var resErr *profile.SecretResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected SecretResolutionError, got %T", err)
	}

	if resErr.Category != profile.ErrorCategoryInvalidSyntax {
		t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryInvalidSyntax)
	}
}

func TestFileProvider_Resolve_NotInAllowlist(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret-value"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{"/etc/secrets/"}, // Different path
	})

	ctx := context.Background()
	_, err := provider.Resolve(ctx, secretFile)

	if err == nil {
		t.Fatal("expected error for path not in allowlist")
	}

	var resErr *profile.SecretResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected SecretResolutionError, got %T", err)
	}

	if resErr.Category != profile.ErrorCategoryAccessDenied {
		t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryAccessDenied)
	}
}

func TestFileProvider_Resolve_Success(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	expectedValue := "ghp_1234567890abcdef"
	if err := os.WriteFile(secretFile, []byte(expectedValue+"\n"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{tmpDir + "/"},
	})

	ctx := context.Background()
	value, err := provider.Resolve(ctx, secretFile)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value != expectedValue {
		t.Errorf("Resolve() = %q, want %q", value, expectedValue)
	}
}

func TestFileProvider_Resolve_TrimsWhitespace(t *testing.T) {
	// Create temp file with trailing whitespace
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	expectedValue := "secret-value"
	if err := os.WriteFile(secretFile, []byte("  "+expectedValue+"  \n\t"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{tmpDir + "/"},
	})

	ctx := context.Background()
	value, err := provider.Resolve(ctx, secretFile)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value != expectedValue {
		t.Errorf("Resolve() = %q, want %q", value, expectedValue)
	}
}

func TestFileProvider_Resolve_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "nonexistent.txt")

	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{tmpDir + "/"},
	})

	ctx := context.Background()
	_, err := provider.Resolve(ctx, secretFile)

	if err == nil {
		t.Fatal("expected error for non-existent file")
	}

	var resErr *profile.SecretResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected SecretResolutionError, got %T", err)
	}

	if resErr.Category != profile.ErrorCategoryNotFound {
		t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryNotFound)
	}
}

func TestFileProvider_Resolve_EmptyFile(t *testing.T) {
	// Create empty file
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(secretFile, []byte(""), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{tmpDir + "/"},
	})

	ctx := context.Background()
	_, err := provider.Resolve(ctx, secretFile)

	if err == nil {
		t.Fatal("expected error for empty file")
	}

	var resErr *profile.SecretResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected SecretResolutionError, got %T", err)
	}

	if resErr.Category != profile.ErrorCategoryNotFound {
		t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryNotFound)
	}
}

func TestFileProvider_Resolve_FileTooLarge(t *testing.T) {
	// Create large file
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "large.txt")
	largeContent := strings.Repeat("x", MaxFileSize+1)
	if err := os.WriteFile(secretFile, []byte(largeContent), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{tmpDir + "/"},
	})

	ctx := context.Background()
	_, err := provider.Resolve(ctx, secretFile)

	if err == nil {
		t.Fatal("expected error for file too large")
	}

	var resErr *profile.SecretResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected SecretResolutionError, got %T", err)
	}

	if resErr.Category != profile.ErrorCategoryInvalidSyntax {
		t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryInvalidSyntax)
	}
}

func TestFileProvider_Resolve_Symlink_NotAllowed(t *testing.T) {
	// Create temp file and symlink
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret-value"), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	symlinkFile := filepath.Join(tmpDir, "secret-link.txt")
	if err := os.Symlink(secretFile, symlinkFile); err != nil {
		t.Skipf("symlink creation failed (may not be supported): %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:        true,
		Allowlist:      []string{tmpDir + "/"},
		FollowSymlinks: false,
	})

	ctx := context.Background()
	_, err := provider.Resolve(ctx, symlinkFile)

	if err == nil {
		t.Fatal("expected error for symlink when FollowSymlinks is false")
	}

	var resErr *profile.SecretResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected SecretResolutionError, got %T", err)
	}

	if resErr.Category != profile.ErrorCategoryAccessDenied {
		t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryAccessDenied)
	}
}

func TestFileProvider_Resolve_Symlink_Allowed(t *testing.T) {
	// Create temp file and symlink
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")
	expectedValue := "secret-value"
	if err := os.WriteFile(secretFile, []byte(expectedValue), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	symlinkFile := filepath.Join(tmpDir, "secret-link.txt")
	if err := os.Symlink(secretFile, symlinkFile); err != nil {
		t.Skipf("symlink creation failed (may not be supported): %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:        true,
		Allowlist:      []string{tmpDir + "/"},
		FollowSymlinks: true,
	})

	ctx := context.Background()
	value, err := provider.Resolve(ctx, symlinkFile)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if value != expectedValue {
		t.Errorf("Resolve() = %q, want %q", value, expectedValue)
	}
}

func TestFileProvider_Resolve_PathTraversal(t *testing.T) {
	// Create directory structure:
	// tmpDir/
	//   allowed/
	//     secret.txt
	//   forbidden/
	//     other.txt
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	forbiddenDir := filepath.Join(tmpDir, "forbidden")

	if err := os.Mkdir(allowedDir, 0755); err != nil {
		t.Fatalf("failed to create allowed dir: %v", err)
	}
	if err := os.Mkdir(forbiddenDir, 0755); err != nil {
		t.Fatalf("failed to create forbidden dir: %v", err)
	}

	forbiddenFile := filepath.Join(forbiddenDir, "other.txt")
	if err := os.WriteFile(forbiddenFile, []byte("forbidden"), 0600); err != nil {
		t.Fatalf("failed to create forbidden file: %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{allowedDir + "/"},
	})

	ctx := context.Background()

	// Try to access forbidden file directly
	_, err := provider.Resolve(ctx, forbiddenFile)
	if err == nil {
		t.Fatal("expected error for path outside allowlist")
	}

	var resErr *profile.SecretResolutionError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected SecretResolutionError, got %T", err)
	}

	if resErr.Category != profile.ErrorCategoryAccessDenied {
		t.Errorf("Category = %q, want %q", resErr.Category, profile.ErrorCategoryAccessDenied)
	}
}

func TestFileProvider_AllowlistMatching(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		allowlist []string
		path      string
		allowed   bool
	}{
		{
			name:      "exact match",
			allowlist: []string{filepath.Join(tmpDir, "secret.txt")},
			path:      filepath.Join(tmpDir, "secret.txt"),
			allowed:   true,
		},
		{
			name:      "directory match with trailing slash",
			allowlist: []string{tmpDir + "/"},
			path:      filepath.Join(tmpDir, "secret.txt"),
			allowed:   true,
		},
		{
			name:      "directory match without trailing slash",
			allowlist: []string{tmpDir},
			path:      filepath.Join(tmpDir, "secret.txt"),
			allowed:   true,
		},
		{
			name:      "subdirectory match",
			allowlist: []string{tmpDir + "/"},
			path:      filepath.Join(tmpDir, "subdir", "secret.txt"),
			allowed:   true,
		},
		{
			name:      "no match",
			allowlist: []string{"/etc/secrets/"},
			path:      filepath.Join(tmpDir, "secret.txt"),
			allowed:   false,
		},
		{
			name:      "empty allowlist",
			allowlist: []string{},
			path:      filepath.Join(tmpDir, "secret.txt"),
			allowed:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := NewFileProvider(FileProviderConfig{
				Enabled:   true,
				Allowlist: tt.allowlist,
			})

			allowed := provider.isAllowed(tt.path)
			if allowed != tt.allowed {
				t.Errorf("isAllowed(%q) = %v, want %v (allowlist: %v)",
					tt.path, allowed, tt.allowed, tt.allowlist)
			}
		})
	}
}

func TestFileProvider_CustomMaxSize(t *testing.T) {
	tmpDir := t.TempDir()
	secretFile := filepath.Join(tmpDir, "secret.txt")

	// Create file slightly under custom limit
	customMaxSize := int64(100)
	content := strings.Repeat("x", int(customMaxSize)-1)
	if err := os.WriteFile(secretFile, []byte(content), 0600); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	provider := NewFileProvider(FileProviderConfig{
		Enabled:   true,
		Allowlist: []string{tmpDir + "/"},
		MaxSize:   customMaxSize,
	})

	ctx := context.Background()

	// Should succeed with custom max size
	value, err := provider.Resolve(ctx, secretFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(value) != int(customMaxSize)-1 {
		t.Errorf("value length = %d, want %d", len(value), customMaxSize-1)
	}

	// Create file over custom limit
	largeFile := filepath.Join(tmpDir, "large.txt")
	largeContent := strings.Repeat("y", int(customMaxSize)+1)
	if err := os.WriteFile(largeFile, []byte(largeContent), 0600); err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}

	// Should fail with custom max size
	_, err = provider.Resolve(ctx, largeFile)
	if err == nil {
		t.Fatal("expected error for file exceeding custom max size")
	}
}
