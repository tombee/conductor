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

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)
// Test command

var mcpTestKeep bool

func newMCPTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <name>",
		Short: "Test MCP server connectivity",
		Long: `Test an MCP server by starting it and verifying it responds correctly.

The test will:
1. Start the server (if not already running)
2. Send initialize request
3. Verify MCP protocol handshake
4. List available tools
5. Stop the server (unless --keep flag is used)

Examples:
  conductor mcp test github
  conductor mcp test my-server --keep`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPTest(args[0], mcpTestKeep)
		},
	}

	cmd.Flags().BoolVar(&mcpTestKeep, "keep", false, "Keep the server running after test")

	return cmd
}

func runMCPTest(name string, keep bool) error {
	client := newMCPAPIClient()
	ctx := context.Background()

	fmt.Printf("Testing MCP server: %s\n\n", name)

	// Step 1: Check if server exists and get status
	fmt.Print("1. Checking server configuration... ")
	data, err := client.get(ctx, "/v1/mcp/servers/"+name)
	if err != nil {
		fmt.Println("FAILED")
		return err
	}

	var server struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &server); err != nil {
		fmt.Println("FAILED")
		return fmt.Errorf("failed to parse server info: %w", err)
	}
	fmt.Println("OK")

	// Step 2: Start server if not running
	wasRunning := server.Status == "running"
	if !wasRunning {
		fmt.Print("2. Starting server... ")
		if _, err := client.post(ctx, "/v1/mcp/servers/"+name+"/start", nil); err != nil {
			fmt.Println("FAILED")
			return err
		}
		fmt.Println("OK")

		// Wait a bit for server to initialize
		time.Sleep(500 * time.Millisecond)
	} else {
		fmt.Println("2. Server already running... OK")
	}

	// Step 3: Health check (ping)
	fmt.Print("3. Checking health (ping)... ")
	healthData, err := client.get(ctx, "/v1/mcp/servers/"+name+"/health")
	if err != nil {
		fmt.Println("FAILED")
		if !wasRunning && !keep {
			_, _ = client.post(ctx, "/v1/mcp/servers/"+name+"/stop", nil)
		}
		return err
	}

	var health struct {
		Status    string `json:"status"`
		LatencyMs int64  `json:"latency_ms"`
	}
	if err := json.Unmarshal(healthData, &health); err != nil {
		fmt.Println("FAILED")
		if !wasRunning && !keep {
			_, _ = client.post(ctx, "/v1/mcp/servers/"+name+"/stop", nil)
		}
		return err
	}

	if health.Status != "healthy" {
		fmt.Printf("UNHEALTHY (%s)\n", health.Status)
		if !wasRunning && !keep {
			_, _ = client.post(ctx, "/v1/mcp/servers/"+name+"/stop", nil)
		}
		return fmt.Errorf("server health check failed: %s", health.Status)
	}
	fmt.Printf("OK (%dms)\n", health.LatencyMs)

	// Step 4: List tools
	fmt.Print("4. Listing tools... ")
	toolsData, err := client.get(ctx, "/v1/mcp/servers/"+name+"/tools")
	if err != nil {
		fmt.Println("FAILED")
		if !wasRunning && !keep {
			_, _ = client.post(ctx, "/v1/mcp/servers/"+name+"/stop", nil)
		}
		return err
	}

	var toolsResp struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(toolsData, &toolsResp); err != nil {
		fmt.Println("FAILED")
		if !wasRunning && !keep {
			_, _ = client.post(ctx, "/v1/mcp/servers/"+name+"/stop", nil)
		}
		return err
	}
	fmt.Printf("OK (%d tools found)\n", len(toolsResp.Tools))

	if len(toolsResp.Tools) > 0 {
		fmt.Println("\n   Available tools:")
		for _, t := range toolsResp.Tools {
			desc := t.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}
			fmt.Printf("   - %s: %s\n", t.Name, desc)
		}
	}

	// Step 5: Stop server if we started it (unless --keep)
	if !wasRunning && !keep {
		fmt.Print("\n5. Stopping server... ")
		if _, err := client.post(ctx, "/v1/mcp/servers/"+name+"/stop", nil); err != nil {
			fmt.Println("FAILED")
			return err
		}
		fmt.Println("OK")
	} else if keep {
		fmt.Println("\n5. Keeping server running (--keep flag)")
	} else {
		fmt.Println("\n5. Server was already running, leaving it running")
	}

	fmt.Printf("\nTest PASSED for MCP server: %s\n", name)
	return nil
}

