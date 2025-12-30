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
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Loader loads and resolves fixtures from a directory.
type Loader struct {
	fixturesDir string
	logger      *slog.Logger
}

// NewLoader creates a new fixture loader for the given directory.
func NewLoader(fixturesDir string, logger *slog.Logger) (*Loader, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Validate fixtures directory exists if specified
	if fixturesDir != "" {
		if _, err := os.Stat(fixturesDir); err != nil {
			return nil, fmt.Errorf("fixtures directory not found: %w", err)
		}
	}

	return &Loader{
		fixturesDir: fixturesDir,
		logger:      logger,
	}, nil
}

// LoadLLMFixture loads a fixture for an LLM step.
// It follows the resolution order: step-specific > global > recorded > error
func (l *Loader) LoadLLMFixture(stepID string) (*LLMFixture, error) {
	// Try step-specific fixture first
	paths := l.buildFixturePaths(stepID)
	for _, path := range paths {
		if fixture, err := l.loadLLMFixtureFromFile(path); err == nil {
			l.logger.Debug("[MOCK] Loaded LLM fixture", "step_id", stepID, "source", path)
			return fixture, nil
		}
	}

	// Try global LLM fixture
	globalPaths := l.buildFixturePaths("_llm")
	for _, path := range globalPaths {
		if fixture, err := l.loadLLMFixtureFromFile(path); err == nil {
			l.logger.Debug("[MOCK] Loaded global LLM fixture", "step_id", stepID, "source", path)
			return fixture, nil
		}
	}

	// Try recorded fixture
	recordedPath := filepath.Join(l.fixturesDir, ".recorded", stepID+".yaml")
	if fixture, err := l.loadLLMFixtureFromFile(recordedPath); err == nil {
		l.logger.Debug("[MOCK] Loaded recorded LLM fixture", "step_id", stepID, "source", recordedPath)
		return fixture, nil
	}

	// No fixture found
	return nil, fmt.Errorf("no fixture found for LLM step %q, tried: %v, %v, %s",
		stepID, paths, globalPaths, recordedPath)
}

// LoadHTTPFixture loads a fixture for an HTTP action.
func (l *Loader) LoadHTTPFixture(stepID string) (*HTTPFixture, error) {
	// Try step-specific fixture first
	paths := l.buildFixturePaths(stepID)
	for _, path := range paths {
		if fixture, err := l.loadHTTPFixtureFromFile(path); err == nil {
			l.logger.Debug("[MOCK] Loaded HTTP fixture", "step_id", stepID, "source", path)
			return fixture, nil
		}
	}

	// Try global HTTP fixture
	globalPaths := l.buildFixturePaths("_http")
	for _, path := range globalPaths {
		if fixture, err := l.loadHTTPFixtureFromFile(path); err == nil {
			l.logger.Debug("[MOCK] Loaded global HTTP fixture", "step_id", stepID, "source", path)
			return fixture, nil
		}
	}

	// Try recorded fixture
	recordedPath := filepath.Join(l.fixturesDir, ".recorded", stepID+".yaml")
	if fixture, err := l.loadHTTPFixtureFromFile(recordedPath); err == nil {
		l.logger.Debug("[MOCK] Loaded recorded HTTP fixture", "step_id", stepID, "source", recordedPath)
		return fixture, nil
	}

	// No fixture found
	return nil, fmt.Errorf("no fixture found for HTTP step %q, tried: %v, %v, %s",
		stepID, paths, globalPaths, recordedPath)
}

// LoadIntegrationFixture loads a fixture for an integration step.
func (l *Loader) LoadIntegrationFixture(stepID, integrationType string) (*IntegrationFixture, error) {
	// Try step-specific fixture first
	paths := l.buildFixturePaths(stepID)
	for _, path := range paths {
		if fixture, err := l.loadIntegrationFixtureFromFile(path); err == nil {
			l.logger.Debug("[MOCK] Loaded integration fixture", "step_id", stepID, "source", path)
			return fixture, nil
		}
	}

	// Try integration-specific fixture
	integrationPaths := l.buildFixturePaths("_" + integrationType)
	for _, path := range integrationPaths {
		if fixture, err := l.loadIntegrationFixtureFromFile(path); err == nil {
			l.logger.Debug("[MOCK] Loaded integration-specific fixture", "step_id", stepID, "type", integrationType, "source", path)
			return fixture, nil
		}
	}

	// Try recorded fixture
	recordedPath := filepath.Join(l.fixturesDir, ".recorded", stepID+".yaml")
	if fixture, err := l.loadIntegrationFixtureFromFile(recordedPath); err == nil {
		l.logger.Debug("[MOCK] Loaded recorded integration fixture", "step_id", stepID, "source", recordedPath)
		return fixture, nil
	}

	// No fixture found
	return nil, fmt.Errorf("no fixture found for integration step %q (type %s), tried: %v, %v, %s",
		stepID, integrationType, paths, integrationPaths, recordedPath)
}

// buildFixturePaths builds all possible file paths for a fixture name.
// Returns paths in order: name.yaml, name.json
func (l *Loader) buildFixturePaths(name string) []string {
	if l.fixturesDir == "" {
		return nil
	}
	return []string{
		filepath.Join(l.fixturesDir, name+".yaml"),
		filepath.Join(l.fixturesDir, name+".json"),
	}
}

// loadLLMFixtureFromFile loads an LLM fixture from a file.
func (l *Loader) loadLLMFixtureFromFile(path string) (*LLMFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fixture LLMFixture

	// Try YAML first
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		// Try JSON
		if jsonErr := json.Unmarshal(data, &fixture); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse fixture as YAML or JSON: yaml=%v, json=%v", err, jsonErr)
		}
	}

	return &fixture, nil
}

// loadHTTPFixtureFromFile loads an HTTP fixture from a file.
func (l *Loader) loadHTTPFixtureFromFile(path string) (*HTTPFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fixture HTTPFixture

	// Try YAML first
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		// Try JSON
		if jsonErr := json.Unmarshal(data, &fixture); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse fixture as YAML or JSON: yaml=%v, json=%v", err, jsonErr)
		}
	}

	return &fixture, nil
}

// loadIntegrationFixtureFromFile loads an integration fixture from a file.
func (l *Loader) loadIntegrationFixtureFromFile(path string) (*IntegrationFixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var fixture IntegrationFixture

	// Try YAML first
	if err := yaml.Unmarshal(data, &fixture); err != nil {
		// Try JSON
		if jsonErr := json.Unmarshal(data, &fixture); jsonErr != nil {
			return nil, fmt.Errorf("failed to parse fixture as YAML or JSON: yaml=%v, json=%v", err, jsonErr)
		}
	}

	return &fixture, nil
}
