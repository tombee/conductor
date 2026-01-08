package harness

import (
	"path/filepath"
	"testing"
)

func TestYAMLFixtures(t *testing.T) {
	fixtures := []string{
		"simple_llm.yaml",
		"multi_step.yaml",
		"with_actions.yaml",
		"error_handling.yaml",
		"tool_calling.yaml",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			h := New(t)
			path := filepath.Join("../testdata", fixture)

			// Attempt to load the workflow
			wf := h.LoadWorkflow(path)

			if wf == nil {
				t.Fatal("LoadWorkflow returned nil")
			}

			if wf.Name == "" {
				t.Error("workflow name is empty")
			}

			t.Logf("Successfully loaded workflow: %s", wf.Name)
		})
	}
}
