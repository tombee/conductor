// Package integration provides infrastructure for integration testing with real components.
//
// This package supports testing against real LLM providers, databases, HTTP servers,
// and MCP server processes instead of mocks. It includes:
//
//   - Cost tracking with per-test and suite budget enforcement
//   - Test configuration from environment variables
//   - Cleanup management for resource tracking
//   - Retry helpers with exponential backoff
//   - Common test fixtures and utilities
//
// # Cost Tracking
//
// Integration tests that call real LLM APIs must track costs to prevent runaway expenses:
//
//	tracker := integration.NewCostTracker()
//	tracker.SetTestBudget(0.50)  // $0.50 per test
//	tracker.SetSuiteBudget(25.0) // $25 total
//
//	// After each API call
//	if err := tracker.Record(usage, modelInfo); err != nil {
//	    t.Fatal(err) // Budget exceeded
//	}
//
// # Environment-Based Test Skipping
//
// Tests requiring external dependencies skip automatically when not configured:
//
//	integration.SkipWithoutEnv(t, "ANTHROPIC_API_KEY")
//
// # Cleanup Management
//
// Track and verify cleanup of test resources:
//
//	cleanup := integration.NewCleanupManager(t)
//	cleanup.Add("database connection", dbConn.Close)
//	cleanup.Add("temp file", func() error { return os.Remove(tmpFile) })
//	// Cleanup runs automatically via t.Cleanup()
//
// # Retry Logic
//
// Retry transient failures with exponential backoff:
//
//	err := integration.Retry(ctx, func() error {
//	    return makeAPICall()
//	}, integration.DefaultRetryConfig())
//
// # Test Build Tags
//
// Integration tests use build tags for selective execution:
//
//   - //go:build integration - Basic integration tests (SQLite, local)
//   - //go:build integration && postgres - Postgres tests via testcontainers
//   - //go:build integration && nightly - Full API coverage (Tier 3)
package integration
