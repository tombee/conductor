package builtin

import (
	"fmt"

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

// IsBuiltinAPI returns true if the connector name is a built-in API connector.
func IsBuiltinAPI(name string) bool {
	_, ok := BuiltinRegistry[name]
	return ok
}

// NewBuiltinAPI creates a built-in API connector by name.
func NewBuiltinAPI(name string, config *api.ConnectorConfig) (connector.Connector, error) {
	factory, ok := BuiltinRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unknown builtin API connector: %s", name)
	}

	return factory(config)
}

// GetBuiltinAPIOperations returns the list of operations for a built-in API connector.
func GetBuiltinAPIOperations(name string) ([]api.OperationInfo, error) {
	// Create a minimal config for introspection
	config := &api.ConnectorConfig{}

	conn, err := NewBuiltinAPI(name, config)
	if err != nil {
		return nil, err
	}

	// Check if the connector implements TypedConnector interface
	if bc, ok := conn.(api.TypedConnector); ok {
		return bc.Operations(), nil
	}

	return nil, fmt.Errorf("connector %s does not implement TypedConnector interface", name)
}
