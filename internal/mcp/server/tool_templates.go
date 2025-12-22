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
	"github.com/tombee/conductor/internal/templates"
)

// TemplatesResult represents the list of available templates
type TemplatesResult struct {
	Templates []TemplateInfo `json:"templates"`
}

// TemplateInfo represents metadata about a template
type TemplateInfo struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Category    string              `json:"category"`
	Parameters  []TemplateParameter `json:"parameters"`
}

// TemplateParameter represents a template parameter
type TemplateParameter struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
}

// handleListTemplates implements the conductor_list_templates tool
func (s *Server) handleListTemplates(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Check rate limit
	if !s.rateLimiter.AllowCall() {
		return errorResponse("Rate limit exceeded. Please try again later."), nil
	}

	// Extract category filter (optional)
	categoryFilter := request.GetString("category", "")

	// Get templates
	templateList, err := templates.List()
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to list templates: %v", err)), nil
	}

	// Convert to TemplateInfo with parameters
	var templateInfos []TemplateInfo
	for _, tmpl := range templateList {
		// Apply category filter if provided
		if categoryFilter != "" && tmpl.Category != categoryFilter {
			continue
		}

		info := TemplateInfo{
			Name:        tmpl.Name,
			Description: tmpl.Description,
			Category:    tmpl.Category,
			Parameters:  getTemplateParameters(tmpl.Name),
		}
		templateInfos = append(templateInfos, info)
	}

	result := TemplatesResult{
		Templates: templateInfos,
	}

	// Marshal to JSON
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return errorResponse(fmt.Sprintf("Failed to encode templates result: %v", err)), nil
	}

	return textResponse(string(resultJSON)), nil
}

// getTemplateParameters returns the parameters for a specific template
// Templates use Go text/template syntax with {{.Name}} as the main parameter
func getTemplateParameters(templateName string) []TemplateParameter {
	// All templates currently support the Name parameter
	params := []TemplateParameter{
		{
			Name:        "Name",
			Description: "The name for the generated workflow",
			Required:    true,
		},
	}

	// Template-specific parameters can be added here as templates evolve
	switch templateName {
	case "code-review":
		// Code review template might have additional parameters in the future
	case "summarize":
		// Summarize template parameters
	}

	return params
}
