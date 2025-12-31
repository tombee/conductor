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

package setup

import (
	"strings"
	"testing"
)

func TestMaskCredential(t *testing.T) {
	tests := []struct {
		name       string
		credential string
		want       string
	}{
		{
			name:       "empty credential",
			credential: "",
			want:       "(not set)",
		},
		{
			name:       "very short credential",
			credential: "abc",
			want:       "abc***",
		},
		{
			name:       "short credential",
			credential: "abcdef",
			want:       "abc***",
		},
		{
			name:       "standard API key",
			credential: "sk-abc123xyz789",
			want:       "sk-ab*****789",
		},
		{
			name:       "GitHub token",
			credential: "ghp_abc123xyz789def456",
			want:       "ghp_a*****456",
		},
		{
			name:       "long credential",
			credential: "very-long-credential-with-many-characters",
			want:       "very-*****ers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskCredential(tt.credential)
			// Strip ANSI codes for comparison (lipgloss adds color codes)
			gotPlain := stripANSI(got)
			if gotPlain != tt.want {
				t.Errorf("MaskCredential() = %q, want %q", gotPlain, tt.want)
			}
		})
	}
}

func TestFormatProviderStatus(t *testing.T) {
	tests := []struct {
		name         string
		providerName string
		providerType string
		isDefault    bool
		wantContains []string
	}{
		{
			name:         "default provider",
			providerName: "claude",
			providerType: "claude-code",
			isDefault:    true,
			wantContains: []string{"claude", "claude-code", "default"},
		},
		{
			name:         "non-default provider",
			providerName: "ollama",
			providerType: "ollama",
			isDefault:    false,
			wantContains: []string{"ollama"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatProviderStatus(tt.providerName, tt.providerType, tt.isDefault)
			gotPlain := stripANSI(got)
			for _, want := range tt.wantContains {
				if !strings.Contains(gotPlain, want) {
					t.Errorf("FormatProviderStatus() = %q, want to contain %q", gotPlain, want)
				}
			}
		})
	}
}

func TestFormatIntegrationStatus(t *testing.T) {
	got := FormatIntegrationStatus("github-main", "github")
	gotPlain := stripANSI(got)
	if !strings.Contains(gotPlain, "github-main") {
		t.Errorf("FormatIntegrationStatus() = %q, want to contain 'github-main'", gotPlain)
	}
	if !strings.Contains(gotPlain, "github") {
		t.Errorf("FormatIntegrationStatus() = %q, want to contain 'github'", gotPlain)
	}
}

func TestStatusIndicators(t *testing.T) {
	tests := []struct {
		name string
		fn   func() string
	}{
		{"StatusOK", StatusOK},
		{"StatusError", StatusError},
		{"StatusWarning", StatusWarning},
		{"StatusPending", StatusPending},
		{"StatusInfo", StatusInfo},
		{"StatusBullet", StatusBullet},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn()
			if result == "" {
				t.Errorf("%s() returned empty string", tt.name)
			}
		})
	}
}

func TestFormatList(t *testing.T) {
	items := []string{"Item 1", "Item 2", "Item 3"}
	got := FormatList(items)
	gotPlain := stripANSI(got)

	for _, item := range items {
		if !strings.Contains(gotPlain, item) {
			t.Errorf("FormatList() = %q, want to contain %q", gotPlain, item)
		}
	}
}

func TestFormatKeyValue(t *testing.T) {
	got := FormatKeyValue("API Key", "sk-abc***789")
	gotPlain := stripANSI(got)
	if !strings.Contains(gotPlain, "API Key") {
		t.Errorf("FormatKeyValue() = %q, want to contain 'API Key'", gotPlain)
	}
	if !strings.Contains(gotPlain, "sk-abc***789") {
		t.Errorf("FormatKeyValue() = %q, want to contain 'sk-abc***789'", gotPlain)
	}
}

// stripANSI removes ANSI escape codes from a string for testing.
func stripANSI(s string) string {
	// Simple ANSI code stripper for testing
	// Matches CSI sequences: \x1b[...m
	var result strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i++ // skip '['
			continue
		}
		if inEscape {
			if s[i] == 'm' {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}
	return result.String()
}
