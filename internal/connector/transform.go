package connector

import (
	"context"
	"time"

	"github.com/tombee/conductor/internal/jq"
)

const (
	// MaxTransformTimeout is the maximum execution time for jq transforms (1 second)
	MaxTransformTimeout = 1 * time.Second

	// MaxTransformInputSize is the maximum input size for transforms (10MB)
	MaxTransformInputSize = 10 * 1024 * 1024
)

// TransformResponse applies a jq expression to transform the response data.
// The expression is executed in a sandboxed environment with timeout and memory limits.
func TransformResponse(expression string, response interface{}) (interface{}, error) {
	if expression == "" {
		// No transform specified, return response as-is
		return response, nil
	}

	// Create executor with standard limits
	executor := jq.NewExecutor(MaxTransformTimeout, MaxTransformInputSize)

	// Execute with background context (timeout is handled by executor)
	result, err := executor.Execute(context.Background(), expression, response)
	if err != nil {
		return nil, NewTransformError(expression, err)
	}

	return result, nil
}

// ValidateTransformExpression validates a jq expression by attempting to compile it.
// This is used during workflow validation to catch syntax errors early.
func ValidateTransformExpression(expression string) error {
	if expression == "" {
		return nil
	}

	executor := jq.NewExecutor(MaxTransformTimeout, MaxTransformInputSize)
	return executor.Validate(expression)
}
