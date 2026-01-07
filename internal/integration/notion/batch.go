package notion

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/operation"
)

// BatchPageResult represents the result of creating a single page in a batch.
type BatchPageResult struct {
	Index   int                    `json:"index"`
	Success bool                   `json:"success"`
	ID      string                 `json:"id,omitempty"`
	URL     string                 `json:"url,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Input   map[string]interface{} `json:"input,omitempty"`
}

// batchCreatePages creates multiple pages in a database with automatic rate limiting.
func (c *NotionIntegration) batchCreatePages(ctx context.Context, inputs map[string]interface{}) (*operation.Result, error) {
	// Validate required parameters
	if err := c.ValidateRequired(inputs, []string{"database_id", "pages"}); err != nil {
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

	// Get pages array
	pages, ok := inputs["pages"].([]interface{})
	if !ok {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   "pages must be an array",
			Category:  ErrorCategoryValidation,
		}
	}

	if len(pages) == 0 {
		return &operation.Result{
			Response: map[string]interface{}{
				"succeeded":     []BatchPageResult{},
				"failed":        []BatchPageResult{},
				"total":         0,
				"success_count": 0,
				"failure_count": 0,
			},
		}, nil
	}

	// Limit batch size to prevent abuse
	maxBatchSize := 100
	if len(pages) > maxBatchSize {
		return nil, &NotionError{
			ErrorCode: "validation_error",
			Message:   fmt.Sprintf("batch size cannot exceed %d pages", maxBatchSize),
			Category:  ErrorCategoryValidation,
		}
	}

	var succeeded []BatchPageResult
	var failed []BatchPageResult

	// Process each page with rate limiting
	for i, pageData := range pages {
		pageMap, ok := pageData.(map[string]interface{})
		if !ok {
			failed = append(failed, BatchPageResult{
				Index:   i,
				Success: false,
				Error:   "page definition must be an object",
				Input:   nil,
			})
			continue
		}

		// Build create request
		createInputs := map[string]interface{}{
			"database_id": databaseID,
		}

		// Copy properties if provided
		if props, ok := pageMap["properties"].(map[string]interface{}); ok {
			createInputs["properties"] = props
		}

		// Copy title if provided (will be used as Name property)
		if title, ok := pageMap["title"].(string); ok {
			createInputs["title"] = title
		}

		// Copy markdown if provided
		if md, ok := pageMap["markdown"].(string); ok {
			createInputs["markdown"] = md
		}

		// Create the page
		result, err := c.createDatabaseItem(ctx, createInputs)
		if err != nil {
			failed = append(failed, BatchPageResult{
				Index:   i,
				Success: false,
				Error:   err.Error(),
				Input:   pageMap,
			})
		} else {
			respMap, _ := result.Response.(map[string]interface{})
			succeeded = append(succeeded, BatchPageResult{
				Index:   i,
				Success: true,
				ID:      getString(respMap, "id"),
				URL:     getString(respMap, "url"),
			})
		}

		// Simple rate limiting: small delay between requests
		// Notion's rate limit is 3 requests per second per token
		if i < len(pages)-1 {
			time.Sleep(350 * time.Millisecond)
		}
	}

	return &operation.Result{
		Response: map[string]interface{}{
			"succeeded":     succeeded,
			"failed":        failed,
			"total":         len(pages),
			"success_count": len(succeeded),
			"failure_count": len(failed),
		},
	}, nil
}

// getString safely extracts a string from a map.
func getString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}
