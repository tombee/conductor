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

// Package endpoint provides named API endpoints that expose workflows.
package endpoint

import (
	"fmt"
	"sync"
	"time"
)

// Endpoint defines an API endpoint that exposes a workflow.
type Endpoint struct {
	// Name is the unique identifier for this endpoint
	Name string `yaml:"name" json:"name"`

	// Description provides documentation for this endpoint
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Workflow is the workflow file to execute (relative to workflows_dir)
	Workflow string `yaml:"workflow" json:"workflow"`

	// Inputs are default inputs merged with caller-provided inputs
	Inputs map[string]any `yaml:"inputs,omitempty" json:"inputs,omitempty"`

	// Scopes defines which API key scopes can call this endpoint
	// Empty list means all keys can call (no restriction)
	Scopes []string `yaml:"scopes,omitempty" json:"scopes,omitempty"`

	// RateLimit specifies request limit (e.g., "100/hour", "10/minute")
	// Empty means no endpoint-specific rate limit
	RateLimit string `yaml:"rate_limit,omitempty" json:"rate_limit,omitempty"`

	// Timeout is the maximum execution time for this endpoint
	// Zero means use controller default
	Timeout time.Duration `yaml:"timeout,omitempty" json:"timeout,omitempty"`

	// Public indicates this endpoint requires no authentication
	// Default: false (authentication required)
	Public bool `yaml:"public,omitempty" json:"public,omitempty"`
}

// Registry manages a collection of endpoints with thread-safe access.
type Registry struct {
	mu        sync.RWMutex
	endpoints map[string]*Endpoint
}

// NewRegistry creates a new endpoint registry.
func NewRegistry() *Registry {
	return &Registry{
		endpoints: make(map[string]*Endpoint),
	}
}

// Add adds an endpoint to the registry.
// Returns an error if an endpoint with the same name already exists.
func (r *Registry) Add(ep *Endpoint) error {
	if ep == nil {
		return fmt.Errorf("endpoint cannot be nil")
	}
	if ep.Name == "" {
		return fmt.Errorf("endpoint name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.endpoints[ep.Name]; exists {
		return fmt.Errorf("endpoint %q already exists", ep.Name)
	}

	r.endpoints[ep.Name] = ep
	return nil
}

// Get retrieves an endpoint by name.
// Returns nil if the endpoint doesn't exist.
func (r *Registry) Get(name string) *Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.endpoints[name]
}

// List returns all registered endpoints.
func (r *Registry) List() []*Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	endpoints := make([]*Endpoint, 0, len(r.endpoints))
	for _, ep := range r.endpoints {
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

// Remove removes an endpoint by name.
// Returns an error if the endpoint doesn't exist.
func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.endpoints[name]; !exists {
		return fmt.Errorf("endpoint %q not found", name)
	}

	delete(r.endpoints, name)
	return nil
}

// Update updates an existing endpoint.
// Returns an error if the endpoint doesn't exist.
func (r *Registry) Update(ep *Endpoint) error {
	if ep == nil {
		return fmt.Errorf("endpoint cannot be nil")
	}
	if ep.Name == "" {
		return fmt.Errorf("endpoint name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.endpoints[ep.Name]; !exists {
		return fmt.Errorf("endpoint %q not found", ep.Name)
	}

	r.endpoints[ep.Name] = ep
	return nil
}

// Count returns the number of registered endpoints.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.endpoints)
}
