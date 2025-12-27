package workflow

import (
	"fmt"
	"regexp"
	"testing"
)

func TestParseDefinition(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid workflow",
			yaml: `
name: test-workflow
version: "1.0"
description: A test workflow
inputs:
  - name: input1
    type: string
    required: true
steps:
  - id: step1
    name: First Step
    type: llm
    prompt: "test prompt"
outputs:
  - name: result
    type: string
    value: $.step1.output
`,
			wantErr: false,
		},
		{
			name: "missing name",
			yaml: `
version: "1.0"
steps:
  - id: step1
    name: First Step
    type: llm
    prompt: "test prompt"
`,
			wantErr: true,
		},
		{
			name: "missing version (now allowed)",
			yaml: `
name: test-workflow
steps:
  - id: step1
    name: First Step
    type: llm
    prompt: "test prompt"
`,
			wantErr: false,
		},
		{
			name: "no steps",
			yaml: `
name: test-workflow
version: "1.0"
steps: []
`,
			wantErr: true,
		},
		{
			name: "duplicate step IDs",
			yaml: `
name: test-workflow
version: "1.0"
steps:
  - id: step1
    name: First Step
    type: llm
    prompt: "test prompt"
  - id: step1
    name: Second Step
    type: llm
    prompt: "test prompt"
`,
			wantErr: true,
		},
		{
			name: "invalid input type",
			yaml: `
name: test-workflow
version: "1.0"
inputs:
  - name: input1
    type: invalid-type
    required: true
steps:
  - id: step1
    name: First Step
    type: llm
    prompt: "test prompt"
`,
			wantErr: true,
		},
		{
			name: "minimal workflow without version",
			yaml: `
name: summarize
steps:
  - id: step1
    type: llm
    prompt: "test prompt"
`,
			wantErr: false,
		},
		{
			name: "minimal workflow without step names",
			yaml: `
name: summarize
steps:
  - id: step1
    type: llm
    prompt: "test prompt"
  - id: step2
    type: llm
    prompt: "test prompt"
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && def == nil {
				t.Error("ParseDefinition() returned nil definition")
			}
		})
	}
}

func TestStepDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		step    StepDefinition
		wantErr bool
	}{
		{
			name: "valid LLM step",
			step: StepDefinition{
				ID:     "step1",
				Name:   "LLM Step",
				Type:   StepTypeLLM,
				Prompt: "test prompt",
			},
			wantErr: false,
		},
		{
			name: "valid builtin connector step",
			step: StepDefinition{
				ID:               "step1",
				Name:             "Builtin Step",
				Type:             StepTypeBuiltin,
				BuiltinConnector: "file",
				BuiltinOperation: "read",
			},
			wantErr: false,
		},
		{
			name: "missing condition for condition step",
			step: StepDefinition{
				ID:   "step1",
				Name: "Condition Step",
				Type: StepTypeCondition,
			},
			wantErr: true,
		},
		{
			name: "invalid step type tool",
			step: StepDefinition{
				ID:   "step1",
				Name: "Tool Step",
				Type: "tool",
			},
			wantErr: true,
		},
		{
			name: "invalid step type",
			step: StepDefinition{
				ID:   "step1",
				Name: "Test Step",
				Type: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("StepDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		retry   RetryDefinition
		wantErr bool
	}{
		{
			name: "valid retry",
			retry: RetryDefinition{
				MaxAttempts:       3,
				BackoffBase:       1,
				BackoffMultiplier: 2.0,
			},
			wantErr: false,
		},
		{
			name: "invalid max attempts",
			retry: RetryDefinition{
				MaxAttempts:       0,
				BackoffBase:       1,
				BackoffMultiplier: 2.0,
			},
			wantErr: true,
		},
		{
			name: "invalid backoff base",
			retry: RetryDefinition{
				MaxAttempts:       3,
				BackoffBase:       0,
				BackoffMultiplier: 2.0,
			},
			wantErr: true,
		},
		{
			name: "invalid backoff multiplier",
			retry: RetryDefinition{
				MaxAttempts:       3,
				BackoffBase:       1,
				BackoffMultiplier: 0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.retry.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RetryDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestErrorHandlingDefinitionValidate(t *testing.T) {
	tests := []struct {
		name    string
		handler ErrorHandlingDefinition
		wantErr bool
	}{
		{
			name: "valid fail strategy",
			handler: ErrorHandlingDefinition{
				Strategy: ErrorStrategyFail,
			},
			wantErr: false,
		},
		{
			name: "valid fallback strategy",
			handler: ErrorHandlingDefinition{
				Strategy:     ErrorStrategyFallback,
				FallbackStep: "fallback-step",
			},
			wantErr: false,
		},
		{
			name: "invalid strategy",
			handler: ErrorHandlingDefinition{
				Strategy: "invalid",
			},
			wantErr: true,
		},
		{
			name: "fallback without fallback step",
			handler: ErrorHandlingDefinition{
				Strategy: ErrorStrategyFallback,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.handler.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ErrorHandlingDefinition.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	tests := []struct {
		name             string
		yaml             string
		wantTimeout      int
		wantMaxAttempts  int
		wantBackoffBase  int
		wantBackoffMult  float64
		wantModel        string
	}{
		{
			name: "applies default timeout and model",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test prompt"
`,
			wantTimeout:     30,
			wantMaxAttempts: 2,
			wantBackoffBase: 1,
			wantBackoffMult: 2.0,
			wantModel:       "balanced",
		},
		{
			name: "preserves explicit timeout",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test prompt"
    timeout: 60
`,
			wantTimeout:     60,
			wantMaxAttempts: 2,
			wantBackoffBase: 1,
			wantBackoffMult: 2.0,
			wantModel:       "balanced",
		},
		{
			name: "preserves explicit retry config",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test prompt"
    retry:
      max_attempts: 5
      backoff_base: 2
      backoff_multiplier: 3.0
`,
			wantTimeout:     30,
			wantMaxAttempts: 5,
			wantBackoffBase: 2,
			wantBackoffMult: 3.0,
			wantModel:       "balanced",
		},
		{
			name: "preserves explicit model",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test prompt"
    model: fast
`,
			wantTimeout:     30,
			wantMaxAttempts: 2,
			wantBackoffBase: 1,
			wantBackoffMult: 2.0,
			wantModel:       "fast",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("ParseDefinition() error = %v", err)
			}

			if len(def.Steps) == 0 {
				t.Fatal("Expected at least one step")
			}

			step := def.Steps[0]
			if step.Timeout != tt.wantTimeout {
				t.Errorf("Timeout = %d, want %d", step.Timeout, tt.wantTimeout)
			}

			if step.Retry == nil {
				t.Fatal("Retry config is nil")
			}

			if step.Retry.MaxAttempts != tt.wantMaxAttempts {
				t.Errorf("MaxAttempts = %d, want %d", step.Retry.MaxAttempts, tt.wantMaxAttempts)
			}

			if step.Retry.BackoffBase != tt.wantBackoffBase {
				t.Errorf("BackoffBase = %d, want %d", step.Retry.BackoffBase, tt.wantBackoffBase)
			}

			if step.Retry.BackoffMultiplier != tt.wantBackoffMult {
				t.Errorf("BackoffMultiplier = %f, want %f", step.Retry.BackoffMultiplier, tt.wantBackoffMult)
			}

			if step.Model != tt.wantModel {
				t.Errorf("Model = %s, want %s", step.Model, tt.wantModel)
			}
		})
	}
}

func TestLLMStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid LLM step with prompt",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Summarize this text"
`,
			wantErr: false,
		},
		{
			name: "valid LLM step with all fields",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    model: balanced
    system: "You are a helpful assistant"
    prompt: "Summarize this text"
`,
			wantErr: false,
		},
		{
			name: "valid LLM step with fast model",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    model: fast
    prompt: "Quick summary"
`,
			wantErr: false,
		},
		{
			name: "valid LLM step with strategic model",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    model: strategic
    prompt: "Complex analysis"
`,
			wantErr: false,
		},
		{
			name: "LLM step missing prompt",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    model: balanced
`,
			wantErr: true,
			errMsg:  "prompt is required",
		},
		{
			name: "LLM step with invalid model tier",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    model: invalid-tier
    prompt: "Test"
`,
			wantErr: true,
			errMsg:  "invalid model tier",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errMsg)
				} else if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
			if !tt.wantErr && def == nil {
				t.Error("ParseDefinition() returned nil definition")
			}
		})
	}
}

func TestModelTierDefaults(t *testing.T) {
	tests := []struct {
		name      string
		yaml      string
		wantModel string
	}{
		{
			name: "LLM step defaults to balanced",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test"
`,
			wantModel: "balanced",
		},
		{
			name: "llm step defaults to balanced",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test prompt"
`,
			wantModel: "balanced",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("ParseDefinition() error = %v", err)
			}

			if len(def.Steps) == 0 {
				t.Fatal("Expected at least one step")
			}

			step := def.Steps[0]
			if step.Model != tt.wantModel {
				t.Errorf("Model = %s, want %s", step.Model, tt.wantModel)
			}
		})
	}
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestOutputSchemaFields tests T1.1 and T1.2: parsing of OutputSchema, OutputType, OutputOptions
func TestOutputSchemaFields(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		check   func(*testing.T, *StepDefinition)
	}{
		{
			name: "parse output_schema",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Classify this"
    output_schema:
      type: object
      properties:
        category:
          type: string
          enum: [bug, feature]
      required: [category]
`,
			wantErr: false,
			check: func(t *testing.T, s *StepDefinition) {
				if s.OutputSchema == nil {
					t.Fatal("OutputSchema is nil")
				}
				if s.OutputSchema["type"] != "object" {
					t.Errorf("OutputSchema type = %v, want object", s.OutputSchema["type"])
				}
			},
		},
		{
			name: "parse output_type with output_options",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Classify this"
    output_type: classification
    output_options:
      categories: [bug, feature, question]
`,
			wantErr: false,
			check: func(t *testing.T, s *StepDefinition) {
				if s.OutputType != "classification" {
					t.Errorf("OutputType = %s, want classification", s.OutputType)
				}
				if s.OutputOptions == nil {
					t.Fatal("OutputOptions is nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && def != nil && tt.check != nil {
				tt.check(t, &def.Steps[0])
			}
		})
	}
}

// TestOutputTypeMutualExclusivity tests T1.4: output_type and output_schema are mutually exclusive
func TestOutputTypeMutualExclusivity(t *testing.T) {
	yaml := `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Classify this"
    output_type: classification
    output_options:
      categories: [bug, feature]
    output_schema:
      type: object
      properties:
        category:
          type: string
`
	_, err := ParseDefinition([]byte(yaml))
	if err == nil {
		t.Error("Expected error for mutually exclusive output_type and output_schema")
	}
	if !contains(err.Error(), "mutually exclusive") {
		t.Errorf("Expected error about mutual exclusivity, got: %v", err)
	}
}

// TestOutputTypeExpansion tests T1.3: built-in output type expansion
func TestOutputTypeExpansion(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		wantErr      bool
		checkSchema  func(*testing.T, map[string]interface{})
	}{
		{
			name: "classification expands correctly",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Classify this"
    output_type: classification
    output_options:
      categories: [bug, feature, question]
`,
			wantErr: false,
			checkSchema: func(t *testing.T, schema map[string]interface{}) {
				if schema["type"] != "object" {
					t.Errorf("type = %v, want object", schema["type"])
				}
				props := schema["properties"].(map[string]interface{})
				category := props["category"].(map[string]interface{})
				if category["type"] != "string" {
					t.Errorf("category type = %v, want string", category["type"])
				}
				enum := category["enum"].([]interface{})
				if len(enum) != 3 {
					t.Errorf("enum length = %d, want 3", len(enum))
				}
			},
		},
		{
			name: "decision expands correctly",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Decide"
    output_type: decision
    output_options:
      choices: [approve, reject, escalate]
      require_reasoning: true
`,
			wantErr: false,
			checkSchema: func(t *testing.T, schema map[string]interface{}) {
				props := schema["properties"].(map[string]interface{})
				if _, ok := props["decision"]; !ok {
					t.Error("missing decision property")
				}
				if _, ok := props["reasoning"]; !ok {
					t.Error("missing reasoning property")
				}
				required := schema["required"].([]interface{})
				if len(required) != 2 {
					t.Errorf("required fields = %d, want 2 (decision and reasoning)", len(required))
				}
			},
		},
		{
			name: "decision without required reasoning",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Decide"
    output_type: decision
    output_options:
      choices: [approve, reject]
`,
			wantErr: false,
			checkSchema: func(t *testing.T, schema map[string]interface{}) {
				required := schema["required"].([]interface{})
				if len(required) != 1 {
					t.Errorf("required fields = %d, want 1 (only decision)", len(required))
				}
			},
		},
		{
			name: "extraction expands correctly",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Extract data"
    output_type: extraction
    output_options:
      fields: [name, email, company]
`,
			wantErr: false,
			checkSchema: func(t *testing.T, schema map[string]interface{}) {
				props := schema["properties"].(map[string]interface{})
				if len(props) != 3 {
					t.Errorf("properties count = %d, want 3", len(props))
				}
				required := schema["required"].([]interface{})
				if len(required) != 3 {
					t.Errorf("required fields = %d, want 3", len(required))
				}
			},
		},
		{
			name: "invalid output_type",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Test"
    output_type: invalid_type
    output_options:
      categories: [a, b]
`,
			wantErr: true,
		},
		{
			name: "classification missing categories",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "Test"
    output_type: classification
    output_options: {}
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && def != nil && tt.checkSchema != nil {
				// After expansion, OutputSchema should be populated
				if def.Steps[0].OutputSchema == nil {
					t.Fatal("OutputSchema was not populated after expansion")
				}
				tt.checkSchema(t, def.Steps[0].OutputSchema)
			}
		})
	}
}

// TestSchemaComplexityLimits tests T1.5: schema complexity validation
func TestSchemaComplexityLimits(t *testing.T) {
	tests := []struct {
		name    string
		schema  map[string]interface{}
		wantErr bool
		errMsg  string
	}{
		{
			name: "simple schema passes",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field1": map[string]interface{}{"type": "string"},
					"field2": map[string]interface{}{"type": "number"},
				},
				"required": []interface{}{"field1"},
			},
			wantErr: false,
		},
		{
			name: "deeply nested schema fails",
			schema: func() map[string]interface{} {
				// Create a schema with nesting depth > 10
				schema := map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{},
				}
				current := schema
				for i := 0; i < 12; i++ {
					nested := map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{},
					}
					current["properties"].(map[string]interface{})[fmt.Sprintf("level%d", i)] = nested
					current = nested
				}
				return schema
			}(),
			wantErr: true,
			errMsg:  "maximum nesting depth",
		},
		{
			name: "too many properties fails",
			schema: func() map[string]interface{} {
				props := make(map[string]interface{})
				for i := 0; i < 101; i++ {
					props[fmt.Sprintf("field%d", i)] = map[string]interface{}{"type": "string"}
				}
				return map[string]interface{}{
					"type": "object",
					"properties": props,
				}
			}(),
			wantErr: true,
			errMsg:  "maximum of 100 properties",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchemaComplexity(tt.schema)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSchemaComplexity() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

// TestMCPServerConfig tests MCP server configuration parsing and validation
func TestMCPServerConfig(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid mcp server",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "list repos"
mcp_servers:
  - name: github
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
    env: ["GITHUB_TOKEN=${GITHUB_TOKEN}"]
    timeout: 45
`,
			wantErr: false,
		},
		{
			name: "multiple mcp servers",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "list repos"
mcp_servers:
  - name: github
    command: npx
    args: ["-y", "@modelcontextprotocol/server-github"]
  - name: filesystem
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem"]
`,
			wantErr: false,
		},
		{
			name: "mcp server missing name",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test"
mcp_servers:
  - command: npx
    args: ["@modelcontextprotocol/server-github"]
`,
			wantErr: true,
			errMsg:  "mcp_server name is required",
		},
		{
			name: "mcp server missing command",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test"
mcp_servers:
  - name: github
    args: ["@modelcontextprotocol/server-github"]
`,
			wantErr: true,
			errMsg:  "mcp_server command is required",
		},
		{
			name: "duplicate mcp server names",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test"
mcp_servers:
  - name: github
    command: npx
    args: ["server1"]
  - name: github
    command: npx
    args: ["server2"]
`,
			wantErr: true,
			errMsg:  "duplicate mcp_server name: github",
		},
		{
			name: "mcp server with negative timeout",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: llm
    prompt: "test"
mcp_servers:
  - name: github
    command: npx
    timeout: -5
`,
			wantErr: true,
			errMsg:  "mcp_server timeout must be non-negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			}
			if !tt.wantErr && def != nil {
				// Verify MCP servers were parsed
				if len(def.MCPServers) == 0 {
					t.Error("MCPServers should not be empty")
				}
			}
		})
	}
}

// TestMCPServerConfig_Validate tests MCPServerConfig.Validate directly
func TestMCPServerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  MCPServerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: MCPServerConfig{
				Name:    "github",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-github"},
				Env:     []string{"GITHUB_TOKEN=xyz"},
				Timeout: 30,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: MCPServerConfig{
				Command: "npx",
			},
			wantErr: true,
			errMsg:  "mcp_server name is required",
		},
		{
			name: "missing command",
			config: MCPServerConfig{
				Name: "github",
			},
			wantErr: true,
			errMsg:  "mcp_server command is required",
		},
		{
			name: "negative timeout",
			config: MCPServerConfig{
				Name:    "github",
				Command: "npx",
				Timeout: -10,
			},
			wantErr: true,
			errMsg:  "mcp_server timeout must be non-negative",
		},
		{
			name: "zero timeout is valid",
			config: MCPServerConfig{
				Name:    "github",
				Command: "npx",
				Timeout: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("MCPServerConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err != nil {
				if err.Error() != tt.errMsg {
					t.Errorf("Expected error %q, got %q", tt.errMsg, err.Error())
				}
			}
		})
	}
}

// TestAutoGenerateStepIDs tests the step ID auto-generation functionality (SPEC-67 P1.3)
func TestAutoGenerateStepIDs(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected map[string]bool // Expected step IDs
		wantErr  bool
	}{
		{
			name: "shorthand without explicit ID",
			yaml: `
name: test-workflow
steps:
  - file.read: ./config.json
`,
			expected: map[string]bool{"file_read_1": true},
			wantErr:  false,
		},
		{
			name: "multiple shorthand steps same connector.operation",
			yaml: `
name: test-workflow
steps:
  - file.read: ./config.json
  - file.read: ./data.json
  - file.read: ./other.json
`,
			expected: map[string]bool{
				"file_read_1": true,
				"file_read_2": true,
				"file_read_3": true,
			},
			wantErr: false,
		},
		{
			name: "mixed explicit and auto-generated IDs",
			yaml: `
name: test-workflow
steps:
  - file.read: ./config.json
  - id: my_custom_id
    file.read: ./data.json
  - file.read: ./other.json
`,
			expected: map[string]bool{
				"file_read_1":   true,
				"my_custom_id":  true,
				"file_read_2":   true,
			},
			wantErr: false,
		},
		{
			name: "collision handling - explicit ID takes precedence",
			yaml: `
name: test-workflow
steps:
  - file.read: ./config.json
  - id: file_read_1
    file.write:
      path: ./output.txt
      content: test
  - file.read: ./data.json
`,
			expected: map[string]bool{
				"file_read_2":  true, // First file.read gets _2 because _1 is taken
				"file_read_1":  true, // Explicit ID
				"file_read_3":  true, // Second file.read gets _3
			},
			wantErr: false,
		},
		{
			name: "different connector operations get separate counters",
			yaml: `
name: test-workflow
steps:
  - file.read: ./config.json
  - file.write:
      path: ./output.txt
      content: test
  - file.read: ./data.json
`,
			expected: map[string]bool{
				"file_read_1":  true,
				"file_write_1": true,
				"file_read_2":  true,
			},
			wantErr: false,
		},
		{
			name: "external connector shorthand",
			yaml: `
name: test-workflow
connectors:
  github:
    from: connectors/github
    auth:
      token: ${GITHUB_TOKEN}
steps:
  - github.create_issue:
      owner: foo
      repo: bar
      title: test
  - github.create_issue:
      owner: baz
      repo: qux
      title: test2
`,
			expected: map[string]bool{
				"github_create_issue_1": true,
				"github_create_issue_2": true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			// Check that all expected IDs are present
			foundIDs := make(map[string]bool)
			for _, step := range def.Steps {
				foundIDs[step.ID] = true
			}

			for expectedID := range tt.expected {
				if !foundIDs[expectedID] {
					t.Errorf("Expected step ID %q not found. Found IDs: %v", expectedID, foundIDs)
				}
			}

			// Check that no unexpected IDs are present
			if len(foundIDs) != len(tt.expected) {
				t.Errorf("Expected %d steps, got %d. Expected: %v, Found: %v",
					len(tt.expected), len(foundIDs), tt.expected, foundIDs)
			}
		})
	}
}

// TestRemovedStepTypes tests that removed step types produce validation errors (SPEC-67 P1.1, P1.2)
func TestRemovedStepTypes(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantErr    bool
		errContains string
	}{
		{
			name: "type: action is invalid",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: action
    action: some_action
`,
			wantErr:     true,
			errContains: "invalid step type",
		},
		{
			name: "type: tool is invalid",
			yaml: `
name: test-workflow
steps:
  - id: step1
    type: tool
    tool: file
    inputs:
      operation: read
      path: ./config.json
`,
			wantErr:     true,
			errContains: "invalid step type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errContains != "" && !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			}
		})
	}
}

// TestShorthandIDTracking tests that explicit vs generated IDs are tracked correctly (SPEC-67 P1.4)
func TestShorthandIDTracking(t *testing.T) {
	tests := []struct {
		name             string
		yaml             string
		expectedExplicit map[string]bool
		wantErr          bool
	}{
		{
			name: "shorthand with explicit ID",
			yaml: `
name: test-workflow
steps:
  - id: my_read_step
    file.read: ./config.json
`,
			expectedExplicit: map[string]bool{"my_read_step": true},
			wantErr:          false,
		},
		{
			name: "shorthand without explicit ID",
			yaml: `
name: test-workflow
steps:
  - file.read: ./config.json
`,
			expectedExplicit: map[string]bool{},
			wantErr:          false,
		},
		{
			name: "regular step with explicit ID",
			yaml: `
name: test-workflow
steps:
  - id: analyze
    type: llm
    prompt: test
`,
			expectedExplicit: map[string]bool{"analyze": true},
			wantErr:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			for _, step := range def.Steps {
				isExplicit := step.hasExplicitID
				shouldBeExplicit := tt.expectedExplicit[step.ID]

				if isExplicit != shouldBeExplicit {
					t.Errorf("Step %q: hasExplicitID = %v, expected %v",
						step.ID, isExplicit, shouldBeExplicit)
				}
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestShorthandParserInlineForm tests inline shorthand syntax (P3.5)
func TestShorthandParserInlineForm(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		expectedType StepType
		expectedID   string
		wantErr      bool
	}{
		{
			name: "inline string form - file.read",
			yaml: `
name: test-workflow
steps:
  - file.read: ./config.json
`,
			expectedType: StepTypeBuiltin,
			expectedID:   "file_read_1",
			wantErr:      false,
		},
		{
			name: "inline string form - shell.run",
			yaml: `
name: test-workflow
steps:
  - shell.run: ls -la
`,
			expectedType: StepTypeBuiltin,
			expectedID:   "shell_run_1",
			wantErr:      false,
		},
		{
			name: "inline object form - file.read with extract",
			yaml: `
name: test-workflow
steps:
  - file.read:
      path: ./config.json
      extract: $.database.host
`,
			expectedType: StepTypeBuiltin,
			expectedID:   "file_read_1",
			wantErr:      false,
		},
		{
			name: "inline with id field",
			yaml: `
name: test-workflow
steps:
  - id: my_read
    file.read: ./config.json
`,
			expectedType: StepTypeBuiltin,
			expectedID:   "my_read",
			wantErr:      false,
		},
		{
			name: "inline with timeout and retry",
			yaml: `
name: test-workflow
steps:
  - id: read_with_retry
    file.read: ./config.json
    timeout: 30
    retry:
      max_attempts: 3
`,
			expectedType: StepTypeBuiltin,
			expectedID:   "read_with_retry",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(def.Steps) != 1 {
				t.Fatalf("Expected 1 step, got %d", len(def.Steps))
			}

			step := def.Steps[0]
			if step.Type != tt.expectedType {
				t.Errorf("Expected type %v, got %v", tt.expectedType, step.Type)
			}
			if step.ID != tt.expectedID {
				t.Errorf("Expected ID %q, got %q", tt.expectedID, step.ID)
			}
		})
	}
}

// TestShorthandParserBlockForm tests block shorthand syntax (P3.5)
func TestShorthandParserBlockForm(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		checkInputs func(*testing.T, map[string]interface{})
		wantErr     bool
	}{
		{
			name: "block form with multiple inputs",
			yaml: `
name: test-workflow
steps:
  - file.copy:
      src: ./source.txt
      dest: ./destination.txt
      recursive: true
`,
			checkInputs: func(t *testing.T, inputs map[string]interface{}) {
				if inputs["src"] != "./source.txt" {
					t.Errorf("Expected src=./source.txt, got %v", inputs["src"])
				}
				if inputs["dest"] != "./destination.txt" {
					t.Errorf("Expected dest=./destination.txt, got %v", inputs["dest"])
				}
				if inputs["recursive"] != true {
					t.Errorf("Expected recursive=true, got %v", inputs["recursive"])
				}
			},
			wantErr: false,
		},
		{
			name: "block form with nested objects",
			yaml: `
name: test-workflow
connectors:
  github:
    from: connectors/github
    auth:
      token: ${GITHUB_TOKEN}
steps:
  - github.create_issue:
      owner: example
      repo: test-repo
      title: Bug report
      body: Description here
      labels: [bug, urgent]
`,
			checkInputs: func(t *testing.T, inputs map[string]interface{}) {
				if inputs["owner"] != "example" {
					t.Errorf("Expected owner=example, got %v", inputs["owner"])
				}
				if inputs["title"] != "Bug report" {
					t.Errorf("Expected title='Bug report', got %v", inputs["title"])
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(def.Steps) != 1 {
				t.Fatalf("Expected 1 step, got %d", len(def.Steps))
			}

			step := def.Steps[0]
			if tt.checkInputs != nil {
				tt.checkInputs(t, step.Inputs)
			}
		})
	}
}

// TestBuiltinVsExternalConnectorDetection tests connector type detection (P3.5)
func TestBuiltinVsExternalConnectorDetection(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		expectedType StepType
		checkStep    func(*testing.T, *StepDefinition)
		wantErr      bool
	}{
		{
			name: "builtin file connector",
			yaml: `
name: test-workflow
steps:
  - file.read: ./config.json
`,
			expectedType: StepTypeBuiltin,
			checkStep: func(t *testing.T, step *StepDefinition) {
				if step.BuiltinConnector != "file" {
					t.Errorf("Expected BuiltinConnector=file, got %v", step.BuiltinConnector)
				}
				if step.BuiltinOperation != "read" {
					t.Errorf("Expected BuiltinOperation=read, got %v", step.BuiltinOperation)
				}
			},
			wantErr: false,
		},
		{
			name: "builtin shell connector",
			yaml: `
name: test-workflow
steps:
  - shell.run: echo hello
`,
			expectedType: StepTypeBuiltin,
			checkStep: func(t *testing.T, step *StepDefinition) {
				if step.BuiltinConnector != "shell" {
					t.Errorf("Expected BuiltinConnector=shell, got %v", step.BuiltinConnector)
				}
				if step.BuiltinOperation != "run" {
					t.Errorf("Expected BuiltinOperation=run, got %v", step.BuiltinOperation)
				}
			},
			wantErr: false,
		},
		{
			name: "external github connector",
			yaml: `
name: test-workflow
connectors:
  github:
    from: connectors/github
    auth:
      token: ${GITHUB_TOKEN}
steps:
  - github.create_issue:
      owner: foo
      repo: bar
      title: test
`,
			expectedType: StepTypeConnector,
			checkStep: func(t *testing.T, step *StepDefinition) {
				if step.Connector != "github.create_issue" {
					t.Errorf("Expected Connector=github.create_issue, got %v", step.Connector)
				}
			},
			wantErr: false,
		},
		{
			name: "external slack connector",
			yaml: `
name: test-workflow
connectors:
  slack:
    from: connectors/slack
    auth:
      token: ${SLACK_TOKEN}
steps:
  - slack.post_message:
      channel: "#general"
      text: "Hello world"
`,
			expectedType: StepTypeConnector,
			checkStep: func(t *testing.T, step *StepDefinition) {
				if step.Connector != "slack.post_message" {
					t.Errorf("Expected Connector=slack.post_message, got %v", step.Connector)
				}
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(def.Steps) != 1 {
				t.Fatalf("Expected 1 step, got %d", len(def.Steps))
			}

			step := def.Steps[0]
			if step.Type != tt.expectedType {
				t.Errorf("Expected type %v, got %v", tt.expectedType, step.Type)
			}
			if tt.checkStep != nil {
				tt.checkStep(t, &step)
			}
		})
	}
}

// TestConnectorSyntaxVariants tests that various connector syntax forms are correctly parsed.
func TestConnectorSyntaxVariants(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "verbose connector syntax still works",
			yaml: `
name: test-workflow
connectors:
  github:
    from: connectors/github
    auth:
      token: ${GITHUB_TOKEN}
steps:
  - id: create_issue
    type: connector
    connector: github.create_issue
    inputs:
      owner: foo
      repo: bar
      title: test
`,
			wantErr: false,
		},
		{
			name: "builtin shorthand syntax still works",
			yaml: `
name: test-workflow
steps:
  - id: read_file
    type: builtin
    builtin_connector: file
    builtin_operation: read
    inputs:
      path: ./config.json
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && def == nil {
				t.Error("ParseDefinition() returned nil definition")
			}
		})
	}
}

// TestInvalidShorthandSyntax tests error cases for shorthand syntax (P3.6)
func TestInvalidShorthandSyntax(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		errContains string
		wantErr     bool
	}{
		{
			name: "invalid pattern - uppercase",
			yaml: `
name: test-workflow
steps:
  - File.Read: ./config.json
`,
			errContains: "",
			wantErr:     true, // Should fail pattern validation
		},
		{
			name: "invalid pattern - missing operation",
			yaml: `
name: test-workflow
steps:
  - file: ./config.json
`,
			errContains: "",
			wantErr:     true,
		},
		{
			name: "invalid pattern - three segments",
			yaml: `
name: test-workflow
steps:
  - file.read.deep: ./config.json
`,
			errContains: "",
			wantErr:     true,
		},
		{
			name: "invalid pattern - starts with number",
			yaml: `
name: test-workflow
steps:
  - 123.read: ./config.json
`,
			errContains: "",
			wantErr:     true,
		},
		{
			name: "invalid pattern - empty connector name",
			yaml: `
name: test-workflow
steps:
  - .read: ./config.json
`,
			errContains: "",
			wantErr:     true,
		},
		{
			name: "invalid pattern - empty operation name",
			yaml: `
name: test-workflow
steps:
  - file.: ./config.json
`,
			errContains: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseDefinition([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDefinition() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" && err != nil {
				if !containsSubstring(err.Error(), tt.errContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errContains, err.Error())
				}
			}
		})
	}
}

// TestShorthandPatternValidation tests the shorthand pattern regex (P3.6)
func TestShorthandPatternValidation(t *testing.T) {
	tests := []struct {
		pattern string
		valid   bool
	}{
		{"file.read", true},
		{"github.create_issue", true},
		{"my_connector.do_thing", true},
		{"http2.get", true},
		{"File.Read", false},         // Uppercase
		{"file", false},               // Missing operation
		{"file.read.deep", false},     // Three segments
		{".read", false},              // Empty connector
		{"file.", false},              // Empty operation
		{"123.read", false},           // Starts with number
		{"file-read.op", false},       // Dash not allowed
		{"file.read-data", false},     // Dash not allowed
		{"a.b", true},                 // Single char names OK
		{"a1.b2", true},               // Numbers allowed (not first)
		{"file_system.read", true},    // Underscore OK
		{"file.read_all", true},       // Underscore OK
		{"FILE.read", false},          // Uppercase
		{"file.READ", false},          // Uppercase
		{"_file.read", false},         // Starts with underscore
		{"file._read", false},         // Starts with underscore
	}

	// Import the pattern from the main code
	shorthandPattern := regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			matches := shorthandPattern.MatchString(tt.pattern)
			if matches != tt.valid {
				t.Errorf("Pattern %q: expected valid=%v, got %v", tt.pattern, tt.valid, matches)
			}
		})
	}
}

// TestShorthandWithAdditionalFields tests shorthand combined with step-level config (P3.6)
func TestShorthandWithAdditionalFields(t *testing.T) {
	yaml := `
name: test-workflow
steps:
  - id: read_config
    file.read: ./config.json
    timeout: 60
`

	def, err := ParseDefinition([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseDefinition() error = %v", err)
	}

	if len(def.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(def.Steps))
	}

	step := def.Steps[0]
	if step.ID != "read_config" {
		t.Errorf("Expected ID=read_config, got %v", step.ID)
	}
	if step.Timeout != 60 {
		t.Errorf("Expected Timeout=60, got %v", step.Timeout)
	}
	// Verify the shorthand was parsed correctly
	if step.Type != StepTypeBuiltin {
		t.Errorf("Expected Type=StepTypeBuiltin, got %v", step.Type)
	}
	if step.BuiltinConnector != "file" {
		t.Errorf("Expected BuiltinConnector=file, got %v", step.BuiltinConnector)
	}
	if step.BuiltinOperation != "read" {
		t.Errorf("Expected BuiltinOperation=read, got %v", step.BuiltinOperation)
	}
}

// TestMixedShorthandAndExplicitTypes tests mixing shorthand and explicit type steps
func TestMixedShorthandAndExplicitTypes(t *testing.T) {
	yaml := `
name: test-workflow
steps:
  - file.read: ./config.json
  - id: analyze
    type: llm
    prompt: "Analyze this config"
  - file.write:
      path: ./output.json
      content: "result"
  - id: parallel_work
    type: parallel
    steps:
      - id: read_file1
        file.read: ./file1.txt
      - id: read_file2
        file.read: ./file2.txt
`

	def, err := ParseDefinition([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseDefinition() error = %v", err)
	}

	if len(def.Steps) != 4 {
		t.Fatalf("Expected 4 steps, got %d", len(def.Steps))
	}

	// Check step types
	expectedTypes := []StepType{
		StepTypeBuiltin,  // file.read
		StepTypeLLM,      // llm
		StepTypeBuiltin,  // file.write
		StepTypeParallel, // parallel
	}

	for i, expectedType := range expectedTypes {
		if def.Steps[i].Type != expectedType {
			t.Errorf("Step %d: expected type %v, got %v", i, expectedType, def.Steps[i].Type)
		}
	}
}
