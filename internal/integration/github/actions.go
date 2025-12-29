package github

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// getWorkflowRuns lists GitHub Actions workflow runs.
func (c *GitHubIntegration) getWorkflowRuns(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"owner", "repo"}); err != nil {
		return nil, err
	}

	// Build URL
	url, err := c.BuildURL("/repos/{owner}/{repo}/actions/runs", inputs)
	if err != nil {
		return nil, err
	}

	// Add query parameters
	pathParams := []string{"owner", "repo"}
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
	var runsResp WorkflowRunsResponse
	if err := c.ParseJSONResponse(resp, &runsResp); err != nil {
		return nil, err
	}

	// Transform workflow runs to simplified format
	workflowRuns := make([]map[string]interface{}, len(runsResp.WorkflowRuns))
	for i, run := range runsResp.WorkflowRuns {
		item := map[string]interface{}{
			"id":         run.ID,
			"name":       run.Name,
			"status":     run.Status,
			"html_url":   run.HTMLURL,
			"created_at": run.CreatedAt,
		}

		if run.Conclusion != nil {
			item["conclusion"] = *run.Conclusion
		}

		workflowRuns[i] = item
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"total_count":   runsResp.TotalCount,
		"workflow_runs": workflowRuns,
	}), nil
}
