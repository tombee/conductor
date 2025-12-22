package file

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFileAction_ReadText(t *testing.T) {
	// Create temp directory for test files
	tempDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create action
	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test read_text operation
	result, err := action.Execute(context.Background(), "read_text", map[string]interface{}{
		"path": "./test.txt",
	})

	if err != nil {
		t.Fatalf("read_text failed: %v", err)
	}

	if result.Response != testContent {
		t.Errorf("Expected content %q, got %q", testContent, result.Response)
	}
}

func TestFileAction_ReadJSON(t *testing.T) {
	tempDir := t.TempDir()

	// Create test JSON file
	testFile := filepath.Join(tempDir, "test.json")
	testContent := `{"name":"John","age":30}`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test read_json operation
	result, err := action.Execute(context.Background(), "read_json", map[string]interface{}{
		"path": "./test.json",
	})

	if err != nil {
		t.Fatalf("read_json failed: %v", err)
	}

	// Check result is a map
	data, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", result.Response)
	}

	if data["name"] != "John" {
		t.Errorf("Expected name=John, got %v", data["name"])
	}
}

func TestFileAction_ReadJSON_WithExtraction(t *testing.T) {
	tempDir := t.TempDir()

	// Create test JSON file
	testFile := filepath.Join(tempDir, "config.json")
	testContent := `{"database":{"host":"localhost","port":5432}}`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test read_json with extraction
	result, err := action.Execute(context.Background(), "read_json", map[string]interface{}{
		"path":    "./config.json",
		"extract": "$.database.host",
	})

	if err != nil {
		t.Fatalf("read_json with extraction failed: %v", err)
	}

	if result.Response != "localhost" {
		t.Errorf("Expected extracted value 'localhost', got %v", result.Response)
	}
}

func TestFileAction_ReadYAML(t *testing.T) {
	tempDir := t.TempDir()

	// Create test YAML file
	testFile := filepath.Join(tempDir, "test.yaml")
	testContent := `name: John
age: 30
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test read_yaml operation
	result, err := action.Execute(context.Background(), "read_yaml", map[string]interface{}{
		"path": "./test.yaml",
	})

	if err != nil {
		t.Fatalf("read_yaml failed: %v", err)
	}

	// Check result is a map
	data, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map, got %T", result.Response)
	}

	if data["name"] != "John" {
		t.Errorf("Expected name=John, got %v", data["name"])
	}
}

func TestFileAction_ReadCSV(t *testing.T) {
	tempDir := t.TempDir()

	// Create test CSV file
	testFile := filepath.Join(tempDir, "test.csv")
	testContent := `name,age,city
John,30,NYC
Jane,25,LA
`
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test read_csv operation
	result, err := action.Execute(context.Background(), "read_csv", map[string]interface{}{
		"path": "./test.csv",
	})

	if err != nil {
		t.Fatalf("read_csv failed: %v", err)
	}

	// Check result is an array
	data, ok := result.Response.([]map[string]string)
	if !ok {
		t.Fatalf("Expected []map[string]string, got %T", result.Response)
	}

	if len(data) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(data))
	}

	if data[0]["name"] != "John" {
		t.Errorf("Expected first row name=John, got %v", data[0]["name"])
	}

	if data[1]["city"] != "LA" {
		t.Errorf("Expected second row city=LA, got %v", data[1]["city"])
	}
}

func TestFileAction_ReadLines(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file with multiple lines
	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "line1\nline2\nline3"
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test read_lines operation
	result, err := action.Execute(context.Background(), "read_lines", map[string]interface{}{
		"path": "./test.txt",
	})

	if err != nil {
		t.Fatalf("read_lines failed: %v", err)
	}

	// Check result is a string slice
	lines, ok := result.Response.([]string)
	if !ok {
		t.Fatalf("Expected []string, got %T", result.Response)
	}

	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	if lines[0] != "line1" {
		t.Errorf("Expected first line 'line1', got %q", lines[0])
	}
}

func TestFileAction_ReadAutoDetect(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantType string
	}{
		{
			name:     "JSON auto-detection",
			filename: "data.json",
			content:  `{"key":"value"}`,
			wantType: "map",
		},
		{
			name:     "YAML auto-detection",
			filename: "data.yaml",
			content:  "key: value\n",
			wantType: "map",
		},
		{
			name:     "CSV auto-detection",
			filename: "data.csv",
			content:  "name,age\nJohn,30\n",
			wantType: "slice",
		},
		{
			name:     "Text fallback",
			filename: "data.txt",
			content:  "plain text content",
			wantType: "string",
		},
	}

	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tempDir, tt.filename)
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test read operation (auto-detect)
			result, err := action.Execute(context.Background(), "read", map[string]interface{}{
				"path": "./" + tt.filename,
			})

			if err != nil {
				t.Fatalf("read failed: %v", err)
			}

			// Check result type matches expected
			switch tt.wantType {
			case "map":
				if _, ok := result.Response.(map[string]interface{}); !ok {
					t.Errorf("Expected map[string]interface{}, got %T", result.Response)
				}
			case "slice":
				if _, ok := result.Response.([]map[string]string); !ok {
					t.Errorf("Expected []map[string]string, got %T", result.Response)
				}
			case "string":
				if _, ok := result.Response.(string); !ok {
					t.Errorf("Expected string, got %T", result.Response)
				}
			}
		})
	}
}

func TestFileAction_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test read_text with non-existent file
	_, err = action.Execute(context.Background(), "read_text", map[string]interface{}{
		"path": "./nonexistent.txt",
	})

	if err == nil {
		t.Fatal("Expected error for non-existent file, got nil")
	}

	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("Expected OperationError, got %T", err)
	}

	if opErr.ErrorType != ErrorTypeFileNotFound {
		t.Errorf("Expected ErrorTypeFileNotFound, got %v", opErr.ErrorType)
	}
}

func TestFileAction_BOMStripping(t *testing.T) {
	tempDir := t.TempDir()

	// Create test file with UTF-8 BOM
	testFile := filepath.Join(tempDir, "bom.txt")
	testContent := []byte{0xEF, 0xBB, 0xBF, 'H', 'e', 'l', 'l', 'o'}
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := &Config{
		WorkflowDir: tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test read_text strips BOM
	result, err := action.Execute(context.Background(), "read_text", map[string]interface{}{
		"path": "./bom.txt",
	})

	if err != nil {
		t.Fatalf("read_text failed: %v", err)
	}

	if result.Response != "Hello" {
		t.Errorf("Expected 'Hello' (BOM stripped), got %q", result.Response)
	}
}

func TestFileAction_UnknownOperation(t *testing.T) {
	config := &Config{}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Test unknown operation
	_, err = action.Execute(context.Background(), "invalid_op", map[string]interface{}{})

	if err == nil {
		t.Fatal("Expected error for unknown operation, got nil")
	}

	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("Expected OperationError, got %T", err)
	}

	if opErr.ErrorType != ErrorTypeValidation {
		t.Errorf("Expected ErrorTypeValidation, got %v", opErr.ErrorType)
	}
}

// Phase 2 tests: Write operations

func TestFileAction_WriteText(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	testPath := "./test.txt"
	testContent := "Hello, World!"

	inputs := map[string]interface{}{
		"path":    testPath,
		"content": testContent,
	}

	result, err := action.Execute(context.Background(), "write_text", inputs)
	if err != nil {
		t.Fatalf("write_text failed: %v", err)
	}

	if result.Metadata["bytes"].(int) != len(testContent) {
		t.Errorf("Expected bytes=%d, got %v", len(testContent), result.Metadata["bytes"])
	}

	// Verify file was written
	fullPath := filepath.Join(tmpDir, "test.txt")
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content %q, got %q", testContent, string(content))
	}
}

func TestFileAction_WriteJSON(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	testPath := "./test.json"
	testData := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	inputs := map[string]interface{}{
		"path":    testPath,
		"content": testData,
	}

	result, err := action.Execute(context.Background(), "write_json", inputs)
	if err != nil {
		t.Fatalf("write_json failed: %v", err)
	}

	if result.Metadata["bytes"] == nil {
		t.Error("Expected bytes in metadata")
	}

	// Verify file was written with valid JSON
	fullPath := filepath.Join(tmpDir, "test.json")
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("Written content is not valid JSON: %v", err)
	}

	if parsed["name"] != "test" || parsed["value"] != float64(42) {
		t.Errorf("JSON content mismatch: %v", parsed)
	}
}

func TestFileAction_WriteYAML(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	testPath := "./test.yaml"
	testData := map[string]interface{}{
		"name":  "test",
		"value": 42,
	}

	inputs := map[string]interface{}{
		"path":    testPath,
		"content": testData,
	}

	result, err := action.Execute(context.Background(), "write_yaml", inputs)
	if err != nil {
		t.Fatalf("write_yaml failed: %v", err)
	}

	if result.Metadata["bytes"] == nil {
		t.Error("Expected bytes in metadata")
	}

	// Verify file was written with valid YAML
	fullPath := filepath.Join(tmpDir, "test.yaml")
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}

	var parsed map[string]interface{}
	if err := yaml.Unmarshal(content, &parsed); err != nil {
		t.Fatalf("Written content is not valid YAML: %v", err)
	}

	if parsed["name"] != "test" || parsed["value"] != 42 {
		t.Errorf("YAML content mismatch: %v", parsed)
	}
}

func TestFileAction_WriteAutoFormat(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	testData := map[string]interface{}{
		"test": "value",
	}

	tests := []struct {
		name         string
		path         string
		shouldBeJSON bool
	}{
		{"JSON extension", "./test.json", true},
		{"YAML extension", "./test.yaml", false},
		{"Text extension", "./test.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputs := map[string]interface{}{
				"path": tt.path,
			}

			if tt.shouldBeJSON {
				inputs["content"] = testData
			} else if strings.HasSuffix(tt.path, ".txt") {
				inputs["content"] = "text content"
			} else {
				inputs["content"] = testData
			}

			_, err := action.Execute(context.Background(), "write", inputs)
			if err != nil {
				t.Fatalf("write failed: %v", err)
			}

			// Verify file exists
			fullPath := filepath.Join(tmpDir, filepath.Base(tt.path))
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				t.Errorf("File was not created: %s", fullPath)
			}
		})
	}
}

func TestFileAction_Append(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	testPath := "./test.txt"

	// Write initial content
	inputs := map[string]interface{}{
		"path":    testPath,
		"content": "Line 1\n",
	}
	_, err = action.Execute(context.Background(), "write_text", inputs)
	if err != nil {
		t.Fatalf("write_text failed: %v", err)
	}

	// Append content
	inputs = map[string]interface{}{
		"path":    testPath,
		"content": "Line 2\n",
	}
	_, err = action.Execute(context.Background(), "append", inputs)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	// Verify both lines are present
	fullPath := filepath.Join(tmpDir, "test.txt")
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "Line 1\nLine 2\n"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestFileAction_Render(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Create template file
	templatePath := filepath.Join(tmpDir, "template.txt")
	templateContent := "Hello, {{.Name}}! Value: {{.Value}}"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// Render template
	inputs := map[string]interface{}{
		"template": "./template.txt",
		"output":   "./output.txt",
		"data": map[string]interface{}{
			"Name":  "World",
			"Value": 42,
		},
	}

	result, err := action.Execute(context.Background(), "render", inputs)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	if result.Metadata["bytes"] == nil {
		t.Error("Expected bytes in metadata")
	}

	// Verify rendered output
	outputPath := filepath.Join(tmpDir, "output.txt")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	expected := "Hello, World! Value: 42"
	if string(content) != expected {
		t.Errorf("Expected %q, got %q", expected, string(content))
	}
}

func TestFileAction_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	testPath := "./test.txt"
	testContent := "atomic write test"

	inputs := map[string]interface{}{
		"path":    testPath,
		"content": testContent,
	}

	_, err = action.Execute(context.Background(), "write_text", inputs)
	if err != nil {
		t.Fatalf("write_text failed: %v", err)
	}

	// Verify no temp files left behind
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".atomic") || strings.Contains(entry.Name(), ".tmp") {
			t.Errorf("Temporary file not cleaned up: %s", entry.Name())
		}
	}
}

func TestFileAction_WriteMissingContent(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	inputs := map[string]interface{}{
		"path": "./test.txt",
		// Missing content parameter
	}

	_, err = action.Execute(context.Background(), "write_text", inputs)
	if err == nil {
		t.Fatal("Expected error for missing content")
	}

	opErr, ok := err.(*OperationError)
	if !ok {
		t.Fatalf("Expected OperationError, got %T", err)
	}

	if opErr.ErrorType != ErrorTypeValidation {
		t.Errorf("Expected ErrorTypeValidation, got %v", opErr.ErrorType)
	}
}

func TestFileAction_List(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Create test files and directories
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.json"), []byte("{}"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file3.txt"), []byte("nested"), 0644)

	t.Run("List all files in directory", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path": ".",
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should have 3 entries: file1.txt, file2.json, subdir
		if len(files) != 3 {
			t.Errorf("Expected 3 files, got %d", len(files))
		}
	})

	t.Run("List with glob pattern", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path":    ".",
			"pattern": "*.txt",
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should only have file1.txt
		if len(files) != 1 {
			t.Errorf("Expected 1 file, got %d", len(files))
		}
	})

	t.Run("List recursively", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path":      ".",
			"recursive": true,
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should have 4 entries: file1.txt, file2.json, subdir, subdir/file3.txt
		if len(files) < 4 {
			t.Errorf("Expected at least 4 files with recursive, got %d", len(files))
		}
	})

	t.Run("Filter by type - files only", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path": ".",
			"type": "files",
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should have 2 files: file1.txt, file2.json (no subdir)
		if len(files) != 2 {
			t.Errorf("Expected 2 files, got %d", len(files))
		}
	})

	t.Run("Filter by type - dirs only", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path": ".",
			"type": "dirs",
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should have 1 directory: subdir
		if len(files) != 1 {
			t.Errorf("Expected 1 directory, got %d", len(files))
		}
	})
}

func TestFileAction_Exists(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	t.Run("File exists", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "exists", map[string]interface{}{
			"path": "./test.txt",
		})
		if err != nil {
			t.Fatalf("exists failed: %v", err)
		}

		exists, ok := result.Response.(bool)
		if !ok {
			t.Fatalf("Expected bool, got %T", result.Response)
		}

		if !exists {
			t.Error("Expected file to exist")
		}
	})

	t.Run("File does not exist", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "exists", map[string]interface{}{
			"path": "./nonexistent.txt",
		})
		if err != nil {
			t.Fatalf("exists failed: %v", err)
		}

		exists, ok := result.Response.(bool)
		if !ok {
			t.Fatalf("Expected bool, got %T", result.Response)
		}

		if exists {
			t.Error("Expected file to not exist")
		}
	})
}

func TestFileAction_Stat(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "test content"
	os.WriteFile(testFile, []byte(testContent), 0644)

	result, err := action.Execute(context.Background(), "stat", map[string]interface{}{
		"path": "./test.txt",
	})
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}

	info, ok := result.Response.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", result.Response)
	}

	// Verify metadata fields
	if info["name"] != "test.txt" {
		t.Errorf("Expected name 'test.txt', got %v", info["name"])
	}

	if info["size"] != int64(len(testContent)) {
		t.Errorf("Expected size %d, got %v", len(testContent), info["size"])
	}

	if info["isDir"] != false {
		t.Errorf("Expected isDir false, got %v", info["isDir"])
	}
}

func TestFileAction_Mkdir(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	t.Run("Create directory with parents", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "mkdir", map[string]interface{}{
			"path":    "./parent/child",
			"parents": true,
		})
		if err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}

		created, ok := result.Metadata["created"].(bool)
		if !ok || !created {
			t.Error("Expected directory to be created")
		}

		// Verify directory exists
		dirPath := filepath.Join(tmpDir, "parent", "child")
		info, err := os.Stat(dirPath)
		if err != nil {
			t.Errorf("Directory was not created: %v", err)
		}
		if info != nil && !info.IsDir() {
			t.Error("Expected path to be a directory")
		}
	})

	t.Run("Directory already exists", func(t *testing.T) {
		_, err := action.Execute(context.Background(), "mkdir", map[string]interface{}{
			"path": "./existing",
		})
		if err != nil {
			t.Fatalf("First mkdir failed: %v", err)
		}

		// Try creating again
		result, err := action.Execute(context.Background(), "mkdir", map[string]interface{}{
			"path": "./existing",
		})
		if err != nil {
			t.Fatalf("Second mkdir failed: %v", err)
		}

		created, ok := result.Metadata["created"].(bool)
		if !ok {
			t.Error("Expected created field in metadata")
		}
		if created {
			t.Error("Expected created to be false (directory already exists)")
		}
	})
}

func TestFileAction_Copy(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	t.Run("Copy file", func(t *testing.T) {
		// Create source file
		sourceFile := filepath.Join(tmpDir, "source.txt")
		testContent := "copy test"
		os.WriteFile(sourceFile, []byte(testContent), 0644)

		result, err := action.Execute(context.Background(), "copy", map[string]interface{}{
			"source": "./source.txt",
			"dest":   "./dest.txt",
		})
		if err != nil {
			t.Fatalf("copy failed: %v", err)
		}

		bytes, ok := result.Metadata["bytes"].(int64)
		if !ok || bytes != int64(len(testContent)) {
			t.Errorf("Expected %d bytes copied, got %v", len(testContent), bytes)
		}

		// Verify destination exists with same content
		destFile := filepath.Join(tmpDir, "dest.txt")
		content, err := os.ReadFile(destFile)
		if err != nil {
			t.Fatalf("Failed to read destination: %v", err)
		}
		if string(content) != testContent {
			t.Errorf("Expected %q, got %q", testContent, string(content))
		}
	})

	t.Run("Copy directory recursively", func(t *testing.T) {
		// Create source directory with files
		sourceDir := filepath.Join(tmpDir, "sourcedir")
		os.Mkdir(sourceDir, 0755)
		os.WriteFile(filepath.Join(sourceDir, "file1.txt"), []byte("file1"), 0644)
		os.Mkdir(filepath.Join(sourceDir, "subdir"), 0755)
		os.WriteFile(filepath.Join(sourceDir, "subdir", "file2.txt"), []byte("file2"), 0644)

		_, err := action.Execute(context.Background(), "copy", map[string]interface{}{
			"source":    "./sourcedir",
			"dest":      "./destdir",
			"recursive": true,
		})
		if err != nil {
			t.Fatalf("copy failed: %v", err)
		}

		// Verify destination directory structure
		destDir := filepath.Join(tmpDir, "destdir")
		if _, err := os.Stat(filepath.Join(destDir, "file1.txt")); err != nil {
			t.Error("file1.txt was not copied")
		}
		if _, err := os.Stat(filepath.Join(destDir, "subdir", "file2.txt")); err != nil {
			t.Error("subdir/file2.txt was not copied")
		}
	})

	t.Run("Copy directory without recursive flag", func(t *testing.T) {
		sourceDir := filepath.Join(tmpDir, "sourcedir2")
		os.Mkdir(sourceDir, 0755)

		_, err := action.Execute(context.Background(), "copy", map[string]interface{}{
			"source": "./sourcedir2",
			"dest":   "./destdir2",
		})
		if err == nil {
			t.Fatal("Expected error when copying directory without recursive flag")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("Expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeValidation {
			t.Errorf("Expected ErrorTypeValidation, got %v", opErr.ErrorType)
		}
	})
}

func TestFileAction_Move(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	t.Run("Move file", func(t *testing.T) {
		// Create source file
		sourceFile := filepath.Join(tmpDir, "move_source.txt")
		testContent := "move test"
		os.WriteFile(sourceFile, []byte(testContent), 0644)

		_, err := action.Execute(context.Background(), "move", map[string]interface{}{
			"source": "./move_source.txt",
			"dest":   "./move_dest.txt",
		})
		if err != nil {
			t.Fatalf("move failed: %v", err)
		}

		// Verify source no longer exists
		if _, err := os.Stat(sourceFile); !os.IsNotExist(err) {
			t.Error("Source file should not exist after move")
		}

		// Verify destination exists with same content
		destFile := filepath.Join(tmpDir, "move_dest.txt")
		content, err := os.ReadFile(destFile)
		if err != nil {
			t.Fatalf("Failed to read destination: %v", err)
		}
		if string(content) != testContent {
			t.Errorf("Expected %q, got %q", testContent, string(content))
		}
	})
}

func TestFileAction_Delete(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	t.Run("Delete file", func(t *testing.T) {
		// Create test file
		testFile := filepath.Join(tmpDir, "delete_test.txt")
		os.WriteFile(testFile, []byte("delete me"), 0644)

		result, err := action.Execute(context.Background(), "delete", map[string]interface{}{
			"path": "./delete_test.txt",
		})
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		deleted, ok := result.Metadata["deleted"].(bool)
		if !ok || !deleted {
			t.Error("Expected file to be deleted")
		}

		// Verify file no longer exists
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("File should not exist after delete")
		}
	})

	t.Run("Delete directory recursively", func(t *testing.T) {
		// Create directory with files
		testDir := filepath.Join(tmpDir, "delete_dir")
		os.Mkdir(testDir, 0755)
		os.WriteFile(filepath.Join(testDir, "file.txt"), []byte("test"), 0644)

		result, err := action.Execute(context.Background(), "delete", map[string]interface{}{
			"path":      "./delete_dir",
			"recursive": true,
		})
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		deleted, ok := result.Metadata["deleted"].(bool)
		if !ok || !deleted {
			t.Error("Expected directory to be deleted")
		}

		// Verify directory no longer exists
		if _, err := os.Stat(testDir); !os.IsNotExist(err) {
			t.Error("Directory should not exist after delete")
		}
	})

	t.Run("Delete directory without recursive flag", func(t *testing.T) {
		// Create directory
		testDir := filepath.Join(tmpDir, "delete_dir2")
		os.Mkdir(testDir, 0755)

		_, err := action.Execute(context.Background(), "delete", map[string]interface{}{
			"path": "./delete_dir2",
		})
		if err == nil {
			t.Fatal("Expected error when deleting directory without recursive flag")
		}

		opErr, ok := err.(*OperationError)
		if !ok {
			t.Fatalf("Expected OperationError, got %T", err)
		}
		if opErr.ErrorType != ErrorTypeValidation {
			t.Errorf("Expected ErrorTypeValidation, got %v", opErr.ErrorType)
		}
	})

	t.Run("Delete non-existent file", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "delete", map[string]interface{}{
			"path": "./nonexistent.txt",
		})
		if err != nil {
			t.Fatalf("delete failed: %v", err)
		}

		deleted, ok := result.Metadata["deleted"].(bool)
		if !ok || deleted {
			t.Error("Expected deleted to be false for non-existent file")
		}
	})
}

func TestFileAction_List_GlobPatterns(t *testing.T) {
	tmpDir := t.TempDir()

	config := &Config{
		WorkflowDir: tmpDir,
		OutputDir:   tmpDir,
		TempDir:     tmpDir,
	}

	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Create test directory structure
	// tmpDir/
	//   ├── file1.go
	//   ├── file2.go
	//   ├── file1.txt
	//   ├── file2.txt
	//   ├── test.md
	//   ├── subdir1/
	//   │   ├── nested1.go
	//   │   └── nested2.go
	//   └── subdir2/
	//       └── deep/
	//           └── deep.go

	// Create files in root
	os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("text1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("text2"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("# Test"), 0644)

	// Create nested directories and files
	os.Mkdir(filepath.Join(tmpDir, "subdir1"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir1", "nested1.go"), []byte("package nested"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir1", "nested2.go"), []byte("package nested"), 0644)

	os.MkdirAll(filepath.Join(tmpDir, "subdir2", "deep"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir2", "deep", "deep.go"), []byte("package deep"), 0644)

	t.Run("Recursive doublestar pattern **/*.go", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path":      ".",
			"pattern":   "**/*.go",
			"recursive": true,
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should find all .go files: file1.go, file2.go, subdir1/nested1.go, subdir1/nested2.go, subdir2/deep/deep.go
		// Verify we found at least 5 .go files (may have duplicates depending on glob implementation)
		if len(files) < 5 {
			t.Errorf("Expected at least 5 .go files with **/*.go pattern, got %d", len(files))
			for _, f := range files {
				t.Logf("Found: %v", f["path"])
			}
		}

		// Verify all are .go files
		for _, file := range files {
			name := file["name"].(string)
			if !strings.HasSuffix(name, ".go") {
				t.Errorf("Expected only .go files, found %s", name)
			}
		}

		// Verify we found the expected files (deduplicate by path)
		foundPaths := make(map[string]bool)
		for _, file := range files {
			foundPaths[file["name"].(string)] = true
		}

		expectedFiles := []string{"file1.go", "file2.go", "nested1.go", "nested2.go", "deep.go"}
		for _, expected := range expectedFiles {
			if !foundPaths[expected] {
				t.Errorf("Expected to find %s in results", expected)
			}
		}
	})

	t.Run("Single char wildcard file?.txt", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path":    ".",
			"pattern": "file?.txt",
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should match file1.txt and file2.txt
		expectedCount := 2
		if len(files) != expectedCount {
			t.Errorf("Expected %d files with file?.txt pattern, got %d", expectedCount, len(files))
		}

		// Verify matched files
		foundNames := make(map[string]bool)
		for _, file := range files {
			foundNames[file["name"].(string)] = true
		}

		expectedNames := []string{"file1.txt", "file2.txt"}
		for _, expected := range expectedNames {
			if !foundNames[expected] {
				t.Errorf("Expected to find %s, but didn't", expected)
			}
		}
	})

	t.Run("Character class file[12].txt", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path":    ".",
			"pattern": "file[12].txt",
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should match file1.txt and file2.txt
		expectedCount := 2
		if len(files) != expectedCount {
			t.Errorf("Expected %d files with file[12].txt pattern, got %d", expectedCount, len(files))
		}

		// Verify matched files
		foundNames := make(map[string]bool)
		for _, file := range files {
			foundNames[file["name"].(string)] = true
		}

		if !foundNames["file1.txt"] || !foundNames["file2.txt"] {
			t.Error("Expected to match file1.txt and file2.txt")
		}
	})

	t.Run("Extension wildcard *.go in root only", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path":    ".",
			"pattern": "*.go",
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should match only file1.go and file2.go (not nested files)
		expectedCount := 2
		if len(files) != expectedCount {
			t.Errorf("Expected %d .go files in root, got %d", expectedCount, len(files))
		}
	})

	t.Run("Nested directory pattern subdir1/*.go", func(t *testing.T) {
		result, err := action.Execute(context.Background(), "list", map[string]interface{}{
			"path":    ".",
			"pattern": "subdir1/*.go",
		})
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}

		files, ok := result.Response.([]map[string]interface{})
		if !ok {
			t.Fatalf("Expected []map[string]interface{}, got %T", result.Response)
		}

		// Should match nested1.go and nested2.go in subdir1
		expectedCount := 2
		if len(files) != expectedCount {
			t.Errorf("Expected %d files in subdir1, got %d", expectedCount, len(files))
		}
	})
}
