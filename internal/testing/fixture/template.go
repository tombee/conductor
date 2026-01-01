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

package fixture

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"strings"
	"text/template"
	"time"

	"github.com/google/uuid"
)

// ExpandTemplate expands a fixture template string with restricted Sprig-like functions.
// Only safe, deterministic functions are allowed to prevent security issues.
func ExpandTemplate(tmpl string, data map[string]interface{}) (string, error) {
	t, err := template.New("fixture").Funcs(restrictedFuncMap()).Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// restrictedFuncMap returns a map of allowed template functions.
// This is a restricted subset of Sprig functions for security.
func restrictedFuncMap() template.FuncMap {
	return template.FuncMap{
		// String functions
		"upper":     strings.ToUpper,
		"lower":     strings.ToLower,
		"trim":      strings.TrimSpace,
		"trimLeft":  func(cutset, s string) string { return strings.TrimLeft(s, cutset) },
		"trimRight": func(cutset, s string) string { return strings.TrimRight(s, cutset) },
		"replace":   func(old, new, s string) string { return strings.ReplaceAll(s, old, new) },
		"contains":  func(substr, s string) bool { return strings.Contains(s, substr) },
		"hasPrefix": func(prefix, s string) bool { return strings.HasPrefix(s, prefix) },
		"hasSuffix": func(suffix, s string) bool { return strings.HasSuffix(s, suffix) },
		"repeat":    func(count int, s string) string { return strings.Repeat(s, count) },
		"split":     func(sep, s string) []string { return strings.Split(s, sep) },
		"join":      func(sep string, elems []string) string { return strings.Join(elems, sep) },
		"title":     strings.Title,

		// Math functions
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"mod": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a % b
		},
		"max": func(a, b int) int {
			if a > b {
				return a
			}
			return b
		},
		"min": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"floor": func(f float64) float64 { return math.Floor(f) },
		"ceil":  func(f float64) float64 { return math.Ceil(f) },
		"round": func(f float64) float64 { return math.Round(f) },

		// Date functions
		"now": time.Now,
		"date": func(fmt string, t time.Time) string {
			return t.Format(fmt)
		},

		// Encoding functions
		"b64enc": func(s string) string {
			return base64.StdEncoding.EncodeToString([]byte(s))
		},
		"b64dec": func(s string) (string, error) {
			data, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return "", err
			}
			return string(data), nil
		},

		// ID generation
		"uuidv4": func() string {
			return uuid.New().String()
		},

		// Default function (like Sprig's default)
		"default": func(def interface{}, val interface{}) interface{} {
			if val == nil || val == "" {
				return def
			}
			return val
		},
	}
}

// IsForbiddenFunction checks if a function name is explicitly forbidden.
// Returns true for dangerous functions that should never be allowed.
func IsForbiddenFunction(name string) bool {
	forbidden := []string{
		// Environment access
		"env", "getenv", "expandenv",

		// Command execution
		"exec", "shell",

		// File I/O
		"readFile", "writeFile", "readDir", "glob",

		// OS access
		"osBase", "osClean", "osDir", "osExt", "osIsAbs",

		// Network access
		"getHostByName",
	}

	for _, fn := range forbidden {
		if fn == name {
			return true
		}
	}
	return false
}
