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

/*
Package client provides an HTTP client for the conductor controller API.

This package enables CLI commands and other tools to communicate with the
conductor controller over its REST API. It supports both Unix socket and TCP
connections, with automatic controller startup if configured.

# Basic Usage

Create a client and make requests:

	c, err := client.New()
	if err != nil {
	    log.Fatal(err)
	}

	// Trigger a workflow
	run, err := c.Trigger(ctx, "my-workflow.yaml", map[string]any{
	    "input": "value",
	})

	// Get run status
	status, err := c.GetRun(ctx, run.ID)

	// List runs
	runs, err := c.ListRuns(ctx, client.ListRunsRequest{
	    Status: "running",
	})

# Connection Options

Configure the client with options:

	// Use API key authentication
	c, _ := client.New(client.WithAPIKey("my-api-key"))

	// Use custom transport (e.g., for testing)
	c, _ := client.New(client.WithTransport(customTransport))

	// Use custom HTTP client
	c, _ := client.New(client.WithHTTPClient(httpClient))

# Transport

The default transport connects via Unix socket:

	~/.local/state/conductor/conductor.sock  (Linux)
	~/Library/Application Support/conductor/conductor.sock  (macOS)

Override with CONDUCTOR_HOST environment variable:

	export CONDUCTOR_HOST=http://localhost:8080

# Auto-Start

When the controller isn't running and auto-start is configured, the client
attempts to start the controller automatically:

	// Ensure controller is running (starts it if needed)
	c, err := client.EnsureController(client.AutoStartConfig{
	    Enabled: true,
	})
	if err != nil {
	    log.Fatal(err)
	}

Platform-specific implementations in autostart_*.go handle controller spawning.

# API Methods

The client provides methods matching the controller's REST API:

  - Trigger: Submit a workflow for execution
  - GetRun: Get run status by ID
  - ListRuns: List runs with optional filtering
  - CancelRun: Cancel a running workflow
  - Health: Check controller health
  - Version: Get controller version info
*/
package client
