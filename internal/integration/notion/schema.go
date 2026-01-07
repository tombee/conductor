package notion

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
)

// DatabaseSchemaResponse represents a database schema.
type DatabaseSchemaResponse struct {
	Object     string                    `json:"object"`
	ID         string                    `json:"id"`
	Title      []RichText                `json:"title"`
	Properties map[string]PropertySchema `json:"properties"`
}

// getDatabaseSchema retrieves the schema (property definitions) of a database.
func (c *NotionIntegration) getDatabaseSchema(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"database_id"}); err != nil {
		return nil, err
	}

	databaseID, _ := inputs["database_id"].(string)

	// Validate database ID format
	if !isValidNotionID(databaseID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "database_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Get database
	url, err := c.BuildURL(fmt.Sprintf("/databases/%s", databaseID), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "GET", url, c.defaultHeaders(), nil)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var dbResp DatabaseSchemaResponse
	if err := c.ParseJSONResponse(resp, &dbResp); err != nil {
		return nil, err
	}

	// Extract title
	title := ""
	for _, rt := range dbResp.Title {
		title += rt.PlainText
	}

	// Transform properties to simplified format
	properties := make(map[string]interface{})
	for name, prop := range dbResp.Properties {
		propInfo := map[string]interface{}{
			"id":   prop.ID,
			"type": prop.Type,
			"name": name,
		}
		properties[name] = propInfo
	}

	return c.ToResult(resp, map[string]interface{}{
		"id":         dbResp.ID,
		"title":      title,
		"properties": properties,
	}), nil
}

// updateDatabaseSchema updates the schema (property definitions) of a database.
func (c *NotionIntegration) updateDatabaseSchema(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"database_id"}); err != nil {
		return nil, err
	}

	databaseID, _ := inputs["database_id"].(string)

	// Validate database ID format
	if !isValidNotionID(databaseID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "database_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build request body
	body := make(map[string]interface{})

	// Handle title update
	if title, ok := inputs["title"].(string); ok && title != "" {
		body["title"] = []map[string]interface{}{
			{
				"text": map[string]interface{}{
					"content": title,
				},
			},
		}
	}

	// Handle properties update
	if props, ok := inputs["properties"].(map[string]interface{}); ok {
		body["properties"] = props
	}

	if len(body) == 0 {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "at least one of 'title' or 'properties' must be provided",
			Category:  ErrorCategoryValidation,
		}
	}

	url, err := c.BuildURL(fmt.Sprintf("/databases/%s", databaseID), nil)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := marshalRequest(body)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "PATCH", url, c.defaultHeaders(), bodyBytes)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	var dbResp DatabaseSchemaResponse
	if err := c.ParseJSONResponse(resp, &dbResp); err != nil {
		return nil, err
	}

	// Extract title
	title := ""
	for _, rt := range dbResp.Title {
		title += rt.PlainText
	}

	return c.ToResult(resp, map[string]interface{}{
		"id":    dbResp.ID,
		"title": title,
	}), nil
}
