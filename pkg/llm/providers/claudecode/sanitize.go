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
	"regexp"
	"strings"
)

var (
	// Patterns for sensitive information to remove from error messages
	pathPatterns = []*regexp.Regexp{
		regexp.MustCompile(`/Users/[^/\s]+`),
		regexp.MustCompile(`/home/[^/\s]+`),
		regexp.MustCompile(`/etc/[^:\s]+`),
		regexp.MustCompile(`C:\\Users\\[^\\]+`),
		regexp.MustCompile(`C:\\Documents and Settings\\[^\\]+`),
	}

	usernamePattern  = regexp.MustCompile(`user(?:name)?[:\s]+[^\s]+`)
	privateIPPattern = regexp.MustCompile(`\b(?:10\.|172\.(?:1[6-9]|2[0-9]|3[01])\.|192\.168\.)[0-9.]+\b`)
	ipPattern        = regexp.MustCompile(`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`)
)

// sanitizeError removes sensitive information from error messages
// to prevent leakage of paths, usernames, IP addresses, and internal details
func sanitizeError(errMsg string) string {
	result := errMsg

	// Remove absolute file paths
	for _, pattern := range pathPatterns {
		result = pattern.ReplaceAllString(result, "[PATH]")
	}

	// Remove username references
	result = usernamePattern.ReplaceAllString(result, "user: [REDACTED]")

	// Remove private network details first (before general IP pattern)
	result = privateIPPattern.ReplaceAllString(result, "[PRIVATE_IP]")

	// Remove IP addresses
	result = ipPattern.ReplaceAllString(result, "[IP]")

	// Remove stack traces (lines starting with "at " or containing file:line references)
	lines := strings.Split(result, "\n")
	var sanitized []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "at ") || strings.Contains(trimmed, ".go:") {
			continue
		}
		sanitized = append(sanitized, line)
	}
	result = strings.Join(sanitized, "\n")

	return strings.TrimSpace(result)
}
