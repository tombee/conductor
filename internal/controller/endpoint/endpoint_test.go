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
	"fmt"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.Count() != 0 {
		t.Errorf("expected empty registry, got count=%d", r.Count())
	}
}

func TestRegistryAdd(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  *Endpoint
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid endpoint",
			endpoint: &Endpoint{
				Name:     "test-endpoint",
				Workflow: "test.yaml",
			},
			wantErr: false,
		},
		{
			name:      "nil endpoint",
			endpoint:  nil,
			wantErr:   true,
			errSubstr: "cannot be nil",
		},
		{
			name: "empty name",
			endpoint: &Endpoint{
				Workflow: "test.yaml",
			},
			wantErr:   true,
			errSubstr: "name cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			err := r.Add(tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Errorf("Add() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errSubstr != "" {
				if !contains(err.Error(), tt.errSubstr) {
					t.Errorf("Add() error = %v, want substring %q", err, tt.errSubstr)
				}
			}
		})
	}
}

func TestRegistryAddDuplicate(t *testing.T) {
	r := NewRegistry()
	ep := &Endpoint{
		Name:     "test",
		Workflow: "test.yaml",
	}

	// First add should succeed
	if err := r.Add(ep); err != nil {
		t.Fatalf("first Add() failed: %v", err)
	}

	// Second add should fail
	err := r.Add(ep)
	if err == nil {
		t.Fatal("Add() duplicate endpoint succeeded, want error")
	}
	if !contains(err.Error(), "already exists") {
		t.Errorf("Add() error = %v, want 'already exists'", err)
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry()
	ep := &Endpoint{
		Name:        "test",
		Workflow:    "test.yaml",
		Description: "Test endpoint",
		Inputs: map[string]any{
			"key": "value",
		},
		Scopes:    []string{"scope1", "scope2"},
		RateLimit: "100/hour",
		Timeout:   5 * time.Minute,
		Public:    false,
	}

	if err := r.Add(ep); err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	// Get existing endpoint
	got := r.Get("test")
	if got == nil {
		t.Fatal("Get() returned nil for existing endpoint")
	}
	if got.Name != ep.Name {
		t.Errorf("Get() name = %v, want %v", got.Name, ep.Name)
	}
	if got.Description != ep.Description {
		t.Errorf("Get() description = %v, want %v", got.Description, ep.Description)
	}

	// Get non-existent endpoint
	notFound := r.Get("nonexistent")
	if notFound != nil {
		t.Errorf("Get() returned %v for non-existent endpoint, want nil", notFound)
	}
}

func TestRegistryList(t *testing.T) {
	r := NewRegistry()

	// Empty registry
	if list := r.List(); len(list) != 0 {
		t.Errorf("List() on empty registry = %v, want empty", list)
	}

	// Add multiple endpoints
	endpoints := []*Endpoint{
		{Name: "ep1", Workflow: "w1.yaml"},
		{Name: "ep2", Workflow: "w2.yaml"},
		{Name: "ep3", Workflow: "w3.yaml"},
	}

	for _, ep := range endpoints {
		if err := r.Add(ep); err != nil {
			t.Fatalf("Add(%s) failed: %v", ep.Name, err)
		}
	}

	list := r.List()
	if len(list) != len(endpoints) {
		t.Errorf("List() returned %d endpoints, want %d", len(list), len(endpoints))
	}

	// Verify all endpoints are in the list
	names := make(map[string]bool)
	for _, ep := range list {
		names[ep.Name] = true
	}
	for _, ep := range endpoints {
		if !names[ep.Name] {
			t.Errorf("List() missing endpoint %s", ep.Name)
		}
	}
}

func TestRegistryRemove(t *testing.T) {
	r := NewRegistry()
	ep := &Endpoint{
		Name:     "test",
		Workflow: "test.yaml",
	}

	// Remove from empty registry
	err := r.Remove("test")
	if err == nil {
		t.Error("Remove() on empty registry succeeded, want error")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Remove() error = %v, want 'not found'", err)
	}

	// Add and remove
	if err := r.Add(ep); err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	if err := r.Remove("test"); err != nil {
		t.Errorf("Remove() failed: %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("Count() after Remove() = %d, want 0", r.Count())
	}

	if got := r.Get("test"); got != nil {
		t.Errorf("Get() after Remove() = %v, want nil", got)
	}
}

func TestRegistryUpdate(t *testing.T) {
	r := NewRegistry()
	original := &Endpoint{
		Name:        "test",
		Workflow:    "test.yaml",
		Description: "Original",
	}

	// Update non-existent endpoint
	err := r.Update(original)
	if err == nil {
		t.Error("Update() on non-existent endpoint succeeded, want error")
	}
	if !contains(err.Error(), "not found") {
		t.Errorf("Update() error = %v, want 'not found'", err)
	}

	// Add original
	if err := r.Add(original); err != nil {
		t.Fatalf("Add() failed: %v", err)
	}

	// Update with new values
	updated := &Endpoint{
		Name:        "test",
		Workflow:    "updated.yaml",
		Description: "Updated",
		Inputs: map[string]any{
			"new": "value",
		},
	}

	if err := r.Update(updated); err != nil {
		t.Errorf("Update() failed: %v", err)
	}

	// Verify update
	got := r.Get("test")
	if got.Description != "Updated" {
		t.Errorf("After Update() description = %v, want 'Updated'", got.Description)
	}
	if got.Workflow != "updated.yaml" {
		t.Errorf("After Update() workflow = %v, want 'updated.yaml'", got.Workflow)
	}
}

func TestRegistryUpdateNil(t *testing.T) {
	r := NewRegistry()
	err := r.Update(nil)
	if err == nil {
		t.Error("Update(nil) succeeded, want error")
	}
	if !contains(err.Error(), "cannot be nil") {
		t.Errorf("Update(nil) error = %v, want 'cannot be nil'", err)
	}
}

func TestRegistryUpdateEmptyName(t *testing.T) {
	r := NewRegistry()
	ep := &Endpoint{
		Workflow: "test.yaml",
	}
	err := r.Update(ep)
	if err == nil {
		t.Error("Update() with empty name succeeded, want error")
	}
	if !contains(err.Error(), "name cannot be empty") {
		t.Errorf("Update() error = %v, want 'name cannot be empty'", err)
	}
}

func TestRegistryCount(t *testing.T) {
	r := NewRegistry()

	if r.Count() != 0 {
		t.Errorf("Count() on new registry = %d, want 0", r.Count())
	}

	for i := 1; i <= 5; i++ {
		ep := &Endpoint{
			Name:     fmt.Sprintf("ep%d", i),
			Workflow: "test.yaml",
		}
		if err := r.Add(ep); err != nil {
			t.Fatalf("Add() failed: %v", err)
		}

		if r.Count() != i {
			t.Errorf("Count() after adding %d endpoints = %d, want %d", i, r.Count(), i)
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
