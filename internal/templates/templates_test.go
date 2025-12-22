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
	"strings"
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestList(t *testing.T) {
	templates, err := List()
	if err != nil {
		t.Fatalf("List() failed: %v", err)
	}

	if len(templates) != 5 {
		t.Errorf("Expected 5 templates, got %d", len(templates))
	}

	expectedTemplates := map[string]bool{
		"blank":       false,
		"summarize":   false,
		"code-review": false,
		"explain":     false,
		"translate":   false,
	}

	for _, tmpl := range templates {
		if _, exists := expectedTemplates[tmpl.Name]; exists {
			expectedTemplates[tmpl.Name] = true
		} else {
			t.Errorf("Unexpected template found: %s", tmpl.Name)
		}

		// Verify metadata fields are populated
		if tmpl.Description == "" {
			t.Errorf("Template %s has empty description", tmpl.Name)
		}
		if tmpl.Category == "" {
			t.Errorf("Template %s has empty category", tmpl.Name)
		}
		if tmpl.FilePath == "" {
			t.Errorf("Template %s has empty file path", tmpl.Name)
		}
	}

	// Verify all expected templates were found
	for name, found := range expectedTemplates {
		if !found {
			t.Errorf("Expected template %s not found", name)
		}
	}
}

func TestGet(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		expectError bool
	}{
		{"blank template", "blank", false},
		{"summarize template", "summarize", false},
		{"code-review template", "code-review", false},
		{"explain template", "explain", false},
		{"translate template", "translate", false},
		{"unknown template", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := Get(tt.template)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for template %q, got nil", tt.template)
				}
			} else {
				if err != nil {
					t.Errorf("Get(%q) failed: %v", tt.template, err)
				}
				if len(content) == 0 {
					t.Errorf("Get(%q) returned empty content", tt.template)
				}
			}
		})
	}
}

func TestExists(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected bool
	}{
		{"blank exists", "blank", true},
		{"summarize exists", "summarize", true},
		{"code-review exists", "code-review", true},
		{"explain exists", "explain", true},
		{"translate exists", "translate", true},
		{"unknown template", "nonexistent", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Exists(tt.template)
			if result != tt.expected {
				t.Errorf("Exists(%q) = %v, want %v", tt.template, result, tt.expected)
			}
		})
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		workflowName string
		expectError  bool
		checkContent func(t *testing.T, content []byte)
	}{
		{
			name:         "render blank template",
			templateName: "blank",
			workflowName: "my-workflow",
			expectError:  false,
			checkContent: func(t *testing.T, content []byte) {
				s := string(content)
				if !strings.Contains(s, "name: my-workflow") {
					t.Errorf("Rendered template does not contain workflow name")
				}
				if strings.Contains(s, "{{.Name}}") {
					t.Errorf("Rendered template still contains {{.Name}} placeholder")
				}
			},
		},
		{
			name:         "render with different name",
			templateName: "summarize",
			workflowName: "text-summarizer",
			expectError:  false,
			checkContent: func(t *testing.T, content []byte) {
				s := string(content)
				if !strings.Contains(s, "name: text-summarizer") {
					t.Errorf("Rendered template does not contain workflow name")
				}
			},
		},
		{
			name:         "render code-review template",
			templateName: "code-review",
			workflowName: "review-pr",
			expectError:  false,
			checkContent: func(t *testing.T, content []byte) {
				s := string(content)
				if !strings.Contains(s, "name: review-pr") {
					t.Errorf("Rendered template does not contain workflow name")
				}
			},
		},
		{
			name:         "nonexistent template",
			templateName: "nonexistent",
			workflowName: "test",
			expectError:  true,
			checkContent: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := Render(tt.templateName, tt.workflowName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for template %q, got nil", tt.templateName)
				}
			} else {
				if err != nil {
					t.Errorf("Render(%q, %q) failed: %v", tt.templateName, tt.workflowName, err)
				}
				if len(content) == 0 {
					t.Errorf("Render(%q, %q) returned empty content", tt.templateName, tt.workflowName)
				}
				if tt.checkContent != nil {
					tt.checkContent(t, content)
				}
			}
		})
	}
}

func TestRenderedTemplatesValidate(t *testing.T) {
	// All rendered templates should pass workflow validation
	templates := []string{"blank", "summarize", "code-review", "explain", "translate"}
	workflowName := "test-workflow"

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			content, err := Render(tmpl, workflowName)
			if err != nil {
				t.Fatalf("Render(%q) failed: %v", tmpl, err)
			}

			// Verify the rendered template is valid YAML and passes workflow validation
			def, err := workflow.ParseDefinition(content)
			if err != nil {
				t.Errorf("Rendered template %q failed validation: %v\nContent:\n%s", tmpl, err, string(content))
			}

			// Verify the workflow name was substituted correctly
			if def.Name != workflowName {
				t.Errorf("Expected workflow name %q, got %q", workflowName, def.Name)
			}
		})
	}
}

func TestGetDescription(t *testing.T) {
	tests := []struct {
		name     string
		template string
		contains string
	}{
		{"blank description", "blank", "Minimal"},
		{"summarize description", "summarize", "Summarize"},
		{"code-review description", "code-review", "Review"},
		{"explain description", "explain", "Explain"},
		{"translate description", "translate", "Translate"},
		{"unknown template", "unknown", "Workflow template"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := getDescription(tt.template)
			if !strings.Contains(desc, tt.contains) {
				t.Errorf("getDescription(%q) = %q, expected to contain %q", tt.template, desc, tt.contains)
			}
		})
	}
}

func TestGetCategory(t *testing.T) {
	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"blank category", "blank", "Basic"},
		{"summarize category", "summarize", "Text Processing"},
		{"code-review category", "code-review", "Development"},
		{"explain category", "explain", "Education"},
		{"translate category", "translate", "Text Processing"},
		{"unknown category", "unknown", "General"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cat := getCategory(tt.template)
			if cat != tt.expected {
				t.Errorf("getCategory(%q) = %q, want %q", tt.template, cat, tt.expected)
			}
		})
	}
}
