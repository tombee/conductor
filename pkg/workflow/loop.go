package workflow

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/tombee/conductor/pkg/errors"
	"github.com/tombee/conductor/pkg/workflow/expression"
)

// executeLoop executes a loop step with do-while semantics.
// The loop executes nested steps sequentially, then evaluates the until condition.
// Loop terminates when:
// - The until condition evaluates to true (terminated_by: "condition")
// - max_iterations is reached (terminated_by: "max_iterations")
// - A timeout occurs (terminated_by: "timeout")
// - An unhandled error occurs (terminated_by: "error")
func (e *Executor) executeLoop(ctx context.Context, step *StepDefinition, inputs map[string]interface{}, workflowContext map[string]interface{}) (map[string]interface{}, error) {
	// Validate required fields
	if len(step.Steps) == 0 {
		return nil, &errors.ValidationError{
			Field:      "steps",
			Message:    "loop step has no nested steps",
			Suggestion: "add at least one nested step to execute in the loop",
		}
	}
	if step.MaxIterations < 1 {
		return nil, &errors.ValidationError{
			Field:      "max_iterations",
			Message:    "loop step requires max_iterations >= 1",
			Suggestion: "set max_iterations to a value between 1 and 100",
		}
	}
	if step.Until == "" {
		return nil, &errors.ValidationError{
			Field:      "until",
			Message:    "loop step requires until expression",
			Suggestion: "add an until expression to define when the loop terminates",
		}
	}

	// Apply step timeout if specified
	var cancel context.CancelFunc
	if step.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(step.Timeout)*time.Second)
		defer cancel()
	}

	// Initialize loop state
	loopStart := time.Now()
	history := make([]IterationRecord, 0, step.MaxIterations)
	var lastStepOutputs map[string]interface{}
	terminatedBy := LoopTerminatedByMaxIterations

	// Create expression evaluator
	eval := expression.New()

	e.logger.Debug("starting loop execution",
		"step_id", step.ID,
		"max_iterations", step.MaxIterations,
		"until", step.Until,
		"nested_steps", len(step.Steps),
	)

	// Do-while loop: execute at least once, then check condition
	for iteration := 0; iteration < step.MaxIterations; iteration++ {
		iterStart := time.Now()

		// Check for context cancellation (timeout or cancellation)
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				terminatedBy = LoopTerminatedByTimeout
				e.logger.Info("loop terminated by timeout",
					"step_id", step.ID,
					"iteration", iteration,
					"duration_ms", time.Since(loopStart).Milliseconds(),
				)
			} else {
				terminatedBy = LoopTerminatedByError
			}
			return e.buildLoopOutput(lastStepOutputs, iteration, terminatedBy, history), ctx.Err()
		default:
		}

		// Create loop context for this iteration
		loopCtx := LoopContext{
			Iteration:     iteration,
			MaxIterations: step.MaxIterations,
			History:       history,
		}

		// Create workflow context with loop context injected
		iterContext := e.createIterationContext(workflowContext, loopCtx)

		e.logger.Debug("starting iteration",
			"step_id", step.ID,
			"iteration", iteration,
			"max_iterations", step.MaxIterations,
		)

		// Execute nested steps sequentially
		stepOutputs := make(map[string]interface{})
		var iterationErr error

		for _, nestedStep := range step.Steps {
			// Check if step should be skipped based on condition
			if nestedStep.Condition != nil && nestedStep.Condition.Expression != "" {
				// Evaluate step condition with loop context
				condResult, condErr := eval.Evaluate(nestedStep.Condition.Expression, iterContext)
				if condErr != nil {
					e.logger.Warn("step condition evaluation failed, executing step",
						"step_id", nestedStep.ID,
						"error", condErr,
					)
				} else if !condResult {
					// Skip step, record null output
					stepOutputs[nestedStep.ID] = nil
					e.logger.Debug("step skipped due to condition",
						"loop_step_id", step.ID,
						"nested_step_id", nestedStep.ID,
						"iteration", iteration,
					)
					continue
				}
			}

			// Execute the nested step
			nestedResult, err := e.Execute(ctx, &nestedStep, iterContext)
			if err != nil {
				// Check step-level error handling
				if nestedStep.OnError != nil && nestedStep.OnError.Strategy == ErrorStrategyIgnore {
					// Record failure and continue
					stepOutputs[nestedStep.ID] = map[string]interface{}{
						"status": "failed",
						"error":  err.Error(),
					}
					e.logger.Debug("step failed but ignored",
						"loop_step_id", step.ID,
						"nested_step_id", nestedStep.ID,
						"iteration", iteration,
						"error", err,
					)
					continue
				}

				// Step failed with fail strategy - record error and break
				iterationErr = err
				stepOutputs[nestedStep.ID] = map[string]interface{}{
					"status": "failed",
					"error":  err.Error(),
				}
				e.logger.Debug("step failed",
					"loop_step_id", step.ID,
					"nested_step_id", nestedStep.ID,
					"iteration", iteration,
					"error", err,
				)
				break
			}

			// Record step output
			if nestedResult != nil && nestedResult.Output != nil {
				stepOutputs[nestedStep.ID] = nestedResult.Output
			} else {
				stepOutputs[nestedStep.ID] = nil
			}

			// Update iteration context with step output for subsequent steps
			if steps, ok := iterContext["steps"].(map[string]interface{}); ok {
				steps[nestedStep.ID] = stepOutputs[nestedStep.ID]
			}
		}

		// Record iteration in history
		record := IterationRecord{
			Iteration:  iteration,
			Steps:      e.maskSensitiveFields(stepOutputs),
			Timestamp:  time.Now(),
			DurationMs: time.Since(iterStart).Milliseconds(),
		}
		history = append(history, record)

		// Truncate history if it exceeds size limit
		history = e.truncateHistoryIfNeeded(history, step.ID)

		// Update last step outputs
		lastStepOutputs = stepOutputs

		// If there was an iteration error, check loop-level error handling
		if iterationErr != nil {
			if step.OnError != nil && step.OnError.Strategy == ErrorStrategyIgnore {
				// Log and continue to next iteration
				e.logger.Debug("iteration failed but ignored at loop level",
					"step_id", step.ID,
					"iteration", iteration,
					"error", iterationErr,
				)
				iterationErr = nil
			} else {
				// Terminate loop with error
				terminatedBy = LoopTerminatedByError
				e.logger.Info("loop terminated by error",
					"step_id", step.ID,
					"iteration", iteration,
					"error", iterationErr,
				)
				return e.buildLoopOutput(lastStepOutputs, iteration+1, terminatedBy, history), iterationErr
			}
		}

		// Evaluate until condition after iteration (do-while semantics)
		// Create context for condition evaluation with updated step outputs
		condCtx := e.createIterationContext(workflowContext, LoopContext{
			Iteration:     iteration,
			MaxIterations: step.MaxIterations,
			History:       history,
		})
		// Add step outputs to context
		if steps, ok := condCtx["steps"].(map[string]interface{}); ok {
			for k, v := range stepOutputs {
				steps[k] = v
			}
		}

		conditionMet, err := eval.Evaluate(step.Until, condCtx)
		if err != nil {
			// Log warning but don't fail - treat as condition not met
			e.logger.Warn("until condition evaluation failed",
				"step_id", step.ID,
				"iteration", iteration,
				"until", step.Until,
				"error", err,
			)
			conditionMet = false
		}

		e.logger.Debug("evaluated until condition",
			"step_id", step.ID,
			"iteration", iteration,
			"until", step.Until,
			"result", conditionMet,
		)

		if conditionMet {
			terminatedBy = LoopTerminatedByCondition
			e.logger.Info("loop terminated by condition",
				"step_id", step.ID,
				"iterations", iteration+1,
				"duration_ms", time.Since(loopStart).Milliseconds(),
			)
			return e.buildLoopOutput(lastStepOutputs, iteration+1, terminatedBy, history), nil
		}
	}

	// Reached max iterations
	e.logger.Info("loop terminated by max iterations",
		"step_id", step.ID,
		"iterations", step.MaxIterations,
		"duration_ms", time.Since(loopStart).Milliseconds(),
	)

	return e.buildLoopOutput(lastStepOutputs, step.MaxIterations, terminatedBy, history), nil
}

// createIterationContext creates a workflow context copy with loop context injected.
// The loop context is injected in two places:
// 1. Directly in the map as "loop" for expression evaluation
// 2. In the TemplateContext for Go template resolution
func (e *Executor) createIterationContext(workflowContext map[string]interface{}, loopCtx LoopContext) map[string]interface{} {
	// Copy workflow context
	iterContext := copyWorkflowContext(workflowContext)

	// Create loop context map
	loopMap := map[string]interface{}{
		"iteration":      loopCtx.Iteration,
		"max_iterations": loopCtx.MaxIterations,
		"history":        loopCtx.History,
	}

	// Inject loop context directly for expression evaluation
	iterContext["loop"] = loopMap

	// Also inject into TemplateContext for Go template resolution
	if templateCtx, ok := iterContext["_templateContext"].(*TemplateContext); ok {
		// Update existing template context with loop variables
		newTemplateCtx := &TemplateContext{
			Inputs: templateCtx.Inputs,
			Steps:  templateCtx.Steps,
			Env:    templateCtx.Env,
			Tools:  templateCtx.Tools,
			Loop:   loopMap,
		}
		iterContext["_templateContext"] = newTemplateCtx
	} else {
		// Create a new template context if one doesn't exist
		// This ensures loop context is available for template resolution
		steps := make(map[string]map[string]interface{})
		if existingSteps, ok := iterContext["steps"].(map[string]interface{}); ok {
			for k, v := range existingSteps {
				if stepMap, ok := v.(map[string]interface{}); ok {
					steps[k] = stepMap
				}
			}
		}
		iterContext["_templateContext"] = &TemplateContext{
			Inputs: make(map[string]interface{}),
			Steps:  steps,
			Env:    make(map[string]string),
			Tools:  make(map[string]interface{}),
			Loop:   loopMap,
		}
	}

	return iterContext
}

// buildLoopOutput constructs the output structure for a completed loop.
func (e *Executor) buildLoopOutput(stepOutputs map[string]interface{}, iterationCount int, terminatedBy string, history []IterationRecord) map[string]interface{} {
	output := map[string]interface{}{
		"step_outputs":    stepOutputs,
		"iteration_count": iterationCount,
		"terminated_by":   terminatedBy,
	}

	// Include history if not too large
	historySize := e.estimateHistorySize(history)
	if historySize <= MaxHistorySizeBytes {
		output["history"] = history
	} else {
		output["history"] = history
		output["history_truncated"] = true
		output["retained_iterations"] = len(history)
		output["total_iterations"] = iterationCount
	}

	return output
}

// estimateHistorySize estimates the JSON serialized size of the history.
func (e *Executor) estimateHistorySize(history []IterationRecord) int {
	data, err := json.Marshal(history)
	if err != nil {
		return 0
	}
	return len(data)
}

// truncateHistoryIfNeeded removes oldest entries if history exceeds size limit.
// Uses FIFO truncation strategy.
func (e *Executor) truncateHistoryIfNeeded(history []IterationRecord, stepID string) []IterationRecord {
	for len(history) > 1 && e.estimateHistorySize(history) > MaxHistorySizeBytes {
		// Remove oldest entry
		history = history[1:]
		e.logger.Info("truncated loop history due to size limit",
			"step_id", stepID,
			"retained_iterations", len(history),
		)
	}
	return history
}

// maskSensitiveFields replaces sensitive field values with a mask.
// Fields matching these patterns are masked: token, password, secret, api_key, credential
func (e *Executor) maskSensitiveFields(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	sensitivePatterns := []string{"token", "password", "secret", "api_key", "credential", "apikey"}
	masked := make(map[string]interface{}, len(data))

	for k, v := range data {
		keyLower := strings.ToLower(k)
		isSensitive := false
		for _, pattern := range sensitivePatterns {
			if strings.Contains(keyLower, pattern) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			masked[k] = "***MASKED***"
		} else {
			// Recursively mask nested maps
			if nestedMap, ok := v.(map[string]interface{}); ok {
				masked[k] = e.maskSensitiveFields(nestedMap)
			} else {
				masked[k] = v
			}
		}
	}

	return masked
}
