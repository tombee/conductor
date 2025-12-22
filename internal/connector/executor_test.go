package connector

import (
	"strings"
	"testing"

	"github.com/tombee/conductor/pkg/workflow"
)

func TestSanitizeHeaderValue(t *testing.T) {
	tests := []struct {
		name      string
		headerKey string
		value     string
		wantError bool
	}{
		{
			name:      "valid header value",
			headerKey: "Authorization",
			value:     "Bearer token123",
			wantError: false,
		},
		{
			name:      "valid header with special chars",
			headerKey: "X-Custom-Header",
			value:     "value-with-dashes_and_underscores.dots",
			wantError: false,
		},
		{
			name:      "carriage return injection",
			headerKey: "X-Injected",
			value:     "value\rInjected: true",
			wantError: true,
		},
		{
			name:      "line feed injection",
			headerKey: "X-Injected",
			value:     "value\nInjected: true",
			wantError: true,
		},
		{
			name:      "CRLF injection",
			headerKey: "X-Injected",
			value:     "value\r\nInjected: true",
			wantError: true,
		},
		{
			name:      "null byte injection",
			headerKey: "X-Injected",
			value:     "value\x00injected",
			wantError: true,
		},
		{
			name:      "multiple CRLF injection",
			headerKey: "X-Injected",
			value:     "value\r\n\r\n<script>alert('xss')</script>",
			wantError: true,
		},
		{
			name:      "empty value",
			headerKey: "X-Empty",
			value:     "",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sanitizeHeaderValue(tt.headerKey, tt.value)

			if tt.wantError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIsSensitiveHeader(t *testing.T) {
	tests := []struct {
		name       string
		headerName string
		want       bool
	}{
		{
			name:       "content-length is sensitive",
			headerName: "Content-Length",
			want:       true,
		},
		{
			name:       "content-length lowercase",
			headerName: "content-length",
			want:       true,
		},
		{
			name:       "content-encoding is sensitive",
			headerName: "Content-Encoding",
			want:       true,
		},
		{
			name:       "transfer-encoding is sensitive",
			headerName: "Transfer-Encoding",
			want:       true,
		},
		{
			name:       "host is sensitive",
			headerName: "Host",
			want:       true,
		},
		{
			name:       "authorization is not sensitive",
			headerName: "Authorization",
			want:       false,
		},
		{
			name:       "custom header is not sensitive",
			headerName: "X-Custom-Header",
			want:       false,
		},
		{
			name:       "content-type is not sensitive",
			headerName: "Content-Type",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSensitiveHeader(tt.headerName)
			if got != tt.want {
				t.Errorf("isSensitiveHeader(%q) = %v, want %v", tt.headerName, got, tt.want)
			}
		})
	}
}

func TestApplyHeaders_ValidationAndProtection(t *testing.T) {
	tests := []struct {
		name          string
		connHeaders   map[string]string
		opHeaders     map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "valid headers",
			connHeaders: map[string]string{
				"User-Agent": "conductor/1.0",
				"Accept":     "application/json",
			},
			opHeaders: map[string]string{
				"X-Custom": "value",
			},
			wantError: false,
		},
		{
			name: "header injection in connector header",
			connHeaders: map[string]string{
				"X-Injected": "value\r\nX-Evil: true",
			},
			wantError:     true,
			errorContains: "invalid character",
		},
		{
			name: "header injection in operation header",
			opHeaders: map[string]string{
				"X-Injected": "value\nX-Evil: true",
			},
			wantError:     true,
			errorContains: "invalid character",
		},
		{
			name: "attempt to override Content-Length",
			connHeaders: map[string]string{
				"Content-Length": "999",
			},
			wantError:     true,
			errorContains: "protected header",
		},
		{
			name: "attempt to override Content-Encoding",
			opHeaders: map[string]string{
				"content-encoding": "gzip",
			},
			wantError:     true,
			errorContains: "protected header",
		},
		{
			name: "attempt to override Transfer-Encoding",
			opHeaders: map[string]string{
				"Transfer-Encoding": "chunked",
			},
			wantError:     true,
			errorContains: "protected header",
		},
		{
			name: "attempt to override Host",
			opHeaders: map[string]string{
				"Host": "evil.com",
			},
			wantError:     true,
			errorContains: "protected header",
		},
		{
			name: "null byte in header value",
			opHeaders: map[string]string{
				"X-Test": "value\x00injected",
			},
			wantError:     true,
			errorContains: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal executor for testing
			executor := &httpExecutor{
				connector: &httpConnector{
					def: &workflow.ConnectorDefinition{
						Headers: tt.connHeaders,
					},
				},
				operation: &workflow.OperationDefinition{
					Method:  "GET",
					Path:    "/test",
					Headers: tt.opHeaders,
				},
			}

			req := newTestRequest()
			err := executor.applyHeaders(req, map[string]interface{}{})

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
