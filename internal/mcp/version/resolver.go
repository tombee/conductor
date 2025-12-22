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

package version

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// ResolvedVersion represents a resolved package version.
type ResolvedVersion struct {
	// Source is the package source (e.g., "npm:@modelcontextprotocol/server-github")
	Source string `json:"source" yaml:"source"`

	// Version is the resolved version string
	Version string `json:"version" yaml:"version"`

	// Integrity is a hash of the package content for verification
	Integrity string `json:"integrity,omitempty" yaml:"integrity,omitempty"`

	// Command is the command to run this version
	Command string `json:"command,omitempty" yaml:"command,omitempty"`

	// Args are the arguments to pass to the command
	Args []string `json:"args,omitempty" yaml:"args,omitempty"`
}

// Resolver resolves package versions.
type Resolver interface {
	// Name returns the resolver name (e.g., "npm", "pypi", "local")
	Name() string

	// Resolve resolves a package source with constraints to a specific version
	Resolve(ctx context.Context, source string, constraint string) (*ResolvedVersion, error)

	// ListVersions lists available versions for a package
	ListVersions(ctx context.Context, source string) ([]string, error)
}

// ResolverRegistry manages package resolvers.
type ResolverRegistry struct {
	resolvers map[string]Resolver
}

// NewResolverRegistry creates a new resolver registry.
func NewResolverRegistry() *ResolverRegistry {
	return &ResolverRegistry{
		resolvers: make(map[string]Resolver),
	}
}

// Register registers a resolver for a source type.
func (r *ResolverRegistry) Register(resolver Resolver) {
	r.resolvers[resolver.Name()] = resolver
}

// GetResolver returns the resolver for a source type.
func (r *ResolverRegistry) GetResolver(sourceType string) (Resolver, bool) {
	resolver, ok := r.resolvers[sourceType]
	return resolver, ok
}

// Resolve resolves a package source with constraints.
// The source should be in the format "type:package" (e.g., "npm:@mcp/server")
func (r *ResolverRegistry) Resolve(ctx context.Context, source string, constraint string) (*ResolvedVersion, error) {
	sourceType, packageName := parseSource(source)

	resolver, ok := r.GetResolver(sourceType)
	if !ok {
		return nil, fmt.Errorf("no resolver for source type: %s", sourceType)
	}

	return resolver.Resolve(ctx, packageName, constraint)
}

// parseSource parses a source string into type and package name.
func parseSource(source string) (sourceType, packageName string) {
	if idx := strings.Index(source, ":"); idx != -1 {
		return source[:idx], source[idx+1:]
	}
	// Default to local if no type specified
	return "local", source
}

// LocalResolver resolves local file paths.
type LocalResolver struct{}

// NewLocalResolver creates a new local resolver.
func NewLocalResolver() *LocalResolver {
	return &LocalResolver{}
}

// Name returns the resolver name.
func (r *LocalResolver) Name() string {
	return "local"
}

// Resolve resolves a local path.
func (r *LocalResolver) Resolve(ctx context.Context, source string, constraint string) (*ResolvedVersion, error) {
	// For local paths, we don't resolve versions - just use the path directly
	return &ResolvedVersion{
		Source:    "local:" + source,
		Version:   "local",
		Integrity: hashString(source), // Simple hash of path for tracking
		Command:   source,
	}, nil
}

// ListVersions lists available versions (always returns ["local"] for local resolver).
func (r *LocalResolver) ListVersions(ctx context.Context, source string) ([]string, error) {
	return []string{"local"}, nil
}

// hashString returns a SHA256 hash of a string.
func hashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return "sha256-" + hex.EncodeToString(h.Sum(nil))[:16]
}

// DefaultRegistry creates a registry with default resolvers.
func DefaultRegistry() *ResolverRegistry {
	registry := NewResolverRegistry()
	registry.Register(NewLocalResolver())
	// npm and pypi resolvers would be added here when implemented
	return registry
}
