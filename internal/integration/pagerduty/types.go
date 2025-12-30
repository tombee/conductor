package pagerduty

// PagerDutyResponse represents the common PagerDuty API response wrapper.
type PagerDutyResponse struct {
	Error *APIError `json:"error,omitempty"`
}

// APIError represents a PagerDuty API error.
type APIError struct {
	Message string   `json:"message"`
	Code    int      `json:"code"`
	Errors  []string `json:"errors,omitempty"`
}

// Incident represents a PagerDuty incident.
type Incident struct {
	ID                string      `json:"id"`
	Type              string      `json:"type"`
	Summary           string      `json:"summary"`
	Self              string      `json:"self"`
	HTMLURL           string      `json:"html_url"`
	IncidentNumber    int         `json:"incident_number"`
	Title             string      `json:"title"`
	CreatedAt         string      `json:"created_at"`
	UpdatedAt         string      `json:"updated_at"`
	Status            string      `json:"status"` // triggered, acknowledged, resolved
	Urgency           string      `json:"urgency"` // high, low
	Priority          *Priority   `json:"priority,omitempty"`
	Service           *Reference  `json:"service,omitempty"`
	Assignments       []Assignment `json:"assignments,omitempty"`
	EscalationPolicy  *Reference  `json:"escalation_policy,omitempty"`
	Teams             []Reference `json:"teams,omitempty"`
	Acknowledgements  []Acknowledgement `json:"acknowledgements,omitempty"`
	LastStatusChangeAt string     `json:"last_status_change_at,omitempty"`
	LastStatusChangeBy *Reference `json:"last_status_change_by,omitempty"`
	FirstTriggerLogEntry *Reference `json:"first_trigger_log_entry,omitempty"`
	ResolveReason     *ResolveReason `json:"resolve_reason,omitempty"`
	AlertCounts       *AlertCounts `json:"alert_counts,omitempty"`
	Description       string      `json:"description,omitempty"`
}

// Priority represents incident priority.
type Priority struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Summary string `json:"summary"`
	Self    string `json:"self"`
	Name    string `json:"name"`
}

// Reference is a generic reference to another PagerDuty object.
type Reference struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Summary string `json:"summary,omitempty"`
	Self    string `json:"self,omitempty"`
	HTMLURL string `json:"html_url,omitempty"`
}

// Assignment represents an incident assignment.
type Assignment struct {
	At       string    `json:"at"`
	Assignee Reference `json:"assignee"`
}

// Acknowledgement represents an incident acknowledgement.
type Acknowledgement struct {
	At           string    `json:"at"`
	Acknowledger Reference `json:"acknowledger"`
}

// ResolveReason contains information about why an incident was resolved.
type ResolveReason struct {
	Type     string    `json:"type"`
	Incident *Reference `json:"incident,omitempty"`
}

// AlertCounts contains alert statistics for an incident.
type AlertCounts struct {
	All       int `json:"all"`
	Triggered int `json:"triggered"`
	Resolved  int `json:"resolved"`
}

// ListIncidentsResponse represents the response from listing incidents.
type ListIncidentsResponse struct {
	Incidents []Incident `json:"incidents"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
	Total     *int       `json:"total,omitempty"`
	More      bool       `json:"more"`
}

// GetIncidentResponse represents the response from getting an incident.
type GetIncidentResponse struct {
	Incident Incident `json:"incident"`
}

// User represents a PagerDuty user.
type User struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Summary     string `json:"summary"`
	Self        string `json:"self"`
	HTMLURL     string `json:"html_url"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	TimeZone    string `json:"time_zone,omitempty"`
	Color       string `json:"color,omitempty"`
	Role        string `json:"role,omitempty"`
	JobTitle    string `json:"job_title,omitempty"`
	Description string `json:"description,omitempty"`
}

// GetCurrentUserResponse represents the response from getting the current user.
type GetCurrentUserResponse struct {
	User User `json:"user"`
}

// Service represents a PagerDuty service.
type Service struct {
	ID               string     `json:"id"`
	Type             string     `json:"type"`
	Summary          string     `json:"summary"`
	Self             string     `json:"self"`
	HTMLURL          string     `json:"html_url"`
	Name             string     `json:"name"`
	Description      string     `json:"description,omitempty"`
	Status           string     `json:"status"` // active, warning, critical, maintenance, disabled
	EscalationPolicy *Reference `json:"escalation_policy,omitempty"`
	Teams            []Reference `json:"teams,omitempty"`
}

// ListServicesResponse represents the response from listing services.
type ListServicesResponse struct {
	Services []Service `json:"services"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
	Total    *int      `json:"total,omitempty"`
	More     bool      `json:"more"`
}

// OnCall represents an on-call entry.
type OnCall struct {
	User             Reference  `json:"user"`
	Schedule         *Reference `json:"schedule,omitempty"`
	EscalationPolicy Reference  `json:"escalation_policy"`
	EscalationLevel  int        `json:"escalation_level"`
	Start            string     `json:"start,omitempty"`
	End              string     `json:"end,omitempty"`
}

// ListOnCallsResponse represents the response from listing on-calls.
type ListOnCallsResponse struct {
	OnCalls []OnCall `json:"oncalls"`
	Limit   int      `json:"limit"`
	Offset  int      `json:"offset"`
	More    bool     `json:"more"`
}

// IncidentNote represents a note on an incident.
type IncidentNote struct {
	ID        string    `json:"id"`
	User      Reference `json:"user"`
	Content   string    `json:"content"`
	CreatedAt string    `json:"created_at"`
}

// ListIncidentNotesResponse represents the response from listing incident notes.
type ListIncidentNotesResponse struct {
	Notes []IncidentNote `json:"notes"`
}

// CreateIncidentNoteResponse represents the response from creating an incident note.
type CreateIncidentNoteResponse struct {
	Note IncidentNote `json:"note"`
}

// LogEntry represents a log entry for an incident.
type LogEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Summary   string    `json:"summary"`
	Self      string    `json:"self"`
	HTMLURL   string    `json:"html_url,omitempty"`
	CreatedAt string    `json:"created_at"`
	Agent     *Reference `json:"agent,omitempty"`
	Channel   *Channel  `json:"channel,omitempty"`
	Incident  *Reference `json:"incident,omitempty"`
	Service   *Reference `json:"service,omitempty"`
}

// Channel represents the channel through which an action was taken.
type Channel struct {
	Type string `json:"type"`
}

// ListLogEntriesResponse represents the response from listing log entries.
type ListLogEntriesResponse struct {
	LogEntries []LogEntry `json:"log_entries"`
	Limit      int        `json:"limit"`
	Offset     int        `json:"offset"`
	More       bool       `json:"more"`
}
