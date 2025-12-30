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

package runner

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/workflow"
)

const (
	// RunStatusDryRun is the status for dry-run executions
	RunStatusDryRun RunStatus = "dry_run"
)

// DryRunOptions configures dry-run behavior.
type DryRunOptions struct {
	Deep            bool // Perform deep template expansion
	ValidateRefs    bool // Validate external references (URLs, files)
	ShowConditions  bool // Show condition evaluation results
	EstimateCost    bool // Provide detailed cost estimation
}

// DryRunPlan represents the execution plan for a workflow without running it.
type DryRunPlan struct {
	Steps             []DryRunStep       `json:"steps"`
	EstimatedCost     string             `json:"estimated_cost,omitempty"`
	DetailedCosts     []StepCostEstimate `json:"detailed_costs,omitempty"`
	SecurityProfile   string             `json:"security_profile,omitempty"`
	TotalSteps        int                `json:"total_steps"`
	EstimatedDuration string             `json:"estimated_duration,omitempty"`
	Warnings          []string           `json:"warnings,omitempty"`
	ValidationErrors  []string           `json:"validation_errors,omitempty"`
}

// DryRunStep represents a planned step execution.
type DryRunStep struct {
	ID                string            `json:"id"`
	Name              string            `json:"name"`
	Type              string            `json:"type"`
	Provider          string            `json:"provider,omitempty"`
	Model             string            `json:"model,omitempty"`
	EstimatedTokens   int               `json:"estimated_tokens,omitempty"`
	ExpandedPrompt    string            `json:"expanded_prompt,omitempty"`
	ExpandedSystem    string            `json:"expanded_system,omitempty"`
	ConditionResult   *ConditionResult  `json:"condition_result,omitempty"`
	WillExecute       bool              `json:"will_execute"`
	ValidationIssues  []string          `json:"validation_issues,omitempty"`
}

// ConditionResult represents the evaluation of a step condition.
type ConditionResult struct {
	Expression string `json:"expression"`
	Result     bool   `json:"result"`
	Error      string `json:"error,omitempty"`
}

// StepCostEstimate provides cost estimation for a single step.
type StepCostEstimate struct {
	StepID         string  `json:"step_id"`
	StepName       string  `json:"step_name"`
	EstimatedCost  float64 `json:"estimated_cost_usd"`
	TokensEstimate int     `json:"tokens_estimate,omitempty"`
	Basis          string  `json:"basis"` // "historical", "model_pricing", "unknown"
}

// DryRun analyzes a workflow and returns an execution plan without running it.
// This is used for the --dry-run flag to preview what would happen.
func (r *Runner) DryRun(ctx context.Context, req SubmitRequest) (*RunSnapshot, error) {
	// Get workflow YAML (either from request or remote)
	var workflowYAML []byte
	var err error

	if req.RemoteRef != "" {
		// Fetch remote workflow
		r.mu.RLock()
		fetcher := r.fetcher
		r.mu.RUnlock()

		if fetcher == nil {
			return nil, fmt.Errorf("remote workflows not supported (fetcher not configured)")
		}

		result, err := fetcher.Fetch(ctx, req.RemoteRef, req.NoCache)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch remote workflow: %w", err)
		}
		workflowYAML = result.Content
	} else {
		workflowYAML = req.WorkflowYAML
	}

	// Parse workflow definition
	definition, err := workflow.ParseDefinition(workflowYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Build dry-run plan
	plan := &DryRunPlan{
		Steps:           make([]DryRunStep, 0, len(definition.Steps)),
		TotalSteps:      len(definition.Steps),
		SecurityProfile: req.Security,
	}

	// Analyze each step
	for _, step := range definition.Steps {
		dryStep := DryRunStep{
			ID:   step.ID,
			Name: step.Name,
			Type: string(step.Type),
		}

		// For LLM steps, include model information
		if step.Type == workflow.StepTypeLLM {
			// Use override model if specified, otherwise step model
			if req.Model != "" {
				dryStep.Model = req.Model
			} else if step.Model != "" {
				dryStep.Model = step.Model
			}

			// Provider would need to be resolved from configuration
			// For now, use override if specified
			if req.Provider != "" {
				dryStep.Provider = req.Provider
			}

			// Rough token estimate (placeholder - real implementation would be more sophisticated)
			if step.Prompt != "" {
				// Simple heuristic: ~4 chars per token
				dryStep.EstimatedTokens = len(step.Prompt) / 4
			}
		}

		plan.Steps = append(plan.Steps, dryStep)
	}

	// Estimated duration (rough heuristic)
	if len(definition.Steps) > 0 {
		// Assume ~5 seconds per step on average
		estimatedSeconds := len(definition.Steps) * 5
		plan.EstimatedDuration = fmt.Sprintf("%ds", estimatedSeconds)
	}

	// Create a snapshot representing the dry-run result
	now := time.Now()
	snapshot := &RunSnapshot{
		ID:          fmt.Sprintf("dry-run-%d", now.Unix()),
		WorkflowID:  definition.Name,
		Workflow:    definition.Name,
		Status:      RunStatusDryRun,
		Inputs:      req.Inputs,
		CreatedAt:   now,
		Workspace:   req.Workspace,
		Profile:     req.Profile,
		Provider:    req.Provider,
		Model:       req.Model,
		Timeout:     req.Timeout,
		Security:    req.Security,
		AllowHosts:  req.AllowHosts,
		AllowPaths:  req.AllowPaths,
		MCPDev:      req.MCPDev,
	}

	// Attach the plan as output
	snapshot.Output = map[string]any{
		"plan": plan,
	}

	return snapshot, nil
}

// DryRunWithOptions performs an enhanced dry-run with optional deep analysis.
func (r *Runner) DryRunWithOptions(ctx context.Context, req SubmitRequest, opts DryRunOptions) (*RunSnapshot, error) {
	// Get workflow YAML
	var workflowYAML []byte
	var err error

	if req.RemoteRef != "" {
		r.mu.RLock()
		fetcher := r.fetcher
		r.mu.RUnlock()

		if fetcher == nil {
			return nil, fmt.Errorf("remote workflows not supported (fetcher not configured)")
		}

		result, err := fetcher.Fetch(ctx, req.RemoteRef, req.NoCache)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch remote workflow: %w", err)
		}
		workflowYAML = result.Content
	} else {
		workflowYAML = req.WorkflowYAML
	}

	// Parse workflow definition
	definition, err := workflow.ParseDefinition(workflowYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	// Build dry-run plan
	plan := &DryRunPlan{
		Steps:           make([]DryRunStep, 0, len(definition.Steps)),
		TotalSteps:      len(definition.Steps),
		SecurityProfile: req.Security,
		Warnings:        []string{},
		ValidationErrors: []string{},
	}

	// Create template context for expansion
	var templateCtx *workflow.TemplateContext
	if opts.Deep {
		templateCtx = workflow.NewTemplateContext()
		// Add provided inputs to template context
		for k, v := range req.Inputs {
			templateCtx.SetInput(k, v)
		}
	}

	// Track estimated total cost
	totalCostUSD := 0.0

	// Analyze each step
	for _, step := range definition.Steps {
		dryStep := DryRunStep{
			ID:          step.ID,
			Name:        step.Name,
			Type:        string(step.Type),
			WillExecute: true, // Assume execution unless condition says otherwise
		}

		// Deep template expansion
		if opts.Deep && templateCtx != nil {
			dryStep.ExpandedPrompt = expandTemplateWithPlaceholders(step.Prompt, templateCtx)
			dryStep.ExpandedSystem = expandTemplateWithPlaceholders(step.System, templateCtx)

			// Mask secrets in expanded templates
			dryStep.ExpandedPrompt = maskSecrets(dryStep.ExpandedPrompt)
			dryStep.ExpandedSystem = maskSecrets(dryStep.ExpandedSystem)
		}

		// Condition evaluation
		if opts.ShowConditions && step.Condition != nil {
			condResult := evaluateStepCondition(step.Condition.Expression, templateCtx)
			dryStep.ConditionResult = condResult
			dryStep.WillExecute = condResult.Result
		}

		// Reference validation
		if opts.ValidateRefs {
			issues := validateStepReferences(ctx, &step)
			dryStep.ValidationIssues = issues
			if len(issues) > 0 {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("Step %s: %v", step.ID, issues))
			}
		}

		// For LLM steps, include model information and cost estimates
		if step.Type == workflow.StepTypeLLM {
			if req.Model != "" {
				dryStep.Model = req.Model
			} else if step.Model != "" {
				dryStep.Model = step.Model
			}

			if req.Provider != "" {
				dryStep.Provider = req.Provider
			}

			// Token estimate
			if step.Prompt != "" {
				dryStep.EstimatedTokens = estimateTokens(step.Prompt, step.System)
			}

			// Cost estimation
			if opts.EstimateCost {
				costEst := estimateStepCost(step, dryStep.Model, dryStep.EstimatedTokens)
				totalCostUSD += costEst.EstimatedCost
				if plan.DetailedCosts == nil {
					plan.DetailedCosts = []StepCostEstimate{}
				}
				plan.DetailedCosts = append(plan.DetailedCosts, costEst)
			}
		}

		plan.Steps = append(plan.Steps, dryStep)
	}

	// Set total cost
	if opts.EstimateCost && totalCostUSD > 0 {
		plan.EstimatedCost = fmt.Sprintf("$%.4f", totalCostUSD)
	}

	// Estimated duration
	if len(definition.Steps) > 0 {
		estimatedSeconds := len(definition.Steps) * 5
		plan.EstimatedDuration = fmt.Sprintf("%ds", estimatedSeconds)
	}

	// Create snapshot
	now := time.Now()
	snapshot := &RunSnapshot{
		ID:          fmt.Sprintf("dry-run-%d", now.Unix()),
		WorkflowID:  definition.Name,
		Workflow:    definition.Name,
		Status:      RunStatusDryRun,
		Inputs:      req.Inputs,
		CreatedAt:   now,
		Workspace:   req.Workspace,
		Profile:     req.Profile,
		Provider:    req.Provider,
		Model:       req.Model,
		Timeout:     req.Timeout,
		Security:    req.Security,
		AllowHosts:  req.AllowHosts,
		AllowPaths:  req.AllowPaths,
		MCPDev:      req.MCPDev,
	}

	snapshot.Output = map[string]any{
		"plan": plan,
	}

	return snapshot, nil
}

// expandTemplateWithPlaceholders expands templates, leaving [DYNAMIC: field] for unavailable values.
func expandTemplateWithPlaceholders(templateStr string, ctx *workflow.TemplateContext) string {
	if templateStr == "" || ctx == nil {
		return templateStr
	}

	// Try to resolve template
	result, err := workflow.ResolveTemplate(templateStr, ctx)
	if err != nil {
		// If template resolution fails, mark unresolvable parts
		return markDynamicFields(templateStr)
	}

	// Check for <no value> which indicates missing template variables
	if strings.Contains(result, "<no value>") {
		return markDynamicFields(templateStr)
	}

	return result
}

// markDynamicFields marks template expressions that couldn't be resolved.
func markDynamicFields(templateStr string) string {
	// Pattern to find {{...}} template expressions
	re := regexp.MustCompile(`\{\{[^}]+\}\}`)
	return re.ReplaceAllStringFunc(templateStr, func(match string) string {
		// Extract field name from {{.field}} or {{.steps.foo.bar}}
		fieldName := strings.TrimSpace(strings.Trim(match, "{}"))
		fieldName = strings.TrimPrefix(fieldName, ".")
		return fmt.Sprintf("[DYNAMIC: %s]", fieldName)
	})
}

// maskSecrets redacts secrets from expanded templates.
func maskSecrets(s string) string {
	// Patterns for common secret formats
	patterns := []struct {
		pattern string
		replace string
	}{
		{`sk-[a-zA-Z0-9]{20,}`, "[REDACTED-OPENAI-KEY]"},
		{`ghp_[a-zA-Z0-9]{36}`, "[REDACTED-GITHUB-TOKEN]"},
		{`AIzaSy[a-zA-Z0-9_-]{33}`, "[REDACTED-API-KEY]"},
		{`sk_live_[a-zA-Z0-9]{24,}`, "[REDACTED-STRIPE-KEY]"},
		{`Bearer\s+[a-zA-Z0-9_-]{20,}`, "[REDACTED-BEARER-TOKEN]"},
	}

	result := s
	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		result = re.ReplaceAllString(result, p.replace)
	}

	return result
}

// evaluateStepCondition evaluates a step condition expression.
func evaluateStepCondition(expr string, ctx *workflow.TemplateContext) *ConditionResult {
	if expr == "" {
		return &ConditionResult{
			Expression: expr,
			Result:     true, // No condition means always execute
		}
	}

	// For dry-run, we do a simple string-based evaluation
	// In reality, this would use a proper expression evaluator
	result := &ConditionResult{
		Expression: expr,
		Result:     true, // Default to true for dry-run
	}

	// Try basic template expansion if context available
	if ctx != nil {
		expanded, err := workflow.ResolveTemplate(expr, ctx)
		if err != nil {
			result.Error = err.Error()
			result.Result = false
			return result
		}

		// Simple boolean conversion
		result.Result = expanded != "false" && expanded != "0" && expanded != ""
	}

	return result
}

// validateStepReferences validates external references in a step.
func validateStepReferences(ctx context.Context, step *workflow.StepDefinition) []string {
	var issues []string

	// Check file references in step definition
	if step.Type == "file" {
		// File operations would be validated here
		// For now, we'll skip complex validation as StepDefinition doesn't expose all action-specific fields
	}

	// Check HTTP URLs if present in inputs
	if step.Type == "http" {
		// Check if URL is in inputs map
		if urlVal, ok := step.Inputs["url"]; ok {
			if urlStr, ok := urlVal.(string); ok && urlStr != "" {
				if _, err := url.Parse(urlStr); err != nil {
					issues = append(issues, fmt.Sprintf("invalid URL: %s", err))
				} else {
					// Try HEAD request to check accessibility (with short timeout)
					headCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()

					req, err := http.NewRequestWithContext(headCtx, "HEAD", urlStr, nil)
					if err == nil {
						resp, err := http.DefaultClient.Do(req)
						if err != nil {
							issues = append(issues, fmt.Sprintf("URL not accessible: %s", err))
						} else {
							resp.Body.Close()
							if resp.StatusCode >= 400 {
								issues = append(issues, fmt.Sprintf("URL returned %d", resp.StatusCode))
							}
						}
					}
				}
			}
		}
	}

	return issues
}

// estimateTokens provides a rough token count estimate for prompts.
func estimateTokens(prompt, system string) int {
	// Simple heuristic: ~4 characters per token
	totalChars := len(prompt) + len(system)
	return totalChars / 4
}

// estimateStepCost estimates the cost for a single LLM step.
func estimateStepCost(step workflow.StepDefinition, model string, tokens int) StepCostEstimate {
	est := StepCostEstimate{
		StepID:         step.ID,
		StepName:       step.Name,
		TokensEstimate: tokens,
		Basis:          "model_pricing",
	}

	// Simple cost model based on common pricing
	// In production, this would query actual pricing tables
	costPer1kTokens := 0.002 // Default GPT-3.5 pricing

	if strings.Contains(strings.ToLower(model), "gpt-4") {
		costPer1kTokens = 0.03
	} else if strings.Contains(strings.ToLower(model), "claude") {
		costPer1kTokens = 0.008
	}

	est.EstimatedCost = float64(tokens) / 1000.0 * costPer1kTokens

	return est
}
