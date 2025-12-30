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

package claudecode

import (
	"strings"
	"testing"
)

func TestSanitizeError_PathLeakage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unix home directory",
			input: "failed to read file /home/alice/secrets.txt: permission denied",
			want:  "failed to read file [PATH]/secrets.txt: permission denied",
		},
		{
			name:  "macOS user directory",
			input: "error accessing /Users/bob/Documents/private.txt",
			want:  "error accessing [PATH]/Documents/private.txt",
		},
		{
			name:  "etc directory",
			input: "cannot read /etc/shadow: access denied",
			want:  "cannot read [PATH]: access denied",
		},
		{
			name:  "windows user directory",
			input: `file not found: C:\Users\charlie\data.txt`,
			want:  `file not found: [PATH]\data.txt`,
		},
		{
			name:  "multiple paths",
			input: "copying from /home/alice/src.txt to /home/bob/dst.txt failed",
			want:  "copying from [PATH]/src.txt to [PATH]/dst.txt failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeError(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeError_UsernameLeakage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "username with colon",
			input: "authentication failed for username: alice",
			want:  "authentication failed for user: [REDACTED]",
		},
		{
			name:  "user with space",
			input: "logged in as user alice",
			want:  "logged in as user: [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeError(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeError_IPLeakage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "public IP address",
			input: "connection to 203.0.113.45 failed",
			want:  "connection to [IP] failed",
		},
		{
			name:  "private IP address",
			input: "internal server 192.168.1.100 unreachable",
			want:  "internal server [PRIVATE_IP] unreachable",
		},
		{
			name:  "10.x private network",
			input: "connecting to database at 10.0.5.23",
			want:  "connecting to database at [PRIVATE_IP]",
		},
		{
			name:  "172.x private network",
			input: "service unavailable at 172.16.0.1",
			want:  "service unavailable at [PRIVATE_IP]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeError(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeError_StackTrace(t *testing.T) {
	input := `error executing command
at executeCommand (executor.go:123)
at processRequest (handler.go:456)
command execution failed`

	result := sanitizeError(input)

	if strings.Contains(result, ".go:") {
		t.Error("sanitized error still contains stack trace references")
	}

	if strings.Contains(result, "at ") {
		t.Error("sanitized error still contains 'at' stack frames")
	}

	if !strings.Contains(result, "error executing command") {
		t.Error("sanitized error removed the actual error message")
	}

	if !strings.Contains(result, "command execution failed") {
		t.Error("sanitized error removed the error description")
	}
}

func TestSanitizeError_NoSensitiveInfo(t *testing.T) {
	input := "operation failed: invalid input format"
	want := "operation failed: invalid input format"

	got := sanitizeError(input)
	if got != want {
		t.Errorf("sanitizeError() modified non-sensitive error: got %q, want %q", got, want)
	}
}

func TestSanitizeError_CombinedPatterns(t *testing.T) {
	input := `file operation failed at /home/alice/project/file.txt
username: alice
connecting to 192.168.1.50
at fileHandler.go:789`

	result := sanitizeError(input)

	if strings.Contains(result, "/home/alice") {
		t.Error("path not sanitized")
	}

	if strings.Contains(result, "alice") {
		t.Error("username not sanitized")
	}

	if strings.Contains(result, "192.168.1.50") {
		t.Error("IP address not sanitized")
	}

	if strings.Contains(result, ".go:") {
		t.Error("stack trace not removed")
	}
}
