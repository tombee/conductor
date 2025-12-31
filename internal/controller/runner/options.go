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
	"go.opentelemetry.io/otel/trace"

	"github.com/tombee/conductor/internal/binding"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/mcp"
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

// WithConfig sets the controller configuration for profile resolution.
func WithConfig(cfg *config.Config) Option {
	return func(r *Runner) {
		r.config = cfg
	}
}

// WithBindingResolver sets the binding resolver for profile-based configuration.
func WithBindingResolver(resolver *binding.Resolver) Option {
	return func(r *Runner) {
		r.resolver = resolver
	}
}

// WithWorkflowTracer sets the workflow tracer for distributed tracing.
func WithWorkflowTracer(tracer trace.Tracer) Option {
	return func(r *Runner) {
		r.mu.Lock()
		r.workflowTracer = tracer
		r.mu.Unlock()
	}
}
