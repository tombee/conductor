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

package forms

import (
	"context"
	"testing"
	"time"

	"github.com/tombee/conductor/internal/commands/setup"
)

func TestDetectCLIProvidersParallel(t *testing.T) {
	ctx := context.Background()

	// Get all provider types
	allProviders := setup.GetProviderTypes()

	// Filter CLI providers
	var cliProviders []setup.ProviderType
	for _, pt := range allProviders {
		if pt.IsCLI() {
			cliProviders = append(cliProviders, pt)
		}
	}

	if len(cliProviders) == 0 {
		t.Skip("No CLI providers registered")
	}

	t.Run("completes within timeout", func(t *testing.T) {
		start := time.Now()
		results := detectCLIProvidersParallel(ctx, cliProviders)
		duration := time.Since(start)

		// Should complete within 3 seconds (2s timeout + 1s buffer)
		if duration > 3*time.Second {
			t.Errorf("detectCLIProvidersParallel took %v, expected < 3s", duration)
		}

		// Should return results for all providers
		if len(results) != len(cliProviders) {
			t.Errorf("got %d results, want %d", len(results), len(cliProviders))
		}
	})

	t.Run("returns result for each provider", func(t *testing.T) {
		results := detectCLIProvidersParallel(ctx, cliProviders)

		// Verify each provider has a result
		providerNames := make(map[string]bool)
		for _, result := range results {
			providerNames[result.ProviderType.Name()] = true
		}

		for _, pt := range cliProviders {
			if !providerNames[pt.Name()] {
				t.Errorf("missing result for provider %s", pt.Name())
			}
		}
	})

	t.Run("detection status is set", func(t *testing.T) {
		results := detectCLIProvidersParallel(ctx, cliProviders)

		for _, result := range results {
			// Each result should have a provider type
			if result.ProviderType == nil {
				t.Error("result missing ProviderType")
			}

			// If detected, should have a path
			if result.Detected && result.Path == "" {
				t.Errorf("provider %s marked as detected but has no path", result.ProviderType.Name())
			}
		}
	})
}

func TestDetectCLIProvidersParallel_WithCancelledContext(t *testing.T) {
	// Get all provider types
	allProviders := setup.GetProviderTypes()

	// Filter CLI providers
	var cliProviders []setup.ProviderType
	for _, pt := range allProviders {
		if pt.IsCLI() {
			cliProviders = append(cliProviders, pt)
		}
	}

	if len(cliProviders) == 0 {
		t.Skip("No CLI providers registered")
	}

	t.Run("handles cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		results := detectCLIProvidersParallel(ctx, cliProviders)

		// Should still return results for all providers
		if len(results) != len(cliProviders) {
			t.Errorf("got %d results, want %d", len(results), len(cliProviders))
		}
	})
}

func TestProviderDetectionResult(t *testing.T) {
	// This is mainly to ensure the type is correctly defined
	result := ProviderDetectionResult{
		ProviderType: nil,
		Detected:     false,
		Path:         "",
		Error:        nil,
	}

	if result.Detected {
		t.Error("expected Detected to be false")
	}
}
