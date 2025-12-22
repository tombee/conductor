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
Package lifecycle manages controller process lifecycle operations.

This package provides secure PID file management, process spawning/validation,
health checking, and lifecycle event logging for the Conductor controller.

# PID File Management

PID files are security-sensitive as they control which process receives shutdown
signals. The package uses exclusive file locking (flock) and atomic creation
(O_EXCL) to prevent race conditions and symlink attacks:

	manager := lifecycle.NewPIDFileManager("/path/to/conductor.pid")
	if err := manager.Create(1234); err != nil {
	    // Handle error
	}
	defer manager.Remove()

# Process Operations

Process validation ensures signals are sent only to conductor controllers,
preventing accidental kills of unrelated processes:

	pid, err := manager.Read()
	if err != nil {
	    // Handle error
	}

	if !lifecycle.IsConductorProcess(pid) {
	    // PID file is stale or corrupted
	}

	if err := lifecycle.SendSignal(pid, syscall.SIGTERM); err != nil {
	    // Handle error
	}

# Health Checking

Health polling uses exponential backoff to wait for controller startup:

	checker := lifecycle.NewHealthChecker("http://localhost:9000/health")
	if err := checker.WaitUntilHealthy(30 * time.Second); err != nil {
	    // Controller failed to start
	}

# Process Spawning

Detached process spawning runs the controller in background mode:

	spawner := lifecycle.NewSpawner()
	pid, err := spawner.SpawnDetached(ctx, "/path/to/conductor", args, logPath)
	if err != nil {
	    // Handle error
	}

# Lifecycle Logging

All lifecycle events are logged for audit purposes:

	logger := lifecycle.NewLifecycleLogger("/path/to/lifecycle.log")
	logger.LogStart("1.0.0", args)
	logger.LogStop(pid, success)
*/
package lifecycle
