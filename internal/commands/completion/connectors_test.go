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

func TestCompleteConnectorNames(t *testing.T) {
	completions, directive := CompleteConnectorNames(nil, nil, "")

	if len(completions) != 4 {
		t.Errorf("expected 4 connector names, got %d", len(completions))
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Verify expected connectors are present with descriptions
	expectedConnectors := map[string]bool{
		"file":      false,
		"shell":     false,
		"transform": false,
		"utility":   false,
	}

	for _, comp := range completions {
		parts := strings.Split(comp, "\t")
		if len(parts) != 2 {
			t.Errorf("completion %q should have exactly 2 parts (name and description)", comp)
			continue
		}
		name := parts[0]
		desc := parts[1]

		if _, ok := expectedConnectors[name]; ok {
			expectedConnectors[name] = true
			if desc == "" {
				t.Errorf("connector %q should have a description", name)
			}
		}
	}

	for connector, found := range expectedConnectors {
		if !found {
			t.Errorf("expected connector %q not found in completions", connector)
		}
	}
}

func TestCompleteConnectorOperations(t *testing.T) {
	completions, directive := CompleteConnectorOperations(nil, nil, "")

	if len(completions) == 0 {
		t.Fatal("expected at least one connector operation")
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Verify format: "connector.operation\tdescription"
	foundFile := false
	foundShell := false
	foundTransform := false
	foundUtility := false

	for _, comp := range completions {
		parts := strings.Split(comp, "\t")
		if len(parts) != 2 {
			t.Errorf("completion %q should have exactly 2 parts (operation and description)", comp)
			continue
		}

		operation := parts[0]
		desc := parts[1]

		// Check format is "connector.operation"
		if !strings.Contains(operation, ".") {
			t.Errorf("operation %q should be in format 'connector.operation'", operation)
			continue
		}

		opParts := strings.SplitN(operation, ".", 2)
		if len(opParts) != 2 {
			t.Errorf("operation %q should have exactly one dot separator", operation)
			continue
		}

		connector := opParts[0]
		opName := opParts[1]

		if connector == "" || opName == "" {
			t.Errorf("both connector and operation name should be non-empty in %q", operation)
		}

		if desc == "" {
			t.Errorf("operation %q should have a description", operation)
		}

		// Track which connectors we've seen
		switch connector {
		case "file":
			foundFile = true
		case "shell":
			foundShell = true
		case "transform":
			foundTransform = true
		case "utility":
			foundUtility = true
		}
	}

	// Verify we have operations from all connectors
	if !foundFile {
		t.Error("expected at least one file.* operation")
	}
	if !foundShell {
		t.Error("expected at least one shell.* operation")
	}
	if !foundTransform {
		t.Error("expected at least one transform.* operation")
	}
	if !foundUtility {
		t.Error("expected at least one utility.* operation")
	}
}

func TestCompleteConnectorOperations_SpecificOperations(t *testing.T) {
	completions, _ := CompleteConnectorOperations(nil, nil, "")

	// Verify some specific operations exist
	expectedOps := map[string]bool{
		"file.read":          false,
		"file.write":         false,
		"shell.run":          false,
		"transform.parse_json": false,
		"utility.random_int": false,
	}

	for _, comp := range completions {
		parts := strings.Split(comp, "\t")
		if len(parts) < 1 {
			continue
		}
		operation := parts[0]
		if _, ok := expectedOps[operation]; ok {
			expectedOps[operation] = true
		}
	}

	for op, found := range expectedOps {
		if !found {
			t.Errorf("expected operation %q not found in completions", op)
		}
	}
}
