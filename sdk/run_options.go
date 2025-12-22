package sdk

// RunOption is a functional option for per-run configuration.
type RunOption func(*runConfig)

// runConfig holds per-run configuration.
type runConfig struct {
	credentials   map[string]string
	mcpServers    []string
	tokenLimit    int
	hasTokenLimit bool
}

// WithCredentials provides credentials for integrations.
// Keys are integration names (e.g., "github", "slack").
//
// Example:
//
//	result, err := s.Run(ctx, wf, inputs,
//		sdk.WithCredentials(map[string]string{
//			"github": os.Getenv("GITHUB_TOKEN"),
//			"slack":  os.Getenv("SLACK_TOKEN"),
//		}),
//	)
func WithCredentials(creds map[string]string) RunOption {
	return func(rc *runConfig) {
		rc.credentials = creds
	}
}

// WithGitHubToken provides a GitHub token for this run.
// This is a convenience wrapper for WithCredentials.
//
// Example:
//
//	result, err := s.Run(ctx, wf, inputs,
//		sdk.WithGitHubToken(os.Getenv("GITHUB_TOKEN")),
//	)
func WithGitHubToken(token string) RunOption {
	return func(rc *runConfig) {
		if rc.credentials == nil {
			rc.credentials = make(map[string]string)
		}
		rc.credentials["github"] = token
	}
}

// WithSlackToken provides a Slack token for this run.
// This is a convenience wrapper for WithCredentials.
//
// Example:
//
//	result, err := s.Run(ctx, wf, inputs,
//		sdk.WithSlackToken(os.Getenv("SLACK_TOKEN")),
//	)
func WithSlackToken(token string) RunOption {
	return func(rc *runConfig) {
		if rc.credentials == nil {
			rc.credentials = make(map[string]string)
		}
		rc.credentials["slack"] = token
	}
}

// WithMCPServers enables specific MCP servers for this run.
// Only servers configured at SDK creation can be enabled.
// No WithMCPServers call = no MCP access (opt-in model).
//
// Example:
//
//	result, err := s.Run(ctx, wf, inputs,
//		sdk.WithMCPServers("github", "filesystem"),
//	)
func WithMCPServers(names ...string) RunOption {
	return func(rc *runConfig) {
		rc.mcpServers = names
	}
}

// WithRunTokenLimit overrides the SDK-level token limit for this run.
//
// Example:
//
//	result, err := s.Run(ctx, wf, inputs,
//		sdk.WithRunTokenLimit(50000), // 50k tokens max for this run
//	)
func WithRunTokenLimit(maxTokens int) RunOption {
	return func(rc *runConfig) {
		rc.tokenLimit = maxTokens
		rc.hasTokenLimit = true
	}
}
