package discord

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/connector"
	"github.com/tombee/conductor/internal/connector/api"
)

// DiscordConnector implements the Connector interface for Discord API.
type DiscordConnector struct {
	*api.BaseConnector
}

// NewDiscordConnector creates a new Discord connector.
func NewDiscordConnector(config *api.ConnectorConfig) (connector.Connector, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://discord.com/api/v10"
	}

	base := api.NewBaseConnector("discord", config)

	return &DiscordConnector{
		BaseConnector: base,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *DiscordConnector) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*connector.Result, error) {
	return nil, fmt.Errorf("discord connector not yet implemented")
}
