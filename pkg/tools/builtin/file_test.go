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
