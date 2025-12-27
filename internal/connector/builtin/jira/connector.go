package jira

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/api"
)

// JiraConnector implements the Connector interface for Jira API.
type JiraConnector struct {
	*api.BaseConnector
}

// NewJiraConnector creates a new Jira connector.
func NewJiraConnector(config *api.ConnectorConfig) (connector.Connector, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("jira connector requires base_url configuration")
	}

	base := api.NewBaseConnector("jira", config)

	return &JiraConnector{
		BaseConnector: base,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *JiraConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*connector.Result, error) {
	return nil, fmt.Errorf("jira connector not yet implemented")
}
