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

package debug

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Inspector provides utilities for inspecting workflow context.
type Inspector struct {
	context map[string]interface{}
}

// NewInspector creates a new inspector for the given context.
func NewInspector(context map[string]interface{}) *Inspector {
	return &Inspector{
		context: context,
	}
}

// Get retrieves a value from the context by key.
// Supports dot notation for nested access (e.g., "step1.output").
func (i *Inspector) Get(key string) (interface{}, bool) {
	parts := strings.Split(key, ".")
	current := i.context

	for idx, part := range parts {
		value, ok := current[part]
		if !ok {
			return nil, false
		}

		// If this is the last part, return the value
		if idx == len(parts)-1 {
			return value, true
		}

		// Otherwise, try to navigate deeper
		nested, ok := value.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = nested
	}

	return nil, false
}

// Keys returns all top-level keys in the context.
func (i *Inspector) Keys() []string {
	keys := make([]string, 0, len(i.context))
	for k := range i.context {
		keys = append(keys, k)
	}
	return keys
}

// Format formats a value for display.
func (i *Inspector) Format(value interface{}) (string, error) {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format value: %w", err)
	}
	return string(bytes), nil
}

// FormatContext formats the entire context for display.
func (i *Inspector) FormatContext() (string, error) {
	return i.Format(i.context)
}

// Summary returns a summary of the context (key names and types).
func (i *Inspector) Summary() string {
	var b strings.Builder
	for key, value := range i.context {
		b.WriteString(fmt.Sprintf("  %s: %T\n", key, value))
	}
	return b.String()
}
