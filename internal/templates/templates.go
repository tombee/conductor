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

package templates

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

// Embed workflow templates into the binary for offline availability
//
//go:embed *.yaml
var embeddedFS embed.FS

// Template represents metadata about an embedded workflow template
type Template struct {
	Name        string
	Description string
	Category    string
	FilePath    string
}

// List returns all available embedded templates
func List() ([]Template, error) {
	entries, err := embeddedFS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded templates: %w", err)
	}

	var templates []Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		templates = append(templates, Template{
			Name:        name,
			Description: getDescription(name),
			Category:    getCategory(name),
			FilePath:    entry.Name(),
		})
	}

	return templates, nil
}

// Get returns the raw content of a specific template by name
func Get(name string) ([]byte, error) {
	// Validate template name to prevent path traversal (defense-in-depth)
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return nil, fmt.Errorf("invalid template name: %q", name)
	}
	filename := name + ".yaml"
	content, err := embeddedFS.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("template %q not found: %w", name, err)
	}
	return content, nil
}

// Exists checks if a template with the given name exists
func Exists(name string) bool {
	// Validate template name to prevent path traversal (defense-in-depth)
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return false
	}
	filename := name + ".yaml"
	_, err := embeddedFS.ReadFile(filename)
	return err == nil
}

// Render renders a template with the given workflow name substituted
// Templates use {{.Name}} placeholder for workflow name
func Render(templateName, workflowName string) ([]byte, error) {
	templateContent, err := Get(templateName)
	if err != nil {
		return nil, err
	}

	// Parse and execute template
	tmpl, err := template.New(templateName).Parse(string(templateContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse template %q: %w", templateName, err)
	}

	var buf bytes.Buffer
	data := map[string]string{
		"Name": workflowName,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to render template %q: %w", templateName, err)
	}

	return buf.Bytes(), nil
}

// getDescription returns a human-readable description for each template
func getDescription(name string) string {
	descriptions := map[string]string{
		"blank":       "Minimal workflow with single LLM step",
		"summarize":   "Summarize text input into key points",
		"code-review": "Review code changes for quality and security",
		"explain":     "Explain code or technical concepts clearly",
		"translate":   "Translate text between languages",
	}

	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return "Workflow template"
}

// getCategory returns the category for each template
func getCategory(name string) string {
	categories := map[string]string{
		"blank":       "Basic",
		"summarize":   "Text Processing",
		"code-review": "Development",
		"explain":     "Education",
		"translate":   "Text Processing",
	}

	if cat, ok := categories[name]; ok {
		return cat
	}
	return "General"
}
