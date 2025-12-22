package shared

import (
	"strings"
	"testing"
)

func TestDryRunOutput_Create(t *testing.T) {
	output := NewDryRunOutput()
	output.DryRunCreate("<config-dir>/config.yaml")

	result := output.String()

	if !strings.Contains(result, "CREATE: <config-dir>/config.yaml") {
		t.Errorf("Expected CREATE action in output, got: %s", result)
	}

	if !strings.Contains(result, "Dry run: The following actions would be performed:") {
		t.Errorf("Expected dry-run header in output, got: %s", result)
	}

	if !strings.Contains(result, "Run without --dry-run to execute.") {
		t.Errorf("Expected footer in output, got: %s", result)
	}
}

func TestDryRunOutput_CreateWithDescription(t *testing.T) {
	output := NewDryRunOutput()
	output.DryRunCreateWithDescription("<config-dir>/config.yaml", "with default providers")

	result := output.String()

	if !strings.Contains(result, "CREATE: <config-dir>/config.yaml (with default providers)") {
		t.Errorf("Expected CREATE action with description in output, got: %s", result)
	}
}

func TestDryRunOutput_Modify(t *testing.T) {
	output := NewDryRunOutput()
	output.DryRunModify("<config-dir>/config.yaml", "add provider 'openai'")

	result := output.String()

	if !strings.Contains(result, "MODIFY: <config-dir>/config.yaml (add provider 'openai')") {
		t.Errorf("Expected MODIFY action in output, got: %s", result)
	}
}

func TestDryRunOutput_Delete(t *testing.T) {
	output := NewDryRunOutput()
	output.DryRunDelete("<cache-dir>/session-123")

	result := output.String()

	if !strings.Contains(result, "DELETE: <cache-dir>/session-123") {
		t.Errorf("Expected DELETE action in output, got: %s", result)
	}
}

func TestDryRunOutput_DeleteWithCount(t *testing.T) {
	output := NewDryRunOutput()
	output.DryRunDeleteWithCount("<cache-dir>", "5 entries")

	result := output.String()

	if !strings.Contains(result, "DELETE: <cache-dir> (5 entries)") {
		t.Errorf("Expected DELETE action with count in output, got: %s", result)
	}
}

func TestDryRunOutput_MultipleActions(t *testing.T) {
	output := NewDryRunOutput()
	output.DryRunCreate("<config-dir>/config.yaml")
	output.DryRunCreate("<config-dir>/workflows/")
	output.DryRunModify("<config-dir>/config.yaml", "add provider")

	result := output.String()

	expectedActions := []string{
		"CREATE: <config-dir>/config.yaml",
		"CREATE: <config-dir>/workflows/",
		"MODIFY: <config-dir>/config.yaml (add provider)",
	}

	for _, expected := range expectedActions {
		if !strings.Contains(result, expected) {
			t.Errorf("Expected action '%s' in output, got: %s", expected, result)
		}
	}
}

func TestDryRunOutput_Empty(t *testing.T) {
	output := NewDryRunOutput()

	result := output.String()

	expected := "Dry run: No actions would be performed."
	if result != expected {
		t.Errorf("Expected '%s', got: %s", expected, result)
	}
}

func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		expected string
	}{
		{
			name:     "API key should be masked",
			key:      "api_key",
			value:    "sk-1234567890",
			expected: "[REDACTED]",
		},
		{
			name:     "Token should be masked",
			key:      "auth_token",
			value:    "abc123",
			expected: "[REDACTED]",
		},
		{
			name:     "Password should be masked",
			key:      "password",
			value:    "secret123",
			expected: "[REDACTED]",
		},
		{
			name:     "Non-sensitive value should not be masked",
			key:      "name",
			value:    "my-provider",
			expected: "my-provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveData(tt.key, tt.value)
			if result != tt.expected {
				t.Errorf("MaskSensitiveData(%s, %s) = %s, want %s", tt.key, tt.value, result, tt.expected)
			}
		})
	}
}

func TestPlaceholderPath(t *testing.T) {
	tests := []struct {
		name        string
		fullPath    string
		baseDir     string
		placeholder string
		expected    string
	}{
		{
			name:        "Replace config directory",
			fullPath:    "/Users/john/.config/conductor/config.yaml",
			baseDir:     "/Users/john/.config/conductor",
			placeholder: "<config-dir>",
			expected:    "<config-dir>/config.yaml",
		},
		{
			name:        "Replace cache directory",
			fullPath:    "/tmp/conductor/cache/session-123",
			baseDir:     "/tmp/conductor/cache",
			placeholder: "<cache-dir>",
			expected:    "<cache-dir>/session-123",
		},
		{
			name:        "No replacement needed",
			fullPath:    "<config-dir>/config.yaml",
			baseDir:     "/Users/john/.config/conductor",
			placeholder: "<config-dir>",
			expected:    "<config-dir>/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PlaceholderPath(tt.fullPath, tt.baseDir, tt.placeholder)
			if result != tt.expected {
				t.Errorf("PlaceholderPath(%s, %s, %s) = %s, want %s",
					tt.fullPath, tt.baseDir, tt.placeholder, result, tt.expected)
			}
		})
	}
}
