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

package secrets

import (
	"context"
	"errors"
	"testing"
)

// mockBackend is a test implementation of SecretBackend
type mockBackend struct {
	name      string
	priority  int
	available bool
	readOnly  bool
	secrets   map[string]string
}

func newMockBackend(name string, priority int) *mockBackend {
	return &mockBackend{
		name:      name,
		priority:  priority,
		available: true,
		readOnly:  false,
		secrets:   make(map[string]string),
	}
}

func (m *mockBackend) Name() string {
	return m.name
}

func (m *mockBackend) Get(ctx context.Context, key string) (string, error) {
	if value, ok := m.secrets[key]; ok {
		return value, nil
	}
	return "", ErrSecretNotFound
}

func (m *mockBackend) Set(ctx context.Context, key string, value string) error {
	if m.readOnly {
		return ErrReadOnlyBackend
	}
	m.secrets[key] = value
	return nil
}

func (m *mockBackend) Delete(ctx context.Context, key string) error {
	if m.readOnly {
		return ErrReadOnlyBackend
	}
	if _, ok := m.secrets[key]; !ok {
		return ErrSecretNotFound
	}
	delete(m.secrets, key)
	return nil
}

func (m *mockBackend) List(ctx context.Context) ([]string, error) {
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}

func (m *mockBackend) Available() bool {
	return m.available
}

func (m *mockBackend) Priority() int {
	return m.priority
}

func (m *mockBackend) ReadOnly() bool {
	return m.readOnly
}

func TestResolver_Get(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name      string
		backends  []SecretBackend
		key       string
		wantValue string
		wantErr   error
	}{
		{
			name: "get from high priority backend",
			backends: func() []SecretBackend {
				high := newMockBackend("high", 100)
				high.secrets["test"] = "high-value"
				low := newMockBackend("low", 50)
				low.secrets["test"] = "low-value"
				return []SecretBackend{low, high}
			}(),
			key:       "test",
			wantValue: "high-value",
			wantErr:   nil,
		},
		{
			name: "fallback to lower priority",
			backends: func() []SecretBackend {
				high := newMockBackend("high", 100)
				low := newMockBackend("low", 50)
				low.secrets["test"] = "low-value"
				return []SecretBackend{high, low}
			}(),
			key:       "test",
			wantValue: "low-value",
			wantErr:   nil,
		},
		{
			name: "secret not found",
			backends: func() []SecretBackend {
				return []SecretBackend{newMockBackend("test", 100)}
			}(),
			key:       "missing",
			wantValue: "",
			wantErr:   ErrSecretNotFound,
		},
		{
			name:      "no backends available",
			backends:  []SecretBackend{},
			key:       "test",
			wantValue: "",
			wantErr:   ErrBackendUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewResolver(tt.backends...)
			got, err := resolver.Get(ctx, tt.key)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Get() unexpected error = %v", err)
				return
			}

			if got != tt.wantValue {
				t.Errorf("Get() = %v, want %v", got, tt.wantValue)
			}
		})
	}
}

func TestResolver_Set(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		backends    []SecretBackend
		key         string
		value       string
		backendName string
		wantErr     bool
		checkFunc   func(t *testing.T, backends []SecretBackend)
	}{
		{
			name: "set in first writable backend",
			backends: func() []SecretBackend {
				ro := newMockBackend("readonly", 100)
				ro.readOnly = true
				rw := newMockBackend("writable", 50)
				return []SecretBackend{ro, rw}
			}(),
			key:         "test",
			value:       "value",
			backendName: "",
			wantErr:     false,
			checkFunc: func(t *testing.T, backends []SecretBackend) {
				// Check that it was written to the writable backend
				rw := backends[1].(*mockBackend)
				if val, ok := rw.secrets["test"]; !ok || val != "value" {
					t.Errorf("Secret not set in writable backend")
				}
			},
		},
		{
			name: "set in specific backend",
			backends: func() []SecretBackend {
				b1 := newMockBackend("backend1", 100)
				b2 := newMockBackend("backend2", 50)
				return []SecretBackend{b1, b2}
			}(),
			key:         "test",
			value:       "value",
			backendName: "backend2",
			wantErr:     false,
			checkFunc: func(t *testing.T, backends []SecretBackend) {
				b2 := backends[1].(*mockBackend)
				if val, ok := b2.secrets["test"]; !ok || val != "value" {
					t.Errorf("Secret not set in backend2")
				}
				b1 := backends[0].(*mockBackend)
				if _, ok := b1.secrets["test"]; ok {
					t.Errorf("Secret incorrectly set in backend1")
				}
			},
		},
		{
			name: "backend not found",
			backends: func() []SecretBackend {
				return []SecretBackend{newMockBackend("test", 100)}
			}(),
			key:         "test",
			value:       "value",
			backendName: "missing",
			wantErr:     true,
		},
		{
			name: "no writable backends",
			backends: func() []SecretBackend {
				ro := newMockBackend("readonly", 100)
				ro.readOnly = true
				return []SecretBackend{ro}
			}(),
			key:         "test",
			value:       "value",
			backendName: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewResolver(tt.backends...)
			err := resolver.Set(ctx, tt.key, tt.value, tt.backendName)

			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkFunc != nil && !tt.wantErr {
				tt.checkFunc(t, tt.backends)
			}
		})
	}
}

func TestResolver_Delete(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		backends    []SecretBackend
		key         string
		backendName string
		wantErr     error
	}{
		{
			name: "delete from specific backend",
			backends: func() []SecretBackend {
				b := newMockBackend("test", 100)
				b.secrets["key"] = "value"
				return []SecretBackend{b}
			}(),
			key:         "key",
			backendName: "test",
			wantErr:     nil,
		},
		{
			name: "delete from all writable backends",
			backends: func() []SecretBackend {
				b1 := newMockBackend("b1", 100)
				b1.secrets["key"] = "value1"
				b2 := newMockBackend("b2", 50)
				b2.secrets["key"] = "value2"
				return []SecretBackend{b1, b2}
			}(),
			key:         "key",
			backendName: "",
			wantErr:     nil,
		},
		{
			name: "key not found",
			backends: func() []SecretBackend {
				return []SecretBackend{newMockBackend("test", 100)}
			}(),
			key:         "missing",
			backendName: "",
			wantErr:     ErrSecretNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewResolver(tt.backends...)
			err := resolver.Delete(ctx, tt.key, tt.backendName)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Delete() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Delete() unexpected error = %v", err)
			}
		})
	}
}

func TestResolver_List(t *testing.T) {
	ctx := context.Background()

	// Set up backends with overlapping keys
	high := newMockBackend("high", 100)
	high.secrets["key1"] = "high1"
	high.secrets["key2"] = "high2"

	low := newMockBackend("low", 50)
	low.secrets["key2"] = "low2" // Overlaps with high
	low.secrets["key3"] = "low3"

	resolver := NewResolver(high, low)
	metadata, err := resolver.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	// Should have 3 keys total
	if len(metadata) != 3 {
		t.Errorf("List() returned %d keys, want 3", len(metadata))
	}

	// Check that key2 is attributed to high priority backend
	for _, m := range metadata {
		if m.Key == "key2" && m.Backend != "high" {
			t.Errorf("key2 backend = %v, want high", m.Backend)
		}
	}
}

func TestResolver_FilterUnavailableBackends(t *testing.T) {
	available := newMockBackend("available", 100)
	unavailable := newMockBackend("unavailable", 50)
	unavailable.available = false

	resolver := NewResolver(available, unavailable)

	backends := resolver.Backends()
	if len(backends) != 1 {
		t.Errorf("Backends() returned %d, want 1", len(backends))
	}

	if backends[0].Name() != "available" {
		t.Errorf("Backends()[0].Name() = %v, want available", backends[0].Name())
	}
}

func TestResolver_SortsByPriority(t *testing.T) {
	low := newMockBackend("low", 25)
	medium := newMockBackend("medium", 50)
	high := newMockBackend("high", 100)

	// Pass in random order
	resolver := NewResolver(low, high, medium)

	backends := resolver.Backends()
	if len(backends) != 3 {
		t.Fatalf("Backends() returned %d, want 3", len(backends))
	}

	// Should be sorted high to low
	if backends[0].Name() != "high" {
		t.Errorf("Backends()[0].Name() = %v, want high", backends[0].Name())
	}
	if backends[1].Name() != "medium" {
		t.Errorf("Backends()[1].Name() = %v, want medium", backends[1].Name())
	}
	if backends[2].Name() != "low" {
		t.Errorf("Backends()[2].Name() = %v, want low", backends[2].Name())
	}
}
