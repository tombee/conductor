package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileAction_RenderTemplate_AllowedFunctions(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		template     string
		data         map[string]interface{}
		wantContains string
	}{
		{
			name:         "upper function",
			template:     `{{ upper .text }}`,
			data:         map[string]interface{}{"text": "hello"},
			wantContains: "HELLO",
		},
		{
			name:         "lower function",
			template:     `{{ lower .text }}`,
			data:         map[string]interface{}{"text": "WORLD"},
			wantContains: "world",
		},
		{
			name:         "trim function",
			template:     `{{ trim .text }}`,
			data:         map[string]interface{}{"text": "  spaces  "},
			wantContains: "spaces",
		},
		{
			name:         "replace function",
			template:     `{{ replace .text "old" "new" }}`,
			data:         map[string]interface{}{"text": "old value"},
			wantContains: "new value",
		},
		{
			name:         "split and join functions",
			template:     `{{ join (split .text ",") "-" }}`,
			data:         map[string]interface{}{"text": "a,b,c"},
			wantContains: "a-b-c",
		},
		{
			name:         "default function with nil",
			template:     `{{ default "fallback" .missing }}`,
			data:         map[string]interface{}{},
			wantContains: "fallback",
		},
		{
			name:         "default function with value",
			template:     `{{ default "fallback" .text }}`,
			data:         map[string]interface{}{"text": "actual"},
			wantContains: "actual",
		},
		{
			name:         "builtin eq function",
			template:     `{{ if eq .count 5 }}equal{{ else }}not equal{{ end }}`,
			data:         map[string]interface{}{"count": 5},
			wantContains: "equal",
		},
		{
			name:         "builtin lt function",
			template:     `{{ if lt .count 10 }}less{{ else }}more{{ end }}`,
			data:         map[string]interface{}{"count": 5},
			wantContains: "less",
		},
		{
			name:         "builtin len function",
			template:     `{{ len .items }}`,
			data:         map[string]interface{}{"items": []int{1, 2, 3}},
			wantContains: "3",
		},
		{
			name:         "builtin index function",
			template:     `{{ index .items 1 }}`,
			data:         map[string]interface{}{"items": []string{"a", "b", "c"}},
			wantContains: "b",
		},
	}

	config := &Config{
		WorkflowDir: tempDir,
		OutputDir:   tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create template file
			templateFile := filepath.Join(tempDir, "template.tmpl")
			if err := os.WriteFile(templateFile, []byte(tt.template), 0644); err != nil {
				t.Fatalf("Failed to create template file: %v", err)
			}

			// Create output file path
			outputFile := filepath.Join(tempDir, "output.txt")

			// Execute render operation
			_, err := action.Execute(context.Background(), "render", map[string]interface{}{
				"template": "./template.tmpl",
				"output":   "./output.txt",
				"data":     tt.data,
			})

			if err != nil {
				t.Fatalf("render failed: %v", err)
			}

			// Read output
			output, err := os.ReadFile(outputFile)
			if err != nil {
				t.Fatalf("Failed to read output: %v", err)
			}

			outputStr := string(output)
			if !strings.Contains(outputStr, tt.wantContains) {
				t.Errorf("Expected output to contain %q, got %q", tt.wantContains, outputStr)
			}

			// Cleanup for next test
			os.Remove(templateFile)
			os.Remove(outputFile)
		})
	}
}

func TestFileAction_RenderTemplate_BlockedFunctions(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		template string
	}{
		{
			name:     "exec function",
			template: `{{ exec "ls" }}`,
		},
		{
			name:     "env function",
			template: `{{ env "HOME" }}`,
		},
		{
			name:     "call function",
			template: `{{ call .func }}`,
		},
	}

	config := &Config{
		WorkflowDir: tempDir,
		OutputDir:   tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create template file
			templateFile := filepath.Join(tempDir, "blocked.tmpl")
			if err := os.WriteFile(templateFile, []byte(tt.template), 0644); err != nil {
				t.Fatalf("Failed to create template file: %v", err)
			}

			// Create output file path
			outputFile := filepath.Join(tempDir, "output.txt")

			// Execute render operation - should fail
			_, err := action.Execute(context.Background(), "render", map[string]interface{}{
				"template": "./blocked.tmpl",
				"output":   "./output.txt",
				"data":     map[string]interface{}{},
			})

			if err == nil {
				t.Fatalf("Expected render to fail with blocked function, but it succeeded")
			}

			// Verify error indicates template issue
			opErr, ok := err.(*OperationError)
			if !ok {
				t.Fatalf("Expected OperationError, got %T", err)
			}

			if opErr.ErrorType != ErrorTypeValidation {
				t.Errorf("Expected ErrorTypeValidation, got %v", opErr.ErrorType)
			}

			// Cleanup
			os.Remove(templateFile)
			os.Remove(outputFile)
		})
	}
}

func TestFileAction_RenderTemplate_WithWorkflowData(t *testing.T) {
	tempDir := t.TempDir()

	// Create a template that uses workflow-style data
	template := `# Report
Title: {{ .title }}
Status: {{ upper .status }}
Items:
{{ range .items }}  - {{ . }}
{{ end }}`

	templateFile := filepath.Join(tempDir, "report.tmpl")
	if err := os.WriteFile(templateFile, []byte(template), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	config := &Config{
		WorkflowDir: tempDir,
		OutputDir:   tempDir,
	}
	action, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create action: %v", err)
	}

	// Simulate workflow data
	data := map[string]interface{}{
		"title":  "Weekly Analysis",
		"status": "complete",
		"items":  []string{"Task 1", "Task 2", "Task 3"},
	}

	outputFile := filepath.Join(tempDir, "report.md")

	// Execute render
	result, err := action.Execute(context.Background(), "render", map[string]interface{}{
		"template": "./report.tmpl",
		"output":   "./report.md",
		"data":     data,
	})

	if err != nil {
		t.Fatalf("render failed: %v", err)
	}

	// Verify metadata
	if result.Metadata["template"] == "" {
		t.Error("Expected metadata to include template path")
	}
	if result.Metadata["output"] == "" {
		t.Error("Expected metadata to include output path")
	}
	if result.Metadata["bytes"] == 0 {
		t.Error("Expected metadata to include bytes written")
	}

	// Read and verify output
	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output: %v", err)
	}

	outputStr := string(output)
	expectedContents := []string{
		"Weekly Analysis",
		"COMPLETE",
		"Task 1",
		"Task 2",
		"Task 3",
	}

	for _, expected := range expectedContents {
		if !strings.Contains(outputStr, expected) {
			t.Errorf("Expected output to contain %q, got:\n%s", expected, outputStr)
		}
	}
}
