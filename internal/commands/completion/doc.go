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

// Package completion provides shell auto-completion functionality for the Conductor CLI.
//
// This package includes:
//   - Shell completion script generators (bash, zsh, fish, PowerShell)
//   - Dynamic completion functions for workflow files, run IDs, provider names, etc.
//   - Secure configuration loading with permission validation
//   - Caching for performance-critical completions
//
// All completion functions follow a consistent pattern:
//   - Silent failure on errors (return empty list)
//   - Timeout protection for network calls
//   - No credential exposure in output
//   - Panic recovery wrappers
package completion
