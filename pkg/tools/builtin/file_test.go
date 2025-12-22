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
