package workflow

import (
	"fmt"
	"time"
)

// ErrKeyNotFound represents an error when a requested key does not exist in the context.
type ErrKeyNotFound struct {
	Key string
}

// Error implements the error interface.
// Security: Does not include the actual value to prevent credential leakage.
func (e ErrKeyNotFound) Error() string {
	return fmt.Sprintf("key %q not found", e.Key)
}

// ErrTypeAssertion represents an error when a value cannot be asserted to the expected type.
type ErrTypeAssertion struct {
	Key  string // The key that was accessed
	Got  string // The actual type received (as string representation)
	Want string // The expected type
}

// Error implements the error interface.
// Security: Does not include the actual value to prevent credential leakage.
func (e ErrTypeAssertion) Error() string {
	return fmt.Sprintf("key %q is %s, not %s", e.Key, e.Got, e.Want)
}

// WorkflowContext provides type-safe access to workflow inputs, outputs, and variables.
// Methods are safe for concurrent reads but NOT safe for concurrent writes.
// Caller must guard mutations with appropriate synchronization.
type WorkflowContext struct {
	inputs  map[string]any
	outputs map[string]StepOutput
	vars    map[string]any
}

// NewWorkflowContext creates a new WorkflowContext with the provided inputs.
func NewWorkflowContext(inputs map[string]any) *WorkflowContext {
	if inputs == nil {
		inputs = make(map[string]any)
	}
	return &WorkflowContext{
		inputs:  inputs,
		outputs: make(map[string]StepOutput),
		vars:    make(map[string]any),
	}
}

// GetString retrieves a string value from the workflow inputs.
// Returns ErrKeyNotFound if key doesn't exist, ErrTypeAssertion if wrong type.
// Security: Error messages do not include the actual value to prevent leaks.
func (c *WorkflowContext) GetString(key string) (string, error) {
	val, ok := c.inputs[key]
	if !ok {
		return "", ErrKeyNotFound{Key: key}
	}
	str, ok := val.(string)
	if !ok {
		return "", ErrTypeAssertion{Key: key, Got: fmt.Sprintf("%T", val), Want: "string"}
	}
	return str, nil
}

// GetStringOr returns a string value or the default if key is missing or wrong type.
// Never panics. Does not log the actual value for security.
func (c *WorkflowContext) GetStringOr(key string, defaultVal string) string {
	str, err := c.GetString(key)
	if err != nil {
		return defaultVal
	}
	return str
}

// GetInt64 retrieves an int64 value from the workflow inputs.
// Returns ErrKeyNotFound if key doesn't exist, ErrTypeAssertion if wrong type.
// Security: Error messages do not include the actual value to prevent leaks.
func (c *WorkflowContext) GetInt64(key string) (int64, error) {
	val, ok := c.inputs[key]
	if !ok {
		return 0, ErrKeyNotFound{Key: key}
	}

	// Handle various integer types that might come from JSON/YAML unmarshaling
	switch v := val.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case float64:
		// JSON numbers are unmarshaled as float64
		return int64(v), nil
	default:
		return 0, ErrTypeAssertion{Key: key, Got: fmt.Sprintf("%T", val), Want: "int64"}
	}
}

// GetInt64Or returns an int64 value or the default if key is missing or wrong type.
// Never panics. Does not log the actual value for security.
func (c *WorkflowContext) GetInt64Or(key string, defaultVal int64) int64 {
	i, err := c.GetInt64(key)
	if err != nil {
		return defaultVal
	}
	return i
}

// GetBool retrieves a bool value from the workflow inputs.
// Returns ErrKeyNotFound if key doesn't exist, ErrTypeAssertion if wrong type.
// Security: Error messages do not include the actual value to prevent leaks.
func (c *WorkflowContext) GetBool(key string) (bool, error) {
	val, ok := c.inputs[key]
	if !ok {
		return false, ErrKeyNotFound{Key: key}
	}
	b, ok := val.(bool)
	if !ok {
		return false, ErrTypeAssertion{Key: key, Got: fmt.Sprintf("%T", val), Want: "bool"}
	}
	return b, nil
}

// GetBoolOr returns a bool value or the default if key is missing or wrong type.
// Never panics. Does not log the actual value for security.
func (c *WorkflowContext) GetBoolOr(key string, defaultVal bool) bool {
	b, err := c.GetBool(key)
	if err != nil {
		return defaultVal
	}
	return b
}

// GetFloat64 retrieves a float64 value from the workflow inputs.
// Returns ErrKeyNotFound if key doesn't exist, ErrTypeAssertion if wrong type.
// Security: Error messages do not include the actual value to prevent leaks.
func (c *WorkflowContext) GetFloat64(key string) (float64, error) {
	val, ok := c.inputs[key]
	if !ok {
		return 0, ErrKeyNotFound{Key: key}
	}

	// Handle various numeric types
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, ErrTypeAssertion{Key: key, Got: fmt.Sprintf("%T", val), Want: "float64"}
	}
}

// GetFloat64Or returns a float64 value or the default if key is missing or wrong type.
// Never panics. Does not log the actual value for security.
func (c *WorkflowContext) GetFloat64Or(key string, defaultVal float64) float64 {
	f, err := c.GetFloat64(key)
	if err != nil {
		return defaultVal
	}
	return f
}

// GetSlice retrieves a slice value from the workflow inputs.
// Returns ErrKeyNotFound if key doesn't exist, ErrTypeAssertion if wrong type.
// Note: Returns []interface{} due to type safety limitations with generic slices.
// Security: Error messages do not include the actual value to prevent leaks.
func (c *WorkflowContext) GetSlice(key string) ([]interface{}, error) {
	val, ok := c.inputs[key]
	if !ok {
		return nil, ErrKeyNotFound{Key: key}
	}
	slice, ok := val.([]interface{})
	if !ok {
		return nil, ErrTypeAssertion{Key: key, Got: fmt.Sprintf("%T", val), Want: "[]interface{}"}
	}
	return slice, nil
}

// GetMap retrieves a map value from the workflow inputs.
// Returns ErrKeyNotFound if key doesn't exist, ErrTypeAssertion if wrong type.
// Note: Returns map[string]interface{} due to type safety limitations with generic maps.
// Security: Error messages do not include the actual value to prevent leaks.
func (c *WorkflowContext) GetMap(key string) (map[string]interface{}, error) {
	val, ok := c.inputs[key]
	if !ok {
		return nil, ErrKeyNotFound{Key: key}
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil, ErrTypeAssertion{Key: key, Got: fmt.Sprintf("%T", val), Want: "map[string]interface{}"}
	}
	return m, nil
}

// GetInputs returns the underlying inputs map for expression evaluation.
// This is used by the expression layer to convert typed context to untyped maps.
// Safe for concurrent reads.
func (c *WorkflowContext) GetInputs() map[string]any {
	return c.inputs
}

// GetOutputs returns the step outputs map for expression evaluation.
// This is used by the expression layer to convert typed context to untyped maps.
// Safe for concurrent reads.
func (c *WorkflowContext) GetOutputs() map[string]StepOutput {
	return c.outputs
}

// SetOutput stores a step output in the context.
// This is used during workflow execution to track step results.
// NOT safe for concurrent writes - caller must synchronize.
func (c *WorkflowContext) SetOutput(stepID string, output StepOutput) {
	c.outputs[stepID] = output
}

// StepOutput represents the structured output of a workflow step.
// This replaces the untyped map[string]interface{} for step results.
type StepOutput struct {
	// Text is the primary text output of the step (e.g., LLM response)
	Text string `json:"text,omitempty"`

	// Data holds arbitrary structured data returned by the step
	Data any `json:"data,omitempty"`

	// Error contains the error message if the step failed
	Error string `json:"error,omitempty"`

	// Metadata contains execution metadata (duration, token usage, etc.)
	Metadata OutputMetadata `json:"metadata"`
}

// OutputMetadata contains metadata about step execution.
type OutputMetadata struct {
	// Duration is the time taken to execute the step
	Duration time.Duration `json:"duration,omitempty"`

	// TokenUsage captures LLM token consumption metrics
	TokenUsage *TokenUsage `json:"token_usage,omitempty"`

	// Provider is the LLM provider used (e.g., "anthropic", "openai")
	Provider string `json:"provider,omitempty"`

	// Model is the specific model used (e.g., "claude-opus-4-5")
	Model string `json:"model,omitempty"`
}

// TokenUsage captures consumption metrics from the LLM provider.
type TokenUsage struct {
	// InputTokens is the number of tokens in the input/prompt
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the number of tokens in the generated output
	OutputTokens int `json:"output_tokens"`

	// TotalTokens is the sum of input and output tokens
	TotalTokens int `json:"total_tokens"`

	// CacheReadTokens is the number of tokens read from cache (optional)
	CacheReadTokens int `json:"cache_read_tokens,omitempty"`

	// CacheWriteTokens is the number of tokens written to cache (optional)
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`
}

// ToMap converts StepOutput to an untyped map for expression evaluation.
// This implements the StepOutputConverter interface for the expression package.
// The expression layer requires untyped maps due to expr-lang limitations.
func (s StepOutput) ToMap() map[string]interface{} {
	result := make(map[string]interface{})

	if s.Text != "" {
		result["text"] = s.Text
		result["response"] = s.Text // Both "text" and "response" are valid accessors
	}

	if s.Error != "" {
		result["error"] = s.Error
	}

	// Merge Data fields if it's a map, otherwise store as-is
	if dataMap, ok := s.Data.(map[string]interface{}); ok {
		for k, v := range dataMap {
			result[k] = v
		}
	} else if s.Data != nil {
		result["data"] = s.Data
	}

	return result
}
