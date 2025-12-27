package jira

import "time"

// Issue represents a Jira issue.
type Issue struct {
	ID     string     `json:"id"`
	Key    string     `json:"key"`
	Self   string     `json:"self"`
	Fields IssueFields `json:"fields"`
}

// IssueFields represents the fields of a Jira issue.
type IssueFields struct {
	Summary     string       `json:"summary"`
	Description interface{}  `json:"description"` // Can be string or ADF object
	IssueType   *IssueType   `json:"issuetype,omitempty"`
	Project     *Project     `json:"project,omitempty"`
	Status      *Status      `json:"status,omitempty"`
	Priority    *Priority    `json:"priority,omitempty"`
	Assignee    *User        `json:"assignee,omitempty"`
	Reporter    *User        `json:"reporter,omitempty"`
	Created     time.Time    `json:"created,omitempty"`
	Updated     time.Time    `json:"updated,omitempty"`
	Resolved    *time.Time   `json:"resolutiondate,omitempty"`
	Labels      []string     `json:"labels,omitempty"`
}

// IssueType represents a Jira issue type.
type IssueType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Subtask     bool   `json:"subtask"`
}

// Project represents a Jira project.
type Project struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Self string `json:"self"`
}

// Status represents a Jira issue status.
type Status struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	StatusCategory *StatusCategory `json:"statusCategory,omitempty"`
}

// StatusCategory represents a Jira status category.
type StatusCategory struct {
	ID    int    `json:"id"`
	Key   string `json:"key"`
	Name  string `json:"name"`
}

// Priority represents a Jira priority.
type Priority struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// User represents a Jira user.
type User struct {
	AccountID    string `json:"accountId"`
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress,omitempty"`
	Active       bool   `json:"active"`
	Self         string `json:"self"`
}

// Comment represents a Jira comment.
type Comment struct {
	ID      string      `json:"id"`
	Self    string      `json:"self"`
	Body    interface{} `json:"body"` // Can be string or ADF object
	Author  User        `json:"author"`
	Created time.Time   `json:"created"`
	Updated time.Time   `json:"updated"`
}

// Transition represents a Jira issue transition.
type Transition struct {
	ID   string              `json:"id"`
	Name string              `json:"name"`
	To   Status              `json:"to"`
	Fields map[string]TransitionField `json:"fields,omitempty"`
}

// TransitionField represents a field in a transition.
type TransitionField struct {
	Required bool   `json:"required"`
	Name     string `json:"name"`
	Type     string `json:"schema.type,omitempty"`
}

// TransitionsResponse represents the response from getting available transitions.
type TransitionsResponse struct {
	Expand      string       `json:"expand"`
	Transitions []Transition `json:"transitions"`
}

// SearchResults represents the response from a JQL search.
type SearchResults struct {
	Expand     string  `json:"expand"`
	StartAt    int     `json:"startAt"`
	MaxResults int     `json:"maxResults"`
	Total      int     `json:"total"`
	Issues     []Issue `json:"issues"`
}

// Attachment represents a Jira attachment.
type Attachment struct {
	ID       string    `json:"id"`
	Self     string    `json:"self"`
	Filename string    `json:"filename"`
	Author   User      `json:"author"`
	Created  time.Time `json:"created"`
	Size     int64     `json:"size"`
	MimeType string    `json:"mimeType"`
	Content  string    `json:"content"` // URL to download
}

// IssueLink represents a link between two Jira issues.
type IssueLink struct {
	ID           string       `json:"id"`
	Self         string       `json:"self"`
	Type         IssueLinkType `json:"type"`
	InwardIssue  *Issue       `json:"inwardIssue,omitempty"`
	OutwardIssue *Issue       `json:"outwardIssue,omitempty"`
}

// IssueLinkType represents the type of link between issues.
type IssueLinkType struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
	Self    string `json:"self"`
}

// CreateIssueRequest represents a request to create an issue.
type CreateIssueRequest struct {
	Fields CreateIssueFields `json:"fields"`
}

// CreateIssueFields represents the fields for creating an issue.
type CreateIssueFields struct {
	Summary     string                 `json:"summary"`
	Description interface{}            `json:"description,omitempty"`
	Project     map[string]string      `json:"project"`
	IssueType   map[string]string      `json:"issuetype"`
	Assignee    map[string]string      `json:"assignee,omitempty"`
	Priority    map[string]string      `json:"priority,omitempty"`
	Labels      []string               `json:"labels,omitempty"`
	// Allow arbitrary additional fields
	Additional  map[string]interface{} `json:"-"`
}

// TransitionRequest represents a request to transition an issue.
type TransitionRequest struct {
	Transition TransitionID           `json:"transition"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
}

// TransitionID represents a transition identifier.
type TransitionID struct {
	ID string `json:"id"`
}

// LinkIssuesRequest represents a request to link two issues.
type LinkIssuesRequest struct {
	Type         IssueLinkTypeRef `json:"type"`
	InwardIssue  IssueRef         `json:"inwardIssue"`
	OutwardIssue IssueRef         `json:"outwardIssue"`
	Comment      *CommentBody     `json:"comment,omitempty"`
}

// IssueLinkTypeRef represents a reference to an issue link type.
type IssueLinkTypeRef struct {
	Name string `json:"name"`
}

// IssueRef represents a reference to an issue.
type IssueRef struct {
	Key string `json:"key"`
}

// CommentBody represents a comment body.
type CommentBody struct {
	Body interface{} `json:"body"`
}
