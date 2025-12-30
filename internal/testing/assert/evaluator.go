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

// Package assert provides assertion evaluation for workflow testing.
package assert

import (
	"fmt"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// Evaluator evaluates assertion expressions for workflow testing.
// It extends expr-lang/expr with testing-specific operators and functions.
// This is separate from the workflow condition evaluator to avoid conflicts.
type Evaluator struct {
	cache map[string]*vm.Program
	mu    sync.RWMutex
}

// New creates a new assertion evaluator.
func New() *Evaluator {
	return &Evaluator{
		cache: make(map[string]*vm.Program),
	}
}

// Result represents the result of an assertion evaluation.
type Result struct {
	// Passed indicates whether the assertion passed
	Passed bool

	// Expression is the assertion expression that was evaluated
	Expression string

	// Actual is the actual value that was compared
	Actual interface{}

	// Expected is a description of the expected value/condition
	Expected string

	// Error is set if evaluation failed (syntax error, runtime error, etc.)
	Error error
}

// Evaluate evaluates an assertion expression against the given context.
// Returns a Result with details about the assertion outcome.
//
// The context should typically contain the step output as top-level keys:
//
//	ctx := map[string]interface{}{
//	    "status_code": 200,
//	    "body": map[string]interface{}{
//	        "items": []interface{}{...},
//	    },
//	}
//
// Example expressions:
//   - status_code == 200
//   - body.items | length > 0
//   - body.name contains "test"
//   - id matches "^[A-Z]{3}-\\d+$"
func (e *Evaluator) Evaluate(expression string, ctx map[string]interface{}) Result {
	if expression == "" {
		return Result{
			Passed:     true,
			Expression: expression,
			Expected:   "empty (always passes)",
		}
	}

	program, err := e.compile(expression)
	if err != nil {
		return Result{
			Passed:     false,
			Expression: expression,
			Error:      fmt.Errorf("failed to compile expression: %w", err),
		}
	}

	// Create evaluation context with custom functions
	evalCtx := make(map[string]interface{})
	for k, v := range ctx {
		evalCtx[k] = v
	}

	// Add assertion-specific functions
	for name, fn := range AssertionFunctions() {
		evalCtx[name] = fn
	}

	// Run the expression
	result, err := expr.Run(program, evalCtx)
	if err != nil {
		return Result{
			Passed:     false,
			Expression: expression,
			Error:      fmt.Errorf("expression evaluation failed: %w", err),
		}
	}

	// Convert result to boolean
	boolResult, ok := result.(bool)
	if !ok {
		return Result{
			Passed:     false,
			Expression: expression,
			Error:      fmt.Errorf("expression must return boolean, got %T (%v)", result, result),
		}
	}

	return Result{
		Passed:     boolResult,
		Expression: expression,
		Actual:     extractActualValue(expression, ctx),
		Expected:   extractExpected(expression),
	}
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

	// Create environment with assertion functions
	env := make(map[string]interface{})
	for name, fn := range AssertionFunctions() {
		env[name] = fn
	}

	// Compile the expression
	prog, err := expr.Compile(expression,
		expr.Env(env),
		expr.AllowUndefinedVariables(),
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

// extractActualValue attempts to extract the actual value from the context.
// This is a best-effort extraction for error reporting.
func extractActualValue(expression string, ctx map[string]interface{}) interface{} {
	// For simple variable references, try to extract the value
	// This is simplified; a full implementation would parse the expression
	// For now, we'll return the context for debugging
	return ctx
}

// extractExpected attempts to extract what was expected from the expression.
// This is a best-effort extraction for error reporting.
func extractExpected(expression string) string {
	// This is simplified; a full implementation would parse the expression
	// and extract the comparison/expectation
	return expression
}
