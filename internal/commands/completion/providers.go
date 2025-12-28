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

package completion

import (
	"github.com/spf13/cobra"
)

// CompleteProviderNames provides completion for provider names from the config.
// Returns names of all configured providers.
func CompleteProviderNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		cfg, err := LoadConfigForCompletion()
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if len(cfg.Providers) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		names := make([]string, 0, len(cfg.Providers))
		for name := range cfg.Providers {
			names = append(names, name)
		}

		return names, cobra.ShellCompDirectiveNoFileComp
	})
}

// CompleteProviderTypes provides completion for provider type values.
// Returns the list of all provider types (claude-code, anthropic, openai, ollama).
func CompleteProviderTypes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		// Return all provider types for completion
		// Users can use any type, even if experimental
		types := []string{
			"claude-code",
			"anthropic",
			"openai",
			"ollama",
		}

		return types, cobra.ShellCompDirectiveNoFileComp
	})
}
