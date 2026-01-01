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

package setup

// IntegrationType defines the interface for integration type definitions.
type IntegrationType interface {
	// ID returns the integration type ID (e.g., "github", "slack")
	ID() string

	// DisplayName returns the human-readable name
	DisplayName() string

	// Description returns a short description
	Description() string

	// RequiresBaseURL returns true if this integration requires a base URL
	RequiresBaseURL() bool

	// DefaultBaseURL returns the default base URL (empty if not applicable)
	DefaultBaseURL() string

	// GetFields returns the list of configuration fields
	GetFields() []IntegrationField
}

// IntegrationField defines a configuration field for an integration
type IntegrationField struct {
	Name        string
	DisplayName string
	Required    bool
	IsSecret    bool
	DefaultEnv  string
}

// integrationRegistry holds all registered integration types
var integrationRegistry = map[string]IntegrationType{
	// Integration types will be added once integration config is implemented
	// For now, this is empty to avoid compilation errors
}

// GetIntegrationTypes returns all registered integration types
func GetIntegrationTypes() []IntegrationType {
	types := make([]IntegrationType, 0, len(integrationRegistry))
	for _, it := range integrationRegistry {
		types = append(types, it)
	}
	return types
}

// GetIntegrationType returns an integration type by ID
func GetIntegrationType(id string) (IntegrationType, bool) {
	it, ok := integrationRegistry[id]
	return it, ok
}

// GitHub integration type implementation (placeholder)
type GitHubIntegrationType struct{}

func (g *GitHubIntegrationType) ID() string             { return "github" }
func (g *GitHubIntegrationType) DisplayName() string    { return "GitHub" }
func (g *GitHubIntegrationType) Description() string    { return "GitHub API integration" }
func (g *GitHubIntegrationType) RequiresBaseURL() bool  { return false } // Optional for enterprise
func (g *GitHubIntegrationType) DefaultBaseURL() string { return "https://api.github.com" }
func (g *GitHubIntegrationType) GetFields() []IntegrationField {
	return []IntegrationField{
		{
			Name:        "token",
			DisplayName: "Access Token",
			Required:    true,
			IsSecret:    true,
			DefaultEnv:  "GITHUB_TOKEN",
		},
		{
			Name:        "base_url",
			DisplayName: "Base URL (for Enterprise)",
			Required:    false,
			IsSecret:    false,
		},
	}
}
