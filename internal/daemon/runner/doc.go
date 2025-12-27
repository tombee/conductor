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
Package runner provides the workflow execution engine for the daemon.

The Runner manages the lifecycle of workflow runs, including submission,
execution, cancellation, and state persistence. It handles concurrency
control through semaphores and integrates with checkpointing for recovery.

# Key Types

  - Runner: The main execution manager
  - Run: Represents a single workflow execution
  - RunSnapshot: Immutable view of run state for API responses
  - ExecutionAdapter: Interface for step execution

# Usage

Create a runner with configuration:

	r := runner.New(runner.Config{
	    MaxParallel:    10,
	    DefaultTimeout: 30 * time.Minute,
	}, backend, checkpointManager)

	// Set execution adapter (required for workflow execution)
	r.SetAdapter(executionAdapter)

Submit a workflow:

	snapshot, err := r.Submit(ctx, runner.SubmitRequest{
	    WorkflowYAML: workflowBytes,
	    Inputs:       map[string]any{"name": "value"},
	})

Query runs:

	run, _ := r.Get(runID)
	runs := r.List(runner.ListFilter{Status: runner.RunStatusRunning})

Cancel a run:

	r.Cancel(runID)

# Concurrency Control

The runner uses a semaphore to limit concurrent executions:

  - MaxParallel controls the maximum simultaneous runs
  - Pending runs queue until a slot is available
  - Cancellation is safe even for queued runs

# Checkpointing

Before each step, the runner saves a checkpoint containing:

  - Current step index
  - Workflow context (inputs and step outputs)
  - Run metadata

On daemon restart, interrupted runs can be resumed from checkpoints.

# Remote Workflows

The runner supports remote workflow references (e.g., github:user/repo):

	r.SetFetcher(remoteFetcher)

	snapshot, err := r.Submit(ctx, runner.SubmitRequest{
	    RemoteRef: "github:myorg/workflows/deploy.yaml",
	    NoCache:   false,
	})

# MCP Integration

Workflows can define MCP servers that are started before execution:

  - Servers are started via the MCP Manager
  - Tools are discovered and registered in the tool registry
  - Servers are stopped after workflow completion

# Metrics

Optional metrics collection for observability:

	r.SetMetrics(metricsCollector)

Metrics recorded:

  - Run start/completion with duration
  - Step completion with status
  - Queue depth changes
*/
package runner
