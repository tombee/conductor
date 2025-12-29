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
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestState_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	endpoints := []*Endpoint{
		{
			Name:        "test1",
			Workflow:    "w1.yaml",
			Description: "Test 1",
			Inputs: map[string]any{
				"key": "value",
			},
			Scopes:    []string{"scope1"},
			RateLimit: "10/minute",
			Timeout:   5 * time.Minute,
		},
		{
			Name:     "test2",
			Workflow: "w2.yaml",
			Public:   true,
		},
	}

	// Save
	if err := state.Save(endpoints); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(state.Path()); os.IsNotExist(err) {
		t.Error("state file was not created")
	}

	// Load
	loaded, err := state.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(loaded))
	}

	// Verify first endpoint
	ep1 := loaded[0]
	if ep1.Name != "test1" {
		t.Errorf("expected name test1, got %s", ep1.Name)
	}
	if ep1.Workflow != "w1.yaml" {
		t.Errorf("expected workflow w1.yaml, got %s", ep1.Workflow)
	}
	if ep1.Description != "Test 1" {
		t.Errorf("expected description 'Test 1', got %s", ep1.Description)
	}
	if len(ep1.Inputs) != 1 {
		t.Errorf("expected 1 input, got %d", len(ep1.Inputs))
	}
	if len(ep1.Scopes) != 1 {
		t.Errorf("expected 1 scope, got %d", len(ep1.Scopes))
	}
	if ep1.RateLimit != "10/minute" {
		t.Errorf("expected rate limit 10/minute, got %s", ep1.RateLimit)
	}

	// Verify second endpoint
	ep2 := loaded[1]
	if ep2.Name != "test2" {
		t.Errorf("expected name test2, got %s", ep2.Name)
	}
	if !ep2.Public {
		t.Error("expected public to be true")
	}
}

func TestState_LoadNonexistent(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	endpoints, err := state.Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(endpoints) != 0 {
		t.Errorf("expected empty list, got %d endpoints", len(endpoints))
	}
}

func TestState_SaveCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "nested", "state")
	state := NewState(subdir)

	endpoints := []*Endpoint{
		{Name: "test", Workflow: "w.yaml"},
	}

	if err := state.Save(endpoints); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Verify nested directory was created
	if _, err := os.Stat(subdir); os.IsNotExist(err) {
		t.Error("state directory was not created")
	}
}

func TestState_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	state := NewState(dir)

	// Save initial state
	initial := []*Endpoint{{Name: "ep1", Workflow: "w1.yaml"}}
	if err := state.Save(initial); err != nil {
		t.Fatalf("failed to save initial: %v", err)
	}

	// Save updated state
	updated := []*Endpoint{{Name: "ep2", Workflow: "w2.yaml"}}
	if err := state.Save(updated); err != nil {
		t.Fatalf("failed to save updated: %v", err)
	}

	// Verify no temp file left behind
	tmpPath := state.Path() + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temporary file was not cleaned up")
	}

	// Load and verify updated state
	loaded, err := state.Load()
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if len(loaded) != 1 || loaded[0].Name != "ep2" {
		t.Error("state was not updated correctly")
	}
}
