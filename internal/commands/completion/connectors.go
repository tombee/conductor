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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/operation"
)

// CompleteConnectorNames provides completion for builtin connector names.
// Returns names with descriptions as hints.
func CompleteConnectorNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		connectorNames := []string{"file", "shell", "transform", "utility"}

		completions := make([]string, 0, len(connectorNames))
		for _, name := range connectorNames {
			desc := operation.GetBuiltinDescription(name)
			completions = append(completions, name+"\t"+desc)
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	})
}

// CompleteConnectorOperations provides completion for connector operations.
// Returns operations in the format "connector.operation" with descriptions.
func CompleteConnectorOperations(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		connectorNames := []string{"file", "shell", "transform", "utility"}

		var completions []string

		// For each connector, add all its operations
		for _, connectorName := range connectorNames {
			operations := operation.GetBuiltinOperations(connectorName)
			for _, op := range operations {
				// Format: "connector.operation\tdescription"
				completion := fmt.Sprintf("%s.%s\t%s operation", connectorName, op, connectorName)
				completions = append(completions, completion)
			}
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	})
}
