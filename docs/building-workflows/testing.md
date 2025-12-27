# Testing Workflows

Strategies for testing Conductor workflows.

## Validation

Always validate before running:

```bash
# Validate syntax
conductor validate workflow.yaml

# Dry run to preview execution
conductor run workflow.yaml --dry-run
```

## Manual Testing

Test with sample inputs:

```bash
conductor run workflow.yaml \
  --input user_id=12345 \
  --input action=review

# Get JSON output for inspection
conductor run workflow.yaml --output json | jq '.steps.analyze'
```

## Test Workflows

Create test workflows for complex logic:

```yaml
# tests/test-code-review.yaml
name: test-code-review
steps:
  - id: load_fixture
    file.read: "tests/fixtures/sample-code.py"

  - id: run_review
    model: balanced
    prompt: "Review this code: {{.steps.load_fixture.content}}"

  - id: validate
    model: fast
    prompt: |
      Check if this review identifies SQL injection:
      {{.steps.run_review.response}}

      Output: PASS or FAIL
```

## CI/CD Integration

Run tests in your pipeline:

```yaml
# .github/workflows/test.yml
name: Test Workflows
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Conductor
        run: |
          curl -L https://github.com/tombee/conductor/releases/latest/download/conductor-linux-amd64 -o conductor
          chmod +x conductor
          sudo mv conductor /usr/local/bin/

      - name: Validate workflows
        run: |
          find workflows/ -name "*.yaml" -exec conductor validate {} \;

      - name: Run tests
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
        run: conductor run tests/test-suite.yaml
```

## Testing LLM Steps

LLM outputs are non-deterministic. Strategies:

1. **Request structured output** for easier validation:
   ```yaml
   prompt: |
     Analyze code. Output JSON: {"issues": [], "severity": "low|medium|high"}
   ```

2. **Lower temperature** for consistency:
   ```yaml
   temperature: 0.1
   ```

3. **Validate patterns**, not exact matches:
   ```yaml
   - id: validate
     condition: 'steps.analyze.response contains "security"'
   ```

## Best Practices

1. **Test edge cases** - Empty inputs, invalid formats, missing fields
2. **Use fixtures** - Reusable test data in `tests/fixtures/`
3. **Keep tests fast** - Use `fast` model tier, mock slow APIs
4. **Make tests deterministic** - Fixed data, low temperature
5. **Test error paths** - Verify retry and timeout behavior

## See Also

- [Debugging](debugging.md) - Troubleshooting workflows
- [Error Handling](error-handling.md) - Testing failure scenarios
