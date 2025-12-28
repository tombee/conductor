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
	"github.com/tombee/conductor/internal/examples"
)

// CompleteExampleNames provides completion for example workflow names.
// Returns example names with descriptions as hints.
func CompleteExampleNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		examplesList, err := examples.List()
		if err != nil || len(examplesList) == 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		completions := make([]string, 0, len(examplesList))
		for _, ex := range examplesList {
			// Format: "name\tdescription"
			completions = append(completions, ex.Name+"\t"+ex.Description)
		}

		return completions, cobra.ShellCompDirectiveNoFileComp
	})
}
