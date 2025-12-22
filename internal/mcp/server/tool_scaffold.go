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
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tombee/conductor/internal/templates"
)

// ScaffoldResult represents the scaffolded workflow
type ScaffoldResult struct {
	WorkflowYAML string     `json:"workflow_yaml"`
	FilesCreated []FileInfo `json:"files_created,omitempty"`
}

// FileInfo represents information about a created file
type FileInfo struct {
	Path        string `json:"path"`
	Description string `json:"description"`
}

// handleScaffold implements the conductor_scaffold tool
func (s *Server) handleScaffold(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check rate limit
	if !s.rateLimiter.AllowCall() {
		return errorResponse("Rate limit exceeded. Please try again later."), nil
	}

	// Extract required arguments
	templateName, err := request.RequireString("template")
	if err != nil {
		return errorResponse("Missing or invalid 'template' argument"), nil
	}

	workflowName, err := request.RequireString("name")
	if err != nil {
		return errorResponse("Missing or invalid 'name' argument"), nil
	}

	// Validate template name (defense in depth - templates.Render also validates)
	if strings.Contains(templateName, "..") || strings.Contains(templateName, "/") || strings.Contains(templateName, "\\") {
		return errorResponse("Invalid template name: path traversal detected"), nil
	}

	// Check if template exists
	if !templates.Exists(templateName) {
		return errorResponse(fmt.Sprintf("Template %q not found. Use conductor_list_templates to see available templates.", templateName)), nil
	}

	// Extract optional parameters (currently only Name is used)
	// In the future, additional parameters could be passed here
	// parameters, _ := request.Params.Arguments["parameters"].(map[string]interface{})

	// Render template
	renderedYAML, err := templates.Render(templateName, workflowName)
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to render template: %v", err)), nil
	}

	// Validate the rendered YAML
	validationResult := validateWorkflowYAML(renderedYAML)
	if !validationResult.Valid {
		// This shouldn't happen with built-in templates, but check anyway
		errMsg := "Rendered workflow is invalid"
		if len(validationResult.Errors) > 0 {
			errMsg = fmt.Sprintf("Rendered workflow is invalid: %s", validationResult.Errors[0].Message)
		}
		return errorResponse(errMsg), nil
	}

	// Create result
	result := ScaffoldResult{
		WorkflowYAML: string(renderedYAML),
		FilesCreated: []FileInfo{}, // No additional files for now
	}

	// Marshal to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to encode scaffold result: %v", err)), nil
	}

	return textResponse(string(resultJSON)), nil
}
