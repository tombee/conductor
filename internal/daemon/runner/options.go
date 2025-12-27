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

package runner

import (
	"github.com/tombee/conductor/internal/binding"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/daemon/checkpoint"
	"github.com/tombee/conductor/internal/mcp"
	"github.com/tombee/conductor/pkg/tools"
)

// Option configures a Runner.
type Option func(*Runner)

// WithMCPManager sets a custom MCP manager for the runner.
// This is primarily used for testing with mock implementations.
func WithMCPManager(manager mcp.MCPManagerProvider) Option {
	return func(r *Runner) {
		// Create a new lifecycle manager with the provided MCP manager
		r.lifecycle = NewLifecycleManager(manager, r.lifecycle.checkpoints, r.lifecycle.toolRegistry)
	}
}

// WithLifecycleManager sets a custom lifecycle manager for the runner.
// This allows full control over MCP servers, checkpoints, and tool registry.
func WithLifecycleManager(lm *LifecycleManager) Option {
	return func(r *Runner) {
		r.lifecycle = lm
	}
}

// WithStateManager sets a custom state manager for the runner.
func WithStateManager(sm *StateManager) Option {
	return func(r *Runner) {
		r.state = sm
	}
}

// WithLogAggregator sets a custom log aggregator for the runner.
func WithLogAggregator(la *LogAggregator) Option {
	return func(r *Runner) {
		r.logs = la
	}
}

// WithToolRegistry sets a custom tool registry.
func WithToolRegistry(tr *tools.Registry) Option {
	return func(r *Runner) {
		// Create a new lifecycle manager with the provided tool registry
		r.lifecycle = NewLifecycleManager(r.lifecycle.mcpManager, r.lifecycle.checkpoints, tr)
	}
}

// WithCheckpointManager sets a custom checkpoint manager.
func WithCheckpointManager(cm *checkpoint.Manager) Option {
	return func(r *Runner) {
		// Create a new lifecycle manager with the provided checkpoint manager
		r.lifecycle = NewLifecycleManager(r.lifecycle.mcpManager, cm, r.lifecycle.toolRegistry)
	}
}

// WithConfig sets the daemon configuration for profile resolution (SPEC-130).
func WithConfig(cfg *config.Config) Option {
	return func(r *Runner) {
		r.config = cfg
	}
}

// WithBindingResolver sets the binding resolver for profile-based configuration (SPEC-130).
func WithBindingResolver(resolver *binding.Resolver) Option {
	return func(r *Runner) {
		r.resolver = resolver
	}
}
