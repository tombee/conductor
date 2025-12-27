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

// NewListCommand creates the endpoint list command.
func NewListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available API endpoints",
		Long: `List all configured API endpoints that can be called to execute workflows.

Each endpoint exposes a workflow under a named API route.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList()
		},
	}
}

func runList() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	c, err := client.FromEnvironment()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := c.Get(ctx, "/v1/endpoints")
	if err != nil {
		return fmt.Errorf("failed to list endpoints: %w", err)
	}

	if shared.GetJSON() {
		return json.NewEncoder(os.Stdout).Encode(resp)
	}

	endpoints, ok := resp["endpoints"].([]any)
	if !ok {
		endpoints = []any{}
	}

	if len(endpoints) == 0 {
		fmt.Println("No endpoints configured")
		fmt.Println("\nCreate one with: conductor endpoint add")
		return nil
	}

	fmt.Println("NAME            WORKFLOW           RATE LIMIT    SCOPES")
	fmt.Println("--------------- ------------------ ------------- ---------------")
	for _, e := range endpoints {
		ep := e.(map[string]any)
		name := ep["name"].(string)
		workflow := ep["workflow"].(string)

		rateLimit := "-"
		if rl, ok := ep["rate_limit"].(string); ok && rl != "" {
			rateLimit = rl
		}

		scopesStr := "-"
		if scopes, ok := ep["scopes"].([]any); ok && len(scopes) > 0 {
			scopeStrs := make([]string, len(scopes))
			for i, s := range scopes {
				scopeStrs[i] = s.(string)
			}
			scopesStr = strings.Join(scopeStrs, ", ")
		}

		fmt.Printf("%-15s %-18s %-13s %s\n",
			truncate(name, 15),
			truncate(workflow, 18),
			truncate(rateLimit, 13),
			truncate(scopesStr, 15))
	}

	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
