package notion

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/tombee/conductor/internal/operation"
	"github.com/tombee/conductor/internal/operation/api"
	"github.com/tombee/conductor/internal/operation/transport"
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
func (c *NotionIntegration) Execute(ctx context.Context, op string, inputs map[string]interface{}) (*operation.Result, error) {
	switch op {
	// Page operations
	case "create_page":
		return c.createPage(ctx, inputs)
	case "get_page":
		return c.getPage(ctx, inputs)
	case "update_page":
		return c.updatePage(ctx, inputs)
	case "delete_page":
		return c.deletePage(ctx, inputs)
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
	case "delete_database_item":
		return c.deleteDatabaseItem(ctx, inputs)
	case "list_databases":
		return c.listDatabases(ctx, inputs)

	// Search
	case "search":
		return c.search(ctx, inputs)

	default:
		return nil, fmt.Errorf("unknown operation: %s", op)
	}
}

// Operations returns the list of available operations.
func (c *NotionIntegration) Operations() []api.OperationInfo {
	return []api.OperationInfo{
		// Page operations
		{Name: "create_page", Description: "Create a new page under a parent page or database", Category: "pages", Tags: []string{"write"}},
		{Name: "get_page", Description: "Retrieve a page's properties", Category: "pages", Tags: []string{"read"}},
		{Name: "update_page", Description: "Update page properties (title, icon, cover)", Category: "pages", Tags: []string{"write"}},
		{Name: "delete_page", Description: "Archive (soft delete) a page", Category: "pages", Tags: []string{"write"}},
		{Name: "upsert_page", Description: "Update if exists by title match, create if not", Category: "pages", Tags: []string{"write"}},

		// Block operations
		{Name: "get_blocks", Description: "Get content blocks from a page (returns text content)", Category: "blocks", Tags: []string{"read"}},
		{Name: "append_blocks", Description: "Append content blocks to an existing page", Category: "blocks", Tags: []string{"write"}},
		{Name: "replace_content", Description: "Replace all content on a page with new blocks", Category: "blocks", Tags: []string{"write"}},

		// Database operations
		{Name: "query_database", Description: "Query a database with optional filters and sorts", Category: "databases", Tags: []string{"read", "paginated"}},
		{Name: "create_database_item", Description: "Create a new item in a database", Category: "databases", Tags: []string{"write"}},
		{Name: "update_database_item", Description: "Update properties on a database item", Category: "databases", Tags: []string{"write"}},
		{Name: "delete_database_item", Description: "Archive (soft delete) a database item", Category: "databases", Tags: []string{"write"}},
		{Name: "list_databases", Description: "List all databases accessible to the integration", Category: "databases", Tags: []string{"read", "paginated"}},

		// Search
		{Name: "search", Description: "Search pages and databases by title or content", Category: "search", Tags: []string{"read", "paginated"}},
	}
}

// OperationSchema returns the schema for an operation.
func (c *NotionIntegration) OperationSchema(op string) *api.OperationSchema {
	schemas := map[string]*api.OperationSchema{
		"create_page": {
			Description: "Create a new page under a parent page or database",
			Parameters: []api.ParameterInfo{
				{Name: "parent_id", Type: "string", Description: "32-character Notion ID of the parent page", Required: true},
				{Name: "title", Type: "string", Description: "Page title (1-2000 characters, whitespace trimmed)", Required: true},
				{Name: "properties", Type: "object", Description: "Additional page properties"},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "id", Type: "string", Description: "Created page ID"},
				{Name: "url", Type: "string", Description: "Page URL"},
				{Name: "created_at", Type: "string", Description: "Creation timestamp"},
			},
		},
		"get_page": {
			Description: "Retrieve a page's properties",
			Parameters: []api.ParameterInfo{
				{Name: "page_id", Type: "string", Description: "32-character Notion page ID", Required: true},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "id", Type: "string", Description: "Page ID"},
				{Name: "url", Type: "string", Description: "Page URL"},
				{Name: "properties", Type: "object", Description: "Page properties"},
				{Name: "parent", Type: "object", Description: "Parent reference"},
			},
		},
		"update_page": {
			Description: "Update page properties (title, icon, cover)",
			Parameters: []api.ParameterInfo{
				{Name: "page_id", Type: "string", Description: "32-character Notion page ID", Required: true},
				{Name: "properties", Type: "object", Description: "Properties to update"},
				{Name: "icon", Type: "object", Description: "Icon configuration (type, emoji)"},
				{Name: "cover", Type: "object", Description: "Cover image configuration"},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "id", Type: "string", Description: "Page ID"},
				{Name: "url", Type: "string", Description: "Page URL"},
			},
		},
		"delete_page": {
			Description: "Archive (soft delete) a page. Archived pages can be restored in Notion.",
			Parameters: []api.ParameterInfo{
				{Name: "page_id", Type: "string", Description: "32-character Notion page ID", Required: true},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "id", Type: "string", Description: "Page ID"},
				{Name: "archived", Type: "boolean", Description: "Archive status (always true)"},
			},
		},
		"upsert_page": {
			Description: "Update existing page by title match, or create new if not found",
			Parameters: []api.ParameterInfo{
				{Name: "parent_id", Type: "string", Description: "32-character Notion ID of the parent page", Required: true},
				{Name: "title", Type: "string", Description: "Page title to match or create", Required: true},
				{Name: "blocks", Type: "array", Description: "Content blocks to add/replace"},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "id", Type: "string", Description: "Page ID"},
				{Name: "url", Type: "string", Description: "Page URL"},
				{Name: "is_new", Type: "boolean", Description: "True if page was created, false if updated"},
			},
		},
		"get_blocks": {
			Description: "Get content blocks from a page",
			Parameters: []api.ParameterInfo{
				{Name: "page_id", Type: "string", Description: "32-character Notion page ID", Required: true},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "content", Type: "string", Description: "Extracted text content"},
				{Name: "block_count", Type: "integer", Description: "Number of blocks"},
				{Name: "raw_blocks", Type: "array", Description: "Raw block data"},
			},
		},
		"append_blocks": {
			Description: "Append content blocks to an existing page",
			Parameters: []api.ParameterInfo{
				{Name: "page_id", Type: "string", Description: "32-character Notion page ID", Required: true},
				{Name: "blocks", Type: "array", Description: "Array of blocks (max 100). Each block needs type and text fields.", Required: true},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "blocks_added", Type: "integer", Description: "Number of blocks added"},
			},
		},
		"replace_content": {
			Description: "Replace all content on a page with new blocks (preserves child pages)",
			Parameters: []api.ParameterInfo{
				{Name: "page_id", Type: "string", Description: "32-character Notion page ID", Required: true},
				{Name: "blocks", Type: "array", Description: "Array of replacement blocks (max 100)", Required: true},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "blocks_added", Type: "integer", Description: "Number of blocks added"},
			},
		},
		"query_database": {
			Description: "Query a database with optional filters and sorts",
			Parameters: []api.ParameterInfo{
				{Name: "database_id", Type: "string", Description: "32-character Notion database ID", Required: true},
				{Name: "filter", Type: "object", Description: "Filter conditions per Notion API spec"},
				{Name: "sorts", Type: "array", Description: "Sort configuration"},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "results", Type: "array", Description: "Database items matching query"},
				{Name: "has_more", Type: "boolean", Description: "Whether more results exist"},
				{Name: "next_cursor", Type: "string", Description: "Cursor for pagination"},
			},
		},
		"create_database_item": {
			Description: "Create a new item in a database",
			Parameters: []api.ParameterInfo{
				{Name: "database_id", Type: "string", Description: "32-character Notion database ID", Required: true},
				{Name: "properties", Type: "object", Description: "Item properties matching database schema", Required: true},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "id", Type: "string", Description: "Created item ID"},
				{Name: "url", Type: "string", Description: "Item URL"},
				{Name: "created_at", Type: "string", Description: "Creation timestamp"},
			},
		},
		"update_database_item": {
			Description: "Update properties on an existing database item",
			Parameters: []api.ParameterInfo{
				{Name: "item_id", Type: "string", Description: "32-character Notion item ID", Required: true},
				{Name: "properties", Type: "object", Description: "Properties to update", Required: true},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "id", Type: "string", Description: "Item ID"},
				{Name: "url", Type: "string", Description: "Item URL"},
			},
		},
		"delete_database_item": {
			Description: "Archive (soft delete) a database item. Archived items can be restored.",
			Parameters: []api.ParameterInfo{
				{Name: "item_id", Type: "string", Description: "32-character Notion item ID", Required: true},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "id", Type: "string", Description: "Item ID"},
				{Name: "archived", Type: "boolean", Description: "Archive status (always true)"},
			},
		},
		"list_databases": {
			Description: "List all databases accessible to the integration",
			Parameters:  []api.ParameterInfo{},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "databases", Type: "array", Description: "Array of databases with id, title, created_time, last_edited_time"},
				{Name: "has_more", Type: "boolean", Description: "Whether more results exist"},
				{Name: "next_cursor", Type: "string", Description: "Cursor for pagination"},
			},
		},
		"search": {
			Description: "Search pages and databases by title or content",
			Parameters: []api.ParameterInfo{
				{Name: "query", Type: "string", Description: "Search query string"},
				{Name: "filter", Type: "object", Description: "Filter by object type (page or database)"},
				{Name: "sort", Type: "object", Description: "Sort configuration (direction, timestamp)"},
				{Name: "start_cursor", Type: "string", Description: "Pagination cursor"},
				{Name: "page_size", Type: "integer", Description: "Results per page (max 100)"},
			},
			ResponseFields: []api.ResponseFieldInfo{
				{Name: "results", Type: "array", Description: "Search results with id, type, title, url"},
				{Name: "has_more", Type: "boolean", Description: "Whether more results exist"},
				{Name: "next_cursor", Type: "string", Description: "Cursor for pagination"},
			},
		},
	}

	return schemas[op]
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

// toResultWithMetadata creates an operation result with Notion-specific metadata.
// Extracts http_status, rate_limit_remaining, and request_id from response headers.
func (c *NotionIntegration) toResultWithMetadata(resp *transport.Response, response map[string]interface{}) *operation.Result {
	// Add metadata to response
	metadata := map[string]interface{}{
		"http_status": resp.StatusCode,
	}

	// Extract rate limit remaining from header
	if values, ok := resp.Headers["X-Ratelimit-Remaining"]; ok && len(values) > 0 {
		if val, err := strconv.Atoi(values[0]); err == nil {
			metadata["rate_limit_remaining"] = val
		}
	}

	// Extract request ID for debugging
	if values, ok := resp.Headers["X-Request-Id"]; ok && len(values) > 0 {
		metadata["request_id"] = values[0]
	}

	// Add metadata to response
	response["metadata"] = metadata

	return c.ToResult(resp, response)
}
