package transform

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tombee/conductor/internal/jq"
)

// extract implements the extract operation using jq expressions.
func (c *TransformConnector) extract(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Extract data parameter
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "extract",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with the value to extract from",
		}
	}

	// Extract expr parameter
	expr, ok := inputs["expr"]
	if !ok {
		return nil, &OperationError{
			Operation:  "extract",
			Message:    "missing required parameter: expr",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide expr parameter with jq expression (e.g., '.field' or 'map(.x)')",
		}
	}

	// Validate expr is a string
	exprStr, ok := expr.(string)
	if !ok {
		return nil, &OperationError{
			Operation:  "extract",
			Message:    fmt.Sprintf("expr must be a string, got %T", expr),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Provide expr as a literal string in your workflow YAML",
		}
	}

	if exprStr == "" {
		return nil, &OperationError{
			Operation:  "extract",
			Message:    "expr cannot be empty",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide a jq expression like '.field' or 'map(.x)'",
		}
	}

	// Check input size
	if err := c.validateInputSize(data); err != nil {
		return nil, &OperationError{
			Operation:  "extract",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Reduce input size or process in smaller chunks",
		}
	}

	// Create jq executor with timeout from config
	timeout := time.Duration(c.config.ExpressionTimeout)
	executor := jq.NewExecutor(timeout, c.config.MaxInputSize)

	// Execute the jq expression
	result, err := executor.Execute(ctx, exprStr, data)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &OperationError{
				Operation:  "extract",
				Message:    fmt.Sprintf("expression evaluation exceeded %v timeout", timeout),
				ErrorType:  ErrorTypeLimitExceeded,
				Cause:      err,
				Suggestion: "Simplify the jq expression or reduce input size",
			}
		}

		// Redact sensitive data from error message
		errMsg := err.Error()
		if containsSensitivePattern(errMsg) {
			errMsg = "[Error message redacted - contains sensitive data]"
		}

		return nil, &OperationError{
			Operation:  "extract",
			Message:    "expression evaluation failed",
			ErrorType:  ErrorTypeExpressionError,
			Cause:      fmt.Errorf("%s", errMsg),
			Suggestion: "Check jq expression syntax and verify it matches your data structure",
		}
	}

	// Check output size
	if err := c.validateOutputSize(result); err != nil {
		return nil, &OperationError{
			Operation:  "extract",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Use a more selective jq expression to reduce output size",
		}
	}

	return &Result{
		Response: result,
		Metadata: map[string]interface{}{
			"expression": exprStr,
		},
	}, nil
}

// validateInputSize checks if the data size is within limits.
func (c *TransformConnector) validateInputSize(data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if int64(len(jsonData)) > c.config.MaxInputSize {
		return fmt.Errorf("data size (%d bytes) exceeds maximum (%d bytes)",
			len(jsonData), c.config.MaxInputSize)
	}

	return nil
}

// validateOutputSize checks if the result size is within limits.
func (c *TransformConnector) validateOutputSize(result interface{}) error {
	jsonData, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	if int64(len(jsonData)) > c.config.MaxOutputSize {
		return fmt.Errorf("result size (%d bytes) exceeds maximum (%d bytes)",
			len(jsonData), c.config.MaxOutputSize)
	}

	return nil
}

// containsSensitivePattern checks if text contains any sensitive pattern.
func containsSensitivePattern(text string) bool {
	lowerText := strings.ToLower(text)
	for _, pattern := range sensitivePatterns {
		if strings.Contains(lowerText, pattern) {
			return true
		}
	}
	return false
}
