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

package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/cli/prompt"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	internalllm "github.com/tombee/conductor/internal/llm"
	"github.com/tombee/conductor/pkg/workflow"

	// Import connector package to register action registry factory
	_ "github.com/tombee/conductor/internal/connector"
)

// runWorkflowLocal executes a workflow locally (not via daemon)
func runWorkflowLocal(workflowPath string, inputArgs []string, inputFile string, dryRun, quiet, verbose, noInteractive, helpInputs, acceptUnenforceablePermissions bool) error {
	// Resolve workflow path
	resolvedPath, err := shared.ResolveWorkflowPath(workflowPath)
	if err != nil {
		return shared.NewInvalidWorkflowError("failed to resolve workflow path", err)
	}

	// Read workflow file
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return shared.NewInvalidWorkflowError("failed to read workflow file", err)
	}

	// Parse and validate workflow
	def, err := workflow.ParseDefinition(data)
	if err != nil {
		return shared.NewInvalidWorkflowError("failed to parse workflow", err)
	}

	// Load config for provider resolution
	cfg, err := loadConfig()
	if err != nil {
		return shared.NewProviderError("failed to load config", err)
	}

	// Handle --help-inputs flag
	if helpInputs {
		showWorkflowInputs(def)
		return nil
	}

	// Parse input arguments
	inputs, err := parseInputs(inputArgs, inputFile)
	if err != nil {
		return shared.NewMissingInputError("failed to parse inputs", err)
	}

	// Analyze inputs and collect missing ones interactively
	analyzer := prompt.NewInputAnalyzer(def.Inputs, inputs)
	missing := analyzer.FindMissingInputs()

	if len(missing) > 0 {
		// Check if interactive mode is allowed
		interactive := isInteractiveModeAllowed(noInteractive)

		if !interactive {
			// Non-interactive mode: fail with structured error
			errMsg := formatMissingInputsError(missing)
			return shared.NewMissingInputNonInteractiveError(errMsg, nil)
		}

		// Interactive mode: prompt for missing inputs
		if !quiet {
			fmt.Fprintf(os.Stderr, "\nMissing required inputs. Please provide:\n\n")
		}

		// Create prompter and collector
		prompter := prompt.NewSurveyPrompter(true)
		collector := prompt.NewInputCollector(prompter)

		// Convert missing inputs to prompt configs
		configs := make([]prompt.PromptConfig, len(missing))
		for i, m := range missing {
			inputType := prompt.InputType(m.Type)
			// Handle enum detection
			if len(m.Enum) > 0 {
				inputType = prompt.InputTypeEnum
			}
			configs[i] = prompt.PromptConfig{
				Name:        m.Name,
				Description: m.Description,
				Type:        inputType,
				Required:    m.Required,
				Default:     m.Default,
				Options:     m.Enum,
			}
		}

		// Collect inputs interactively
		ctx := context.Background()
		collected, err := collector.CollectInputs(ctx, configs)
		if err != nil {
			return shared.NewMissingInputError("failed to collect inputs", err)
		}

		// Merge collected inputs with provided inputs
		for k, v := range collected {
			inputs[k] = v
		}
	}

	// Recreate analyzer with merged inputs and apply defaults
	analyzer = prompt.NewInputAnalyzer(def.Inputs, inputs)
	inputs = analyzer.ApplyDefaults()

	// Resolve providers for each step
	resolver := newProviderResolver(cfg, quiet, verbose)
	plan, err := resolver.resolvePlan(def)
	if err != nil {
		return shared.NewProviderError("provider resolution failed", err)
	}

	// Show warnings if any (before execution plan)
	if !quiet && len(plan.Warnings) > 0 {
		for _, warning := range plan.Warnings {
			fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Show execution plan
	if verbose || dryRun {
		fmt.Println("Execution Plan:")
		for i, step := range plan.Steps {
			fmt.Printf("  %d. %s (%s)\n", i+1, step.ID, step.Type)
			if step.ProviderName != "" {
				fmt.Printf("     Provider: %s (%s)\n", step.ProviderName, step.ProviderType)
				if step.ModelTier != "" {
					fmt.Printf("     Model: %s -> %s\n", step.ModelTier, step.ResolvedModel)
				}
			}
		}
		fmt.Println()
	}

	if dryRun {
		if shared.GetJSON() {
			// Return dry-run result in JSON format
			type dryRunResponse struct {
				shared.JSONResponse
				Plan struct {
					Steps int `json:"steps"`
				} `json:"plan"`
			}

			resp := dryRunResponse{
				JSONResponse: shared.JSONResponse{
					Version: "1.0",
					Command: "run",
					Success: true,
				},
			}
			resp.Plan.Steps = len(plan.Steps)

			return emitJSON(resp)
		}
		fmt.Println("Dry run complete. No workflow executed.")
		return nil
	}

	// Get workflow directory for action resolution
	workflowDir := filepath.Dir(resolvedPath)

	// Execute the workflow
	return executeWorkflow(def, cfg, plan, inputs, workflowDir, quiet, verbose, acceptUnenforceablePermissions)
}

// executeWorkflow runs the workflow using the step executor
func executeWorkflow(def *workflow.Definition, cfg *config.Config, plan *ExecutionPlan, inputs map[string]interface{}, workflowDir string, quiet, verbose, acceptUnenforceablePermissions bool) error {
	ctx := context.Background()
	startTime := time.Now()

	if !quiet {
		fmt.Printf("Running workflow: %s\n", def.Name)
		if verbose {
			fmt.Printf("Steps: %d\n", len(def.Steps))
		}
		fmt.Println()
	}

	// Get the default provider name
	providerName := cfg.DefaultProvider
	if providerName == "" {
		return shared.NewProviderError("no default provider configured", nil)
	}

	// Warn if using unsupported provider
	if providerCfg, exists := cfg.Providers[providerName]; exists {
		config.WarnUnsupportedProvider(providerCfg.Type)
	}

	// Create the LLM provider
	llmProvider, err := internalllm.CreateProvider(cfg, providerName)
	if err != nil {
		return shared.NewProviderError("failed to create LLM provider", err)
	}

	// Wrap the provider with our adapter
	adapter := internalllm.NewProviderAdapter(llmProvider)

	// Create the step executor with actions initialized
	executor := workflow.NewExecutor(nil, adapter).
		WithWorkflowDir(workflowDir)

	// Build template context from inputs
	templateCtx := workflow.NewTemplateContext()
	for k, v := range inputs {
		templateCtx.SetInput(k, v)
	}

	// Build workflow context
	workflowContext := map[string]interface{}{
		"inputs":           inputs,
		"steps":            make(map[string]interface{}),
		"_templateContext": templateCtx,
	}

	// Execute each step in sequence
	var lastOutput map[string]interface{}
	for i, step := range def.Steps {
		stepStart := time.Now()

		if !quiet {
			fmt.Printf("[%d/%d] %s", i+1, len(def.Steps), step.ID)
			if step.Name != "" {
				fmt.Printf(" (%s)", step.Name)
			}
			fmt.Print("...")
			if verbose {
				fmt.Println()
			}
		}

		// Execute the step
		result, err := executor.Execute(ctx, &step, workflowContext)

		stepDuration := time.Since(stepStart)

		if err != nil {
			if !quiet {
				fmt.Printf(" FAILED (%s)\n", stepDuration.Round(time.Millisecond))
				fmt.Printf("  Error: %s\n", err.Error())
			}

			// Handle based on error strategy
			if step.OnError != nil && step.OnError.Strategy == workflow.ErrorStrategyIgnore {
				if !quiet && verbose {
					fmt.Println("  (error ignored per step configuration)")
				}
				continue
			}

			return shared.NewExecutionError(fmt.Sprintf("step %s failed", step.ID), err)
		}

		// Update workflow context with step results
		if result != nil && result.Output != nil {
			workflowContext["steps"].(map[string]interface{})[step.ID] = result.Output
			templateCtx.SetStepOutput(step.ID, result.Output)
			lastOutput = result.Output
		}

		if !quiet {
			if result.Status == workflow.StepStatusSkipped {
				fmt.Printf(" SKIPPED (%s)\n", stepDuration.Round(time.Millisecond))
			} else {
				fmt.Printf(" OK (%s)\n", stepDuration.Round(time.Millisecond))
			}

			// Show step output preview in verbose mode
			if verbose && result.Output != nil {
				if response, ok := result.Output["response"].(string); ok {
					preview := response
					if len(preview) > 200 {
						preview = preview[:200] + "..."
					}
					fmt.Printf("  Output: %s\n", strings.ReplaceAll(preview, "\n", " "))
				}
			}
		}
	}

	totalDuration := time.Since(startTime)

	// Handle JSON output mode
	if shared.GetJSON() {
		type runResponse struct {
			shared.JSONResponse
			RunID   string                 `json:"run_id"`
			Outputs map[string]interface{} `json:"outputs"`
			Stats   struct {
				DurationMS int     `json:"duration_ms"`
				TokensIn   int     `json:"tokens_in"`
				TokensOut  int     `json:"tokens_out"`
				CostUSD    float64 `json:"cost_usd"`
			} `json:"stats"`
		}

		// Build outputs from workflow definition
		outputs := make(map[string]interface{})
		for _, outputDef := range def.Outputs {
			// Try to resolve the output value from step results
			if value, err := workflow.ResolveTemplate(outputDef.Value, templateCtx); err == nil && value != "" {
				outputs[outputDef.Name] = value
			} else if lastOutput != nil {
				// Fallback to last step output
				if response, ok := lastOutput["response"]; ok {
					outputs[outputDef.Name] = response
				}
			}
		}

		resp := runResponse{
			JSONResponse: shared.JSONResponse{
				Version: "1.0",
				Command: "run",
				Success: true,
			},
			RunID:   fmt.Sprintf("run-%d", time.Now().Unix()),
			Outputs: outputs,
		}
		resp.Stats.DurationMS = int(totalDuration.Milliseconds())
		// Token/cost tracking not yet implemented for CLI mode

		return emitJSON(resp)
	}

	// Display final output
	if !quiet {
		fmt.Println()
		fmt.Println("---")
		fmt.Printf("Workflow completed in %s\n", totalDuration.Round(time.Millisecond))

		// Show outputs
		if len(def.Outputs) > 0 && lastOutput != nil {
			fmt.Println()
			for _, outputDef := range def.Outputs {
				var value string
				if resolved, err := workflow.ResolveTemplate(outputDef.Value, templateCtx); err == nil && resolved != "" {
					value = resolved
				} else if response, ok := lastOutput["response"].(string); ok {
					value = response
				}
				if value != "" {
					// Print output with proper formatting
					fmt.Printf("%s:\n", outputDef.Name)
					fmt.Println(value)
				}
			}
		} else if lastOutput != nil {
			// No defined outputs, show the last step's response
			if response, ok := lastOutput["response"].(string); ok {
				fmt.Println()
				fmt.Println("Result:")
				fmt.Println(response)
			}
		}
	}

	return nil
}

// loadConfig loads the configuration file
func loadConfig() (*config.Config, error) {
	// Try XDG config directory first
	configPath, err := config.ConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no configuration found. Run 'conductor init' to set up")
	}

	return config.Load(configPath)
}

// emitJSON marshals a response to JSON and outputs it to stdout
func emitJSON(response interface{}) error {
	return shared.EmitJSON(response)
}

// ProviderResolver resolves providers for workflow steps
type ProviderResolver struct {
	cfg     *config.Config
	quiet   bool
	verbose bool
}

// newProviderResolver creates a new provider resolver
func newProviderResolver(cfg *config.Config, quiet, verbose bool) *ProviderResolver {
	return &ProviderResolver{
		cfg:     cfg,
		quiet:   quiet,
		verbose: verbose,
	}
}

// resolvePlan resolves providers for all steps in the workflow
func (r *ProviderResolver) resolvePlan(def *workflow.Definition) (*ExecutionPlan, error) {
	plan := &ExecutionPlan{
		Steps:    make([]ResolvedStep, 0, len(def.Steps)),
		Warnings: make([]string, 0),
	}

	for _, step := range def.Steps {
		resolved := ResolvedStep{
			ID:   step.ID,
			Type: step.Type,
		}

		// Only resolve provider for LLM steps
		if step.Type == workflow.StepTypeLLM {
			providerName, err := r.resolveProvider(step, def)
			if err != nil {
				return nil, fmt.Errorf("step %s: %w", step.ID, err)
			}

			resolved.ProviderName = providerName

			// Get provider config
			providerCfg, exists := r.cfg.Providers[providerName]
			if !exists {
				return nil, fmt.Errorf("step %s: provider %q not found in config", step.ID, providerName)
			}
			resolved.ProviderType = providerCfg.Type

			// Resolve model tier
			if step.Inputs != nil {
				if modelTier, ok := step.Inputs["model"].(string); ok {
					resolved.ModelTier = modelTier
					resolved.ResolvedModel = r.resolveModelTier(modelTier, providerCfg)
				}
			}

			// Check for unmapped agents
			if step.Agent != "" {
				if agent, exists := def.Agents[step.Agent]; exists {
					if warning := r.checkUnmappedAgent(step.Agent, agent, providerName); warning != "" {
						plan.Warnings = append(plan.Warnings, warning)
					}
				}
			}
		}

		plan.Steps = append(plan.Steps, resolved)
	}

	return plan, nil
}

// resolveProvider resolves the provider for a workflow step
func (r *ProviderResolver) resolveProvider(step workflow.StepDefinition, def *workflow.Definition) (string, error) {
	// 1. Agent mapping lookup
	if step.Agent != "" {
		if providerName, exists := r.cfg.AgentMappings[step.Agent]; exists {
			return providerName, nil
		}
	}

	// 2. CONDUCTOR_PROVIDER environment variable
	if envProvider := os.Getenv("CONDUCTOR_PROVIDER"); envProvider != "" {
		if _, exists := r.cfg.Providers[envProvider]; exists {
			return envProvider, nil
		}
		return "", fmt.Errorf("CONDUCTOR_PROVIDER=%q not found in configured providers", envProvider)
	}

	// 3. default_provider from config
	if r.cfg.DefaultProvider != "" {
		return r.cfg.DefaultProvider, nil
	}

	// 4. Auto-detection fallback (not implemented yet)
	return "", fmt.Errorf("no provider configured. Run 'conductor init' to set up a provider")
}

// resolveModelTier resolves an abstract model tier to a provider-specific model
func (r *ProviderResolver) resolveModelTier(tier string, providerCfg config.ProviderConfig) string {
	// Check custom model mappings first
	switch tier {
	case "fast":
		if providerCfg.Models.Fast != "" {
			return providerCfg.Models.Fast
		}
	case "balanced":
		if providerCfg.Models.Balanced != "" {
			return providerCfg.Models.Balanced
		}
	case "strategic":
		if providerCfg.Models.Strategic != "" {
			return providerCfg.Models.Strategic
		}
	}

	// Use provider-specific defaults
	return r.getDefaultModel(tier, providerCfg.Type)
}

// getDefaultModel returns the default model for a tier and provider type
func (r *ProviderResolver) getDefaultModel(tier, providerType string) string {
	defaults := map[string]map[string]string{
		"claude-code": {
			"fast":      "haiku",
			"balanced":  "sonnet",
			"strategic": "opus",
		},
		"anthropic": {
			"fast":      "haiku",
			"balanced":  "sonnet",
			"strategic": "opus",
		},
		"openai": {
			"fast":      "gpt-4o-mini",
			"balanced":  "gpt-4o",
			"strategic": "o1",
		},
	}

	if models, exists := defaults[providerType]; exists {
		if model, ok := models[tier]; ok {
			return model
		}
	}

	// Fallback
	return tier
}

// checkUnmappedAgent checks if an agent should generate a warning
func (r *ProviderResolver) checkUnmappedAgent(agentName string, agent workflow.AgentDefinition, usedProvider string) string {
	// Check if agent mapping is explicitly configured
	if _, isMapped := r.cfg.AgentMappings[agentName]; isMapped {
		return "" // Explicitly mapped, no warning
	}

	// Check if acknowledged
	for _, ack := range r.cfg.AcknowledgedDefaults {
		if ack == agentName {
			return "" // Acknowledged default use
		}
	}

	// Check global suppression
	if r.cfg.SuppressUnmappedWarnings {
		return ""
	}

	// Generate warning message
	var parts []string

	if agent.Prefers != "" {
		parts = append(parts, fmt.Sprintf("Agent '%s' prefers '%s'", agentName, agent.Prefers))
	}

	if len(agent.Capabilities) > 0 {
		capStr := strings.Join(agent.Capabilities, ", ")
		parts = append(parts, fmt.Sprintf("requires capabilities [%s]", capStr))
	}

	if len(parts) == 0 {
		return "" // No preferences or capabilities, no warning needed
	}

	msg := strings.Join(parts, " and ")
	msg += fmt.Sprintf(" but no mapping configured. Using default provider '%s'.", usedProvider)
	msg += fmt.Sprintf("\n  Add to config to suppress: agent_mappings.%s: <provider>", agentName)

	return msg
}
