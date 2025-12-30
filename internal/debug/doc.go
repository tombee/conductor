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

// Package debug provides workflow debugging capabilities for Conductor.
//
// The debug package enables step-by-step execution with breakpoints,
// state inspection, and interactive debugging shells. It is designed
// to help developers understand workflow behavior and troubleshoot issues.
//
// # Debug Adapter
//
// The Adapter wraps workflow execution to intercept step transitions
// and implement breakpoint logic. It communicates with a debugger shell
// via channels for events and commands.
//
// # Debug Configuration
//
// Config holds debugging settings including breakpoint locations and
// log level overrides. Configurations are validated against workflow
// definitions to ensure breakpoints reference valid steps.
//
// # Event Protocol
//
// Events are emitted during execution to notify the debugger of state
// changes (step start, paused, resumed, completed, etc.). Commands are
// sent from the debugger to control execution flow (continue, next,
// skip, abort, inspect).
//
// # Example Usage
//
//	cfg := debug.New([]string{"step1", "step2"}, "debug")
//	adapter := debug.NewAdapter(cfg, logger)
//
//	// Start debugger shell in a goroutine
//	go func() {
//		shell := debug.NewShell(adapter)
//		shell.Run()
//	}()
//
//	// Execute workflow with debug adapter
//	err := executor.ExecuteWorkflow(ctx, wf, adapter)
package debug
