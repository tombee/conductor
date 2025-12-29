package transform

import (
	"context"
	"fmt"
	"time"

	"github.com/tombee/conductor/internal/jq"
)

// split operation - pass through array for fan-out, error for non-arrays
func (c *TransformConnector) split(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation: "split",
			Message:   "missing required parameter: data",
			ErrorType: ErrorTypeValidation,
		}
	}

	// Check if data is nil/null
	if data == nil {
		return nil, &OperationError{
			Operation: "split",
			Message:   "cannot split null or undefined value",
			ErrorType: ErrorTypeEmptyInput,
		}
	}

	// Verify input is an array
	arr, ok := data.([]interface{})
	if !ok {
		return nil, &OperationError{
			Operation: "split",
			Message:   "input must be an array",
			ErrorType: ErrorTypeTypeError,
			Suggestion: "Use transform.split only with array inputs. For non-array values, consider using transform.extract to create an array first.",
		}
	}

	// Pass through the array unchanged
	return &Result{
		Response: arr,
		Metadata: map[string]interface{}{
			"item_count": len(arr),
		},
	}, nil
}

// filter operation - filter array elements using jq predicate expression
func (c *TransformConnector) filter(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "filter",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with array to filter",
		}
	}

	// Check if data is nil/null
	if data == nil {
		return nil, &OperationError{
			Operation:  "filter",
			Message:    "cannot filter null or undefined value",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Provide a valid array to filter",
		}
	}

	// Verify input is an array
	arr, ok := data.([]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "filter",
			Message:    "input must be an array",
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Use transform.filter only with array inputs. For non-array values, use transform.extract instead.",
		}
	}

	// Get expr parameter
	expr, ok := inputs["expr"]
	if !ok {
		return nil, &OperationError{
			Operation:  "filter",
			Message:    "missing required parameter: expr",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide expr parameter with jq predicate expression (e.g., '.active' or '.count > 5')",
		}
	}

	// Validate expr is a string
	exprStr, ok := expr.(string)
	if !ok {
		return nil, &OperationError{
			Operation:  "filter",
			Message:    fmt.Sprintf("expr must be a string, got %T", expr),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Provide expr as a literal string in your workflow YAML",
		}
	}

	if exprStr == "" {
		return nil, &OperationError{
			Operation:  "filter",
			Message:    "expr cannot be empty",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide a jq predicate expression like '.active' or '.count > 5'",
		}
	}

	// Check input size
	if err := c.validateInputSize(data); err != nil {
		return nil, &OperationError{
			Operation:  "filter",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Reduce input size or process in smaller chunks",
		}
	}

	// Build jq filter expression: map(select(expr))
	filterExpr := fmt.Sprintf("map(select(%s))", exprStr)

	// Create jq executor with timeout from config
	timeout := time.Duration(c.config.ExpressionTimeout)
	executor := jq.NewExecutor(timeout, c.config.MaxInputSize)

	// Execute the filter expression
	result, err := executor.Execute(ctx, filterExpr, arr)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &OperationError{
				Operation:  "filter",
				Message:    fmt.Sprintf("expression evaluation exceeded %v timeout", timeout),
				ErrorType:  ErrorTypeLimitExceeded,
				Cause:      err,
				Suggestion: "Simplify the filter expression or reduce input size",
			}
		}

		// Redact sensitive data from error message
		errMsg := err.Error()
		if containsSensitivePattern(errMsg) {
			errMsg = "[Error message redacted - contains sensitive data]"
		}

		return nil, &OperationError{
			Operation:  "filter",
			Message:    "filter expression evaluation failed",
			ErrorType:  ErrorTypeExpressionError,
			Cause:      fmt.Errorf("%s", errMsg),
			Suggestion: "Check filter expression syntax and verify it returns a boolean for each element",
		}
	}

	// Result should be an array
	resultArr, ok := result.([]interface{})
	if !ok {
		// Handle case where result is nil (all items filtered out)
		if result == nil {
			resultArr = []interface{}{}
		} else {
			return nil, &OperationError{
				Operation:  "filter",
				Message:    fmt.Sprintf("filter expression must return an array, got %T", result),
				ErrorType:  ErrorTypeExpressionError,
				Suggestion: "Ensure your filter expression returns a boolean predicate",
			}
		}
	}

	// Check output size
	if err := c.validateOutputSize(resultArr); err != nil {
		return nil, &OperationError{
			Operation:  "filter",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Use a more selective filter expression to reduce output size",
		}
	}

	return &Result{
		Response: resultArr,
		Metadata: map[string]interface{}{
			"expression":   exprStr,
			"input_count":  len(arr),
			"output_count": len(resultArr),
		},
	}, nil
}

// mapArray operation - transform each array element using jq expression
func (c *TransformConnector) mapArray(ctx context.Context, inputs map[string]interface{}) (*Result, error) {
	// Get data input
	data, ok := inputs["data"]
	if !ok {
		return nil, &OperationError{
			Operation:  "map",
			Message:    "missing required parameter: data",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide data parameter with array to transform",
		}
	}

	// Check if data is nil/null
	if data == nil {
		return nil, &OperationError{
			Operation:  "map",
			Message:    "cannot map over null or undefined value",
			ErrorType:  ErrorTypeEmptyInput,
			Suggestion: "Provide a valid array to transform",
		}
	}

	// Verify input is an array
	arr, ok := data.([]interface{})
	if !ok {
		return nil, &OperationError{
			Operation:  "map",
			Message:    "input must be an array",
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Use transform.map only with array inputs. For non-array values, use transform.extract instead.",
		}
	}

	// Get expr parameter
	expr, ok := inputs["expr"]
	if !ok {
		return nil, &OperationError{
			Operation:  "map",
			Message:    "missing required parameter: expr",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide expr parameter with jq transformation expression (e.g., '.name' or '{id: .id, title: .title}')",
		}
	}

	// Validate expr is a string
	exprStr, ok := expr.(string)
	if !ok {
		return nil, &OperationError{
			Operation:  "map",
			Message:    fmt.Sprintf("expr must be a string, got %T", expr),
			ErrorType:  ErrorTypeTypeError,
			Suggestion: "Provide expr as a literal string in your workflow YAML",
		}
	}

	if exprStr == "" {
		return nil, &OperationError{
			Operation:  "map",
			Message:    "expr cannot be empty",
			ErrorType:  ErrorTypeValidation,
			Suggestion: "Provide a jq transformation expression like '.name' or '{id: .id, title: .title}'",
		}
	}

	// Check input size
	if err := c.validateInputSize(data); err != nil {
		return nil, &OperationError{
			Operation:  "map",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Reduce input size or process in smaller chunks",
		}
	}

	// Build jq map expression: map(expr)
	mapExpr := fmt.Sprintf("map(%s)", exprStr)

	// Create jq executor with timeout from config
	timeout := time.Duration(c.config.ExpressionTimeout)
	executor := jq.NewExecutor(timeout, c.config.MaxInputSize)

	// Execute the map expression
	result, err := executor.Execute(ctx, mapExpr, arr)
	if err != nil {
		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &OperationError{
				Operation:  "map",
				Message:    fmt.Sprintf("expression evaluation exceeded %v timeout", timeout),
				ErrorType:  ErrorTypeLimitExceeded,
				Cause:      err,
				Suggestion: "Simplify the transformation expression or reduce input size",
			}
		}

		// Redact sensitive data from error message
		errMsg := err.Error()
		if containsSensitivePattern(errMsg) {
			errMsg = "[Error message redacted - contains sensitive data]"
		}

		return nil, &OperationError{
			Operation:  "map",
			Message:    "map expression evaluation failed",
			ErrorType:  ErrorTypeExpressionError,
			Cause:      fmt.Errorf("%s", errMsg),
			Suggestion: "Check transformation expression syntax and verify it can process each array element",
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
				Operation:  "map",
				Message:    fmt.Sprintf("map expression must return an array, got %T", result),
				ErrorType:  ErrorTypeExpressionError,
				Suggestion: "Ensure your transformation expression returns a value for each element",
			}
		}
	}

	// Check output size
	if err := c.validateOutputSize(resultArr); err != nil {
		return nil, &OperationError{
			Operation:  "map",
			Message:    err.Error(),
			ErrorType:  ErrorTypeLimitExceeded,
			Cause:      err,
			Suggestion: "Use a more compact transformation to reduce output size",
		}
	}

	return &Result{
		Response: resultArr,
		Metadata: map[string]interface{}{
			"expression":   exprStr,
			"input_count":  len(arr),
			"output_count": len(resultArr),
		},
	}, nil
}
