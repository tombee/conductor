package slack

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/api"
)

// SlackConnector implements the Connector interface for Slack API.
type SlackConnector struct {
	*api.BaseConnector
}

// NewSlackConnector creates a new Slack connector.
func NewSlackConnector(config *api.ConnectorConfig) (connector.Connector, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://slack.com/api"
	}

	base := api.NewBaseConnector("slack", config)

	return &SlackConnector{
		BaseConnector: base,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *SlackConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*connector.Result, error) {
	return nil, fmt.Errorf("slack connector not yet implemented")
}
