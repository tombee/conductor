package notion

import (
	"context"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
)

// queryDatabase queries a database with optional filters and sorts.
func (c *NotionIntegration) queryDatabase(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
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

	// Build request
	req := QueryDatabaseRequest{
		PageSize: 100, // Default to max page size
	}

	// Add optional filter
	if filter, ok := inputs["filter"]; ok {
		req.Filter = filter
	}

	// Add optional sorts
	if sorts, ok := inputs["sorts"]; ok {
		req.Sorts = sorts
	}

	// Build URL
	url, err := c.BuildURL(fmt.Sprintf("/databases/%s/query", databaseID), inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var queryResp QueryDatabaseResponse
	if err := c.ParseJSONResponse(resp, &queryResp); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"results":     queryResp.Results,
		"has_more":    queryResp.HasMore,
		"next_cursor": queryResp.NextCursor,
	}), nil
}

// createDatabaseItem creates a new item in a database.
func (c *NotionIntegration) createDatabaseItem(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"database_id", "properties"}); err != nil {
		return nil, err
	}

	databaseID, _ := inputs["database_id"].(string)
	properties, ok := inputs["properties"].(map[string]interface{})
	if !ok {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "properties must be an object",
			Category:  ErrorCategoryValidation,
		}
	}

	// Validate database ID format
	if !isValidNotionID(databaseID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "database_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build request
	req := CreatePageRequest{
		Parent: Parent{
			Type:       "database_id",
			DatabaseID: databaseID,
		},
		Properties: properties,
	}

	// Build URL
	url, err := c.BuildURL("/pages", inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var page Page
	if err := c.ParseJSONResponse(resp, &page); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"id":         page.ID,
		"url":        page.URL,
		"created_at": page.CreatedTime,
	}), nil
}

// updateDatabaseItem updates properties on an existing database item.
func (c *NotionIntegration) updateDatabaseItem(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"item_id", "properties"}); err != nil {
		return nil, err
	}

	itemID, _ := inputs["item_id"].(string)
	properties, ok := inputs["properties"].(map[string]interface{})
	if !ok {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "properties must be an object",
			Category:  ErrorCategoryValidation,
		}
	}

	// Validate item ID format
	if !isValidNotionID(itemID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "item_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build request
	req := UpdatePageRequest{
		Properties: properties,
	}

	// Build URL
	url, err := c.BuildURL(fmt.Sprintf("/pages/%s", itemID), inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "PATCH", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var page Page
	if err := c.ParseJSONResponse(resp, &page); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"id":  page.ID,
		"url": page.URL,
	}), nil
}

// deleteDatabaseItem archives (soft deletes) a database item.
// Notion doesn't support permanent deletion via API; archived items can be restored.
func (c *NotionIntegration) deleteDatabaseItem(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"item_id"}); err != nil {
		return nil, err
	}

	itemID, _ := inputs["item_id"].(string)

	// Validate item ID format
	if !isValidNotionID(itemID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "item_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build request to archive the item
	req := map[string]interface{}{
		"archived": true,
	}

	// Build URL (database items are pages in Notion)
	url, err := c.BuildURL(fmt.Sprintf("/pages/%s", itemID), inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "PATCH", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var page Page
	if err := c.ParseJSONResponse(resp, &page); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"id":       page.ID,
		"archived": true,
	}), nil
}

// listDatabases lists all databases accessible to the integration.
func (c *NotionIntegration) listDatabases(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Build search request with database filter
	req := map[string]interface{}{
		"filter": map[string]interface{}{
			"property": "object",
			"value":    "database",
		},
	}

	// Build URL
	url, err := c.BuildURL("/search", inputs)
	if err != nil {
		return nil, err
	}

	// Execute request
	body, err := marshalRequest(req)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "POST", url, c.defaultHeaders(), body)
	if err != nil {
		return nil, err
	}

	// Parse error if any
	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse response
	var searchResp SearchResponse
	if err := c.ParseJSONResponse(resp, &searchResp); err != nil {
		return nil, err
	}

	// Transform results to simplified database list
	databases := make([]map[string]interface{}, 0, len(searchResp.Results))
	for _, result := range searchResp.Results {
		db := map[string]interface{}{
			"id":               result.ID,
			"created_time":     result.CreatedTime,
			"last_edited_time": result.LastEditedTime,
		}
		// Extract title from database title property
		if titleArr, ok := result.Title.([]interface{}); ok && len(titleArr) > 0 {
			if titleObj, ok := titleArr[0].(map[string]interface{}); ok {
				if plainText, ok := titleObj["plain_text"].(string); ok {
					db["title"] = plainText
				}
			}
		}
		databases = append(databases, db)
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"databases":   databases,
		"has_more":    searchResp.HasMore,
		"next_cursor": searchResp.NextCursor,
	}), nil
}
