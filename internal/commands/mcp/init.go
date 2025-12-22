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
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var (
	mcpLang     string
	mcpTemplate string
)

// serverNameRegex validates MCP server names to prevent invalid identifiers
var serverNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

func newMCPInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a new MCP server from a template",
		Long: `Create a new MCP server project from a template.

The command scaffolds a complete MCP server with:
- Server implementation in your chosen language
- README with usage instructions
- Example tools to get started
- Dependencies configuration (requirements.txt or package.json)

Examples:
  conductor mcp init my-server                    # Create Python MCP server
  conductor mcp init my-server --lang typescript  # Create TypeScript server
  conductor mcp init api-wrapper --template http  # Use HTTP wrapper template`,
		Args: cobra.ExactArgs(1),
		RunE: runMCPInit,
	}

	cmd.Flags().StringVarP(&mcpLang, "lang", "l", "python", "Language for the server (python, typescript)")
	cmd.Flags().StringVarP(&mcpTemplate, "template", "t", "blank", "Template to use (blank, http, database)")

	return cmd
}

func runMCPInit(cmd *cobra.Command, args []string) error {
	name := args[0]

	// Validate server name
	if !serverNameRegex.MatchString(name) {
		return fmt.Errorf("invalid server name: must start with a letter and contain only letters, numbers, hyphens, and underscores")
	}

	// Validate language
	if mcpLang != "python" && mcpLang != "typescript" {
		return fmt.Errorf("unsupported language: %s (must be 'python' or 'typescript')", mcpLang)
	}

	// Validate template
	validTemplates := map[string]bool{
		"blank":    true,
		"http":     true,
		"database": true,
	}
	if !validTemplates[mcpTemplate] {
		return fmt.Errorf("unsupported template: %s (must be 'blank', 'http', or 'database')", mcpTemplate)
	}

	// TypeScript only supports blank template for now
	if mcpLang == "typescript" && mcpTemplate != "blank" {
		return fmt.Errorf("typescript only supports 'blank' template currently")
	}

	// Create project directory
	projectDir := name
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Generate the MCP server project
	if err := generateMCPServer(projectDir, name, mcpLang, mcpTemplate); err != nil {
		return fmt.Errorf("failed to generate MCP server: %w", err)
	}

	fmt.Printf("✓ Created MCP server: %s\n", name)
	fmt.Printf("  Language: %s\n", mcpLang)
	fmt.Printf("  Template: %s\n", mcpTemplate)
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  cd %s\n", name)

	if mcpLang == "python" {
		fmt.Printf("  pip install -r requirements.txt\n")
		fmt.Printf("  python server.py\n")
	} else {
		fmt.Printf("  npm install\n")
		fmt.Printf("  npm run dev\n")
	}

	fmt.Printf("\nSee README.md for more information.\n")

	return nil
}

// generateMCPServer creates the MCP server files from templates.
func generateMCPServer(projectDir, name, lang, template string) error {
	// Ensure project directory exists
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// Get template files based on language
	files := map[string]string{}

	if lang == "python" {
		files = getPythonTemplateFiles(template)
	} else if lang == "typescript" {
		files = getTypeScriptTemplateFiles(template)
	}

	// Render and write files
	for filename, content := range files {
		// Apply template substitutions
		rendered := renderTemplate(content, name)

		// Write file
		filePath := filepath.Join(projectDir, filename)
		if err := os.WriteFile(filePath, []byte(rendered), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Make server file executable (for Python)
	if lang == "python" {
		serverPath := filepath.Join(projectDir, "server.py")
		if err := os.Chmod(serverPath, 0755); err != nil {
			return fmt.Errorf("failed to make server.py executable: %w", err)
		}
	}

	return nil
}

// renderTemplate applies variable substitution to template content.
func renderTemplate(content, name string) string {
	// Generate valid Python/TypeScript identifier from name
	identifier := makeIdentifier(name)

	replacements := map[string]string{
		"{{.Name}}":       name,
		"{{.Identifier}}": identifier,
		"{{.NameTitle}}":  strings.Title(identifier),
	}

	result := content
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// makeIdentifier converts a name into a valid Python/TypeScript identifier.
func makeIdentifier(name string) string {
	// Replace hyphens with underscores
	identifier := strings.ReplaceAll(name, "-", "_")

	// Remove any characters that aren't letters, numbers, or underscores
	var cleaned strings.Builder
	for i, r := range identifier {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			cleaned.WriteRune(r)
		} else if i > 0 {
			// Replace invalid characters with underscore (except at start)
			cleaned.WriteRune('_')
		}
	}

	return cleaned.String()
}
