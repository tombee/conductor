package notion

import "time"

// NotionResponse represents the common Notion API response structure.
type NotionResponse struct {
	Object string `json:"object"`
}

// Page represents a Notion page.
type Page struct {
	Object         string                 `json:"object"`
	ID             string                 `json:"id"`
	CreatedTime    time.Time              `json:"created_time"`
	LastEditedTime time.Time              `json:"last_edited_time"`
	URL            string                 `json:"url"`
	Parent         Parent                 `json:"parent"`
	Properties     map[string]interface{} `json:"properties"`
	Icon           *Icon                  `json:"icon,omitempty"`
	Cover          *Cover                 `json:"cover,omitempty"`
}

// Parent represents a page or database parent reference.
type Parent struct {
	Type       string `json:"type"`
	PageID     string `json:"page_id,omitempty"`
	DatabaseID string `json:"database_id,omitempty"`
}

// Icon represents a page icon.
type Icon struct {
	Type  string `json:"type"`
	Emoji string `json:"emoji,omitempty"`
}

// Cover represents a page cover image.
type Cover struct {
	Type     string    `json:"type"`
	External *External `json:"external,omitempty"`
}

// External represents an external URL reference.
type External struct {
	URL string `json:"url"`
}

// Block represents a Notion content block.
type Block struct {
	Object         string                 `json:"object"`
	ID             string                 `json:"id,omitempty"`
	Type           string                 `json:"type"`
	CreatedTime    time.Time              `json:"created_time,omitempty"`
	LastEditedTime time.Time              `json:"last_edited_time,omitempty"`
	HasChildren    bool                   `json:"has_children,omitempty"`
	Content        map[string]interface{} `json:"-"`
}

// BlockContent is used to dynamically set block type-specific content.
// The block type determines which field is set (paragraph, heading_1, etc.).
type BlockContent map[string]interface{}

// RichText represents formatted text content in Notion.
type RichText struct {
	Type        string       `json:"type"`
	Text        *TextContent `json:"text,omitempty"`
	Annotations *Annotations `json:"annotations,omitempty"`
	PlainText   string       `json:"plain_text,omitempty"`
	Href        string       `json:"href,omitempty"`
}

// TextContent represents the text portion of a RichText object.
type TextContent struct {
	Content string `json:"content"`
	Link    *Link  `json:"link,omitempty"`
}

// Link represents a hyperlink.
type Link struct {
	URL string `json:"url"`
}

// Annotations represents text formatting.
type Annotations struct {
	Bold          bool   `json:"bold"`
	Italic        bool   `json:"italic"`
	Strikethrough bool   `json:"strikethrough"`
	Underline     bool   `json:"underline"`
	Code          bool   `json:"code"`
	Color         string `json:"color"`
}

// Database represents a Notion database.
type Database struct {
	Object         string                    `json:"object"`
	ID             string                    `json:"id"`
	CreatedTime    time.Time                 `json:"created_time"`
	LastEditedTime time.Time                 `json:"last_edited_time"`
	Title          []RichText                `json:"title"`
	Properties     map[string]PropertySchema `json:"properties"`
}

// PropertySchema defines the structure of a database property.
type PropertySchema struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// DatabaseItem represents an item in a Notion database.
type DatabaseItem struct {
	Object         string                 `json:"object"`
	ID             string                 `json:"id"`
	CreatedTime    time.Time              `json:"created_time"`
	LastEditedTime time.Time              `json:"last_edited_time"`
	Parent         Parent                 `json:"parent"`
	Properties     map[string]interface{} `json:"properties"`
}

// QueryDatabaseResponse represents the response from querying a database.
type QueryDatabaseResponse struct {
	Object     string         `json:"object"`
	Results    []DatabaseItem `json:"results"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// AppendBlocksResponse represents the response from appending blocks to a page.
type AppendBlocksResponse struct {
	Object  string  `json:"object"`
	Results []Block `json:"results"`
}

// ErrorResponse represents a Notion API error response.
type ErrorResponse struct {
	Object  string `json:"object"`
	Status  int    `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CreatePageRequest represents a request to create a new page.
type CreatePageRequest struct {
	Parent     Parent                 `json:"parent"`
	Properties map[string]interface{} `json:"properties"`
	Icon       *Icon                  `json:"icon,omitempty"`
	Cover      *Cover                 `json:"cover,omitempty"`
	Children   []Block                `json:"children,omitempty"`
}

// UpdatePageRequest represents a request to update a page.
type UpdatePageRequest struct {
	Properties map[string]interface{} `json:"properties,omitempty"`
	Icon       *Icon                  `json:"icon,omitempty"`
	Cover      *Cover                 `json:"cover,omitempty"`
}

// AppendBlocksRequest represents a request to append blocks to a page.
type AppendBlocksRequest struct {
	Children []map[string]interface{} `json:"children"`
}

// QueryDatabaseRequest represents a request to query a database.
type QueryDatabaseRequest struct {
	Filter      interface{} `json:"filter,omitempty"`
	Sorts       interface{} `json:"sorts,omitempty"`
	StartCursor string      `json:"start_cursor,omitempty"`
	PageSize    int         `json:"page_size,omitempty"`
}

// SearchRequest represents a request to search Notion content.
type SearchRequest struct {
	Query       string      `json:"query,omitempty"`
	Filter      interface{} `json:"filter,omitempty"`
	Sort        interface{} `json:"sort,omitempty"`
	StartCursor string      `json:"start_cursor,omitempty"`
	PageSize    int         `json:"page_size,omitempty"`
}

// SearchResponse represents the response from a search request.
type SearchResponse struct {
	Object     string         `json:"object"`
	Results    []SearchResult `json:"results"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

// SearchResult represents a single search result (page or database).
type SearchResult struct {
	Object         string      `json:"object"`
	ID             string      `json:"id"`
	CreatedTime    string      `json:"created_time"`
	LastEditedTime string      `json:"last_edited_time"`
	URL            string      `json:"url,omitempty"`
	Title          interface{} `json:"title,omitempty"`      // For databases
	Properties     interface{} `json:"properties,omitempty"` // For pages
}
