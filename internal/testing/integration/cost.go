package integration

import (
	"fmt"
	"sync"

	"github.com/tombee/conductor/pkg/llm"
)

// Default budget limits for integration tests (in tokens).
const (
	DefaultTestTokenBudget  = 50000   // 50k tokens per test
	DefaultSuiteTokenBudget = 1000000 // 1M tokens per suite run
)

// CostTracker tracks LLM token usage during integration tests with budget enforcement.
type CostTracker struct {
	mu               sync.Mutex
	testTokenBudget  int
	suiteTokenBudget int
	testTokens       int
	suiteTokens      int
}

// NewCostTracker creates a new cost tracker with default budgets.
func NewCostTracker() *CostTracker {
	return &CostTracker{
		testTokenBudget:  DefaultTestTokenBudget,
		suiteTokenBudget: DefaultSuiteTokenBudget,
	}
}

// SetTestBudget sets the maximum tokens per test (default 50k).
func (ct *CostTracker) SetTestBudget(budget int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.testTokenBudget = budget
}

// SetSuiteBudget sets the maximum tokens for the entire test suite (default 1M).
func (ct *CostTracker) SetSuiteBudget(budget int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.suiteTokenBudget = budget
}

// Record records the token usage of an API call and checks budgets.
// Returns an error if either budget is exceeded.
func (ct *CostTracker) Record(usage llm.TokenUsage) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Update running totals
	ct.testTokens += usage.TotalTokens
	ct.suiteTokens += usage.TotalTokens

	// Check test budget
	if ct.testTokens > ct.testTokenBudget {
		return fmt.Errorf("test token budget exceeded: %d > %d", ct.testTokens, ct.testTokenBudget)
	}

	// Check suite budget
	if ct.suiteTokens > ct.suiteTokenBudget {
		return fmt.Errorf("suite token budget exceeded: %d > %d", ct.suiteTokens, ct.suiteTokenBudget)
	}

	return nil
}

// ResetTest resets the test-level token counter (call at start of each test).
func (ct *CostTracker) ResetTest() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.testTokens = 0
}

// GetTestTokens returns the current test token usage.
func (ct *CostTracker) GetTestTokens() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.testTokens
}

// GetSuiteTokens returns the total suite token usage.
func (ct *CostTracker) GetSuiteTokens() int {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.suiteTokens
}

// GetBudgets returns the current budget limits.
func (ct *CostTracker) GetBudgets() (testBudget, suiteBudget int) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.testTokenBudget, ct.suiteTokenBudget
}
