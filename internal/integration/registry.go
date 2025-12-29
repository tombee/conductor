package integration

import (
	"strings"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/integration/cloudwatch"
	"github.com/tombee/conductor/internal/integration/datadog"
	"github.com/tombee/conductor/internal/integration/discord"
	"github.com/tombee/conductor/internal/integration/elasticsearch"
	"github.com/tombee/conductor/internal/integration/github"
	"github.com/tombee/conductor/internal/integration/jenkins"
	"github.com/tombee/conductor/internal/integration/jira"
	"github.com/tombee/conductor/internal/integration/loki"
	"github.com/tombee/conductor/internal/integration/slack"
	"github.com/tombee/conductor/internal/integration/splunk"
)

// BuiltinRegistry holds all built-in API integration factories.
// These integrations provide type-safe, Go-based implementations with
// API-specific error handling and pagination support.
var BuiltinRegistry = map[string]func(config *api.ConnectorConfig) (operation.Connector, error){
	"github":        github.NewGitHubIntegration,
	"slack":         slack.NewSlackIntegration,
	"jira":          jira.NewJiraIntegration,
	"discord":       discord.NewDiscordIntegration,
	"jenkins":       jenkins.NewJenkinsIntegration,
	"cloudwatch":    cloudwatch.NewCloudWatchIntegration,
	"datadog":       datadog.NewDatadogIntegration,
	"elasticsearch": elasticsearch.NewElasticsearchIntegration,
	"loki":          loki.NewLokiIntegration,
	"splunk":        splunk.NewSplunkIntegration,
}

func init() {
	// Register all builtin API integrations with the parent operation package
	for name := range BuiltinRegistry {
		connName := name // capture for closure
		operation.RegisterBuiltinAPI(name, func(connectorName string, baseURL string, authType string, authToken string) (operation.Connector, error) {
			config := &api.ConnectorConfig{
				BaseURL: baseURL,
				Token:   authToken,
			}

			// Handle basic auth by parsing username:password
			if authType == "basic" && authToken != "" {
				parts := splitOnce(authToken, ":")
				if len(parts) == 2 {
					config.AdditionalAuth = map[string]string{
						"email":    parts[0], // For Jira, username is email
						"username": parts[0], // For Jenkins
						"password": parts[1],
					}
					config.Token = parts[1] // API token is the password part
				}
			}
			return BuiltinRegistry[connName](config)
		})
	}
}

// splitOnce splits a string on the first occurrence of sep.
func splitOnce(s, sep string) []string {
	idx := strings.Index(s, sep)
	if idx == -1 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+len(sep):]}
}
