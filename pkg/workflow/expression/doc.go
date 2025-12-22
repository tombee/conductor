// Package expression provides condition expression evaluation for workflow steps.
//
// It uses the expr-lang/expr library to evaluate boolean expressions that
// determine whether workflow steps should execute. Expressions support:
//
//   - Variable access: inputs.name, steps.step_id.content
//   - Comparisons: ==, !=, <, >, <=, >=
//   - Boolean logic: &&, ||, !
//   - Membership: "value" in array (built-in operator)
//   - Custom functions: has(array, element), includes(array, element)
//
// Example expressions:
//
//	"security" in inputs.personas
//	has(inputs.personas, "security")
//	inputs.mode == "strict" && inputs.count > 0
//	!inputs.disabled
//
// The evaluator caches compiled expressions for performance.
//
// Note: The expr library uses "contains" as a string operator (for substring matching),
// so use "in" or "has()" for array membership checks.
package expression
