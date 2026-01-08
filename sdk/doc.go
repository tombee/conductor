// Package sdk provides an embeddable workflow execution library for Go applications.
//
// ConductorSDK enables developers to embed the same battle-tested workflow engine,
// multi-provider LLM abstraction, and agent loops from Conductor into their own
// applications without requiring a daemon or CLI dependency.
//
// # Quick Start
//
// Create an SDK instance with a provider and execute a workflow:
//
//	import conductorsdk "github.com/tombee/conductor/sdk"
//
//	func main() {
//		// Create SDK with Anthropic provider
//		s, err := conductorsdk.New(
//			conductorsdk.WithAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY")),
//		)
//		if err != nil {
//			log.Fatal(err)
//		}
//		defer s.Close()
//
//		// Define workflow programmatically
//		wf, err := s.NewWorkflow("hello").
//			Step("greet").LLM().
//				Model("claude-sonnet-4-20250514").
//				System("You are a helpful assistant.").
//				Prompt("Say hello to the world!").
//				Done().
//			Build()
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		// Execute workflow
//		result, err := s.Run(context.Background(), wf, nil)
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		log.Printf("Result: %v", result.Output)
//		log.Printf("Cost: $%.4f", result.Cost.EstimatedCost)
//	}
//
// # When to Use ConductorSDK
//
// Use ConductorSDK when you need:
//   - Custom UI/UX: Building desktop, mobile, or web apps with your own interface
//   - Programmatic workflows: Workflows generated dynamically at runtime
//   - No external processes: Embedding where you can't run a daemon
//   - Deep integration: Workflows as a feature of your product
//
// Most users should use the Conductor platform directly for faster development
// and built-in features like webhooks, scheduling, and triggers.
//
// # Key Features
//
//   - Workflow Execution: Multi-step workflows with LLM calls, dependencies, parallel steps
//   - LLM Abstraction: Multi-provider support (Anthropic, OpenAI, Ollama) with cost tracking
//   - Action System: Built-in actions (file, shell, http) plus custom tool registration
//   - Agent Loops: ReAct-style agent execution with tool use
//   - Code Truncation: Language-aware code truncation for context window optimization
//   - Event Streaming: Real-time events for UI integration
//   - Security: Credential handling, MCP server trust model
//
// # Code Truncation
//
// TruncateCode intelligently shortens code files while preserving structural integrity.
// Instead of naive character or line truncation, it understands code structure and
// truncates at natural boundaries (between functions, after imports, at class boundaries).
//
// Example: Truncate a large file for LLM context:
//
//	result, err := sdk.TruncateCode(sourceCode, sdk.TruncateOptions{
//		MaxLines:     500,           // Limit to 500 lines
//		Language:     "go",          // Structure-aware Go truncation
//		PreserveTop:  true,          // Keep imports and package declaration
//		PreserveFunc: true,          // Don't cut mid-function
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Truncated from %d to %d lines\n",
//		result.OriginalLines, result.FinalLines)
//	fmt.Printf("Omitted %d functions\n", len(result.OmittedItems))
//
// Supported languages: Go, TypeScript, Python, JavaScript. Unknown languages fall back
// to line-based truncation. Token-based truncation is also supported using the
// MaxTokens option with a chars/4 estimation heuristic.
//
// The function is thread-safe, deterministic, and includes panic recovery for graceful
// error handling.
//
// # Architecture
//
// The SDK wraps existing pkg/* packages with a stable public API:
//   - pkg/workflow: Core workflow executor
//   - pkg/llm: Provider interface and implementations
//   - pkg/tools: Tool registry for LLM function calling
//   - pkg/agent: ReAct-style agent loops
//
// The SDK operates entirely standalone with no daemon dependency, no shared config,
// and no environment variable reads.
package sdk
