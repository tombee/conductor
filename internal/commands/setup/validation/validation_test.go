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

package validation

import (
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid https", url: "https://example.com", wantErr: false},
		{name: "valid https with path", url: "https://example.com/api", wantErr: false},
		{name: "valid localhost http", url: "http://localhost:3000", wantErr: false},
		{name: "valid 127.0.0.1", url: "http://127.0.0.1:8080", wantErr: false},
		{name: "empty url", url: "", wantErr: true},
		{name: "http non-localhost", url: "http://example.com", wantErr: true},
		{name: "path traversal", url: "https://example.com/../admin", wantErr: true},
		{name: "invalid scheme", url: "ftp://example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHTTPSURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid https", url: "https://example.com", wantErr: false},
		{name: "http localhost", url: "http://localhost", wantErr: true},
		{name: "http", url: "http://example.com", wantErr: true},
		{name: "empty", url: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHTTPSURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateHTTPSURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "add https", url: "example.com", want: "https://example.com"},
		{name: "remove trailing slash", url: "https://example.com/", want: "https://example.com"},
		{name: "both", url: "example.com/", want: "https://example.com"},
		{name: "already normalized", url: "https://example.com", want: "https://example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeURL(tt.url)
			if got != tt.want {
				t.Errorf("NormalizeURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateAPIKey(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		key          string
		wantErr      bool
	}{
		{name: "empty key", providerType: "anthropic", key: "", wantErr: true},
		{name: "openai-compatible any key", providerType: "openai-compatible", key: "any-key-works", wantErr: false},
		{name: "unknown provider", providerType: "unknown", key: "some-key", wantErr: false},
		{name: "anthropic valid format", providerType: "anthropic", key: "sk-ant-api03-" + randomString(93), wantErr: false},
		{name: "anthropic invalid", providerType: "anthropic", key: "invalid-key", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAPIKey(tt.providerType, tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAPIKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetectAPIKeyType(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "github pat", key: "ghp_1234567890123456789012345678901234AB", want: "github-pat"},
		{name: "slack bot", key: "xoxb-123-456-abcdef", want: "slack-bot"},
		{name: "unknown", key: "random-key", want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectAPIKeyType(tt.key)
			if got != tt.want {
				t.Errorf("DetectAPIKeyType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{name: "long key", key: "sk-ant-api03-abcdefghijk123456", want: "sk-a•••••3456"},
		{name: "short key", key: "short", want: "••••••••"},
		{name: "8 chars", key: "12345678", want: "••••••••"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskAPIKey(tt.key)
			if got != tt.want {
				t.Errorf("MaskAPIKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPlaintextCredential(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "github pat", value: "ghp_1234567890123456789012345678901234AB", want: true},
		{name: "secret ref", value: "$secret:my-key", want: false},
		{name: "env ref", value: "$env:MY_VAR", want: false},
		{name: "empty", value: "", want: false},
		{name: "unknown", value: "not-a-real-key", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsPlaintextCredential(tt.value)
			if got != tt.want {
				t.Errorf("IsPlaintextCredential() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid simple", input: "my-provider", wantErr: false},
		{name: "valid with underscore", input: "my_provider", wantErr: false},
		{name: "valid numbers", input: "provider123", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "too long", input: randomString(65), wantErr: true},
		{name: "consecutive hyphens", input: "my--provider", wantErr: true},
		{name: "consecutive underscores", input: "my__provider", wantErr: true},
		{name: "starts with hyphen", input: "-provider", wantErr: true},
		{name: "invalid chars", input: "my provider!", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSuggestName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "spaces to hyphens", input: "My Provider", want: "My-Provider"},
		{name: "remove invalid", input: "my provider!", want: "my-provider"},
		{name: "collapse hyphens", input: "my---provider", want: "my-provider"},
		{name: "trim hyphens", input: "-provider-", want: "provider"},
		{name: "truncate long", input: randomString(70), want: randomString(64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestName(tt.input)
			// For truncate test, just check length
			if tt.name == "truncate long" {
				if len(got) != 64 {
					t.Errorf("SuggestName() length = %v, want 64", len(got))
				}
			} else if got != tt.want {
				t.Errorf("SuggestName() = %v, want %v", got, tt.want)
			}
		})
	}
}

// randomString generates a random string of the given length for testing
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}
