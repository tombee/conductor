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

package sandbox

import (
	"context"
	"fmt"
)

// FactorySelector provides intelligent sandbox factory selection.
type FactorySelector struct {
	factories []Factory
}

// NewFactorySelector creates a factory selector with default factories.
func NewFactorySelector() *FactorySelector {
	return &FactorySelector{
		factories: []Factory{
			NewDockerFactory(),
			NewFallbackFactory(),
		},
	}
}

// SelectFactory chooses the best available factory.
//
// Returns:
//   - Factory: The selected factory
//   - bool: True if the selected factory is degraded (fallback)
//   - error: If no factory is available
func (s *FactorySelector) SelectFactory(ctx context.Context) (Factory, bool, error) {
	for _, factory := range s.factories {
		if factory.Available(ctx) {
			// Docker/Podman is not degraded
			degraded := factory.Type() == TypeFallback
			return factory, degraded, nil
		}
	}

	return nil, false, fmt.Errorf("no sandbox factory available")
}

// GetDegradedModeWarning returns a user-friendly warning about degraded mode.
func GetDegradedModeWarning(profileName string) string {
	return fmt.Sprintf(`WARNING: Running in degraded sandbox mode

Profile: %s (requires container isolation)
Status:  Container runtime (Docker/Podman) not available

Security Impact:
  ✓ Filesystem allowlists still enforced
  ✓ Network allowlists still enforced
  ✓ Command allowlists still enforced
  ✗ No memory/CPU resource limits
  ✗ No process isolation
  ✗ No network isolation

Recommendation:
  Install Docker or Podman for full isolation.

  macOS:   brew install --cask docker
  Linux:   apt-get install docker.io  (or use podman)

For more information: https://conductor.dev/docs/security/sandbox

`, profileName)
}

// GetAvailableSandboxTypes returns information about available sandbox types.
func GetAvailableSandboxTypes(ctx context.Context) map[Type]bool {
	types := make(map[Type]bool)

	dockerFactory := NewDockerFactory()
	types[TypeDocker] = dockerFactory.Available(ctx)

	fallbackFactory := NewFallbackFactory()
	types[TypeFallback] = fallbackFactory.Available(ctx)

	return types
}
