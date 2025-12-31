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
	"fmt"
	"regexp"
)

var (
	// namePattern matches valid provider/integration names
	// Alphanumeric, hyphens, underscores, 1-64 characters
	namePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
)

// ValidateName validates a provider or integration name.
// Requirements:
//   - 1-64 characters
//   - Start with alphanumeric
//   - Can contain alphanumeric, hyphens, underscores
//   - No consecutive hyphens or underscores
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}

	if len(name) > 64 {
		return fmt.Errorf("name must be 64 characters or less")
	}

	if !namePattern.MatchString(name) {
		return fmt.Errorf("name must start with a letter or number and contain only letters, numbers, hyphens, and underscores")
	}

	// Check for consecutive special characters
	if regexp.MustCompile(`--`).MatchString(name) {
		return fmt.Errorf("name cannot contain consecutive hyphens")
	}
	if regexp.MustCompile(`__`).MatchString(name) {
		return fmt.Errorf("name cannot contain consecutive underscores")
	}

	return nil
}

// SuggestName suggests a valid name based on input by:
//   - Converting to lowercase
//   - Replacing spaces with hyphens
//   - Removing invalid characters
func SuggestName(input string) string {
	// Replace spaces with hyphens
	name := regexp.MustCompile(`\s+`).ReplaceAllString(input, "-")

	// Remove invalid characters
	name = regexp.MustCompile(`[^a-zA-Z0-9_-]`).ReplaceAllString(name, "")

	// Remove leading/trailing hyphens
	name = regexp.MustCompile(`^-+|-+$`).ReplaceAllString(name, "")

	// Collapse multiple hyphens
	name = regexp.MustCompile(`-+`).ReplaceAllString(name, "-")

	// Truncate to 64 chars
	if len(name) > 64 {
		name = name[:64]
	}

	return name
}
