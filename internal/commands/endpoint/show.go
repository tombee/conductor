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
	"github.com/tombee/conductor/internal/client"
	"github.com/tombee/conductor/internal/commands/shared"
)

// NewShowCommand creates the endpoint show command.
func NewShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show endpoint details",
		Long: `Display detailed configuration and metadata for a specific API endpoint.

The output includes the workflow file, default inputs, scopes, rate limits,
and timeout settings.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(args[0])
		},
	}
}

func runShow(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := c.Get(ctx, "/v1/endpoints/"+name)
	if err != nil {
		return fmt.Errorf("failed to get endpoint: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	// Display endpoint details in human-readable format
	fmt.Printf("Name:        %s\n", resp["name"])
	fmt.Printf("Workflow:    %s\n", resp["workflow"])

	if desc, ok := resp["description"].(string); ok && desc != "" {
		fmt.Printf("Description: %s\n", desc)
	}

	// Display default inputs if any
	if inputs, ok := resp["inputs"].(map[string]any); ok && len(inputs) > 0 {
		fmt.Println("\nDefault Inputs:")
		for key, value := range inputs {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}

	// Display scopes
	if scopes, ok := resp["scopes"].([]any); ok && len(scopes) > 0 {
		scopeStrs := make([]string, len(scopes))
		for i, s := range scopes {
			scopeStrs[i] = s.(string)
		}
		fmt.Printf("\nScopes:      %s\n", strings.Join(scopeStrs, ", "))
	}

	// Display rate limit
	if rateLimit, ok := resp["rate_limit"].(string); ok && rateLimit != "" {
		fmt.Printf("Rate Limit:  %s\n", rateLimit)
	}

	// Display timeout
	if timeout, ok := resp["timeout"].(string); ok && timeout != "" && timeout != "0s" {
		fmt.Printf("Timeout:     %s\n", timeout)
	}

	// Display public flag
	if public, ok := resp["public"].(bool); ok && public {
		fmt.Printf("Public:      %v (no authentication required)\n", public)
	}

	// Display usage example
	fmt.Println("\nUsage:")
	fmt.Printf("  curl -X POST http://localhost:9000/v1/endpoints/%s/runs \\\n", name)
	fmt.Println("    -H \"Authorization: Bearer $API_KEY\" \\")
	fmt.Println("    -d '{}'")

	return nil
}
