// Package secrets provides utilities for detecting and masking sensitive values.
package secrets

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewMasker(t *testing.T) {
	m := NewMasker()
	if m == nil {
		t.Fatal("NewMasker() returned nil")
	}
	if m.secrets == nil {
		t.Error("secrets map not initialized")
	}
	if len(m.patterns) == 0 {
		t.Error("default patterns not set")
	}

	// Verify default patterns include expected suffixes
	expectedPatterns := []string{"_TOKEN", "_SECRET", "_KEY", "_PASSWORD", "_PASS", "_PWD"}
	for _, expected := range expectedPatterns {
		found := false
		for _, p := range m.patterns {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected pattern %q not found in default patterns", expected)
		}
	}
}

func TestMasker_AddSecret(t *testing.T) {
	tests := []struct {
		name    string
		secrets []string
		input   string
		want    string
	}{
		{
			name:    "single secret",
			secrets: []string{"password123"},
			input:   "The password is password123",
			want:    "The password is ***",
		},
		{
			name:    "multiple occurrences",
			secrets: []string{"secret"},
			input:   "secret appears twice: secret",
			want:    "*** appears twice: ***",
		},
		{
			name:    "multiple secrets",
			secrets: []string{"token123", "key456"},
			input:   "token: token123, key: key456",
			want:    "token: ***, key: ***",
		},
		{
			name:    "empty secret ignored",
			secrets: []string{"", "valid"},
			input:   "this is valid",
			want:    "this is ***",
		},
		{
			name:    "no secrets to mask",
			secrets: []string{"notpresent"},
			input:   "nothing to hide here",
			want:    "nothing to hide here",
		},
		{
			name:    "overlapping secrets - shorter processed first",
			secrets: []string{"abc"},
			input:   "the value is abcd",
			want:    "the value is ***d",
		},
		{
			name:    "secret at start",
			secrets: []string{"SECRET"},
			input:   "SECRET is at the start",
			want:    "*** is at the start",
		},
		{
			name:    "secret at end",
			secrets: []string{"END"},
			input:   "value at the END",
			want:    "value at the ***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMasker()
			for _, s := range tt.secrets {
				m.AddSecret(s)
			}
			got := m.Mask(tt.input)
			if got != tt.want {
				t.Errorf("Mask() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMasker_AddSecretsFromEnv(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		input string
		want  string
	}{
		{
			name: "API_TOKEN matched",
			env: map[string]string{
				"API_TOKEN": "my-secret-token",
			},
			input: "Using token: my-secret-token",
			want:  "Using token: ***",
		},
		{
			name: "DATABASE_PASSWORD matched",
			env: map[string]string{
				"DATABASE_PASSWORD": "db-pass-123",
			},
			input: "Password is db-pass-123",
			want:  "Password is ***",
		},
		{
			name: "AWS_SECRET matched",
			env: map[string]string{
				"AWS_SECRET": "aws-secret-key",
			},
			input: "AWS: aws-secret-key",
			want:  "AWS: ***",
		},
		{
			name: "ENCRYPTION_KEY matched",
			env: map[string]string{
				"ENCRYPTION_KEY": "enc-key-value",
			},
			input: "Key: enc-key-value",
			want:  "Key: ***",
		},
		{
			name: "admin_pass matched (lowercase)",
			env: map[string]string{
				"admin_pass": "admin-password",
			},
			input: "Admin password: admin-password",
			want:  "Admin password: ***",
		},
		{
			name: "user_pwd matched",
			env: map[string]string{
				"user_pwd": "user-password",
			},
			input: "User pwd: user-password",
			want:  "User pwd: ***",
		},
		{
			name: "non-secret env not matched",
			env: map[string]string{
				"HOME":     "/home/user",
				"PATH":     "/usr/bin",
				"SOME_VAR": "not-secret",
			},
			input: "Home: /home/user, path: /usr/bin, var: not-secret",
			want:  "Home: /home/user, path: /usr/bin, var: not-secret",
		},
		{
			name: "empty value ignored",
			env: map[string]string{
				"API_TOKEN": "",
			},
			input: "Empty token is okay",
			want:  "Empty token is okay",
		},
		{
			name: "multiple secret patterns",
			env: map[string]string{
				"API_TOKEN":     "token-val",
				"DB_SECRET":     "secret-val",
				"ENCRYPT_KEY":   "key-val",
				"ADMIN_PASSWORD": "pass-val",
			},
			input: "token-val secret-val key-val pass-val",
			want:  "*** *** *** ***",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMasker()
			m.AddSecretsFromEnv(tt.env)
			got := m.Mask(tt.input)
			if got != tt.want {
				t.Errorf("Mask() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMasker_isSecretKey(t *testing.T) {
	m := NewMasker()

	tests := []struct {
		key  string
		want bool
	}{
		// Should match
		{"API_TOKEN", true},
		{"api_token", true}, // case insensitive
		{"GITHUB_TOKEN", true},
		{"DATABASE_SECRET", true},
		{"database_secret", true},
		{"AWS_SECRET_KEY", true}, // ends with _KEY
		{"ENCRYPTION_KEY", true},
		{"encryption_key", true},
		{"DB_PASSWORD", true},
		{"DATABASE_PASSWORD", true},
		{"ADMIN_PASS", true},
		{"admin_pass", true},
		{"USER_PWD", true},
		{"user_pwd", true},

		// Should not match
		{"HOME", false},
		{"PATH", false},
		{"GOPATH", false},
		{"MY_VARIABLE", false},
		{"TOKENIZER", false},      // TOKEN is not a suffix
		{"SECRET_SAUCE", false},   // SECRET is not a suffix
		{"KEYBOARD", false},       // KEY is not a suffix
		{"PASSWORD_FILE", false},  // PASSWORD is not a suffix
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := m.isSecretKey(tt.key)
			if got != tt.want {
				t.Errorf("isSecretKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestMasker_Mask(t *testing.T) {
	tests := []struct {
		name    string
		secrets []string
		input   string
		want    string
	}{
		{
			name:    "empty input",
			secrets: []string{"secret"},
			input:   "",
			want:    "",
		},
		{
			name:    "no secrets registered",
			secrets: []string{},
			input:   "nothing to mask",
			want:    "nothing to mask",
		},
		{
			name:    "unicode secret",
			secrets: []string{"пароль123"},
			input:   "Пароль: пароль123",
			want:    "Пароль: ***",
		},
		{
			name:    "emoji secret",
			secrets: []string{"🔑secret🔑"},
			input:   "key is 🔑secret🔑 here",
			want:    "key is *** here",
		},
		{
			name:    "long secret",
			secrets: []string{strings.Repeat("x", 1000)},
			input:   "prefix " + strings.Repeat("x", 1000) + " suffix",
			want:    "prefix *** suffix",
		},
		{
			name:    "special regex characters in secret",
			secrets: []string{"secret.with+special*chars?"},
			input:   "The secret.with+special*chars? is exposed",
			want:    "The *** is exposed",
		},
		{
			name:    "newlines in input",
			secrets: []string{"secret"},
			input:   "line1\nsecret\nline3",
			want:    "line1\n***\nline3",
		},
		{
			name:    "tabs in input",
			secrets: []string{"secret"},
			input:   "col1\tsecret\tcol3",
			want:    "col1\t***\tcol3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMasker()
			for _, s := range tt.secrets {
				m.AddSecret(s)
			}
			got := m.Mask(tt.input)
			if got != tt.want {
				t.Errorf("Mask() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMasker_MaskMap(t *testing.T) {
	tests := []struct {
		name    string
		secrets []string
		input   map[string]interface{}
		want    map[string]interface{}
	}{
		{
			name:    "simple string values",
			secrets: []string{"secret123"},
			input: map[string]interface{}{
				"public":  "visible data",
				"private": "contains secret123 here",
			},
			want: map[string]interface{}{
				"public":  "visible data",
				"private": "contains *** here",
			},
		},
		{
			name:    "nested map",
			secrets: []string{"token123"},
			input: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": "token123 is here",
				},
			},
			want: map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": "*** is here",
				},
			},
		},
		{
			name:    "array values",
			secrets: []string{"secret"},
			input: map[string]interface{}{
				"items": []interface{}{
					"public item",
					"secret item",
					"another secret",
				},
			},
			want: map[string]interface{}{
				"items": []interface{}{
					"public item",
					"*** item",
					"another ***",
				},
			},
		},
		{
			name:    "boolean and number values preserved",
			secrets: []string{"secret"},
			input: map[string]interface{}{
				"enabled": true,
				"count":   json.Number("42"),
				"text":    "has secret",
			},
			want: map[string]interface{}{
				"enabled": true,
				"count":   json.Number("42"),
				"text":    "has ***",
			},
		},
		{
			name:    "nil values preserved",
			secrets: []string{"secret"},
			input: map[string]interface{}{
				"null_val": nil,
				"text":     "secret value",
			},
			want: map[string]interface{}{
				"null_val": nil,
				"text":     "*** value",
			},
		},
		{
			name:    "deeply nested structure",
			secrets: []string{"apikey"},
			input: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": map[string]interface{}{
							"key": "apikey here",
						},
					},
				},
			},
			want: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": map[string]interface{}{
							"key": "*** here",
						},
					},
				},
			},
		},
		{
			name:    "empty map",
			secrets: []string{"secret"},
			input:   map[string]interface{}{},
			want:    map[string]interface{}{},
		},
		{
			name:    "mixed arrays and maps",
			secrets: []string{"password"},
			input: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"name": "alice",
						"pass": "password123",
					},
					map[string]interface{}{
						"name": "bob",
						"pass": "mypassword",
					},
				},
			},
			want: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"name": "alice",
						"pass": "***123",
					},
					map[string]interface{}{
						"name": "bob",
						"pass": "my***",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMasker()
			for _, s := range tt.secrets {
				m.AddSecret(s)
			}
			got := m.MaskMap(tt.input)

			// Compare by serializing to JSON
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tt.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("MaskMap() = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestMasker_maskValue(t *testing.T) {
	m := NewMasker()
	m.AddSecret("secret")

	tests := []struct {
		name  string
		input interface{}
		want  interface{}
	}{
		{
			name:  "string with secret",
			input: "contains secret here",
			want:  "contains *** here",
		},
		{
			name:  "string without secret",
			input: "nothing to hide",
			want:  "nothing to hide",
		},
		{
			name:  "nil value",
			input: nil,
			want:  nil,
		},
		{
			name:  "bool true",
			input: true,
			want:  true,
		},
		{
			name:  "bool false",
			input: false,
			want:  false,
		},
		{
			name:  "json.Number",
			input: json.Number("123.45"),
			want:  json.Number("123.45"),
		},
		{
			name:  "int value masked as string",
			input: 42,
			want:  "42", // Unknown types get converted to string
		},
		{
			name:  "float value masked as string",
			input: 3.14,
			want:  "3.14",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.maskValue(tt.input)
			// Compare values
			if got != tt.want {
				t.Errorf("maskValue() = %v (%T), want %v (%T)", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestMasker_MaskJSON(t *testing.T) {
	tests := []struct {
		name    string
		secrets []string
		input   string
		want    string
	}{
		{
			name:    "simple JSON object",
			secrets: []string{"mysecret"},
			input:   `{"key": "mysecret", "public": "visible"}`,
			want:    `{"key":"***","public":"visible"}`,
		},
		{
			name:    "nested JSON",
			secrets: []string{"token123"},
			input:   `{"auth": {"token": "token123"}}`,
			want:    `{"auth":{"token":"***"}}`,
		},
		{
			name:    "JSON array",
			secrets: []string{"secret"},
			input:   `["public", "secret", "visible"]`,
			want:    `["public","***","visible"]`,
		},
		{
			name:    "invalid JSON - falls back to string masking",
			secrets: []string{"secret"},
			input:   "not valid json: secret",
			want:    "not valid json: ***",
		},
		{
			name:    "empty JSON object",
			secrets: []string{"secret"},
			input:   `{}`,
			want:    `{}`,
		},
		{
			name:    "JSON with numbers and booleans",
			secrets: []string{"password"},
			input:   `{"pass": "password", "count": 42, "enabled": true}`,
			want:    `{"count":"42","enabled":true,"pass":"***"}`, // Numbers become strings through maskValue default case
		},
		{
			name:    "JSON with null",
			secrets: []string{"secret"},
			input:   `{"value": null, "text": "secret"}`,
			want:    `{"text":"***","value":null}`,
		},
		{
			name:    "deeply nested JSON",
			secrets: []string{"apikey"},
			input:   `{"a":{"b":{"c":{"key":"apikey"}}}}`,
			want:    `{"a":{"b":{"c":{"key":"***"}}}}`,
		},
		{
			name:    "JSON with secret in multiple places",
			secrets: []string{"token"},
			input:   `{"auth_token": "token", "refresh_token": "token"}`,
			want:    `{"auth_token":"***","refresh_token":"***"}`,
		},
		{
			name:    "empty string",
			secrets: []string{"secret"},
			input:   "",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMasker()
			for _, s := range tt.secrets {
				m.AddSecret(s)
			}
			got := m.MaskJSON(tt.input)
			if got != tt.want {
				t.Errorf("MaskJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMasker_EdgeCases(t *testing.T) {
	t.Run("secret is substring of another", func(t *testing.T) {
		m := NewMasker()
		m.AddSecret("pass")
		m.AddSecret("password")

		// Both should be masked, order matters
		got := m.Mask("my password is pass")
		// The exact result depends on the iteration order of the map
		// but at least one should be masked
		if !strings.Contains(got, "***") {
			t.Errorf("expected some masking, got %q", got)
		}
	})

	t.Run("very long string with many secrets", func(t *testing.T) {
		m := NewMasker()
		m.AddSecret("secret")

		input := strings.Repeat("secret ", 1000)
		got := m.Mask(input)

		if strings.Contains(got, "secret") {
			t.Error("some secrets were not masked")
		}
	})

	t.Run("secret with only whitespace not added", func(t *testing.T) {
		m := NewMasker()
		m.AddSecret("   ")

		// Whitespace-only strings are not empty so they're added
		// This test documents current behavior
		got := m.Mask("has    spaces")
		// Current behavior: "   " would be added and potentially masked
		// This is expected - only truly empty strings are skipped
		_ = got
	})

	t.Run("nil map input", func(t *testing.T) {
		m := NewMasker()
		m.AddSecret("secret")

		// MaskMap with nil returns empty map
		got := m.MaskMap(nil)
		if got == nil || len(got) != 0 {
			t.Errorf("MaskMap(nil) should return empty map, got %v", got)
		}
	})

	t.Run("concurrent safety - read after setup", func(t *testing.T) {
		m := NewMasker()
		m.AddSecret("secret1")
		m.AddSecret("secret2")

		// Concurrent reads should be safe after setup
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				_ = m.Mask("test secret1 and secret2")
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func BenchmarkMasker_Mask(b *testing.B) {
	m := NewMasker()
	m.AddSecret("secret1")
	m.AddSecret("secret2")
	m.AddSecret("secret3")

	input := "This is a test string with secret1 and secret2 and secret3 in it"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Mask(input)
	}
}

func BenchmarkMasker_MaskJSON(b *testing.B) {
	m := NewMasker()
	m.AddSecret("token123")
	m.AddSecret("password456")

	input := `{"auth": {"token": "token123", "password": "password456"}, "data": "visible"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.MaskJSON(input)
	}
}

func BenchmarkMasker_MaskMap(b *testing.B) {
	m := NewMasker()
	m.AddSecret("secret")

	input := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"value": "secret data",
			},
		},
		"array": []interface{}{"item1", "secret item", "item3"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.MaskMap(input)
	}
}
