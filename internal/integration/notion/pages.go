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
	title, _ := inputs["title"].(string)

	// Validate parent ID format
	if !isValidNotionID(parentID) {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "parent_id must be a 32-character Notion ID",
			Category:  ErrorCategoryValidation,
		}
	}

	// Search for existing pages with matching title under parent
	// Use Notion's search API to find pages with this title
	searchURL, err := c.BuildURL("/search", nil)
	if err != nil {
		return nil, err
	}

	searchReq := map[string]interface{}{
		"query": title,
		"filter": map[string]interface{}{
			"value":    "page",
			"property": "object",
		},
	}

	searchBody, err := marshalRequest(searchReq)
	if err != nil {
		return nil, err
	}

	resp, err := c.ExecuteRequest(ctx, "POST", searchURL, c.defaultHeaders(), searchBody)
	if err != nil {
		return nil, err
	}

	if err := ParseError(resp); err != nil {
		return nil, err
	}

	// Parse search results
	var searchResults struct {
		Results []Page `json:"results"`
	}
	if err := c.ParseJSONResponse(resp, &searchResults); err != nil {
		return nil, err
	}

	// Filter results to exact title match within parent scope
	var matchingPages []Page
	for _, page := range searchResults.Results {
		// Check if page has matching title (exact, case-sensitive)
		if pageTitle := extractPageTitle(page.Properties); pageTitle == title {
			// Check if parent matches
			if page.Parent.PageID == parentID || page.Parent.DatabaseID == parentID {
				matchingPages = append(matchingPages, page)
			}
		}
	}

	switch len(matchingPages) {
	case 0:
		// No match found - create new page
		return c.createPage(ctx, inputs)

	case 1:
		// Exactly one match - update existing page
		updateInputs := map[string]interface{}{
			"page_id": matchingPages[0].ID,
		}
		// Copy properties if provided
		if props, ok := inputs["properties"]; ok {
			updateInputs["properties"] = props
		}
		if icon, ok := inputs["icon"]; ok {
			updateInputs["icon"] = icon
		}
		if cover, ok := inputs["cover"]; ok {
			updateInputs["cover"] = cover
		}
		return c.updatePage(ctx, updateInputs)

	default:
		// Multiple matches - return error directing user to use explicit page_id
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   fmt.Sprintf("multiple pages found with title %q under parent. Use explicit page_id instead of upsert_page", title),
			Category:  ErrorCategoryValidation,
		}
	}
}

// extractPageTitle extracts the title from page properties.
func extractPageTitle(properties map[string]interface{}) string {
	titleProp, ok := properties["title"]
	if !ok {
		return ""
	}

	titleMap, ok := titleProp.(map[string]interface{})
	if !ok {
		return ""
	}

	titleArray, ok := titleMap["title"].([]interface{})
	if !ok || len(titleArray) == 0 {
		return ""
	}

	firstTitle, ok := titleArray[0].(map[string]interface{})
	if !ok {
		return ""
	}

	textMap, ok := firstTitle["text"].(map[string]interface{})
	if !ok {
		return ""
	}

	content, _ := textMap["content"].(string)
	return content
}

// isValidNotionID validates a Notion ID format (32 characters, alphanumeric).
func isValidNotionID(id string) bool {
	// Remove hyphens if present (Notion IDs can be formatted with or without hyphens)
	id = strings.ReplaceAll(id, "-", "")

	if len(id) != 32 {
		return false
	}

	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) {
			return false
		}
	}

	return true
}
