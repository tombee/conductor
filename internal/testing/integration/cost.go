package integration

import (
	"fmt"
	"sync"

	"github.com/tombee/conductor/pkg/llm"
)

// Default budget limits for integration tests.
const (
	DefaultTestBudget  = 0.50 // $0.50 per test
	DefaultSuiteBudget = 25.0 // $25.00 per suite run
)

// CostTracker tracks LLM API costs during integration tests with budget enforcement.
// It wraps the existing llm.CostTracker infrastructure but adds fail-fast budgets.
type CostTracker struct {
	mu          sync.Mutex
	testBudget  float64
	suiteBudget float64
	testCost    float64
	suiteCost   float64
}

// NewCostTracker creates a new cost tracker with default budgets.
func NewCostTracker() *CostTracker {
	return &CostTracker{
		testBudget:  DefaultTestBudget,
		suiteBudget: DefaultSuiteBudget,
	}
}

// SetTestBudget sets the maximum cost per test (default $0.50).
func (ct *CostTracker) SetTestBudget(budget float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.testBudget = budget
}

// SetSuiteBudget sets the maximum cost for the entire test suite (default $25).
func (ct *CostTracker) SetSuiteBudget(budget float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.suiteBudget = budget
}

// Record records the cost of an API call and checks budgets.
// Returns an error if either budget is exceeded.
func (ct *CostTracker) Record(usage llm.TokenUsage, modelInfo llm.ModelInfo) error {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	// Calculate cost using existing infrastructure (with cache support)
	costInfo := modelInfo.CalculateCostWithCache(usage)

	// Update running totals
	ct.testCost += costInfo.Amount
	ct.suiteCost += costInfo.Amount

	// Check test budget
	if ct.testCost > ct.testBudget {
		return fmt.Errorf("test budget exceeded: $%.4f > $%.2f", ct.testCost, ct.testBudget)
	}

	// Check suite budget
	if ct.suiteCost > ct.suiteBudget {
		return fmt.Errorf("suite budget exceeded: $%.4f > $%.2f", ct.suiteCost, ct.suiteBudget)
	}

	return nil
}

// ResetTest resets the test-level cost counter (call at start of each test).
func (ct *CostTracker) ResetTest() {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.testCost = 0
}

// GetTestCost returns the current test cost.
func (ct *CostTracker) GetTestCost() float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.testCost
}

// GetSuiteCost returns the total suite cost.
func (ct *CostTracker) GetSuiteCost() float64 {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.suiteCost
}

// GetBudgets returns the current budget limits.
func (ct *CostTracker) GetBudgets() (testBudget, suiteBudget float64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	return ct.testBudget, ct.suiteBudget
}
