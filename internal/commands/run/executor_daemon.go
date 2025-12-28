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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/cli/prompt"
	"github.com/tombee/conductor/internal/client"
	"github.com/tombee/conductor/internal/commands/shared"
	"github.com/tombee/conductor/internal/config"
	"github.com/tombee/conductor/internal/remote"
	"github.com/tombee/conductor/pkg/workflow"
)

// runWorkflowViaDaemon submits a workflow to the daemon for execution
func runWorkflowViaDaemon(workflowPath string, inputArgs []string, inputFile, outputFile string, noStats, background, mcpDev, noCache, quiet, verbose, noInteractive, helpInputs, dryRun bool, provider, model, timeout, workspace, profile, security string, allowHosts, allowPaths []string) error {
	ctx := context.Background()

	// Apply environment variable defaults for workspace and profile
	// CLI flags take precedence over env vars
	if workspace == "" {
		workspace = os.Getenv("CONDUCTOR_WORKSPACE")
	}
	if profile == "" {
		profile = os.Getenv("CONDUCTOR_PROFILE")
	}

	var data []byte
	var isRemote bool

	// Check if this is a remote reference
	if isRemoteWorkflow(workflowPath) {
		isRemote = true
		if !quiet {
			fmt.Fprintf(os.Stderr, "Using remote workflow: %s\n", workflowPath)
		}
		// Remote workflows are fetched by the daemon
		// We'll pass the reference in the request
		data = nil
	} else {
		// Resolve local workflow path
		resolvedPath, err := shared.ResolveWorkflowPath(workflowPath)
		if err != nil {
			return shared.NewInvalidWorkflowError("failed to resolve workflow path", err)
		}

		// Read workflow file
		data, err = os.ReadFile(resolvedPath)
		if err != nil {
			return shared.NewInvalidWorkflowError("failed to read workflow file", err)
		}
	}

	// Parse workflow definition for input analysis (needed for --help-inputs and interactive prompting)
	var def *workflow.Definition
	if data != nil {
		var err error
		def, err = workflow.ParseDefinition(data)
		if err != nil {
			return shared.NewInvalidWorkflowError("failed to parse workflow", err)
		}
	}

	// Handle --help-inputs flag
	if helpInputs && def != nil {
		showWorkflowInputs(def)
		return nil
	}

	// Parse inputs
	inputs, err := parseInputs(inputArgs, inputFile)
	if err != nil {
		return shared.NewMissingInputError("failed to parse inputs", err)
	}

	// For daemon mode, collect missing inputs BEFORE submitting to daemon
	if def != nil {
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
	}

	// Load config for socket path
	cfg, err := loadConfig()
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Auto-start is always enabled - daemon mode is the only mode
	autoStartCfg := client.AutoStartConfig{
		Enabled: true,
	}
	if cfg != nil {
		autoStartCfg.SocketPath = cfg.Daemon.SocketPath
	}

	c, err := client.EnsureDaemon(autoStartCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w\n\nHint: Ensure 'conductord' is in your PATH", err)
	}

	// Submit workflow to daemon
	submitPath := "/v1/runs"
	params := url.Values{}

	// Add inputs as query parameters
	if len(inputs) > 0 {
		for key, value := range inputs {
			params.Add(key, fmt.Sprintf("%v", value))
		}
	}

	// Add remote reference and flags if applicable
	if isRemote {
		params.Add("remote_ref", workflowPath)
		if noCache {
			params.Add("no_cache", "true")
		}
	}

	// Add workspace and profile parameters
	if workspace != "" {
		params.Add("workspace", workspace)
	}
	if profile != "" {
		params.Add("profile", profile)
	}

	// Add runtime override parameters
	if provider != "" {
		params.Add("provider", provider)
	}
	if model != "" {
		params.Add("model", model)
	}
	if timeout != "" {
		params.Add("timeout", timeout)
	}
	if dryRun {
		params.Add("dry_run", "true")
	}
	if security != "" {
		params.Add("security", security)
	}
	for _, host := range allowHosts {
		params.Add("allow_hosts", host)
	}
	for _, path := range allowPaths {
		params.Add("allow_paths", path)
	}
	if mcpDev {
		params.Add("mcp_dev", "true")
	}

	if len(params) > 0 {
		submitPath = fmt.Sprintf("%s?%s", submitPath, params.Encode())
	}

	resp, err := c.PostYAML(ctx, submitPath, data)
	if err != nil {
		return shared.NewProviderError("failed to submit workflow", err)
	}

	// The /v1/runs endpoint returns "id"
	runID, _ := resp["id"].(string)
	if runID == "" {
		return shared.NewExecutionError("daemon did not return run ID", nil)
	}

	// If --background, just print run ID and exit
	if background {
		if shared.GetJSON() {
			return json.NewEncoder(os.Stdout).Encode(map[string]string{"run_id": runID})
		}
		if !quiet {
			fmt.Printf("Workflow submitted. Run ID: %s\n", runID)
			fmt.Println("Check status with: conductor runs show", runID)
		}
		return nil
	}

	// Stream logs and wait for completion
	if !quiet {
		fmt.Printf("Running workflow... (run ID: %s)\n", runID)
	}

	output, stats, err := streamRunLogs(ctx, c, runID, quiet, verbose)
	if err != nil {
		return err
	}

	// Write output to file if requested
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(output), 0600); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		if !quiet {
			fmt.Printf("Output written to %s\n", outputFile)
		}
	}

	// Display statistics unless suppressed
	if !noStats && stats != nil && !quiet {
		displayStats(stats)
	}

	return nil
}

// streamRunLogs streams logs from a run until completion and returns output and stats
func streamRunLogs(ctx context.Context, c *client.Client, runID string, quiet, verbose bool) (string, *RunStats, error) {
	var outputText string
	var stats *RunStats

	// Stream logs via SSE
	resp, err := c.GetStream(ctx, fmt.Sprintf("/v1/runs/%s/logs", runID), "text/event-stream")
	if err != nil {
		// Fall back to polling if SSE not available
		output, s, pollErr := pollRunStatus(ctx, c, runID, quiet)
		return output, s, pollErr
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Parse SSE events
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			var event map[string]any
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			// Handle different event types
			eventType, _ := event["type"].(string)
			switch eventType {
			case "log":
				if !quiet {
					if message, ok := event["message"].(string); ok {
						fmt.Println(message)
					}
				}
			case "step_complete":
				// Display per-step cost information
				if !quiet {
					displayStepCost(event)
				}
				// Update running stats
				if stats == nil {
					stats = &RunStats{
						StepCosts: make(map[string]StepCost),
					}
				}
				updateStatsWithStep(stats, event)
			case "status":
				if status, ok := event["status"].(string); ok {
					if status == "completed" || status == "failed" || status == "cancelled" {
						if !quiet {
							fmt.Printf("\nWorkflow %s\n", status)
						}
						if status == "failed" {
							if errMsg, ok := event["error"].(string); ok {
								return "", stats, shared.NewExecutionError("workflow failed", fmt.Errorf("%s", errMsg))
							}
							return "", stats, shared.NewExecutionError("workflow failed", nil)
						}
						// Fetch final output and stats
						output, s := fetchRunOutput(ctx, c, runID)
						return output, s, nil
					}
				}
			case "output":
				if outputData, ok := event["output"]; ok {
					if outputStr, ok := outputData.(string); ok {
						outputText = outputStr
					}
					if !quiet && verbose {
						outputJSON, _ := json.MarshalIndent(outputData, "", "  ")
						fmt.Printf("Output: %s\n", outputJSON)
					}
				}
			case "stats":
				// Parse statistics if provided
				stats = parseStats(event)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", stats, fmt.Errorf("error reading logs: %w", err)
	}

	return outputText, stats, nil
}

// pollRunStatus polls run status until completion (fallback when SSE unavailable)
func pollRunStatus(ctx context.Context, c *client.Client, runID string, quiet bool) (string, *RunStats, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case <-ticker.C:
			status, err := c.Get(ctx, fmt.Sprintf("/v1/runs/%s", runID))
			if err != nil {
				return "", nil, fmt.Errorf("failed to get run status: %w", err)
			}

			runStatus, _ := status["status"].(string)
			switch runStatus {
			case "completed":
				if !quiet {
					fmt.Println("Workflow completed")
				}
				output, stats := fetchRunOutput(ctx, c, runID)
				return output, stats, nil
			case "failed":
				errMsg, _ := status["error"].(string)
				if errMsg != "" {
					return "", nil, shared.NewExecutionError("workflow failed", fmt.Errorf("%s", errMsg))
				}
				return "", nil, shared.NewExecutionError("workflow failed", nil)
			case "cancelled":
				return "", nil, shared.NewExecutionError("workflow cancelled", nil)
			}
		}
	}
}

// fetchRunOutput fetches the final output and stats for a run
func fetchRunOutput(ctx context.Context, c *client.Client, runID string) (string, *RunStats) {
	// Fetch output
	outputResp, err := c.Get(ctx, fmt.Sprintf("/v1/runs/%s/output", runID))
	if err != nil {
		return "", nil
	}

	var output string
	if outputMap, ok := outputResp["output"].(map[string]any); ok {
		// Try to get the main output field
		if outputStr, ok := outputMap["result"].(string); ok {
			output = outputStr
		} else if outputStr, ok := outputMap["output"].(string); ok {
			output = outputStr
		} else {
			// Marshal the whole output object
			outputJSON, _ := json.MarshalIndent(outputMap, "", "  ")
			output = string(outputJSON)
		}
	}

	// Fetch run details for stats
	runResp, err := c.Get(ctx, fmt.Sprintf("/v1/runs/%s", runID))
	if err != nil {
		return output, nil
	}

	stats := parseStatsFromRun(runResp)
	return output, stats
}

// isRemoteWorkflow checks if a path is a remote workflow reference
func isRemoteWorkflow(path string) bool {
	return remote.IsRemote(path)
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
