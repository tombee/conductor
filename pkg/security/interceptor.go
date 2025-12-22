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

package security

import (
	"context"
	"fmt"
)

// Tool represents a minimal interface for tools that the interceptor needs.
// This avoids circular dependency with pkg/tools.
type Tool interface {
	Name() string
}

// Interceptor wraps tool execution with security checks.
type Interceptor interface {
	// Intercept is called before tool execution
	Intercept(ctx context.Context, tool Tool, inputs map[string]interface{}) error

	// PostExecute is called after tool execution
	PostExecute(ctx context.Context, tool Tool, outputs map[string]interface{}, err error)
}

// interceptor implements the Interceptor interface.
type interceptor struct {
	manager Manager
}

// NewInterceptor creates a new security interceptor.
func NewInterceptor(manager Manager) Interceptor {
	return &interceptor{
		manager: manager,
	}
}

// Intercept validates tool execution against security policy.
func (i *interceptor) Intercept(ctx context.Context, tool Tool, inputs map[string]interface{}) error {
	toolName := tool.Name()

	// Extract security context from context
	secCtx := GetSecurityContext(ctx)
	if secCtx == nil {
		// No security context means we're in unrestricted mode or testing
		return nil
	}

	// Create access requests based on tool type and inputs
	requests := i.extractAccessRequests(secCtx, toolName, inputs)

	// Check each access request
	for _, req := range requests {
		decision := i.manager.CheckAccess(req)

		// Determine event type
		var eventType EventType
		if decision.Allowed {
			eventType = EventAccessGranted
		} else {
			eventType = EventAccessDenied
		}

		// Log the security event
		event := NewSecurityEvent(eventType, req, decision)
		event.UserID = secCtx.UserID
		i.manager.LogEvent(event)

		// Deny if not allowed
		if !decision.Allowed {
			return &AccessDeniedError{
				ToolName:     toolName,
				ResourceType: req.ResourceType,
				Resource:     req.Resource,
				Action:       req.Action,
				Reason:       decision.Reason,
				Profile:      decision.Profile,
			}
		}
	}

	return nil
}

// PostExecute is called after tool execution.
func (i *interceptor) PostExecute(ctx context.Context, tool Tool, outputs map[string]interface{}, err error) {
	// Currently just a hook for future functionality
	// Could be used for:
	// - Output filtering/sanitization
	// - Resource usage tracking
	// - Post-execution auditing
}

// extractAccessRequests extracts access requests from tool inputs.
func (i *interceptor) extractAccessRequests(secCtx *SecurityContext, toolName string, inputs map[string]interface{}) []AccessRequest {
	var requests []AccessRequest

	// Extract based on tool type
	switch toolName {
	case "file":
		requests = append(requests, i.extractFileRequests(secCtx, inputs)...)
	case "shell":
		requests = append(requests, i.extractShellRequests(secCtx, inputs)...)
	case "http":
		requests = append(requests, i.extractHTTPRequests(secCtx, inputs)...)
	default:
		// For unknown tools, check if there are common parameters
		if path, ok := inputs["path"].(string); ok {
			// Assume file access
			operation := "read"
			if op, ok := inputs["operation"].(string); ok {
				operation = op
			}
			action := ActionRead
			if operation == "write" {
				action = ActionWrite
			}
			requests = append(requests, AccessRequest{
				WorkflowID:   secCtx.WorkflowID,
				StepID:       secCtx.StepID,
				ToolName:     toolName,
				ResourceType: ResourceTypeFile,
				Resource:     path,
				Action:       action,
			})
		}
	}

	return requests
}

// extractFileRequests extracts file access requests.
func (i *interceptor) extractFileRequests(secCtx *SecurityContext, inputs map[string]interface{}) []AccessRequest {
	var requests []AccessRequest

	operation, ok := inputs["operation"].(string)
	if !ok {
		return requests
	}

	path, ok := inputs["path"].(string)
	if !ok {
		return requests
	}

	action := ActionRead
	if operation == "write" {
		action = ActionWrite
	}

	requests = append(requests, AccessRequest{
		WorkflowID:   secCtx.WorkflowID,
		StepID:       secCtx.StepID,
		ToolName:     "file",
		ResourceType: ResourceTypeFile,
		Resource:     path,
		Action:       action,
	})

	return requests
}

// extractShellRequests extracts command execution requests.
func (i *interceptor) extractShellRequests(secCtx *SecurityContext, inputs map[string]interface{}) []AccessRequest {
	var requests []AccessRequest

	command, ok := inputs["command"].(string)
	if !ok {
		return requests
	}

	requests = append(requests, AccessRequest{
		WorkflowID:   secCtx.WorkflowID,
		StepID:       secCtx.StepID,
		ToolName:     "shell",
		ResourceType: ResourceTypeCommand,
		Resource:     command,
		Action:       ActionExecute,
	})

	return requests
}

// extractHTTPRequests extracts HTTP access requests.
func (i *interceptor) extractHTTPRequests(secCtx *SecurityContext, inputs map[string]interface{}) []AccessRequest {
	var requests []AccessRequest

	url, ok := inputs["url"].(string)
	if !ok {
		return requests
	}

	// Extract host from URL
	host := extractHostFromURL(url)

	requests = append(requests, AccessRequest{
		WorkflowID:   secCtx.WorkflowID,
		StepID:       secCtx.StepID,
		ToolName:     "http",
		ResourceType: ResourceTypeNetwork,
		Resource:     host,
		Action:       ActionConnect,
	})

	return requests
}

// extractHostFromURL extracts the host from a URL string.
func extractHostFromURL(url string) string {
	// Simple extraction - just remove protocol
	// More robust URL parsing should be done in the HTTP tool itself
	if idx := findSubstring(url, "://"); idx != -1 {
		url = url[idx+3:]
	}
	if idx := findSubstring(url, "/"); idx != -1 {
		url = url[:idx]
	}
	return url
}

// findSubstring finds the index of a substring (helper to avoid importing strings).
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// SecurityContext holds security state for a workflow execution.
type SecurityContext struct {
	// WorkflowID identifies the workflow
	WorkflowID string

	// StepID identifies the current step
	StepID string

	// UserID identifies the user running the workflow
	UserID string

	// Profile is the active security profile
	Profile *SecurityProfile
}

// contextKey is a private type for context keys.
type contextKey string

const securityContextKey contextKey = "security_context"

// WithSecurityContext attaches a security context to a context.
func WithSecurityContext(ctx context.Context, secCtx *SecurityContext) context.Context {
	return context.WithValue(ctx, securityContextKey, secCtx)
}

// GetSecurityContext retrieves the security context from a context.
func GetSecurityContext(ctx context.Context) *SecurityContext {
	if secCtx, ok := ctx.Value(securityContextKey).(*SecurityContext); ok {
		return secCtx
	}
	return nil
}

// AccessDeniedError is returned when access is denied.
type AccessDeniedError struct {
	ToolName     string
	ResourceType ResourceType
	Resource     string
	Action       AccessAction
	Reason       string
	Profile      string
}

// Error implements the error interface.
func (e *AccessDeniedError) Error() string {
	return fmt.Sprintf("security: access denied - tool=%s, resource_type=%s, resource=%s, action=%s, profile=%s, reason=%s",
		e.ToolName, e.ResourceType, e.Resource, e.Action, e.Profile, e.Reason)
}
