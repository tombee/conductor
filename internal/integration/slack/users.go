package slack

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// listUsers lists workspace members.
func (c *SlackIntegration) listUsers(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build URL
	url, err := c.BuildURL("/users.list", inputs)
	if err != nil {
		return nil, err
	}

	// Build query string from inputs (cursor, limit)
	queryString := c.BuildQueryString(inputs, nil)
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
	var listResp ListUsersResponse
	if err := c.ParseJSONResponse(resp, &listResp); err != nil {
		return nil, err
	}

	// Transform to simplified format
	users := make([]map[string]interface{}, len(listResp.Members))
	for i, user := range listResp.Members {
		users[i] = map[string]interface{}{
			"id":        user.ID,
			"name":      user.Name,
			"real_name": user.RealName,
			"is_bot":    user.IsBot,
			"deleted":   user.Deleted,
		}
		if user.Profile.Email != "" {
			users[i]["email"] = user.Profile.Email
		}
	}

	result := map[string]interface{}{
		"users": users,
		"ok":    listResp.OK,
	}

	// Include next cursor if available for pagination
	if listResp.ResponseMetadata.NextCursor != "" {
		result["next_cursor"] = listResp.ResponseMetadata.NextCursor
	}

	return c.ToConnectorResult(resp, result), nil
}

// getUser gets information about a specific user.
func (c *SlackIntegration) getUser(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"user"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/users.info", inputs)
	if err != nil {
		return nil, err
	}

	// Build query string from inputs
	queryString := c.BuildQueryString(inputs, nil)
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
	var userResp GetUserResponse
	if err := c.ParseJSONResponse(resp, &userResp); err != nil {
		return nil, err
	}

	// Return connector result
	result := map[string]interface{}{
		"id":        userResp.User.ID,
		"name":      userResp.User.Name,
		"real_name": userResp.User.RealName,
		"is_bot":    userResp.User.IsBot,
		"deleted":   userResp.User.Deleted,
		"ok":        userResp.OK,
	}

	if userResp.User.Profile.Email != "" {
		result["email"] = userResp.User.Profile.Email
	}
	if userResp.User.Profile.DisplayName != "" {
		result["display_name"] = userResp.User.Profile.DisplayName
	}

	return c.ToConnectorResult(resp, result), nil
}
