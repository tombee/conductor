package sdk

// StepBuilder provides fluent step definition.
type StepBuilder struct {
	workflow *WorkflowBuilder
	id       string
	stepDef  *stepDef
}

// LLM configures this as an LLM step.
//
// Example:
//
//	.Step("analyze").LLM().
//		Model("claude-sonnet-4-20250514").
//		Prompt("Analyze this: {{.inputs.text}}").
//		Done()
func (s *StepBuilder) LLM() *LLMStepBuilder {
	s.stepDef = &stepDef{
		id:       s.id,
		stepType: "llm",
	}
	return &LLMStepBuilder{
		stepBuilder: s,
	}
}

// Action configures this as an action step.
//
// Example:
//
//	.Step("write").Action("file").
//		Input("path", "/tmp/output.txt").
//		Input("content", "{{.steps.generate.output}}").
//		Done()
func (s *StepBuilder) Action(name string) *ActionStepBuilder {
	s.stepDef = &stepDef{
		id:           s.id,
		stepType:     "action",
		actionName:   name,
		actionInputs: make(map[string]any),
	}
	return &ActionStepBuilder{
		stepBuilder: s,
	}
}

// Agent configures this as an agent step.
//
// Example:
//
//	.Step("research").Agent().
//		Prompt("Research {{.inputs.topic}} and summarize findings").
//		MaxIterations(5).
//		Done()
func (s *StepBuilder) Agent() *AgentStepBuilder {
	s.stepDef = &stepDef{
		id:       s.id,
		stepType: "agent",
	}
	return &AgentStepBuilder{
		stepBuilder: s,
	}
}

// Parallel configures this as a parallel step group.
//
// Example:
//
//	.Step("parallel").Parallel().
//		Step("task1").LLM().Prompt("Task 1").Done().
//		Step("task2").LLM().Prompt("Task 2").Done().
//		Done()
func (s *StepBuilder) Parallel() *ParallelStepBuilder {
	s.stepDef = &stepDef{
		id:            s.id,
		stepType:      "parallel",
		parallelSteps: make([]*stepDef, 0),
	}
	return &ParallelStepBuilder{
		stepBuilder: s,
	}
}

// Condition configures this as a conditional step.
//
// Example:
//
//	.Step("check").Condition("{{.inputs.mode}} == 'prod'").
//		Then().Step("deploy").Action("shell").Done().
//		Else().Step("test").Action("shell").Done().
//		Done()
func (s *StepBuilder) Condition(expr string) *ConditionStepBuilder {
	s.stepDef = &stepDef{
		id:        s.id,
		stepType:  "condition",
		condition: expr,
		thenSteps: make([]*stepDef, 0),
		elseSteps: make([]*stepDef, 0),
	}
	return &ConditionStepBuilder{
		stepBuilder: s,
	}
}

// DependsOn declares step dependencies.
// The step will wait for all dependencies to complete before executing.
//
// Example:
//
//	.Step("summarize").LLM().
//		DependsOn("research", "analyze").
//		Prompt("Summarize findings").
//		Done()
func (s *StepBuilder) DependsOn(stepIDs ...string) *StepBuilder {
	if s.stepDef == nil {
		s.stepDef = &stepDef{
			id: s.id,
		}
	}
	s.stepDef.dependencies = append(s.stepDef.dependencies, stepIDs...)
	return s
}

// LLMStepBuilder provides LLM step configuration.
type LLMStepBuilder struct {
	stepBuilder *StepBuilder
}

// Model sets the LLM model.
//
// Example:
//
//	.Model("claude-sonnet-4-20250514")
func (l *LLMStepBuilder) Model(model string) *LLMStepBuilder {
	l.stepBuilder.stepDef.model = model
	return l
}

// System sets the system prompt.
//
// Example:
//
//	.System("You are a helpful code reviewer.")
func (l *LLMStepBuilder) System(prompt string) *LLMStepBuilder {
	l.stepBuilder.stepDef.system = prompt
	return l
}

// Prompt sets the user prompt (supports templates).
//
// Example:
//
//	.Prompt("Review this code: {{.inputs.code}}")
func (l *LLMStepBuilder) Prompt(prompt string) *LLMStepBuilder {
	l.stepBuilder.stepDef.prompt = prompt
	return l
}

// OutputSchema sets expected output schema for structured output.
//
// Example:
//
//	.OutputSchema(map[string]any{
//		"type": "object",
//		"properties": map[string]any{
//			"score": map[string]any{"type": "number"},
//			"issues": map[string]any{"type": "array"},
//		},
//	})
func (l *LLMStepBuilder) OutputSchema(schema map[string]any) *LLMStepBuilder {
	l.stepBuilder.stepDef.outputSchema = schema
	return l
}

// Tools limits available tools for this step.
//
// Example:
//
//	.Tools("get_weather", "search_web")
func (l *LLMStepBuilder) Tools(names ...string) *LLMStepBuilder {
	l.stepBuilder.stepDef.tools = names
	return l
}

// Temperature sets the sampling temperature.
//
// Example:
//
//	.Temperature(0.7)
func (l *LLMStepBuilder) Temperature(t float64) *LLMStepBuilder {
	l.stepBuilder.stepDef.temperature = &t
	return l
}

// MaxTokens sets the maximum response tokens.
//
// Example:
//
//	.MaxTokens(1000)
func (l *LLMStepBuilder) MaxTokens(n int) *LLMStepBuilder {
	l.stepBuilder.stepDef.maxTokens = &n
	return l
}

// DependsOn declares step dependencies.
func (l *LLMStepBuilder) DependsOn(stepIDs ...string) *LLMStepBuilder {
	l.stepBuilder.stepDef.dependencies = append(l.stepBuilder.stepDef.dependencies, stepIDs...)
	return l
}

// Done completes the step definition and returns to workflow builder.
func (l *LLMStepBuilder) Done() *WorkflowBuilder {
	l.stepBuilder.workflow.steps = append(l.stepBuilder.workflow.steps, l.stepBuilder.stepDef)
	return l.stepBuilder.workflow
}

// ActionStepBuilder provides action step configuration.
type ActionStepBuilder struct {
	stepBuilder *StepBuilder
}

// Input sets an action input parameter.
//
// Example:
//
//	.Action("file").
//		Input("path", "/tmp/output.txt").
//		Input("content", "{{.steps.generate.output}}")
func (a *ActionStepBuilder) Input(name string, value any) *ActionStepBuilder {
	a.stepBuilder.stepDef.actionInputs[name] = value
	return a
}

// DependsOn declares step dependencies.
func (a *ActionStepBuilder) DependsOn(stepIDs ...string) *ActionStepBuilder {
	a.stepBuilder.stepDef.dependencies = append(a.stepBuilder.stepDef.dependencies, stepIDs...)
	return a
}

// Done completes the step definition and returns to workflow builder.
func (a *ActionStepBuilder) Done() *WorkflowBuilder {
	a.stepBuilder.workflow.steps = append(a.stepBuilder.workflow.steps, a.stepBuilder.stepDef)
	return a.stepBuilder.workflow
}

// AgentStepBuilder provides agent step configuration.
type AgentStepBuilder struct {
	stepBuilder *StepBuilder
}

// Prompt sets the agent's objective.
//
// Example:
//
//	.Agent().
//		Prompt("Research {{.inputs.topic}} and provide key findings")
func (a *AgentStepBuilder) Prompt(prompt string) *AgentStepBuilder {
	a.stepBuilder.stepDef.agentPrompt = prompt
	return a
}

// System sets the system prompt for the agent.
//
// Example:
//
//	.Agent().
//		System("You are a helpful research assistant.").
//		Prompt("Research {{.inputs.topic}}")
func (a *AgentStepBuilder) System(prompt string) *AgentStepBuilder {
	a.stepBuilder.stepDef.agentSystemPrompt = prompt
	return a
}

// Model sets the LLM model for the agent.
//
// Example:
//
//	.Agent().
//		Model("claude-sonnet-4-20250514").
//		Prompt("Research the topic")
func (a *AgentStepBuilder) Model(model string) *AgentStepBuilder {
	a.stepBuilder.stepDef.model = model
	return a
}

// Tools sets the tools available to the agent.
//
// Example:
//
//	.Agent().
//		Tools("web_search", "file_read").
//		Prompt("Find information about the topic")
func (a *AgentStepBuilder) Tools(names ...string) *AgentStepBuilder {
	a.stepBuilder.stepDef.tools = names
	return a
}

// MaxIterations sets the maximum number of agent loop iterations.
//
// Example:
//
//	.Agent().
//		MaxIterations(10).
//		Prompt("Complete the task")
func (a *AgentStepBuilder) MaxIterations(n int) *AgentStepBuilder {
	a.stepBuilder.stepDef.agentMaxIter = n
	return a
}

// TokenLimit sets the cumulative token limit across all iterations.
//
// Example:
//
//	.Agent().
//		TokenLimit(50000).
//		Prompt("Complete the task")
func (a *AgentStepBuilder) TokenLimit(n int) *AgentStepBuilder {
	a.stepBuilder.stepDef.agentTokenLimit = n
	return a
}

// StopOnError determines whether the agent should stop on first tool error.
//
// Example:
//
//	.Agent().
//		StopOnError(true).
//		Prompt("Complete the task")
func (a *AgentStepBuilder) StopOnError(stop bool) *AgentStepBuilder {
	a.stepBuilder.stepDef.agentStopOnError = stop
	return a
}

// DependsOn declares step dependencies.
func (a *AgentStepBuilder) DependsOn(stepIDs ...string) *AgentStepBuilder {
	a.stepBuilder.stepDef.dependencies = append(a.stepBuilder.stepDef.dependencies, stepIDs...)
	return a
}

// Done completes the step definition and returns to workflow builder.
func (a *AgentStepBuilder) Done() *WorkflowBuilder {
	a.stepBuilder.workflow.steps = append(a.stepBuilder.workflow.steps, a.stepBuilder.stepDef)
	return a.stepBuilder.workflow
}

// ParallelStepBuilder provides parallel step group configuration.
type ParallelStepBuilder struct {
	stepBuilder *StepBuilder
}

// WithMaxConcurrency limits concurrent step execution.
//
// Example:
//
//	.Parallel().
//		WithMaxConcurrency(3)
func (p *ParallelStepBuilder) WithMaxConcurrency(n int) *ParallelStepBuilder {
	p.stepBuilder.stepDef.maxConcurrency = n
	return p
}

// Step adds a sub-step to the parallel group.
func (p *ParallelStepBuilder) Step(id string) *StepBuilder {
	// TODO: Return a StepBuilder that appends to parallelSteps
	return &StepBuilder{
		workflow: p.stepBuilder.workflow,
		id:       id,
	}
}

// Done completes the step definition and returns to workflow builder.
func (p *ParallelStepBuilder) Done() *WorkflowBuilder {
	p.stepBuilder.workflow.steps = append(p.stepBuilder.workflow.steps, p.stepBuilder.stepDef)
	return p.stepBuilder.workflow
}

// ConditionStepBuilder provides conditional step configuration.
type ConditionStepBuilder struct {
	stepBuilder *StepBuilder
}

// Then starts defining steps to execute if condition is true.
func (c *ConditionStepBuilder) Then() *ConditionalBranchBuilder {
	return &ConditionalBranchBuilder{
		parent: c,
		isThen: true,
	}
}

// Else starts defining steps to execute if condition is false.
func (c *ConditionStepBuilder) Else() *ConditionalBranchBuilder {
	return &ConditionalBranchBuilder{
		parent: c,
		isThen: false,
	}
}

// Done completes the step definition and returns to workflow builder.
func (c *ConditionStepBuilder) Done() *WorkflowBuilder {
	c.stepBuilder.workflow.steps = append(c.stepBuilder.workflow.steps, c.stepBuilder.stepDef)
	return c.stepBuilder.workflow
}

// ConditionalBranchBuilder provides then/else branch configuration.
type ConditionalBranchBuilder struct {
	parent *ConditionStepBuilder
	isThen bool
}

// Step adds a step to the conditional branch.
func (c *ConditionalBranchBuilder) Step(id string) *StepBuilder {
	// TODO: Return a StepBuilder that appends to thenSteps or elseSteps
	return &StepBuilder{
		workflow: c.parent.stepBuilder.workflow,
		id:       id,
	}
}

// Done returns to the condition builder.
func (c *ConditionalBranchBuilder) Done() *ConditionStepBuilder {
	return c.parent
}
