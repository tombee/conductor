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

package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// discoverTests finds all test files in the given paths
func discoverTests(paths []string) ([]TestFile, error) {
	var tests []TestFile
	seen := make(map[string]bool) // Avoid duplicates

	for _, path := range paths {
		// Check if path exists
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("path %q not found: %w", path, err)
		}

		if info.IsDir() {
			// Walk directory recursively
			err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Skip directories and non-YAML files
				if info.IsDir() || !isTestFile(filePath) {
					return nil
				}

				// Skip duplicates
				absPath, err := filepath.Abs(filePath)
				if err != nil {
					return err
				}
				if seen[absPath] {
					return nil
				}
				seen[absPath] = true

				// Parse test file
				test, err := parseTestFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to parse test file %q: %w", filePath, err)
				}

				tests = append(tests, test)
				return nil
			})
			if err != nil {
				return nil, err
			}
		} else {
			// Single file
			if !isTestFile(path) {
				return nil, fmt.Errorf("file %q does not match test file pattern (*_test.yaml, test_*.yaml)", path)
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			if !seen[absPath] {
				seen[absPath] = true
				test, err := parseTestFile(path)
				if err != nil {
					return nil, fmt.Errorf("failed to parse test file %q: %w", path, err)
				}
				tests = append(tests, test)
			}
		}
	}

	if len(tests) == 0 {
		return nil, fmt.Errorf("no test files found in: %s", strings.Join(paths, ", "))
	}

	return tests, nil
}

// isTestFile checks if a file matches test file naming patterns
func isTestFile(path string) bool {
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return false
	}

	basename := filepath.Base(path)
	return strings.HasSuffix(basename, "_test.yaml") ||
		strings.HasSuffix(basename, "_test.yml") ||
		strings.HasPrefix(basename, "test_")
}

// parseTestFile parses a test file into a TestFile struct
func parseTestFile(path string) (TestFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TestFile{}, err
	}

	var raw struct {
		Workflow string                 `yaml:"workflow"`
		Name     string                 `yaml:"name"`
		Fixtures string                 `yaml:"fixtures"`
		Inputs   map[string]interface{} `yaml:"inputs"`
		Assert   map[string]string      `yaml:"assert"`
	}

	if err := yaml.Unmarshal(data, &raw); err != nil {
		return TestFile{}, fmt.Errorf("invalid YAML: %w", err)
	}

	// Validate required fields
	if raw.Workflow == "" {
		return TestFile{}, fmt.Errorf("missing required field: workflow")
	}

	// Default test name to filename if not specified
	name := raw.Name
	if name == "" {
		name = filepath.Base(path)
		// Remove _test.yaml suffix for cleaner names
		name = strings.TrimSuffix(name, "_test.yaml")
		name = strings.TrimSuffix(name, "_test.yml")
		name = strings.TrimPrefix(name, "test_")
	}

	// Resolve workflow path relative to test file
	workflowPath := raw.Workflow
	if !filepath.IsAbs(workflowPath) {
		testDir := filepath.Dir(path)
		workflowPath = filepath.Join(testDir, workflowPath)
	}

	// Resolve fixtures path if specified
	fixturesPath := raw.Fixtures
	if fixturesPath != "" && !filepath.IsAbs(fixturesPath) {
		testDir := filepath.Dir(path)
		fixturesPath = filepath.Join(testDir, fixturesPath)
	}

	return TestFile{
		Path:     path,
		Workflow: workflowPath,
		Name:     name,
		Fixtures: fixturesPath,
		Inputs:   raw.Inputs,
		Assert:   raw.Assert,
	}, nil
}
