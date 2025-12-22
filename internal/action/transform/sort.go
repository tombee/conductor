package transform

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/jq"
)

// sort operation - sorts array elements, optionally by key expression.
func (c *TransformAction) sort(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "sort",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with array to sort",
		}
	}

	// Check if data is nil/null
	if data == nil {
		return nil, &OperationError{
			Operation:  "sort",
			Message:    "cannot sort null or undefined value",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Provide a valid array to sort",
		}
	}

	// Verify input is an array
	arr, ok := data.([]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "sort",
			Message:    "input must be an array",
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Use transform.sort only with array inputs",
		}
	}

	// Check array size limit
	if len(arr) > c.config.MaxArrayItems {
		return nil, &OperationError{
			Operation:  "sort",
			Message:    fmt.Sprintf("array size (%d items) exceeds maximum (%d items)", len(arr), c.config.MaxArrayItems),
			ErrorType:  ErrorTypeLimitExceeded,
			Suggestion: "Reduce input size before sorting",
		}
	}

	// Get optional expr parameter for sort key
	var sortExpr string
	if expr, ok := inputs["expr"]; ok {
		// Validate expr is a string
		exprStr, ok := expr.(string)
		if !ok {
			return nil, &OperationError{
				Operation:  "sort",
				Message:    fmt.Sprintf("expr must be a string, got %T", expr),
				ErrorType:  ErrorTypeTypeError,
				Suggestion: "Provide expr as a literal string in your workflow YAML",
			}
		}

		if exprStr == "" {
			return nil, &OperationError{
				Operation:  "sort",
				Message:    "expr cannot be empty",
				ErrorType:  ErrorTypeValidation,
				Suggestion: "Provide a jq expression for sort key (e.g., '.priority' or '.date')",
			}
		}

		sortExpr = exprStr
	}

	// Check input size
	if err := c.validateInputSize(data); err != nil {
		return nil, &OperationError{
			Operation:  "sort",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Reduce input size or process in smaller chunks",
		}
	}

	// Build jq sort expression
	var jqExpr string
	if sortExpr != "" {
		// Sort by key expression using sort_by
		jqExpr = fmt.Sprintf("sort_by(%s)", sortExpr)
	} else {
		// Simple sort (no key)
		jqExpr = "sort"
	}

	// Create jq executor with timeout from config
	timeout := time.Duration(c.config.ExpressionTimeout)
	executor := jq.NewExecutor(timeout, c.config.MaxInputSize)

	// Execute the sort expression
	result, err := executor.Execute(ctx, jqExpr, arr)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &OperationError{
				Operation:  "sort",
				Message:    fmt.Sprintf("expression evaluation exceeded %v timeout", timeout),
				ErrorType:  ErrorTypeLimitExceeded,
				Cause:      err,
				Suggestion: "Simplify the sort expression or reduce input size",
			}
		}

		// Redact sensitive data from error message
		errMsg := err.Error()
		if containsSensitivePattern(errMsg) {
			errMsg = "[Error message redacted - contains sensitive data]"
		}

		return nil, &OperationError{
			Operation:  "sort",
			Message:    "sort expression evaluation failed",
			ErrorType:  ErrorTypeExpressionError,
			Cause:      fmt.Errorf("%s", errMsg),
			Suggestion: "Check sort expression syntax and verify it returns a comparable value for each element",
		}
	}

	// Result should be an array
	resultArr, ok := result.([]interface{})
	if !ok {
		// Handle case where result is nil (empty array)
		if result == nil {
			resultArr = []interface{}{}
		} else {
			return nil, &OperationError{
				Operation:  "sort",
				Message:    fmt.Sprintf("sort expression must return an array, got %T", result),
				ErrorType:  ErrorTypeExpressionError,
				Suggestion: "Ensure your sort expression can process the array elements",
			}
		}
	}

	// Check output size
	if err := c.validateOutputSize(resultArr); err != nil {
		return nil, &OperationError{
			Operation:  "sort",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Reduce input size before sorting",
		}
	}

	metadata := map[string]interface{}{
		"item_count": len(resultArr),
	}
	if sortExpr != "" {
		metadata["expression"] = sortExpr
	}

	return &Result{
		Response: resultArr,
		Metadata: metadata,
	}, nil
}

// group operation - groups array elements by key expression.
func (c *TransformAction) group(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "group",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with array to group",
		}
	}

	// Check if data is nil/null
	if data == nil {
		return nil, &OperationError{
			Operation:  "group",
			Message:    "cannot group null or undefined value",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Provide a valid array to group",
		}
	}

	// Verify input is an array
	arr, ok := data.([]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "group",
			Message:    "input must be an array",
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Use transform.group only with array inputs",
		}
	}

	// Check array size limit
	if len(arr) > c.config.MaxArrayItems {
		return nil, &OperationError{
			Operation:  "group",
			Message:    fmt.Sprintf("array size (%d items) exceeds maximum (%d items)", len(arr), c.config.MaxArrayItems),
			ErrorType:  ErrorTypeLimitExceeded,
			Suggestion: "Reduce input size before grouping",
		}
	}

	// Get required expr parameter for group key
	expr, ok := inputs["expr"]
	if !ok {
		return nil, &OperationError{
			Operation:  "group",
			Message:    "missing required parameter: expr",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide expr parameter with jq expression for group key (e.g., '.category' or '.status')",
		}
	}

	// Validate expr is a string
	exprStr, ok := expr.(string)
	if !ok {
		return nil, &OperationError{
			Operation:  "group",
			Message:    fmt.Sprintf("expr must be a string, got %T", expr),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Provide expr as a literal string in your workflow YAML",
		}
	}

	if exprStr == "" {
		return nil, &OperationError{
			Operation:  "group",
			Message:    "expr cannot be empty",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide a jq expression for group key (e.g., '.category' or '.status')",
		}
	}

	// Check input size
	if err := c.validateInputSize(data); err != nil {
		return nil, &OperationError{
			Operation:  "group",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Reduce input size or process in smaller chunks",
		}
	}

	// Build jq group expression using group_by
	jqExpr := fmt.Sprintf("group_by(%s)", exprStr)

	// Create jq executor with timeout from config
	timeout := time.Duration(c.config.ExpressionTimeout)
	executor := jq.NewExecutor(timeout, c.config.MaxInputSize)

	// Execute the group expression
	result, err := executor.Execute(ctx, jqExpr, arr)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &OperationError{
				Operation:  "group",
				Message:    fmt.Sprintf("expression evaluation exceeded %v timeout", timeout),
				ErrorType:  ErrorTypeLimitExceeded,
				Cause:      err,
				Suggestion: "Simplify the group expression or reduce input size",
			}
		}

		// Redact sensitive data from error message
		errMsg := err.Error()
		if containsSensitivePattern(errMsg) {
			errMsg = "[Error message redacted - contains sensitive data]"
		}

		return nil, &OperationError{
			Operation:  "group",
			Message:    "group expression evaluation failed",
			ErrorType:  ErrorTypeExpressionError,
			Cause:      fmt.Errorf("%s", errMsg),
			Suggestion: "Check group expression syntax and verify it returns a comparable value for each element",
		}
	}

	// Result should be an array of arrays
	resultArr, ok := result.([]interface{})
	if !ok {
		// Handle case where result is nil (empty array)
		if result == nil {
			resultArr = []interface{}{}
		} else {
			return nil, &OperationError{
				Operation:  "group",
				Message:    fmt.Sprintf("group expression must return an array, got %T", result),
				ErrorType:  ErrorTypeExpressionError,
				Suggestion: "Ensure your group expression can process the array elements",
			}
		}
	}

	// Check output size
	if err := c.validateOutputSize(resultArr); err != nil {
		return nil, &OperationError{
			Operation:  "group",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Use a more selective group expression to reduce output size",
		}
	}

	return &Result{
		Response: resultArr,
		Metadata: map[string]interface{}{
			"expression":  exprStr,
			"group_count": len(resultArr),
			"item_count":  len(arr),
		},
	}, nil
}
