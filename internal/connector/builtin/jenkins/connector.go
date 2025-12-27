package jenkins

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/api"
)

// JenkinsConnector implements the Connector interface for Jenkins API.
type JenkinsConnector struct {
	*api.BaseConnector
}

// NewJenkinsConnector creates a new Jenkins connector.
func NewJenkinsConnector(config *api.ConnectorConfig) (connector.Connector, error) {
	if config.BaseURL == "" {
		return nil, fmt.Errorf("jenkins connector requires base_url configuration")
	}

	base := api.NewBaseConnector("jenkins", config)

	return &JenkinsConnector{
		BaseConnector: base,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *JenkinsConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*connector.Result, error) {
	return nil, fmt.Errorf("jenkins connector not yet implemented")
}
