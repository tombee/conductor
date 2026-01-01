package sdk_test

import (
	"context"
	"fmt"
	"log"
	"os"

	conductorsdk "github.com/tombee/conductor/sdk"
)

// Example_builtinActions demonstrates using builtin actions (file, shell, http, etc).
func Example_builtinActions() {
	s, err := conductorsdk.New(
		conductorsdk.WithBuiltinActions(), // Enable file, shell, http, transform, utility
	)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// Workflows can now use builtin actions
	// (Full workflow execution will be implemented in future phases)
	fmt.Println("SDK created with builtin actions enabled")
	// Output: SDK created with builtin actions enabled
}

// Example_builtinIntegrations demonstrates using builtin integrations (GitHub, Slack, etc).
func Example_builtinIntegrations() {
	s, err := conductorsdk.New(
		conductorsdk.WithBuiltinIntegrations(), // Enable GitHub, Slack, Jira, etc
	)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// Workflows can now use builtin integrations
	// Credentials are provided at runtime via RunOptions
	fmt.Println("SDK created with builtin integrations enabled")
	// Output: SDK created with builtin integrations enabled
}

// Example_customTool demonstrates creating and registering a custom tool.
func Example_customTool() {
	s, err := conductorsdk.New()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// Create a custom tool using FuncTool helper
	weatherTool := conductorsdk.FuncTool(
		"get_weather",
		"Get current weather for a location",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]any{
					"type":        "string",
					"description": "City name or coordinates",
				},
			},
			"required": []string{"location"},
		},
		func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			location := inputs["location"].(string)
			// In a real implementation, this would call a weather API
			return map[string]any{
				"temperature": 72,
				"conditions":  "sunny",
				"location":    location,
			}, nil
		},
	)

	// Register the tool
	if err := s.RegisterTool(weatherTool); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Custom tool registered:", weatherTool.Name())
	// Output: Custom tool registered: get_weather
}

// Example_loadWorkflowYAML demonstrates loading workflows from YAML files.
func Example_loadWorkflowYAML() {
	s, err := conductorsdk.New()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// Load workflow from YAML bytes
	yamlContent := []byte(`
name: greeting-workflow
inputs:
  - name: user_name
    type: string
steps:
  - id: greet
    model: claude-sonnet-4-20250514
    prompt: "Greet {{.inputs.user_name}} warmly"
`)

	wf, err := s.LoadWorkflow(yamlContent)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Workflow loaded:", wf.Name)
	// Output: Workflow loaded: greeting-workflow
}

// Example_loadWorkflowFile demonstrates loading workflows from files.
func Example_loadWorkflowFile() {
	s, err := conductorsdk.New()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// Create a temporary workflow file
	tmpfile, err := os.CreateTemp("", "workflow-*.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := []byte(`
name: file-workflow
steps:
  - id: hello
    model: claude-sonnet-4-20250514
    prompt: "Say hello"
`)

	if _, err := tmpfile.Write(content); err != nil {
		log.Fatal(err)
	}
	tmpfile.Close()

	// Load workflow from file
	wf, err := s.LoadWorkflowFile(tmpfile.Name())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Workflow loaded from file:", wf.Name)
	// Output: Workflow loaded from file: file-workflow
}

// Example_loadWorkflowWithWarnings demonstrates handling platform-specific features.
func Example_loadWorkflowWithWarnings() {
	s, err := conductorsdk.New()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// Load a workflow that has platform-only features
	yamlContent := []byte(`
name: platform-workflow
trigger:
  webhook:
    path: /trigger
steps:
  - id: process
    model: claude-sonnet-4-20250514
    prompt: "Process the request"
`)

	wf, warnings, err := s.LoadWorkflowWithWarnings(yamlContent)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Workflow:", wf.Name)
	fmt.Println("Warnings:", len(warnings) > 0)
	// Output:
	// Workflow: platform-workflow
	// Warnings: true
}

// Example_extendWorkflow demonstrates extending YAML workflows with programmatic steps.
func Example_extendWorkflow() {
	s, err := conductorsdk.New()
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// Load base workflow from YAML
	baseYAML := []byte(`
name: base-workflow
inputs:
  - name: topic
    type: string
steps:
  - id: research
    model: claude-sonnet-4-20250514
    prompt: "Research {{.inputs.topic}}"
`)

	baseWf, err := s.LoadWorkflow(baseYAML)
	if err != nil {
		log.Fatal(err)
	}

	// Extend it with additional programmatic steps
	extendedWf, err := s.ExtendWorkflow(baseWf).
		Step("summarize").LLM().
		Model("claude-sonnet-4-20250514").
		Prompt("Summarize the research findings").
		DependsOn("research").
		Done().
		Build()

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Extended workflow:", extendedWf.Name)
	fmt.Println("Total steps:", extendedWf.StepCount())
	// Output:
	// Extended workflow: base-workflow
	// Total steps: 2
}

// Example_mcpServer demonstrates configuring MCP servers.
func Example_mcpServer() {
	config := conductorsdk.MCPConfig{
		Transport:      "stdio",
		Command:        "mcp-server-gh",
		Args:           []string{"--token", "ghp_..."},
		ConnectTimeout: 5000,
		RequestTimeout: 30000,
	}

	s, err := conductorsdk.New(
		conductorsdk.WithMCPServer("github", config),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	// MCP servers can be connected at runtime
	// (Full MCP integration will be implemented in future phases)
	fmt.Println("SDK created with MCP server configured")
	// Output: SDK created with MCP server configured
}
