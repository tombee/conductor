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

func TestCompleteExampleNames(t *testing.T) {
	completions, directive := CompleteExampleNames(nil, nil, "")

	if len(completions) == 0 {
		t.Fatal("expected at least one example completion")
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}

	// Verify format: "name\tdescription"
	for _, comp := range completions {
		if !strings.Contains(comp, "\t") {
			t.Errorf("completion %q should contain tab-separated description", comp)
		}
		parts := strings.Split(comp, "\t")
		if len(parts) != 2 {
			t.Errorf("completion %q should have exactly 2 parts (name and description)", comp)
		}
		if parts[0] == "" {
			t.Error("example name should not be empty")
		}
		if parts[1] == "" {
			t.Error("example description should not be empty")
		}
	}
}

func TestCompleteExampleNames_FiltersPrefix(t *testing.T) {
	// Note: Cobra handles prefix filtering, but we can test that all completions
	// are returned for the shell to filter
	completions, directive := CompleteExampleNames(nil, nil, "hello")

	if len(completions) == 0 {
		t.Fatal("expected at least one example completion")
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}
}

func TestCompleteExampleNames_EmptyPrefix(t *testing.T) {
	completions, directive := CompleteExampleNames(nil, nil, "")

	if len(completions) == 0 {
		t.Fatal("expected at least one example completion")
	}

	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got %v", directive)
	}
}
