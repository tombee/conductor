# Project Memory

- Don't create session summary docs
- Don't add "Generated with Claude Code" or "Co-Authored-By" lines to git commits
- Don't put SPEC IDs (e.g., SPEC-130) in code comments

## Canonical Terminology

When writing code or documentation for Conductor, use these terms consistently:

- **controller** — The long-running service process (not "daemon")
- **action** — Local operations like file, shell, http, utility, transform (not "connector")
- **integration** — External service APIs like GitHub, Slack, Jira (not "connector")
- **trigger** — Workflow invocation configuration in YAML (not "listen")
- **executor** — The component that executes workflow steps (not "engine")
