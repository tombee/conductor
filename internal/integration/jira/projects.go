package jira

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// listProjects retrieves all projects accessible to the user.
func (c *JiraIntegration) listProjects(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build URL
	url, err := c.BuildURL("/project", inputs)
	if err != nil {
		return nil, err
	}

	// Add optional query parameters
	pathParams := []string{}
	queryString := c.BuildQueryString(inputs, pathParams)
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
	var projects []Project
	if err := c.ParseJSONResponse(resp, &projects); err != nil {
		return nil, err
	}

	// Transform to simplified format
	result := make([]map[string]interface{}, len(projects))
	for i, project := range projects {
		result[i] = map[string]interface{}{
			"id":   project.ID,
			"key":  project.Key,
			"name": project.Name,
			"self": project.Self,
		}
	}

	return c.ToConnectorResult(resp, result), nil
}
