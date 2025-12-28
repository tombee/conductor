package builtin

import (
	"strings"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/api"
	"github.com/tombee/conductor/internal/connector/builtin/discord"
	"github.com/tombee/conductor/internal/connector/builtin/github"
	"github.com/tombee/conductor/internal/connector/builtin/jenkins"
	"github.com/tombee/conductor/internal/connector/builtin/jira"
	"github.com/tombee/conductor/internal/connector/builtin/slack"
)

// BuiltinRegistry holds all built-in API connector factories.
// These connectors provide type-safe, Go-based implementations with
// API-specific error handling and pagination support.
var BuiltinRegistry = map[string]func(config *api.ConnectorConfig) (connector.Connector, error){
	"github":  github.NewGitHubConnector,
	"slack":   slack.NewSlackConnector,
	"jira":    jira.NewJiraConnector,
	"discord": discord.NewDiscordConnector,
	"jenkins": jenkins.NewJenkinsConnector,
}

func init() {
	// Register all builtin API connectors with the parent connector package
	for name := range BuiltinRegistry {
		connName := name // capture for closure
		connector.RegisterBuiltinAPI(name, func(connectorName string, baseURL string, authType string, authToken string) (connector.Connector, error) {
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
