package slack

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
)

// uploadFile uploads a file to Slack channels.
func (c *SlackIntegration) uploadFile(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"channels"}); err != nil {
		return nil, err
	}

	// Either content or file must be provided
	_, hasContent := inputs["content"]
	_, hasFile := inputs["file"]
	if !hasContent && !hasFile {
		return nil, fmt.Errorf("validation error: either 'content' or 'file' parameter is required")
	}

	// Build URL
	url, err := c.BuildURL("/files.upload", inputs)
	if err != nil {
		return nil, err
	}

	// Build request body
	body, err := c.BuildRequestBody(inputs, nil)
	if err != nil {
		return nil, err
	}

	// Execute request
	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var fileResp FileUploadResponse
	if err := c.ParseJSONResponse(resp, &fileResp); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToConnectorResult(resp, map[string]interface{}{
		"file_id":   fileResp.File.ID,
		"name":      fileResp.File.Name,
		"permalink": fileResp.File.Permalink,
		"ok":        fileResp.OK,
	}), nil
}
