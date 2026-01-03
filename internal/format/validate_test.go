package format

import (
	"testing"
)

func TestValidateString(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		{"empty string", "", false},
		{"non-empty string", "hello", false},
		{"number", 123, false},
		{"boolean", true, false},
		{"null", nil, false},
		{"object", map[string]interface{}{"key": "value"}, false},
		{"array", []string{"a", "b"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateString(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateString() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNumber(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		// Valid cases
		{"integer string", "123", false},
		{"float string", "123.45", false},
		{"negative integer", "-456", false},
		{"negative float", "-123.45", false},
		{"scientific notation", "1.5e10", false},
		{"negative scientific", "-2.5e-3", false},
		{"zero", "0", false},
		{"float zero", "0.0", false},
		{"native int", 123, false},
		{"native float", 123.45, false},
		{"native int64", int64(999), false},

		// Invalid cases
		{"empty string", "", true},
		{"whitespace", "   ", true},
		{"null", nil, true},
		{"non-numeric string", "abc", true},
		{"boolean true", true, true},
		{"boolean false", false, true},
		{"object", map[string]interface{}{}, true},
		{"array", []interface{}{}, true},
		{"mixed string", "123abc", true},
		{"incomplete scientific", "1e", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNumber(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNumber() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateMarkdown(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		// Valid cases
		{"empty string", "", false},
		{"simple text", "Hello, world!", false},
		{"markdown heading", "# Title", false},
		{"markdown list", "- Item 1\n- Item 2", false},
		{"markdown bold", "**bold text**", false},
		{"complex markdown", "# Heading\n\n## Subheading\n\n- List item\n- Another item\n\n**Bold** and *italic*", false},

		// Invalid cases
		{"null", nil, true},
		{"number", 123, true},
		{"boolean", true, true},
		{"object", map[string]interface{}{}, true},
		{"array", []string{"a"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMarkdown(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMarkdown() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		// Valid cases
		{"valid object string", `{"key": "value"}`, false},
		{"valid array string", `["a", "b", "c"]`, false},
		{"valid number", `123`, false},
		{"valid string value", `"hello"`, false},
		{"valid boolean", `true`, false},
		{"valid null", `null`, false},
		{"nested object", `{"outer": {"inner": "value"}}`, false},
		{"native map", map[string]interface{}{"key": "value"}, false},
		{"native slice", []string{"a", "b"}, false},

		// Invalid cases
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"null value", nil, true},
		{"malformed json - missing brace", `{"key": "value"`, true},
		{"malformed json - trailing comma", `{"key": "value",}`, true},
		{"malformed json - unquoted key", `{key: "value"}`, true},
		{"malformed json - single quotes", `{'key': 'value'}`, true},
		{"plain text", "not json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSON(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCode(t *testing.T) {
	tests := []struct {
		name    string
		value   interface{}
		wantErr bool
	}{
		// Valid cases
		{"simple code", "print('hello')", false},
		{"multiline code", "def foo():\n    return 42", false},
		{"code with symbols", "if (x > 0) { return true; }", false},

		// Invalid cases
		{"empty string", "", true},
		{"whitespace only", "   ", true},
		{"null", nil, true},
		{"number", 123, true},
		{"boolean", false, true},
		{"object", map[string]interface{}{}, true},
		{"array", []string{"code"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCode(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		format  string
		value   interface{}
		wantErr bool
	}{
		// String format
		{"string format - any value", "string", "anything", false},
		{"string format - empty", "string", "", false},
		{"empty format defaults to string", "", "value", false},

		// Number format
		{"number format - valid", "number", "123.45", false},
		{"number format - invalid", "number", "abc", true},
		{"number format case insensitive", "NUMBER", "456", false},

		// Markdown format
		{"markdown format - valid", "markdown", "# Heading", false},
		{"markdown format - empty", "markdown", "", false},
		{"markdown format - null", "markdown", nil, true},

		// JSON format
		{"json format - valid", "json", `{"key": "value"}`, false},
		{"json format - invalid", "json", "not json", true},
		{"json format case insensitive", "JSON", `[]`, false},

		// Code format
		{"code format - valid", "code", "print('hi')", false},
		{"code format - empty", "code", "", true},
		{"code with language - valid", "code:python", "def foo(): pass", false},
		{"code with language - empty", "code:python", "", true},
		{"code with language case insensitive", "CODE:JavaScript", "console.log('hi')", false},

		// Unknown format
		{"unknown format", "unknown", "value", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.format, tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
