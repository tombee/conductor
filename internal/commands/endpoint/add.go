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

package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/cli/prompt"
	"github.com/tombee/conductor/internal/client"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewAddCommand creates the endpoint add command.
func NewAddCommand() *cobra.Command {
	var (
		workflow    string
		description string
		inputs      []string
		scopes      []string
		rateLimit   string
		timeout     string
		public      bool
		interactive bool
	)

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Add a new API endpoint",
		Long: `Add a new API endpoint that exposes a workflow.

Direct mode (for scripts/automation):
  conductor endpoint add review-pr \
    --workflow code-review.yaml \
    --description "Review pull requests" \
    --input branch=main \
    --input personas=security,performance \
    --scope code-ops \
    --rate-limit 10/minute

Interactive mode (recommended for beginners):
  conductor endpoint add

The interactive mode will prompt you for all required fields.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			if len(args) > 0 {
				name = args[0]
			}

			// If no name and no workflow specified, assume interactive mode
			if name == "" && workflow == "" {
				interactive = true
			}

			if interactive {
				return runAddInteractive()
			}

			// Direct mode requires name and workflow
			if name == "" {
				return fmt.Errorf("endpoint name is required in direct mode")
			}
			if workflow == "" {
				return fmt.Errorf("--workflow is required in direct mode")
			}

			return runAddDirect(name, workflow, description, inputs, scopes, rateLimit, timeout, public)
		},
	}

	cmd.Flags().StringVar(&workflow, "workflow", "", "Workflow file to execute (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description of this endpoint")
	cmd.Flags().StringArrayVar(&inputs, "input", []string{}, "Default input key=value (can be specified multiple times)")
	cmd.Flags().StringArrayVar(&scopes, "scope", []string{}, "Allowed scopes (can be specified multiple times)")
	cmd.Flags().StringVar(&rateLimit, "rate-limit", "", "Rate limit (e.g., 10/minute, 100/hour)")
	cmd.Flags().StringVar(&timeout, "timeout", "", "Execution timeout (e.g., 5m, 30s)")
	cmd.Flags().BoolVar(&public, "public", false, "Make endpoint public (no authentication required)")
	cmd.Flags().BoolVarP(&interactive, "interactive", "i", false, "Use interactive mode")

	return cmd
}

func runAddDirect(name, workflow, description string, inputs, scopes []string, rateLimit, timeout string, public bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Parse inputs into map
	inputMap := make(map[string]any)
	for _, input := range inputs {
		parts := strings.SplitN(input, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid input format %q, expected key=value", input)
		}
		inputMap[parts[0]] = parts[1]
	}

	// Build request payload
	payload := map[string]any{
		"name":     name,
		"workflow": workflow,
	}
	if description != "" {
		payload["description"] = description
	}
	if len(inputMap) > 0 {
		payload["inputs"] = inputMap
	}
	if len(scopes) > 0 {
		payload["scopes"] = scopes
	}
	if rateLimit != "" {
		payload["rate_limit"] = rateLimit
	}
	if timeout != "" {
		payload["timeout"] = timeout
	}
	if public {
		payload["public"] = public
	}

	resp, err := c.Post(ctx, "/v1/admin/endpoints", payload)
	if err != nil {
		return fmt.Errorf("failed to create endpoint: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	fmt.Printf("✓ Created endpoint '%s'\n\n", name)
	fmt.Println("Test it with:")
	fmt.Printf("  curl -X POST http://localhost:9000/v1/endpoints/%s/runs \\\n", name)
	fmt.Println("    -H \"Authorization: Bearer $API_KEY\" \\")
	fmt.Println("    -d '{}'")

	return nil
}

func runAddInteractive() error {
	ctx := context.Background()
	p := prompt.NewSurveyPrompter(true)

	if !p.IsInteractive() {
		return fmt.Errorf("interactive mode not available (not a TTY or in CI environment)")
	}

	fmt.Println("Add a new API endpoint")
	fmt.Println()

	// Prompt for endpoint name
	name, err := p.PromptString(ctx, "Endpoint name", "Unique name for this endpoint", "")
	if err != nil {
		return fmt.Errorf("failed to get endpoint name: %w", err)
	}
	if name == "" {
		return fmt.Errorf("endpoint name is required")
	}

	// Prompt for workflow file
	workflow, err := p.PromptString(ctx, "Workflow file", "Workflow file to execute (e.g., code-review.yaml)", "")
	if err != nil {
		return fmt.Errorf("failed to get workflow file: %w", err)
	}
	if workflow == "" {
		return fmt.Errorf("workflow file is required")
	}

	// Prompt for description (optional)
	description, err := p.PromptString(ctx, "Description", "Description of this endpoint (optional)", "")
	if err != nil {
		return fmt.Errorf("failed to get description: %w", err)
	}

	// Prompt for default inputs (optional, repeating)
	fmt.Println()
	fmt.Println("Default inputs (key=value, empty to finish):")
	inputMap := make(map[string]any)
	for {
		input, err := p.PromptString(ctx, "", "> ", "")
		if err != nil {
			return fmt.Errorf("failed to get input: %w", err)
		}
		if input == "" {
			break
		}
		parts := strings.SplitN(input, "=", 2)
		if len(parts) != 2 {
			fmt.Println("  Invalid format, use key=value")
			continue
		}
		inputMap[parts[0]] = parts[1]
	}

	// Prompt for rate limit (optional)
	fmt.Println()
	rateLimit, err := p.PromptString(ctx, "Rate limit", "Rate limit (e.g., 10/minute, 100/hour) or empty for none", "")
	if err != nil {
		return fmt.Errorf("failed to get rate limit: %w", err)
	}

	// Prompt for scopes (optional)
	fmt.Println()
	scopesStr, err := p.PromptString(ctx, "Scopes", "Allowed scopes (comma-separated) or empty for all", "")
	if err != nil {
		return fmt.Errorf("failed to get scopes: %w", err)
	}
	var scopes []string
	if scopesStr != "" {
		scopes = strings.Split(scopesStr, ",")
		for i := range scopes {
			scopes[i] = strings.TrimSpace(scopes[i])
		}
	}

	// Create the endpoint via Admin API
	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	payload := map[string]any{
		"name":     name,
		"workflow": workflow,
	}
	if description != "" {
		payload["description"] = description
	}
	if len(inputMap) > 0 {
		payload["inputs"] = inputMap
	}
	if len(scopes) > 0 {
		payload["scopes"] = scopes
	}
	if rateLimit != "" {
		payload["rate_limit"] = rateLimit
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := c.Post(ctx, "/v1/admin/endpoints", payload)
	if err != nil {
		return fmt.Errorf("failed to create endpoint: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	fmt.Println()
	fmt.Printf("✓ Created endpoint '%s'\n\n", name)
	fmt.Println("Test it with:")
	fmt.Printf("  curl -X POST http://localhost:9000/v1/endpoints/%s/runs \\\n", name)
	fmt.Println("    -H \"Authorization: Bearer $API_KEY\" \\")
	fmt.Println("    -d '{}'")

	return nil
}
