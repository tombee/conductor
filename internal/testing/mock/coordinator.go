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

package mock

import (
	"log/slog"

	"github.com/tombee/conductor/internal/testing/fixture"
	"github.com/tombee/conductor/pkg/workflow"
)

// Coordinator orchestrates mock providers for testing workflows.
// It wraps both LLM providers and operation registries to intercept
// calls and return fixture-based responses.
type Coordinator struct {
	fixtureLoader *fixture.Loader
	logger        *slog.Logger

	// Wrapped real providers for hybrid mode (--record)
	realLLMProvider       workflow.LLMProvider
	realOperationRegistry workflow.OperationRegistry
}

// Config holds configuration for creating a MockCoordinator.
type Config struct {
	// FixturesDir is the directory containing fixture files
	FixturesDir string

	// Logger for mock operations (will log with [MOCK] prefix)
	Logger *slog.Logger

	// Real providers for hybrid mode (optional)
	RealLLMProvider       workflow.LLMProvider
	RealOperationRegistry workflow.OperationRegistry
}

// NewCoordinator creates a new mock coordinator with the given configuration.
func NewCoordinator(cfg Config) (*Coordinator, error) {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	loader, err := fixture.NewLoader(cfg.FixturesDir, cfg.Logger)
	if err != nil {
		return nil, err
	}

	return &Coordinator{
		fixtureLoader:         loader,
		logger:                cfg.Logger,
		realLLMProvider:       cfg.RealLLMProvider,
		realOperationRegistry: cfg.RealOperationRegistry,
	}, nil
}

// LLMProvider returns a mock LLM provider that uses fixtures.
func (c *Coordinator) LLMProvider() workflow.LLMProvider {
	return NewLLMProvider(c.fixtureLoader, c.realLLMProvider, c.logger)
}

// OperationRegistry returns a mock operation registry that uses fixtures.
func (c *Coordinator) OperationRegistry() workflow.OperationRegistry {
	return NewOperationRegistry(c.fixtureLoader, c.realOperationRegistry, c.logger)
}
