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
Package daemon provides the core conductord daemon server.

The daemon is Conductor's persistent server process that executes workflows,
manages scheduled runs, handles webhooks, and provides a REST API for workflow
orchestration.

# Architecture

The Daemon struct is the central component that orchestrates all subsystems:

  - Runner: Executes workflows with concurrency control and checkpointing
  - Backend: Persists run state (memory for dev, PostgreSQL for production)
  - Scheduler: Triggers workflows on cron schedules
  - Webhook Router: Processes incoming webhooks from GitHub, Slack, etc.
  - MCP Registry: Manages Model Context Protocol server lifecycles
  - Auth Middleware: Validates API keys for secure access
  - Leader Elector: Coordinates distributed deployments

# Usage

Create and start a daemon:

	cfg, _ := config.Load()
	d, err := daemon.New(cfg, daemon.Options{
	    Version: "1.0.0",
	})
	if err != nil {
	    log.Fatal(err)
	}

	// Start blocks until context is cancelled
	go func() {
	    if err := d.Start(ctx); err != nil {
	        log.Fatal(err)
	    }
	}()

	// Graceful shutdown
	d.Shutdown(context.Background())

# Subpackages

The daemon package has several subpackages:

  - api: HTTP handlers for the REST API
  - auth: API key authentication middleware
  - backend: Run persistence (memory, postgres)
  - checkpoint: Checkpoint management for run recovery
  - leader: Leader election for distributed mode
  - listener: Network listener setup (Unix socket, TCP)
  - runner: Workflow execution engine
  - scheduler: Cron-based workflow scheduling
  - webhook: Webhook processing and routing

# Configuration

The daemon is configured via [config.Config], which supports:

  - Listen address (Unix socket or TCP)
  - Backend type (memory or postgres)
  - Concurrency limits
  - Webhook routes
  - Schedule definitions
  - Authentication settings
*/
package daemon
