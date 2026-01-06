package notion

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
)

const (
	// NotionAPIVersion is the Notion API version we're targeting
	NotionAPIVersion = "2022-06-28"
)

// NotionIntegration implements the Provider interface for Notion API.
type NotionIntegration struct {
	*api.BaseProvider
}

// NewNotionIntegration creates a new Notion integration.
func NewNotionIntegration(config *api.ProviderConfig) (operation.Provider, error) {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.notion.com/v1"
	}

	base := api.NewBaseProvider("notion", config)

	return &NotionIntegration{
		BaseProvider: base,
	}, nil
}

// Execute runs a named operation with the given inputs.
func (c *NotionIntegration) Execute(ctx context.Context, operation string, inputs map[string]interface{}) (*operation.Result, error) {
	switch operation {
	// Page operations
	case "create_page":
		return c.createPage(ctx, inputs)
	case "get_page":
		return c.getPage(ctx, inputs)
	case "update_page":
		return c.updatePage(ctx, inputs)
	case "upsert_page":
		return c.upsertPage(ctx, inputs)

	// Block operations
	case "get_blocks":
		return c.getBlocks(ctx, inputs)
	case "append_blocks":
		return c.appendBlocks(ctx, inputs)
	case "replace_content":
		return c.replaceContent(ctx, inputs)

	// Database operations
	case "query_database":
		return c.queryDatabase(ctx, inputs)
	case "create_database_item":
		return c.createDatabaseItem(ctx, inputs)
	case "update_database_item":
		return c.updateDatabaseItem(ctx, inputs)

	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// Operations returns the list of available operations.
func (c *NotionIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		// Page operations
		{Name: "create_page", Description: "Create a new page under a parent page or database", Category: "pages", Tags: []string{"write"}},
		{Name: "get_page", Description: "Retrieve a page's properties", Category: "pages", Tags: []string{"read"}},
		{Name: "update_page", Description: "Update page properties (title, icon, cover)", Category: "pages", Tags: []string{"write"}},
		{Name: "upsert_page", Description: "Update if exists by title match, create if not", Category: "pages", Tags: []string{"write"}},

		// Block operations
		{Name: "get_blocks", Description: "Get content blocks from a page (returns text content)", Category: "blocks", Tags: []string{"read"}},
		{Name: "append_blocks", Description: "Append content blocks to an existing page", Category: "blocks", Tags: []string{"write"}},
		{Name: "replace_content", Description: "Replace all content on a page with new blocks", Category: "blocks", Tags: []string{"write"}},

		// Database operations
		{Name: "query_database", Description: "Query a database with optional filters and sorts", Category: "databases", Tags: []string{"read", "paginated"}},
		{Name: "create_database_item", Description: "Create a new item in a database", Category: "databases", Tags: []string{"write"}},
		{Name: "update_database_item", Description: "Update properties on a database item", Category: "databases", Tags: []string{"write"}},
	}
}

// OperationSchema returns the schema for an operation.
func (c *NotionIntegration) OperationSchema(operation string) *api.OperationSchema {
	// This would return detailed schema information for each operation
	// For now, returning nil (would be implemented based on requirements)
	return nil
}

// defaultHeaders returns default headers for Notion API requests.
func (c *NotionIntegration) defaultHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "application/json",
		"Notion-Version": NotionAPIVersion,
	}
}

// marshalRequest marshals a request struct to JSON bytes.
func marshalRequest(req interface{}) ([]byte, error) {
	if req == nil {
		return nil, nil
	}
	return json.Marshal(req)
}
