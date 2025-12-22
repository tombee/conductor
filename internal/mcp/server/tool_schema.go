// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	workflowschema "github.com/tombee/conductor/pkg/workflow/schema"
)

// SchemaResult represents the schema response
type SchemaResult struct {
	Schema  interface{} `json:"schema"`
	Version string      `json:"version"`
}

// handleSchema implements the conductor_schema tool
func (s *Server) handleSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check rate limit
	if !s.rateLimiter.AllowCall() {
		return errorResponse("Rate limit exceeded. Please try again later."), nil
	}

	// Get embedded schema
	schemaBytes := workflowschema.GetEmbeddedSchema()

	// Parse schema JSON
	var schemaData interface{}
	if err := json.Unmarshal(schemaBytes, &schemaData); err != nil {
		return errorResponse(fmt.Sprintf("Failed to parse embedded schema: %v", err)), nil
	}

	// Create result
	result := SchemaResult{
		Schema:  schemaData,
		Version: s.version,
	}

	// Marshal to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to encode schema result: %v", err)), nil
	}

	return textResponse(string(resultJSON)), nil
}
