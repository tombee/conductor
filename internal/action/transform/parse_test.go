package transform

import (
	"context"
	"testing"
)

func TestParseJSON(t *testing.T) {
	conn, err := New(nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name    string
		input   interface{}
		wantErr bool
		errType ErrorType
		check   func(*testing.T, interface{})
	}{
		{
			name:    "missing data parameter",
			input:   nil,
			wantErr: true,
			errType: ErrorTypeValidation,
		},
		{
			name: "null input",
			input: map[string]interface{}{
				"data": nil,
			},
			wantErr: true,
			errType: ErrorTypeEmptyInput,
		},
		{
			name: "already parsed object",
			input: map[string]interface{}{
				"data": map[string]interface{}{"foo": "bar"},
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("result type = %T, want map[string]interface{}", result)
					return
				}
				if m["foo"] != "bar" {
					t.Errorf("result[foo] = %v, want bar", m["foo"])
				}
			},
		},
		{
			name: "already parsed array",
			input: map[string]interface{}{
				"data": []interface{}{1, 2, 3},
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				if !ok {
					t.Errorf("result type = %T, want []interface{}", result)
					return
				}
				if len(arr) != 3 {
					t.Errorf("len(result) = %v, want 3", len(arr))
				}
			},
		},
		{
			name: "simple JSON string",
			input: map[string]interface{}{
				"data": `{"name": "test", "value": 42}`,
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("result type = %T, want map[string]interface{}", result)
					return
				}
				if m["name"] != "test" {
					t.Errorf("result[name] = %v, want test", m["name"])
				}
			},
		},
		{
			name: "JSON with leading text",
			input: map[string]interface{}{
				"data": `Here is the JSON you requested:
{"items": [{"id": 1}, {"id": 2}]}`,
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("result type = %T, want map[string]interface{}", result)
					return
				}
				items, ok := m["items"].([]interface{})
				if !ok {
					t.Errorf("items type = %T, want []interface{}", m["items"])
					return
				}
				if len(items) != 2 {
					t.Errorf("len(items) = %v, want 2", len(items))
				}
			},
		},
		{
			name: "markdown json fence",
			input: map[string]interface{}{
				"data": "```json\n{\"status\": \"ok\"}\n```",
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("result type = %T, want map[string]interface{}", result)
					return
				}
				if m["status"] != "ok" {
					t.Errorf("result[status] = %v, want ok", m["status"])
				}
			},
		},
		{
			name: "markdown plain fence",
			input: map[string]interface{}{
				"data": "```\n[1, 2, 3]\n```",
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				if !ok {
					t.Errorf("result type = %T, want []interface{}", result)
					return
				}
				if len(arr) != 3 {
					t.Errorf("len(result) = %v, want 3", len(arr))
				}
			},
		},
		{
			name: "nested brackets",
			input: map[string]interface{}{
				"data": `{"outer": {"inner": {"deep": "value"}}}`,
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				m, ok := result.(map[string]interface{})
				if !ok {
					t.Errorf("result type = %T, want map[string]interface{}", result)
					return
				}
				outer, ok := m["outer"].(map[string]interface{})
				if !ok {
					t.Error("outer not a map")
					return
				}
				inner, ok := outer["inner"].(map[string]interface{})
				if !ok {
					t.Error("inner not a map")
					return
				}
				if inner["deep"] != "value" {
					t.Errorf("deep = %v, want value", inner["deep"])
				}
			},
		},
		{
			name: "malformed JSON",
			input: map[string]interface{}{
				"data": `{"broken": true, "missing": }`,
			},
			wantErr: true,
			errType: ErrorTypeParseError,
		},
		{
			name: "no JSON in text",
			input: map[string]interface{}{
				"data": "This is just plain text with no JSON",
			},
			wantErr: true,
			errType: ErrorTypeParseError,
		},
		{
			name: "wrong input type",
			input: map[string]interface{}{
				"data": 12345,
			},
			wantErr: true,
			errType: ErrorTypeTypeError,
		},
		{
			name: "array in text",
			input: map[string]interface{}{
				"data": `The results are: [1, 2, 3]`,
			},
			wantErr: false,
			check: func(t *testing.T, result interface{}) {
				arr, ok := result.([]interface{})
				if !ok {
					t.Errorf("result type = %T, want []interface{}", result)
					return
				}
				if len(arr) != 3 {
					t.Errorf("len(result) = %v, want 3", len(arr))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inputs map[string]interface{}
			if tt.input != nil {
				inputs, _ = tt.input.(map[string]interface{})
			}

			result, err := conn.Execute(context.Background(), "parse_json", inputs)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				opErr, ok := err.(*OperationError)
				if !ok {
					t.Errorf("error type = %T, want *OperationError", err)
					return
				}
				if opErr.ErrorType != tt.errType {
					t.Errorf("error type = %v, want %v", opErr.ErrorType, tt.errType)
				}
				return
			}

			if tt.check != nil && result != nil {
				tt.check(t, result.Response)
			}
		})
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		method string
	}{
		{
			name:   "direct object",
			input:  `{"key": "value"}`,
			want:   `{"key": "value"}`,
			method: "direct",
		},
		{
			name:   "direct array",
			input:  `[1, 2, 3]`,
			want:   `[1, 2, 3]`,
			method: "direct",
		},
		{
			name:   "markdown json fence",
			input:  "```json\n{\"test\": true}\n```",
			want:   `{"test": true}`,
			method: "markdown_json_fence",
		},
		{
			name:   "markdown plain fence",
			input:  "```\n{\"test\": true}\n```",
			want:   `{"test": true}`,
			method: "markdown_fence",
		},
		{
			name:   "JSON with prefix text",
			input:  `Here is the response: {"status": "ok"}`,
			want:   `{"status": "ok"}`,
			method: "bracket_matching",
		},
		{
			name:   "no JSON",
			input:  "Just plain text",
			want:   "",
			method: "none",
		},
		{
			name:   "nested brackets",
			input:  `{"a": {"b": {"c": "d"}}}`,
			want:   `{"a": {"b": {"c": "d"}}}`,
			method: "direct",
		},
		{
			name:   "JSON with strings containing brackets",
			input:  `{"text": "this has { and } inside"}`,
			want:   `{"text": "this has { and } inside"}`,
			method: "direct",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, method := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON() = %v, want %v", got, tt.want)
			}
			if method != tt.method {
				t.Errorf("extractJSON() method = %v, want %v", method, tt.method)
			}
		})
	}
}

func TestRedactSensitive(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no sensitive data",
			input: "regular text",
			want:  "regular text",
		},
		{
			name:  "contains password",
			input: "userPassword: secret123",
			want:  "[REDACTED - contains sensitive data]",
		},
		{
			name:  "contains token",
			input: "api_token: abc123",
			want:  "[REDACTED - contains sensitive data]",
		},
		{
			name:  "contains API_KEY (case insensitive)",
			input: "API_KEY: xyz",
			want:  "[REDACTED - contains sensitive data]",
		},
		{
			name:  "contains secret",
			input: "mySecret value",
			want:  "[REDACTED - contains sensitive data]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactSensitive(tt.input)
			if got != tt.want {
				t.Errorf("redactSensitive() = %v, want %v", got, tt.want)
			}
		})
	}
}
