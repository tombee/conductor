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

package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMakeIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple name",
			input:    "myserver",
			expected: "myserver",
		},
		{
			name:     "hyphenated name",
			input:    "my-server",
			expected: "my_server",
		},
		{
			name:     "mixed case",
			input:    "MyServer",
			expected: "MyServer",
		},
		{
			name:     "with numbers",
			input:    "server123",
			expected: "server123",
		},
		{
			name:     "special characters",
			input:    "my@server!",
			expected: "my_server_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := makeIdentifier(tt.input)
			if result != tt.expected {
				t.Errorf("makeIdentifier(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestRenderTemplate(t *testing.T) {
	template := `Name: {{.Name}}
Identifier: {{.Identifier}}
Title: {{.NameTitle}}`

	rendered := renderTemplate(template, "my-server")

	expected := `Name: my-server
Identifier: my_server
Title: My_server`

	if rendered != expected {
		t.Errorf("renderTemplate() failed\nGot:\n%s\n\nWant:\n%s", rendered, expected)
	}
}

func TestGenerateMCPServer_Python(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-server")

	// Generate Python blank server
	if err := generateMCPServer(projectDir, "test-server", "python", "blank"); err != nil {
		t.Fatalf("generateMCPServer() failed: %v", err)
	}

	// Check that expected files exist
	expectedFiles := []string{
		"server.py",
		"requirements.txt",
		"README.md",
	}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(projectDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", filename)
		}
	}

	// Verify server.py is executable
	info, err := os.Stat(filepath.Join(projectDir, "server.py"))
	if err != nil {
		t.Fatalf("Failed to stat server.py: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("server.py is not executable")
	}

	// Verify template substitution in README
	readmeContent, err := os.ReadFile(filepath.Join(projectDir, "README.md"))
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}
	readmeStr := string(readmeContent)
	if !contains(readmeStr, "test-server") {
		t.Error("README.md does not contain server name")
	}
	if contains(readmeStr, "{{.Name}}") {
		t.Error("README.md contains unreplaced template variable")
	}
}

func TestGenerateMCPServer_TypeScript(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "ts-server")

	// Generate TypeScript blank server
	if err := generateMCPServer(projectDir, "ts-server", "typescript", "blank"); err != nil {
		t.Fatalf("generateMCPServer() failed: %v", err)
	}

	// Check that expected files exist
	expectedFiles := []string{
		"server.ts",
		"package.json",
		"tsconfig.json",
		"README.md",
	}

	for _, filename := range expectedFiles {
		filePath := filepath.Join(projectDir, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Errorf("Expected file %s does not exist", filename)
		}
	}

	// Verify template substitution in package.json
	pkgContent, err := os.ReadFile(filepath.Join(projectDir, "package.json"))
	if err != nil {
		t.Fatalf("Failed to read package.json: %v", err)
	}
	pkgStr := string(pkgContent)
	if !contains(pkgStr, "ts_server") {
		t.Error("package.json does not contain identifier")
	}
	if contains(pkgStr, "{{.Identifier}}") {
		t.Error("package.json contains unreplaced template variable")
	}
}

func TestGenerateMCPServer_HTTPTemplate(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "http-server")

	// Generate Python HTTP server
	if err := generateMCPServer(projectDir, "http-server", "python", "http"); err != nil {
		t.Fatalf("generateMCPServer() failed: %v", err)
	}

	// Verify server.py contains HTTP-specific code
	serverContent, err := os.ReadFile(filepath.Join(projectDir, "server.py"))
	if err != nil {
		t.Fatalf("Failed to read server.py: %v", err)
	}
	serverStr := string(serverContent)
	if !contains(serverStr, "http_get") {
		t.Error("HTTP template server.py does not contain http_get tool")
	}
	if !contains(serverStr, "urllib") {
		t.Error("HTTP template server.py does not contain urllib import")
	}
}

func TestGenerateMCPServer_DatabaseTemplate(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "db-server")

	// Generate Python database server
	if err := generateMCPServer(projectDir, "db-server", "python", "database"); err != nil {
		t.Fatalf("generateMCPServer() failed: %v", err)
	}

	// Verify server.py contains database-specific code
	serverContent, err := os.ReadFile(filepath.Join(projectDir, "server.py"))
	if err != nil {
		t.Fatalf("Failed to read server.py: %v", err)
	}
	serverStr := string(serverContent)
	if !contains(serverStr, "query") {
		t.Error("Database template server.py does not contain query tool")
	}
	if !contains(serverStr, "sql") {
		t.Error("Database template server.py does not reference SQL")
	}
}

func TestGetPythonTemplateFiles(t *testing.T) {
	tests := []struct {
		name           string
		template       string
		expectedFiles  []string
		expectedInCode string
	}{
		{
			name:           "blank template",
			template:       "blank",
			expectedFiles:  []string{"server.py", "requirements.txt", "README.md"},
			expectedInCode: "example_tool",
		},
		{
			name:           "http template",
			template:       "http",
			expectedFiles:  []string{"server.py", "requirements.txt", "README.md"},
			expectedInCode: "http_get",
		},
		{
			name:           "database template",
			template:       "database",
			expectedFiles:  []string{"server.py", "requirements.txt", "README.md"},
			expectedInCode: "query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := getPythonTemplateFiles(tt.template)

			// Check all expected files are present
			for _, expectedFile := range tt.expectedFiles {
				if _, exists := files[expectedFile]; !exists {
					t.Errorf("Expected file %s not found in template files", expectedFile)
				}
			}

			// Check server.py contains expected code
			if serverPy, exists := files["server.py"]; exists {
				if !contains(serverPy, tt.expectedInCode) {
					t.Errorf("server.py does not contain expected code: %s", tt.expectedInCode)
				}
			} else {
				t.Error("server.py not found in template files")
			}
		})
	}
}

func TestGetTypeScriptTemplateFiles(t *testing.T) {
	files := getTypeScriptTemplateFiles("blank")

	expectedFiles := []string{"server.ts", "package.json", "tsconfig.json", "README.md"}
	for _, expectedFile := range expectedFiles {
		if _, exists := files[expectedFile]; !exists {
			t.Errorf("Expected file %s not found in TypeScript template files", expectedFile)
		}
	}

	// Check server.ts contains expected code
	if serverTS, exists := files["server.ts"]; exists {
		if !contains(serverTS, "example_tool") {
			t.Error("server.ts does not contain example_tool")
		}
		if !contains(serverTS, "@modelcontextprotocol/sdk") {
			t.Error("server.ts does not import MCP SDK")
		}
	} else {
		t.Error("server.ts not found in TypeScript template files")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
