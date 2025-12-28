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
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompleteSecurityModes(t *testing.T) {
	completions, directive := CompleteSecurityModes(nil, nil, "")

	if len(completions) != 4 {
		t.Errorf("expected 4 security modes, got %d", len(completions))
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Verify expected modes
	expectedModes := map[string]bool{
		"unrestricted": false,
		"standard":     false,
		"strict":       false,
		"air-gapped":   false,
	}

	for _, comp := range completions {
		parts := strings.Split(comp, "\t")
		if len(parts) < 1 {
			continue
		}
		mode := parts[0]
		if _, ok := expectedModes[mode]; ok {
			expectedModes[mode] = true
		}
	}

	for mode, found := range expectedModes {
		if !found {
			t.Errorf("expected security mode %q not found", mode)
		}
	}
}

func TestCompleteRunStatus(t *testing.T) {
	completions, directive := CompleteRunStatus(nil, nil, "")

	if len(completions) != 5 {
		t.Errorf("expected 5 run statuses, got %d", len(completions))
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Verify expected statuses
	expectedStatuses := map[string]bool{
		"pending":   false,
		"running":   false,
		"completed": false,
		"failed":    false,
		"cancelled": false,
	}

	for _, comp := range completions {
		parts := strings.Split(comp, "\t")
		if len(parts) < 1 {
			continue
		}
		status := parts[0]
		if _, ok := expectedStatuses[status]; ok {
			expectedStatuses[status] = true
		}
	}

	for status, found := range expectedStatuses {
		if !found {
			t.Errorf("expected run status %q not found", status)
		}
	}
}

func TestCompleteSecretsBackend(t *testing.T) {
	completions, directive := CompleteSecretsBackend(nil, nil, "")

	if len(completions) != 3 {
		t.Errorf("expected 3 secrets backends, got %d", len(completions))
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Verify expected backends
	expectedBackends := map[string]bool{
		"env":      false,
		"keychain": false,
		"file":     false,
	}

	for _, comp := range completions {
		parts := strings.Split(comp, "\t")
		if len(parts) < 1 {
			continue
		}
		backend := parts[0]
		if _, ok := expectedBackends[backend]; ok {
			expectedBackends[backend] = true
		}
	}

	for backend, found := range expectedBackends {
		if !found {
			t.Errorf("expected secrets backend %q not found", backend)
		}
	}
}

func TestCompleteMCPTemplates(t *testing.T) {
	completions, directive := CompleteMCPTemplates(nil, nil, "")

	if len(completions) < 3 {
		t.Errorf("expected at least 3 MCP templates, got %d", len(completions))
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Verify some expected templates exist
	foundFilesystem := false
	foundGitHub := false
	foundCustom := false

	for _, comp := range completions {
		parts := strings.Split(comp, "\t")
		if len(parts) < 1 {
			continue
		}
		template := parts[0]
		switch template {
		case "filesystem":
			foundFilesystem = true
		case "github":
			foundGitHub = true
		case "custom":
			foundCustom = true
		}
	}

	if !foundFilesystem {
		t.Error("expected filesystem template not found")
	}
	if !foundGitHub {
		t.Error("expected github template not found")
	}
	if !foundCustom {
		t.Error("expected custom template not found")
	}
}

func TestFlagCompletions_HaveDescriptions(t *testing.T) {
	testCases := []struct {
		name string
		fn   func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)
	}{
		{"SecurityModes", CompleteSecurityModes},
		{"RunStatus", CompleteRunStatus},
		{"SecretsBackend", CompleteSecretsBackend},
		{"MCPTemplates", CompleteMCPTemplates},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			completions, _ := tc.fn(nil, nil, "")

			for _, comp := range completions {
				if !strings.Contains(comp, "\t") {
					t.Errorf("%s completion %q should have a description separated by tab", tc.name, comp)
				}
			}
		})
	}
}
