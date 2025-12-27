package expression

import (
	"fmt"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/tombee/conductor/pkg/errors"
)

// Evaluator evaluates condition expressions against a workflow context.
// It caches compiled expressions for improved performance on repeated evaluations.
type Evaluator struct {
	cache map[string]*vm.Program
	mu    sync.RWMutex
}

// New creates a new expression evaluator.
func New() *Evaluator {
	return &Evaluator{
		cache: make(map[string]*vm.Program),
	}
}

// Evaluate evaluates an expression against the given context.
// Returns the boolean result or an error if evaluation fails.
//
// The context should contain:
//   - inputs: map of workflow input values
//   - steps: map of step results keyed by step ID
//
// Example:
//
//	ctx := map[string]interface{}{
//	    "inputs": map[string]interface{}{"personas": []string{"security"}},
//	    "steps":  map[string]interface{}{"fetch": map[string]interface{}{"content": "..."}},
//	}
//	result, err := eval.Evaluate(`contains(inputs.personas, "security")`, ctx)
func (e *Evaluator) Evaluate(expression string, ctx map[string]interface{}) (bool, error) {
	if expression == "" {
		return true, nil // Empty expression defaults to true
	}

	program, err := e.compile(expression)
	if err != nil {
		return false, &errors.ValidationError{
			Field:      "expression",
			Message:    fmt.Sprintf("failed to compile expression: %s", err.Error()),
			Suggestion: "check expression syntax and ensure all referenced variables exist",
		}
	}

	// Merge custom functions into context for runtime
	// Note: "contains" is reserved in expr for string operations
	evalCtx := make(map[string]interface{})
	for k, v := range ctx {
		evalCtx[k] = v
	}
	evalCtx["has"] = containsFunc
	evalCtx["includes"] = containsFunc
	evalCtx["length"] = lenFunc

	result, err := expr.Run(program, evalCtx)
	if err != nil {
		return false, &errors.ValidationError{
			Field:      "expression",
			Message:    fmt.Sprintf("expression evaluation failed: %s", err.Error()),
			Suggestion: "verify that all referenced variables exist in the workflow context",
		}
	}

	boolResult, ok := result.(bool)
	if !ok {
		return false, &errors.ValidationError{
			Field:      "expression",
			Message:    fmt.Sprintf("expression must return boolean, got %T (%v)", result, result),
			Suggestion: "use comparison operators (==, !=, <, >, etc.) or boolean functions",
		}
	}

	return boolResult, nil
}

// compile compiles an expression and caches the result.
func (e *Evaluator) compile(expression string) (*vm.Program, error) {
	// Check cache first (read lock)
	e.mu.RLock()
	if prog, ok := e.cache[expression]; ok {
		e.mu.RUnlock()
		return prog, nil
	}
	e.mu.RUnlock()

	// Create environment with custom functions
	// Note: "contains" is a reserved string operator in expr, so we use "has" and "includes"
	env := map[string]interface{}{
		"has":      containsFunc,
		"includes": containsFunc, // Alias
		"length":   lenFunc,
	}

	// Compile the expression
	prog, err := expr.Compile(expression,
		expr.Env(env),
		// Allow any environment (we pass the context at runtime)
		expr.AllowUndefinedVariables(),
		// Expression must return boolean
		expr.AsBool(),
	)
	if err != nil {
		return nil, err
	}

	// Cache the compiled program (write lock)
	e.mu.Lock()
	e.cache[expression] = prog
	e.mu.Unlock()

	return prog, nil
}

// ClearCache clears the expression cache.
// This is mainly useful for testing.
func (e *Evaluator) ClearCache() {
	e.mu.Lock()
	e.cache = make(map[string]*vm.Program)
	e.mu.Unlock()
}

// CacheSize returns the number of cached expressions.
func (e *Evaluator) CacheSize() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.cache)
}
