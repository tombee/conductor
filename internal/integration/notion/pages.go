package notion

import (
	"context"
	"fmt"
	"strings"

	"github.com/tombee/conductor/internal/operation"
)

// createPage creates a new page under a parent page or database.
func (c *NotionIntegration) createPage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"parent_id", "title"}); err != nil {
		return nil, err
	}

	parentID, _ := inputs["parent_id"].(string)
	title, _ := inputs["title"].(string)

	// Validate parent ID format (32 characters alphanumeric)
	if !isValidNotionID(parentID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "parent_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Validate title length
	if len(title) < 1 || len(title) > 500 {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "title must be between 1 and 500 characters",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build request
	req := CreatePageRequest{
		Parent: Parent{
			Type:   "page_id",
			PageID: parentID,
		},
		Properties: map[string]interface{}{
			"title": map[string]interface{}{
				"title": []map[string]interface{}{
					{
						"text": map[string]interface{}{
							"content": title,
						},
					},
				},
			},
		},
	}

	// Include optional properties if provided
	if props, ok := inputs["properties"].(map[string]interface{}); ok {
		for k, v := range props {
			req.Properties[k] = v
		}
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

// getPage retrieves a page's properties.
func (c *NotionIntegration) getPage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"page_id"}); err != nil {
		return nil, err
	}

	pageID, _ := inputs["page_id"].(string)

	// Validate page ID format
	if !isValidNotionID(pageID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "page_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build URL
	url, err := c.BuildURL(fmt.Sprintf("/pages/%s", pageID), inputs)
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
	var page Page
	if err := c.ParseJSONResponse(resp, &page); err != nil {
		return nil, err
	}

	// Return operation result
	return c.ToResult(resp, map[string]interface{}{
		"id":         page.ID,
		"url":        page.URL,
		"properties": page.Properties,
		"parent":     page.Parent,
	}), nil
}

// updatePage updates page properties (title, icon, cover).
func (c *NotionIntegration) updatePage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"page_id"}); err != nil {
		return nil, err
	}

	pageID, _ := inputs["page_id"].(string)

	// Validate page ID format
	if !isValidNotionID(pageID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "page_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Build request with optional properties
	req := UpdatePageRequest{}

	if props, ok := inputs["properties"].(map[string]interface{}); ok {
		req.Properties = props
	}

	if icon, ok := inputs["icon"].(map[string]interface{}); ok {
		iconType, _ := icon["type"].(string)
		emoji, _ := icon["emoji"].(string)
		req.Icon = &Icon{
			Type:  iconType,
			Emoji: emoji,
		}
	}

	if cover, ok := inputs["cover"].(map[string]interface{}); ok {
		coverType, _ := cover["type"].(string)
		if external, ok := cover["external"].(map[string]interface{}); ok {
			url, _ := external["url"].(string)
			req.Cover = &Cover{
				Type: coverType,
				External: &External{
					URL: url,
				},
			}
		}
	}

	// Build URL
	url, err := c.BuildURL(fmt.Sprintf("/pages/%s", pageID), inputs)
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

// upsertPage updates if exists by title match, creates if not.
func (c *NotionIntegration) upsertPage(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"parent_id", "title"}); err != nil {
		return nil, err
	}

	parentID, _ := inputs["parent_id"].(string)

	// Validate parent ID format
	if !isValidNotionID(parentID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "parent_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Search for existing page with matching title under parent
	// For MVP, we'll attempt to create and handle conflicts
	// A more robust implementation would query child pages first
	result, err := c.createPage(ctx, inputs)
	if err != nil {
		// If creation fails due to existing page, we could query and update
		// For now, return the error
		return nil, err
	}

	return result, nil
}

// isValidNotionID validates a Notion ID format (32 characters, alphanumeric).
func isValidNotionID(id string) bool {
	// Remove hyphens if present (Notion IDs can be formatted with or without hyphens)
	id = strings.ReplaceAll(id, "-", "")

	if len(id) != 32 {
		return false
	}

	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}

	return true
}
