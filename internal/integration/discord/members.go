package discord

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// listMembers lists members in a Discord guild.
// Supports snowflake-based pagination with the "after" parameter.
func (c *DiscordIntegration) listMembers(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"guild_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/guilds/{guild_id}/members", inputs)
	if err != nil {
		return nil, err
	}

	// Add query string for pagination (limit, after)
	queryString := c.BuildQueryString(inputs, []string{"guild_id"})
	url += queryString

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var members []Member
	if err := c.ParseJSONResponse(resp, &members); err != nil {
		return nil, err
	}

	// Convert to simplified response
	memberList := make([]map[string]interface{}, len(members))
	for i, m := range members {
		userData := map[string]interface{}{}
		if m.User != nil {
			userData = map[string]interface{}{
				"id":       m.User.ID,
				"username": m.User.Username,
				"bot":      m.User.Bot,
			}
		}

		memberList[i] = map[string]interface{}{
			"user":      userData,
			"nick":      m.Nick,
			"roles":     m.Roles,
			"joined_at": m.JoinedAt,
		}
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"members": memberList,
	}), nil
}

// getMember gets details about a specific guild member.
func (c *DiscordIntegration) getMember(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"guild_id", "user_id"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/guilds/{guild_id}/members/{user_id}", inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var member Member
	if err := c.ParseJSONResponse(resp, &member); err != nil {
		return nil, err
	}

	// Build user data
	userData := map[string]interface{}{}
	if member.User != nil {
		userData = map[string]interface{}{
			"id":       member.User.ID,
			"username": member.User.Username,
			"bot":      member.User.Bot,
		}
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"user":      userData,
		"nick":      member.Nick,
		"roles":     member.Roles,
		"joined_at": member.JoinedAt,
	}), nil
}
