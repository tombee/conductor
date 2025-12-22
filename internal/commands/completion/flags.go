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

// CompleteSecurityModes provides completion for --security flag values.
func CompleteSecurityModes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		modes := []string{
			"unrestricted\tNo security restrictions",
			"standard\tStandard security policy",
			"strict\tStrict security policy",
			"air-gapped\tAir-gapped mode with no network access",
		}
		return modes, cobra.ShellCompDirectiveNoFileComp
	})
}

// CompleteRunStatus provides completion for --status flag values.
func CompleteRunStatus(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		statuses := []string{
			"pending\tRun is queued",
			"running\tRun is currently executing",
			"completed\tRun finished successfully",
			"failed\tRun failed with an error",
			"cancelled\tRun was cancelled",
		}
		return statuses, cobra.ShellCompDirectiveNoFileComp
	})
}

// CompleteSecretsBackend provides completion for --backend flag values.
func CompleteSecretsBackend(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		backends := []string{
			"env\tEnvironment variables",
			"keychain\tSystem keychain (macOS/Linux)",
			"file\tEncrypted file storage",
		}
		return backends, cobra.ShellCompDirectiveNoFileComp
	})
}

// CompleteMCPTemplates provides completion for --template flag values in mcp init.
func CompleteMCPTemplates(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return SafeCompletionWrapper(func() ([]string, cobra.ShellCompDirective) {
		// Common MCP server templates
		templates := []string{
			"filesystem\tLocal filesystem access",
			"github\tGitHub integration",
			"postgres\tPostgreSQL database",
			"sqlite\tSQLite database",
			"puppeteer\tBrowser automation",
			"fetch\tHTTP requests",
			"custom\tCustom server template",
		}
		return templates, cobra.ShellCompDirectiveNoFileComp
	})
}
