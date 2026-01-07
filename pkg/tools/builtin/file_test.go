package builtin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileTool_Name(t *testing.T) {
	tool := NewFileTool()
	if tool.Name() != "file" {
		t.Errorf("Name() = %s, want file", tool.Name())
	}
}

func TestFileTool_Schema(t *testing.T) {
	tool := NewFileTool()
	schema := tool.Schema()

	if schema == nil {
		t.Fatal("Schema() returned nil")
	}

	if schema.Inputs == nil {
		t.Fatal("Schema inputs is nil")
	}

	// Check required fields
	requiredFields := map[string]bool{
		"operation": false,
		"path":      false,
	}
	for _, field := range schema.Inputs.Required {
		requiredFields[field] = true
	}
	for field, found := range requiredFields {
		if !found {
			t.Errorf("Required field %s not in schema", field)
		}
	}
}

func TestFileTool_ReadWrite(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"

	tool := NewFileTool()
	ctx := context.Background()

	// Test write
	t.Run("write file", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"operation": "write",
			"path":      testFile,
			"content":   testContent,
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if success, ok := result["success"].(bool); !ok || !success {
			t.Errorf("Write failed: %v", result)
		}

		// Verify file exists
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("File was not created")
		}
	})

	// Test read
	t.Run("read file", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"operation": "read",
			"path":      testFile,
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if success, ok := result["success"].(bool); !ok || !success {
			t.Errorf("Read failed: %v", result)
		}

		content, ok := result["content"].(string)
		if !ok {
			t.Fatal("Content is not a string")
		}

		if content != testContent {
			t.Errorf("Content = %q, want %q", content, testContent)
		}
	})
}

func TestFileTool_ReadNonExistent(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      "/nonexistent/file.txt",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if success, ok := result["success"].(bool); !ok || success {
		t.Error("Read should have failed for nonexistent file")
	}

	if _, ok := result["error"]; !ok {
		t.Error("Result should contain error message")
	}
}

func TestFileTool_InvalidOperation(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "invalid",
		"path":      "/tmp/test.txt",
	})
	if err == nil {
		t.Error("Execute() should fail with invalid operation")
	}
}

func TestFileTool_MaxFileSize(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create a file larger than the limit
	largeContent := make([]byte, 1024)
	if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create tool with small size limit
	tool := NewFileTool().WithMaxFileSize(512)
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      testFile,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if success, ok := result["success"].(bool); !ok || success {
		t.Error("Read should have failed due to file size limit")
	}
}

func TestFileTool_AllowedPaths(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed directory: %v", err)
	}

	tool := NewFileTool().WithAllowedPaths([]string{allowedDir})
	ctx := context.Background()

	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{
			name:      "allowed path",
			path:      filepath.Join(allowedDir, "test.txt"),
			shouldErr: false,
		},
		{
			name:      "disallowed path",
			path:      filepath.Join(tmpDir, "disallowed.txt"),
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(ctx, map[string]interface{}{
				"operation": "write",
				"path":      tt.path,
				"content":   "test",
			})

			if tt.shouldErr && err == nil {
				t.Error("Execute() should have failed for disallowed path")
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("Execute() unexpected error: %v", err)
			}
		})
	}
}

func TestFileTool_WriteCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "subdir", "nested", "test.txt")

	tool := NewFileTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "write",
		"path":      testFile,
		"content":   "nested content",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Write should create nested directories: %v", result)
	}

	// Verify file was created
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("File was not created in nested directory")
	}
}

func TestFileTool_ReadEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	// Create empty file
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewFileTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      testFile,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Read should succeed for empty file: %v", result)
	}

	content, ok := result["content"].(string)
	if !ok {
		t.Fatal("Content is not a string")
	}

	if content != "" {
		t.Errorf("Content = %q, want empty string", content)
	}
}

func TestFileTool_OverwriteExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "overwrite.txt")

	tool := NewFileTool()
	ctx := context.Background()

	// Write initial content
	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "write",
		"path":      testFile,
		"content":   "initial content",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Overwrite with new content
	result, err = tool.Execute(ctx, map[string]interface{}{
		"operation": "write",
		"path":      testFile,
		"content":   "new content",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Overwrite should succeed: %v", result)
	}

	// Verify new content
	result, err = tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      testFile,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	content, ok := result["content"].(string)
	if !ok {
		t.Fatal("Content is not a string")
	}

	if content != "new content" {
		t.Errorf("Content = %q, want %q", content, "new content")
	}
}

func TestFileTool_WriteMissingContent(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	tool := NewFileTool()
	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "write",
		"path":      testFile,
		// Missing "content" field
	})
	if err == nil {
		t.Error("Execute() should fail when content is missing for write operation")
	}
}

func TestFileTool_DirectoryTraversalPrevention(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	// These should fail due to path normalization check
	dangerousPaths := []string{
		"/tmp/../../../etc/passwd",
		"./../../sensitive.txt",
	}

	for _, path := range dangerousPaths {
		t.Run(path, func(t *testing.T) {
			// Note: The validation logic checks if cleanPath != path
			// So paths with ".." will fail
			_, err := tool.Execute(ctx, map[string]interface{}{
				"operation": "read",
				"path":      path,
			})
			if err == nil {
				t.Error("Execute() should prevent directory traversal")
			}
		})
	}
}

func TestFileTool_AllowedPathsAbsoluteCheck(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed directory: %v", err)
	}

	tool := NewFileTool().WithAllowedPaths([]string{allowedDir})
	ctx := context.Background()

	// Test with relative path inside allowed directory
	relPath := filepath.Join(allowedDir, "test.txt")

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "write",
		"path":      relPath,
		"content":   "test",
	})

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Write should succeed for allowed path: %v", result)
	}
}

func TestFileTool_Description(t *testing.T) {
	tool := NewFileTool()
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestFileTool_MissingOperation(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"path": "/tmp/test.txt",
		// Missing "operation" field
	})
	if err == nil {
		t.Error("Execute() should fail when operation is missing")
	}
}

func TestFileTool_InvalidOperationType(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"operation": 123, // Invalid type
		"path":      "/tmp/test.txt",
	})
	if err == nil {
		t.Error("Execute() should fail when operation is not a string")
	}
}

func TestFileTool_InvalidPathType(t *testing.T) {
	tool := NewFileTool()
	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      123, // Invalid type
	})
	if err == nil {
		t.Error("Execute() should fail when path is not a string")
	}
}

func TestFileTool_InvalidContentType(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	tool := NewFileTool()
	ctx := context.Background()

	_, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "write",
		"path":      testFile,
		"content":   123, // Invalid type
	})
	if err == nil {
		t.Error("Execute() should fail when content is not a string")
	}
}

// TestFileTool_SymlinkOutsideAllowedDir tests that symlinks pointing outside allowed directories are blocked.
func TestFileTool_SymlinkOutsideAllowedDir(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	outsideDir := filepath.Join(tmpDir, "outside")

	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed directory: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("Failed to create outside directory: %v", err)
	}

	// Create a file outside the allowed directory
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret data"), 0644); err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}

	// Create a symlink inside allowed directory pointing to outside file
	symlinkPath := filepath.Join(allowedDir, "link_to_secret")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Configure tool with allowed read paths
	tool := NewFileTool().WithAllowedReadPaths([]string{allowedDir})
	ctx := context.Background()

	// Attempt to read via symlink should fail
	_, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      symlinkPath,
	})
	if err == nil {
		t.Error("Execute() should block reading symlink pointing outside allowed directory")
	}
}

// TestFileTool_SymlinkInsideAllowedDir tests that symlinks within allowed directories are permitted.
func TestFileTool_SymlinkInsideAllowedDir(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")

	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed directory: %v", err)
	}

	// Create a file inside allowed directory
	realFile := filepath.Join(allowedDir, "real.txt")
	testContent := "test content"
	if err := os.WriteFile(realFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create real file: %v", err)
	}

	// Create symlink to file (both in allowed directory)
	symlinkPath := filepath.Join(allowedDir, "link_to_real")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Configure tool with allowed read paths
	tool := NewFileTool().
		WithAllowedReadPaths([]string{allowedDir}).
		WithVerboseErrors(true)
	ctx := context.Background()

	// Reading via symlink should succeed
	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      symlinkPath,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Read via symlink should succeed when target is in allowed dir: %v", result)
	}

	content, ok := result["content"].(string)
	if !ok || content != testContent {
		t.Errorf("Content = %q, want %q", content, testContent)
	}
}

// TestFileTool_NonExistentFileInAllowedDir tests that writing non-existent files in allowed directories is permitted.
func TestFileTool_NonExistentFileInAllowedDir(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")

	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed directory: %v", err)
	}

	// Configure tool with allowed write paths
	tool := NewFileTool().WithAllowedWritePaths([]string{allowedDir})
	ctx := context.Background()

	// Write to non-existent file should succeed
	nonExistentFile := filepath.Join(allowedDir, "new_file.txt")
	testContent := "new content"

	result, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "write",
		"path":      nonExistentFile,
		"content":   testContent,
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if success, ok := result["success"].(bool); !ok || !success {
		t.Errorf("Write to non-existent file in allowed dir should succeed: %v", result)
	}

	// Verify file was created
	if _, err := os.Stat(nonExistentFile); os.IsNotExist(err) {
		t.Error("File should have been created")
	}
}

// TestFileTool_SymlinkChainEscape tests that symlink chains are fully resolved and escapes are blocked.
func TestFileTool_SymlinkChainEscape(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	outsideDir := filepath.Join(tmpDir, "outside")

	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed directory: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("Failed to create outside directory: %v", err)
	}

	// Create file outside allowed directory
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}

	// Create symlink chain: link1 -> link2 -> outside_file
	link2 := filepath.Join(allowedDir, "link2")
	if err := os.Symlink(outsideFile, link2); err != nil {
		t.Fatalf("Failed to create link2: %v", err)
	}

	link1 := filepath.Join(allowedDir, "link1")
	if err := os.Symlink(link2, link1); err != nil {
		t.Fatalf("Failed to create link1: %v", err)
	}

	// Configure tool with allowed read paths
	tool := NewFileTool().WithAllowedReadPaths([]string{allowedDir})
	ctx := context.Background()

	// Attempt to read via symlink chain should fail
	_, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      link1,
	})
	if err == nil {
		t.Error("Execute() should block symlink chain escaping allowed directory")
	}
}

// TestFileTool_SymlinkDirEscape tests that symlinks to directories outside allowed paths are blocked.
func TestFileTool_SymlinkDirEscape(t *testing.T) {
	tmpDir := t.TempDir()
	allowedDir := filepath.Join(tmpDir, "allowed")
	outsideDir := filepath.Join(tmpDir, "outside")

	if err := os.MkdirAll(allowedDir, 0755); err != nil {
		t.Fatalf("Failed to create allowed directory: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("Failed to create outside directory: %v", err)
	}

	// Create a file in the outside directory
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0644); err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}

	// Create symlink to directory outside allowed path
	symlinkDir := filepath.Join(allowedDir, "link_to_outside_dir")
	if err := os.Symlink(outsideDir, symlinkDir); err != nil {
		t.Fatalf("Failed to create directory symlink: %v", err)
	}

	// Configure tool with allowed read paths
	tool := NewFileTool().WithAllowedReadPaths([]string{allowedDir})
	ctx := context.Background()

	// Attempt to read file through symlinked directory should fail
	pathThroughSymlink := filepath.Join(symlinkDir, "secret.txt")
	_, err := tool.Execute(ctx, map[string]interface{}{
		"operation": "read",
		"path":      pathThroughSymlink,
	})
	if err == nil {
		t.Error("Execute() should block access through directory symlink to outside allowed path")
	}
}

// TestFileTool_SeparateReadWritePaths tests that WithAllowedReadPaths and WithAllowedWritePaths work independently.
func TestFileTool_SeparateReadWritePaths(t *testing.T) {
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "read_only")
	writeOnlyDir := filepath.Join(tmpDir, "write_only")

	if err := os.MkdirAll(readOnlyDir, 0755); err != nil {
		t.Fatalf("Failed to create read-only directory: %v", err)
	}
	if err := os.MkdirAll(writeOnlyDir, 0755); err != nil {
		t.Fatalf("Failed to create write-only directory: %v", err)
	}

	// Create a file in read-only directory
	readFile := filepath.Join(readOnlyDir, "readable.txt")
	if err := os.WriteFile(readFile, []byte("readable content"), 0644); err != nil {
		t.Fatalf("Failed to create readable file: %v", err)
	}

	// Configure tool with separate read/write paths
	tool := NewFileTool().
		WithAllowedReadPaths([]string{readOnlyDir}).
		WithAllowedWritePaths([]string{writeOnlyDir})
	ctx := context.Background()

	// Test 1: Read from read-only directory should succeed
	t.Run("read from read-only dir", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"operation": "read",
			"path":      readFile,
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if success, ok := result["success"].(bool); !ok || !success {
			t.Errorf("Read from read-only dir should succeed: %v", result)
		}
	})

	// Test 2: Write to read-only directory should fail
	t.Run("write to read-only dir", func(t *testing.T) {
		writeFileInReadDir := filepath.Join(readOnlyDir, "should_fail.txt")
		_, err := tool.Execute(ctx, map[string]interface{}{
			"operation": "write",
			"path":      writeFileInReadDir,
			"content":   "should not write",
		})
		if err == nil {
			t.Error("Execute() should fail when writing to read-only directory")
		}
	})

	// Test 3: Write to write-only directory should succeed
	t.Run("write to write-only dir", func(t *testing.T) {
		writeFile := filepath.Join(writeOnlyDir, "writable.txt")
		result, err := tool.Execute(ctx, map[string]interface{}{
			"operation": "write",
			"path":      writeFile,
			"content":   "writable content",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		if success, ok := result["success"].(bool); !ok || !success {
			t.Errorf("Write to write-only dir should succeed: %v", result)
		}
	})

	// Test 4: Read from write-only directory should fail
	t.Run("read from write-only dir", func(t *testing.T) {
		writeFile := filepath.Join(writeOnlyDir, "writable.txt")
		_, err := tool.Execute(ctx, map[string]interface{}{
			"operation": "read",
			"path":      writeFile,
		})
		if err == nil {
			t.Error("Execute() should fail when reading from write-only directory")
		}
	})
}

// TestFileTool_FilePermissions tests that files are written with appropriate permissions based on filename patterns.
func TestFileTool_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name         string
		filename     string
		expectedPerm os.FileMode
		description  string
	}{
		{
			name:         "config file",
			filename:     "config.yaml",
			expectedPerm: 0600,
			description:  "config files should be 0600",
		},
		{
			name:         "secret file",
			filename:     "secrets.json",
			expectedPerm: 0600,
			description:  "secret files should be 0600",
		},
		{
			name:         "env file",
			filename:     ".env",
			expectedPerm: 0600,
			description:  "env files should be 0600",
		},
		{
			name:         "key file",
			filename:     "private.key",
			expectedPerm: 0600,
			description:  "key files should be 0600",
		},
		{
			name:         "general file",
			filename:     "output.txt",
			expectedPerm: 0640,
			description:  "general files should be 0640",
		},
		{
			name:         "data file",
			filename:     "data.json",
			expectedPerm: 0640,
			description:  "data files should be 0640",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.filename)

			// Write file
			result, err := tool.Execute(ctx, map[string]interface{}{
				"operation": "write",
				"path":      testFile,
				"content":   "test content",
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if success, ok := result["success"].(bool); !ok || !success {
				t.Errorf("Write failed: %v", result)
			}

			// Check file permissions
			info, err := os.Stat(testFile)
			if err != nil {
				t.Fatalf("failed to stat file: %v", err)
			}

			actualPerm := info.Mode().Perm()
			if actualPerm != tt.expectedPerm {
				t.Errorf("%s: file permissions = %o, want %o", tt.description, actualPerm, tt.expectedPerm)
			}
		})
	}
}

// TestFileTool_DirectoryPermissions tests that parent directories are created with appropriate permissions.
func TestFileTool_DirectoryPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name         string
		filepath     string
		expectedPerm os.FileMode
		description  string
	}{
		{
			name:         "config file parent",
			filepath:     "configs/app/config.yaml",
			expectedPerm: 0700,
			description:  "parent dir of sensitive file should be 0700",
		},
		{
			name:         "general file parent",
			filepath:     "data/output/result.txt",
			expectedPerm: 0750,
			description:  "parent dir of general file should be 0750",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.filepath)

			// Write file
			result, err := tool.Execute(ctx, map[string]interface{}{
				"operation": "write",
				"path":      testFile,
				"content":   "test content",
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if success, ok := result["success"].(bool); !ok || !success {
				t.Errorf("Write failed: %v", result)
			}

			// Check parent directory permissions
			parentDir := filepath.Dir(testFile)
			info, err := os.Stat(parentDir)
			if err != nil {
				t.Fatalf("failed to stat parent directory: %v", err)
			}

			actualPerm := info.Mode().Perm()
			if actualPerm != tt.expectedPerm {
				t.Errorf("%s: directory permissions = %o, want %o", tt.description, actualPerm, tt.expectedPerm)
			}
		})
	}
}

// Tests for line-limited reading functionality

func TestFileTool_ReadWithMaxLines(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file with 10 lines
	content := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name            string
		maxLines        int
		expectedLines   int
		expectedContent string
		expectTruncated bool
	}{
		{
			name:            "read 5 lines from 10",
			maxLines:        5,
			expectedLines:   5,
			expectedContent: "line 1\nline 2\nline 3\nline 4\nline 5",
			expectTruncated: true,
		},
		{
			name:            "read all lines (max_lines equals total)",
			maxLines:        10,
			expectedLines:   10,
			expectedContent: content,
			expectTruncated: false,
		},
		{
			name:            "read more lines than available",
			maxLines:        20,
			expectedLines:   10,
			expectedContent: content,
			expectTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]interface{}{
				"operation": "read",
				"path":      testFile,
				"max_lines": float64(tt.maxLines), // JSON unmarshaling produces float64
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if success, ok := result["success"].(bool); !ok || !success {
				t.Errorf("Read failed: %v", result)
			}

			readContent, ok := result["content"].(string)
			if !ok {
				t.Fatal("Content is not a string")
			}

			if readContent != tt.expectedContent {
				t.Errorf("Content = %q, want %q", readContent, tt.expectedContent)
			}

			// Check metadata
			metadata, ok := result["metadata"].(map[string]interface{})
			if !ok {
				t.Fatal("Metadata is missing or not a map")
			}

			truncated, ok := metadata["truncated"].(bool)
			if !ok {
				t.Fatal("Metadata truncated field missing or not bool")
			}
			if truncated != tt.expectTruncated {
				t.Errorf("Truncated = %v, want %v", truncated, tt.expectTruncated)
			}

			linesShown, ok := metadata["lines_shown"].(int)
			if !ok {
				t.Fatal("Metadata lines_shown field missing or not int")
			}
			if linesShown != tt.expectedLines {
				t.Errorf("lines_shown = %d, want %d", linesShown, tt.expectedLines)
			}

			totalLines, ok := metadata["total_lines"].(int)
			if !ok {
				t.Fatal("Metadata total_lines field missing or not int")
			}
			if totalLines != 10 {
				t.Errorf("total_lines = %d, want 10", totalLines)
			}
		})
	}
}

func TestFileTool_ReadWithOffset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file with 10 lines
	content := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name            string
		offset          int
		expectedContent string
		expectedLines   int
		expectMetadata  bool
	}{
		{
			name:            "skip 5 lines",
			offset:          5,
			expectedContent: "line 6\nline 7\nline 8\nline 9\nline 10",
			expectedLines:   5,
			expectMetadata:  true,
		},
		{
			name:            "skip 9 lines (read last line)",
			offset:          9,
			expectedContent: "line 10",
			expectedLines:   1,
			expectMetadata:  true,
		},
		{
			name:            "offset equals file length",
			offset:          10,
			expectedContent: "",
			expectedLines:   0,
			expectMetadata:  true,
		},
		{
			name:            "offset beyond file length",
			offset:          20,
			expectedContent: "",
			expectedLines:   0,
			expectMetadata:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]interface{}{
				"operation": "read",
				"path":      testFile,
				"offset":    float64(tt.offset),
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if success, ok := result["success"].(bool); !ok || !success {
				t.Errorf("Read failed: %v", result)
			}

			readContent, ok := result["content"].(string)
			if !ok {
				t.Fatal("Content is not a string")
			}

			if readContent != tt.expectedContent {
				t.Errorf("Content = %q, want %q", readContent, tt.expectedContent)
			}

			// Check metadata if expected
			if tt.expectMetadata {
				metadata, ok := result["metadata"].(map[string]interface{})
				if !ok {
					t.Fatal("Metadata is missing or not a map")
				}

				linesShown, ok := metadata["lines_shown"].(int)
				if !ok {
					t.Fatal("Metadata lines_shown field missing or not int")
				}
				if linesShown != tt.expectedLines {
					t.Errorf("lines_shown = %d, want %d", linesShown, tt.expectedLines)
				}

				startLine, ok := metadata["start_line"].(int)
				if !ok {
					t.Fatal("Metadata start_line field missing or not int")
				}
				if startLine != tt.offset {
					t.Errorf("start_line = %d, want %d", startLine, tt.offset)
				}
			}
		})
	}
}

func TestFileTool_ReadWithMaxLinesAndOffset(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create a file with 10 lines
	content := "line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name            string
		offset          int
		maxLines        int
		expectedContent string
		expectedLines   int
		expectTruncated bool
	}{
		{
			name:            "skip 3, read 4",
			offset:          3,
			maxLines:        4,
			expectedContent: "line 4\nline 5\nline 6\nline 7",
			expectedLines:   4,
			expectTruncated: true,
		},
		{
			name:            "skip 5, read 10 (but only 5 available)",
			offset:          5,
			maxLines:        10,
			expectedContent: "line 6\nline 7\nline 8\nline 9\nline 10",
			expectedLines:   5,
			expectTruncated: false,
		},
		{
			name:            "skip 8, read 1",
			offset:          8,
			maxLines:        1,
			expectedContent: "line 9",
			expectedLines:   1,
			expectTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, map[string]interface{}{
				"operation": "read",
				"path":      testFile,
				"offset":    float64(tt.offset),
				"max_lines": float64(tt.maxLines),
			})
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if success, ok := result["success"].(bool); !ok || !success {
				t.Errorf("Read failed: %v", result)
			}

			readContent, ok := result["content"].(string)
			if !ok {
				t.Fatal("Content is not a string")
			}

			if readContent != tt.expectedContent {
				t.Errorf("Content = %q, want %q", readContent, tt.expectedContent)
			}

			// Check metadata
			metadata, ok := result["metadata"].(map[string]interface{})
			if !ok {
				t.Fatal("Metadata is missing or not a map")
			}

			truncated, ok := metadata["truncated"].(bool)
			if !ok {
				t.Fatal("Metadata truncated field missing or not bool")
			}
			if truncated != tt.expectTruncated {
				t.Errorf("Truncated = %v, want %v", truncated, tt.expectTruncated)
			}

			linesShown, ok := metadata["lines_shown"].(int)
			if !ok {
				t.Fatal("Metadata lines_shown field missing or not int")
			}
			if linesShown != tt.expectedLines {
				t.Errorf("lines_shown = %d, want %d", linesShown, tt.expectedLines)
			}
		})
	}
}

func TestFileTool_ParameterValidation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name        string
		maxLines    interface{}
		offset      interface{}
		expectError bool
		errorField  string
	}{
		{
			name:        "negative max_lines",
			maxLines:    float64(-5),
			expectError: true,
			errorField:  "max_lines",
		},
		{
			name:        "negative offset",
			offset:      float64(-3),
			expectError: true,
			errorField:  "offset",
		},
		{
			name:        "invalid max_lines type (string)",
			maxLines:    "not a number",
			expectError: true,
			errorField:  "max_lines",
		},
		{
			name:        "invalid offset type (string)",
			offset:      "not a number",
			expectError: true,
			errorField:  "offset",
		},
		{
			name:        "valid max_lines as int",
			maxLines:    5,
			expectError: false,
		},
		{
			name:        "valid offset as int",
			offset:      2,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]interface{}{
				"operation": "read",
				"path":      testFile,
			}
			if tt.maxLines != nil {
				inputs["max_lines"] = tt.maxLines
			}
			if tt.offset != nil {
				inputs["offset"] = tt.offset
			}

			result, err := tool.Execute(ctx, inputs)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if success, ok := result["success"].(bool); !ok || !success {
					t.Errorf("Read failed: %v", result)
				}
			}
		})
	}
}

func TestFileTool_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "line 1\nline 2\nline 3\nline 4\nline 5"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewFileTool()
	ctx := context.Background()

	t.Run("read without parameters returns full file", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"operation": "read",
			"path":      testFile,
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if success, ok := result["success"].(bool); !ok || !success {
			t.Errorf("Read failed: %v", result)
		}

		readContent, ok := result["content"].(string)
		if !ok {
			t.Fatal("Content is not a string")
		}

		if readContent != content {
			t.Errorf("Content = %q, want %q", readContent, content)
		}

		// Should not have metadata for unlimited read
		if _, hasMetadata := result["metadata"]; hasMetadata {
			t.Error("Unexpected metadata for unlimited read")
		}
	})

	t.Run("explicit nil max_lines means unlimited", func(t *testing.T) {
		result, err := tool.Execute(ctx, map[string]interface{}{
			"operation": "read",
			"path":      testFile,
			"max_lines": nil,
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if success, ok := result["success"].(bool); !ok || !success {
			t.Errorf("Read failed: %v", result)
		}

		readContent, ok := result["content"].(string)
		if !ok {
			t.Fatal("Content is not a string")
		}

		if readContent != content {
			t.Errorf("Content = %q, want %q", readContent, content)
		}
	})
}

func TestFileTool_EmptyFileWithLimits(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty.txt")

	// Create empty file
	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tool := NewFileTool()
	ctx := context.Background()

	tests := []struct {
		name     string
		maxLines int
		offset   int
	}{
		{"empty with max_lines", 10, 0},
		{"empty with offset", 0, 5},
		{"empty with both", 10, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]interface{}{
				"operation": "read",
				"path":      testFile,
			}
			if tt.maxLines > 0 {
				inputs["max_lines"] = float64(tt.maxLines)
			}
			if tt.offset > 0 {
				inputs["offset"] = float64(tt.offset)
			}

			result, err := tool.Execute(ctx, inputs)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if success, ok := result["success"].(bool); !ok || !success {
				t.Errorf("Read failed: %v", result)
			}

			readContent, ok := result["content"].(string)
			if !ok {
				t.Fatal("Content is not a string")
			}

			if readContent != "" {
				t.Errorf("Expected empty content, got %q", readContent)
			}

			// Check metadata indicates empty file
			metadata, ok := result["metadata"].(map[string]interface{})
			if !ok {
				t.Fatal("Metadata is missing or not a map")
			}

			linesShown, ok := metadata["lines_shown"].(int)
			if !ok {
				t.Fatal("Metadata lines_shown field missing or not int")
			}
			if linesShown != 0 {
				t.Errorf("lines_shown = %d, want 0", linesShown)
			}
		})
	}
}
