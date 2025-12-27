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
	"fmt"

	"github.com/tombee/conductor/pkg/profile"
)

// TestEnvProvider is a test-only environment variable provider that uses an in-memory map
// instead of the process environment. This allows tests to be isolated and repeatable.
type TestEnvProvider struct {
	env map[string]string
}

// NewTestEnvProvider creates a new test environment provider with the given variables.
func NewTestEnvProvider(env map[string]string) *TestEnvProvider {
	return &TestEnvProvider{
		env: env,
	}
}

// Scheme returns the provider's URI scheme identifier.
func (t *TestEnvProvider) Scheme() string {
	return "env"
}

// Resolve retrieves a secret value from the in-memory environment map.
func (t *TestEnvProvider) Resolve(ctx context.Context, reference string) (string, error) {
	value, exists := t.env[reference]
	if !exists {
		return "", profile.NewSecretResolutionError(
			profile.ErrorCategoryNotFound,
			"env:"+reference,
			"env",
			"environment variable not set",
			nil,
		)
	}
	return value, nil
}

// PlainProvider is a test provider that returns values as-is (for "plain" scheme).
// This is used internally by the registry for non-secret values.
type PlainProvider struct{}

// NewPlainProvider creates a new plain provider.
func NewPlainProvider() *PlainProvider {
	return &PlainProvider{}
}

// Scheme returns the provider's URI scheme identifier.
func (p *PlainProvider) Scheme() string {
	return "plain"
}

// Resolve returns the reference as-is without any transformation.
func (p *PlainProvider) Resolve(ctx context.Context, reference string) (string, error) {
	if reference == "" {
		return "", fmt.Errorf("empty plain reference")
	}
	return reference, nil
}
