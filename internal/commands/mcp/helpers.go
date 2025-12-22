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

package mcp

import (
	"strings"
	"time"

	"github.com/tombee/conductor/internal/commands/shared"
)

func formatDuration(d time.Duration) string {
	return shared.FormatDuration(d)
}

func wrapText(text string, width int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		if currentLine.Len()+len(word)+1 > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)
	}
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return strings.Join(lines, "\n")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func isValidServerName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	// Must start with letter
	if name[0] < 'a' || (name[0] > 'z' && name[0] < 'A') || name[0] > 'Z' {
		return false
	}
	// Rest can be letters, numbers, hyphens, underscores
	for _, c := range name[1:] {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func containsShellMeta(s string) bool {
	shellMeta := []string{";", "|", "&", "`", "$", "(", ")", "\n", "\r"}
	for _, m := range shellMeta {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
