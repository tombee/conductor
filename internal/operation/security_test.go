package operation

import (
	"testing"
)

func TestValidateURL_SSRF(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		allowedHosts []string
		blockedHosts []string
		wantError    bool
	}{
		{
			name:      "public URL allowed",
			url:       "https://api.github.com/repos",
			wantError: false,
		},
		{
			name:      "localhost blocked by default",
			url:       "http://localhost:8080/api",
			wantError: true,
		},
		{
			name:      "127.0.0.1 blocked by default",
			url:       "http://127.0.0.1:8080/api",
			wantError: true,
		},
		{
			name:      "private IP 10.x blocked",
			url:       "http://10.0.0.1/api",
			wantError: true,
		},
		{
			name:      "private IP 192.168.x blocked",
			url:       "http://192.168.1.1/api",
			wantError: true,
		},
		{
			name:      "cloud metadata endpoint blocked",
			url:       "http://169.254.169.254/latest/meta-data",
			wantError: true,
		},
		{
			name:         "explicitly allowed host",
			url:          "https://example.com/api",
			allowedHosts: []string{"example.com"},
			wantError:    false,
		},
		{
			name:         "wildcard allowed host",
			url:          "https://api.example.com/v1",
			allowedHosts: []string{"*.example.com"},
			wantError:    false,
		},
		{
			name:         "explicitly blocked host",
			url:          "https://blocked.example.com/api",
			blockedHosts: []string{"blocked.example.com"},
			wantError:    true,
		},
		{
			name:         "wildcard blocked host",
			url:          "https://api.blocked.com/v1",
			blockedHosts: []string{"*.blocked.com"},
			wantError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url, tt.allowedHosts, tt.blockedHosts)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check error type
			if tt.wantError && err != nil {
				connErr, ok := err.(*Error)
				if !ok {
					t.Errorf("expected *Error, got %T", err)
				} else if connErr.Type != ErrorTypeSSRF {
					t.Errorf("expected SSRF error, got %s", connErr.Type)
				}
			}
		})
	}
}

func TestValidatePathParameter(t *testing.T) {
	tests := []struct {
		name      string
		paramName string
		value     string
		wantError bool
	}{
		{
			name:      "safe value",
			paramName: "username",
			value:     "john-doe",
			wantError: false,
		},
		{
			name:      "path traversal ../",
			paramName: "file",
			value:     "../../../etc/passwd",
			wantError: true,
		},
		{
			name:      "path traversal ..\\",
			paramName: "file",
			value:     "..\\..\\windows\\system32",
			wantError: true,
		},
		{
			name:      "URL encoded path traversal",
			paramName: "file",
			value:     "%2e%2e%2f",
			wantError: true,
		},
		{
			name:      "null byte",
			paramName: "file",
			value:     "file.txt\x00.png",
			wantError: true,
		},
		{
			name:      "URL encoded null byte",
			paramName: "file",
			value:     "file.txt%00.png",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePathParameter(tt.paramName, tt.value)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Check error type
			if tt.wantError && err != nil {
				connErr, ok := err.(*Error)
				if !ok {
					t.Errorf("expected *Error, got %T", err)
				} else if connErr.Type != ErrorTypePathInjection {
					t.Errorf("expected path injection error, got %s", connErr.Type)
				}
			}
		})
	}
}

func TestMaskSensitiveValue(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{
			name:  "token masked",
			key:   "github_token",
			value: "ghp_abc123",
			want:  "[REDACTED]",
		},
		{
			name:  "secret masked",
			key:   "api_secret",
			value: "secret123",
			want:  "[REDACTED]",
		},
		{
			name:  "password masked",
			key:   "password",
			value: "pass123",
			want:  "[REDACTED]",
		},
		{
			name:  "api_key masked",
			key:   "api_key",
			value: "key123",
			want:  "[REDACTED]",
		},
		{
			name:  "normal value not masked",
			key:   "username",
			value: "john-doe",
			want:  "john-doe",
		},
		{
			name:  "case insensitive",
			key:   "GITHUB_TOKEN",
			value: "ghp_abc123",
			want:  "[REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskSensitiveValue(tt.key, tt.value)
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestMaskSensitiveHeaders(t *testing.T) {
	headers := map[string][]string{
		"Authorization":   {"Bearer token123"},
		"X-API-Key":       {"key123"},
		"Content-Type":    {"application/json"},
		"User-Agent":      {"conductor/1.0"},
		"X-Auth-Token":    {"auth123"},
		"X-Client-Secret": {"secret123"},
	}

	masked := MaskSensitiveHeaders(headers)

	// Check sensitive headers are masked
	sensitiveHeaders := []string{"Authorization", "X-API-Key", "X-Auth-Token", "X-Client-Secret"}
	for _, header := range sensitiveHeaders {
		if masked[header][0] != "[REDACTED]" {
			t.Errorf("expected %s to be masked, got %v", header, masked[header])
		}
	}

	// Check non-sensitive headers are not masked
	if masked["Content-Type"][0] != "application/json" {
		t.Errorf("Content-Type should not be masked, got %v", masked["Content-Type"])
	}

	if masked["User-Agent"][0] != "conductor/1.0" {
		t.Errorf("User-Agent should not be masked, got %v", masked["User-Agent"])
	}
}
