package httpclient

import (
	"net/url"
	"testing"
)

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no sensitive params",
			input:    "https://api.example.com/resource?foo=bar&baz=qux",
			expected: "https://api.example.com/resource?baz=qux&foo=bar",
		},
		{
			name:     "api_key param",
			input:    "https://api.example.com/resource?api_key=secret123&foo=bar",
			expected: "https://api.example.com/resource?api_key=%5BREDACTED%5D&foo=bar",
		},
		{
			name:     "token param",
			input:    "https://api.example.com/resource?token=abc123&foo=bar",
			expected: "https://api.example.com/resource?foo=bar&token=%5BREDACTED%5D",
		},
		{
			name:     "password param",
			input:    "https://api.example.com/resource?password=secret&user=john",
			expected: "https://api.example.com/resource?password=%5BREDACTED%5D&user=john",
		},
		{
			name:     "multiple sensitive params",
			input:    "https://api.example.com/resource?api_key=key1&token=tok1&password=pass1",
			expected: "https://api.example.com/resource?api_key=%5BREDACTED%5D&password=%5BREDACTED%5D&token=%5BREDACTED%5D",
		},
		{
			name:     "case insensitive - uppercase",
			input:    "https://api.example.com/resource?API_KEY=secret&TOKEN=tok",
			expected: "https://api.example.com/resource?API_KEY=%5BREDACTED%5D&TOKEN=%5BREDACTED%5D",
		},
		{
			name:     "case insensitive - mixed case",
			input:    "https://api.example.com/resource?Api_Key=secret&ToKeN=tok",
			expected: "https://api.example.com/resource?Api_Key=%5BREDACTED%5D&ToKeN=%5BREDACTED%5D",
		},
		{
			name:     "apikey without underscore",
			input:    "https://api.example.com/resource?apikey=secret123",
			expected: "https://api.example.com/resource?apikey=%5BREDACTED%5D",
		},
		{
			name:     "auth param",
			input:    "https://api.example.com/resource?auth=bearer123",
			expected: "https://api.example.com/resource?auth=%5BREDACTED%5D",
		},
		{
			name:     "secret param",
			input:    "https://api.example.com/resource?secret=mysecret",
			expected: "https://api.example.com/resource?secret=%5BREDACTED%5D",
		},
		{
			name:     "key param",
			input:    "https://api.example.com/resource?key=mykey123",
			expected: "https://api.example.com/resource?key=%5BREDACTED%5D",
		},
		{
			name:     "credential param",
			input:    "https://api.example.com/resource?credential=cred123",
			expected: "https://api.example.com/resource?credential=%5BREDACTED%5D",
		},
		{
			name:     "no query string",
			input:    "https://api.example.com/resource",
			expected: "https://api.example.com/resource",
		},
		{
			name:     "empty query string",
			input:    "https://api.example.com/resource?",
			expected: "https://api.example.com/resource?",
		},
		{
			name:     "substring match in param name",
			input:    "https://api.example.com/resource?my_api_key_value=secret",
			expected: "https://api.example.com/resource?my_api_key_value=%5BREDACTED%5D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.input)
			if err != nil {
				t.Fatalf("failed to parse input URL: %v", err)
			}

			result := sanitizeURL(u)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSanitizeURL_Nil(t *testing.T) {
	result := sanitizeURL(nil)
	if result != "" {
		t.Errorf("expected empty string for nil URL, got %q", result)
	}
}

func TestIsSensitiveParam(t *testing.T) {
	tests := []struct {
		param    string
		expected bool
	}{
		{"api_key", true},
		{"API_KEY", true},
		{"Api_Key", true},
		{"apikey", true},
		{"APIKEY", true},
		{"token", true},
		{"TOKEN", true},
		{"password", true},
		{"PASSWORD", true},
		{"auth", true},
		{"secret", true},
		{"key", true},
		{"credential", true},
		{"my_api_key", true},
		{"api_key_value", true},
		{"bearer_token", true},
		{"user_password", true},
		{"foo", false},
		{"bar", false},
		{"user", false},
		{"id", false},
		{"name", false},
	}

	for _, tt := range tests {
		t.Run(tt.param, func(t *testing.T) {
			result := isSensitiveParam(tt.param)
			if result != tt.expected {
				t.Errorf("isSensitiveParam(%q) = %v, expected %v", tt.param, result, tt.expected)
			}
		})
	}
}
