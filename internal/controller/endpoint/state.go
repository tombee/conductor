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

package endpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// State manages persistence of endpoint configuration.
type State struct {
	path string
}

// NewState creates a new state manager.
// The state file is stored in the conductor state directory.
func NewState(stateDir string) *State {
	return &State{
		path: filepath.Join(stateDir, "endpoints.json"),
	}
}

// Load loads endpoints from the state file.
// Returns an empty list if the file doesn't exist.
func (s *State) Load() ([]*Endpoint, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Endpoint{}, nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var endpoints []*Endpoint
	if err := json.Unmarshal(data, &endpoints); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return endpoints, nil
}

// Save persists endpoints to the state file.
func (s *State) Save(endpoints []*Endpoint) error {
	// Ensure directory exists
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Marshal endpoints to JSON
	data, err := json.MarshalIndent(endpoints, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal endpoints: %w", err)
	}

	// Write to temporary file first
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath) // cleanup temp file
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// Path returns the path to the state file.
func (s *State) Path() string {
	return s.path
}
