package builtin

import (
	"strings"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/connector/builtin/cloudwatch"
	"github.com/tombee/conductor/internal/connector/builtin/discord"
	"github.com/tombee/conductor/internal/connector/builtin/github"
	"github.com/tombee/conductor/internal/connector/builtin/jenkins"
	"github.com/tombee/conductor/internal/connector/builtin/jira"
	"github.com/tombee/conductor/internal/connector/builtin/slack"
)

// BuiltinRegistry holds all built-in API connector factories.
// These connectors provide type-safe, Go-based implementations with
// API-specific error handling and pagination support.
var BuiltinRegistry = map[string]func(config *api.ConnectorConfig) (operation.Connector, error){
	"github":     github.NewGitHubConnector,
	"slack":      slack.NewSlackConnector,
	"jira":       jira.NewJiraConnector,
	"discord":    discord.NewDiscordConnector,
	"jenkins":    jenkins.NewJenkinsConnector,
	"cloudwatch": cloudwatch.NewCloudWatchConnector,
}

func init() {
	// Register all builtin API connectors with the parent operation package
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
