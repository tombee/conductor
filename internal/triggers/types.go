package triggers

import "time"

// WebhookTrigger represents a webhook trigger configuration.
type WebhookTrigger struct {
	Path         string            `json:"path"`
	Source       string            `json:"source"`
	Workflow     string            `json:"workflow"`
	Events       []string          `json:"events,omitempty"`
	Secret       string            `json:"secret,omitempty"`
	InputMapping map[string]string `json:"input_mapping,omitempty"`
}

// ScheduleTrigger represents a schedule trigger configuration.
type ScheduleTrigger struct {
	Name     string         `json:"name"`
	Cron     string         `json:"cron"`
	Workflow string         `json:"workflow"`
	Inputs   map[string]any `json:"inputs,omitempty"`
	Enabled  bool           `json:"enabled"`
	Timezone string         `json:"timezone,omitempty"`
}

// EndpointTrigger represents an API endpoint trigger configuration.
type EndpointTrigger struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Workflow    string         `json:"workflow"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	Scopes      []string       `json:"scopes,omitempty"`
	RateLimit   string         `json:"rate_limit,omitempty"`
	Timeout     time.Duration  `json:"timeout,omitempty"`
	Secret      string         `json:"secret,omitempty"`
}

// CreateWebhookRequest is the request to create a webhook trigger.
type CreateWebhookRequest struct {
	Workflow     string            `json:"workflow"`
	Path         string            `json:"path"`
	Source       string            `json:"source"`
	Events       []string          `json:"events,omitempty"`
	Secret       string            `json:"secret,omitempty"`
	InputMapping map[string]string `json:"input_mapping,omitempty"`
}

// CreateScheduleRequest is the request to create a schedule trigger.
type CreateScheduleRequest struct {
	Workflow string         `json:"workflow"`
	Name     string         `json:"name"`
	Cron     string         `json:"cron,omitempty"`
	Every    string         `json:"every,omitempty"`
	At       string         `json:"at,omitempty"`
	Timezone string         `json:"timezone,omitempty"`
	Inputs   map[string]any `json:"inputs,omitempty"`
}

// CreateEndpointRequest is the request to create an endpoint trigger.
type CreateEndpointRequest struct {
	Workflow    string         `json:"workflow"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Secret      string         `json:"secret,omitempty"`
	Inputs      map[string]any `json:"inputs,omitempty"`
	Scopes      []string       `json:"scopes,omitempty"`
	RateLimit   string         `json:"rate_limit,omitempty"`
	Timeout     time.Duration  `json:"timeout,omitempty"`
}

// FileWatcherTrigger represents a file watcher trigger configuration.
type FileWatcherTrigger struct {
	Name                 string         `json:"name"`
	Path                 string         `json:"path"`
	Workflow             string         `json:"workflow"`
	Events               []string       `json:"events,omitempty"`
	IncludePatterns      []string       `json:"include_patterns,omitempty"`
	ExcludePatterns      []string       `json:"exclude_patterns,omitempty"`
	DebounceWindow       time.Duration  `json:"debounce_window,omitempty"`
	BatchMode            bool           `json:"batch_mode,omitempty"`
	MaxTriggersPerMinute int            `json:"max_triggers_per_minute,omitempty"`
	Inputs               map[string]any `json:"inputs,omitempty"`
	Enabled              bool           `json:"enabled"`
}

// CreateFileWatcherRequest is the request to create a file watcher trigger.
type CreateFileWatcherRequest struct {
	Workflow             string         `json:"workflow"`
	Path                 string         `json:"path"`
	Events               []string       `json:"events,omitempty"`
	IncludePatterns      []string       `json:"include_patterns,omitempty"`
	ExcludePatterns      []string       `json:"exclude_patterns,omitempty"`
	DebounceWindow       time.Duration  `json:"debounce_window,omitempty"`
	BatchMode            bool           `json:"batch_mode,omitempty"`
	MaxTriggersPerMinute int            `json:"max_triggers_per_minute,omitempty"`
	Inputs               map[string]any `json:"inputs,omitempty"`
}
