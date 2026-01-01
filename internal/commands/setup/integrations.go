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
	"github":  &GitHubIntegrationType{},
	"slack":   &SlackIntegrationType{},
	"jira":    &JiraIntegrationType{},
	"discord": &DiscordIntegrationType{},
	"jenkins": &JenkinsIntegrationType{},
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

// Slack integration type
type SlackIntegrationType struct{}

func (s *SlackIntegrationType) ID() string             { return "slack" }
func (s *SlackIntegrationType) DisplayName() string    { return "Slack" }
func (s *SlackIntegrationType) Description() string    { return "Slack workspace integration" }
func (s *SlackIntegrationType) RequiresBaseURL() bool  { return false }
func (s *SlackIntegrationType) DefaultBaseURL() string { return "https://slack.com/api" }
func (s *SlackIntegrationType) GetFields() []IntegrationField {
	return []IntegrationField{
		{
			Name:        "bot_token",
			DisplayName: "Bot Token",
			Required:    true,
			IsSecret:    true,
			DefaultEnv:  "SLACK_BOT_TOKEN",
		},
	}
}

// Jira integration type
type JiraIntegrationType struct{}

func (j *JiraIntegrationType) ID() string             { return "jira" }
func (j *JiraIntegrationType) DisplayName() string    { return "Jira" }
func (j *JiraIntegrationType) Description() string    { return "Atlassian Jira integration" }
func (j *JiraIntegrationType) RequiresBaseURL() bool  { return true }
func (j *JiraIntegrationType) DefaultBaseURL() string { return "" }
func (j *JiraIntegrationType) GetFields() []IntegrationField {
	return []IntegrationField{
		{
			Name:        "base_url",
			DisplayName: "Jira URL",
			Required:    true,
			IsSecret:    false,
		},
		{
			Name:        "email",
			DisplayName: "Email",
			Required:    true,
			IsSecret:    false,
		},
		{
			Name:        "api_token",
			DisplayName: "API Token",
			Required:    true,
			IsSecret:    true,
			DefaultEnv:  "JIRA_API_TOKEN",
		},
	}
}

// Discord integration type
type DiscordIntegrationType struct{}

func (d *DiscordIntegrationType) ID() string             { return "discord" }
func (d *DiscordIntegrationType) DisplayName() string    { return "Discord" }
func (d *DiscordIntegrationType) Description() string    { return "Discord bot integration" }
func (d *DiscordIntegrationType) RequiresBaseURL() bool  { return false }
func (d *DiscordIntegrationType) DefaultBaseURL() string { return "https://discord.com/api" }
func (d *DiscordIntegrationType) GetFields() []IntegrationField {
	return []IntegrationField{
		{
			Name:        "bot_token",
			DisplayName: "Bot Token",
			Required:    true,
			IsSecret:    true,
			DefaultEnv:  "DISCORD_BOT_TOKEN",
		},
	}
}

// Jenkins integration type
type JenkinsIntegrationType struct{}

func (j *JenkinsIntegrationType) ID() string             { return "jenkins" }
func (j *JenkinsIntegrationType) DisplayName() string    { return "Jenkins" }
func (j *JenkinsIntegrationType) Description() string    { return "Jenkins CI/CD integration" }
func (j *JenkinsIntegrationType) RequiresBaseURL() bool  { return true }
func (j *JenkinsIntegrationType) DefaultBaseURL() string { return "" }
func (j *JenkinsIntegrationType) GetFields() []IntegrationField {
	return []IntegrationField{
		{
			Name:        "base_url",
			DisplayName: "Jenkins URL",
			Required:    true,
			IsSecret:    false,
		},
		{
			Name:        "username",
			DisplayName: "Username",
			Required:    true,
			IsSecret:    false,
		},
		{
			Name:        "api_token",
			DisplayName: "API Token",
			Required:    true,
			IsSecret:    true,
			DefaultEnv:  "JENKINS_API_TOKEN",
		},
	}
}
