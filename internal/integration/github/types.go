package github

import "time"

// Issue represents a GitHub issue.
type Issue struct {
	ID        int64      `json:"id"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	User      User       `json:"user"`
	Labels    []Label    `json:"labels"`
	Assignees []User     `json:"assignees"`
	Milestone *Milestone `json:"milestone,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	ID        int64      `json:"id"`
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	User      User       `json:"user"`
	Labels    []Label    `json:"labels"`
	Head      Branch     `json:"head"`
	Base      Branch     `json:"base"`
	Merged    bool       `json:"merged"`
	Mergeable *bool      `json:"mergeable,omitempty"`
	MergedAt  *time.Time `json:"merged_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
}

// Repository represents a GitHub repository.
type Repository struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	FullName      string     `json:"full_name"`
	Description   string     `json:"description"`
	Private       bool       `json:"private"`
	HTMLURL       string     `json:"html_url"`
	CloneURL      string     `json:"clone_url"`
	DefaultBranch string     `json:"default_branch"`
	Owner         User       `json:"owner"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	PushedAt      *time.Time `json:"pushed_at,omitempty"`
}

// Release represents a GitHub release.
type Release struct {
	ID          int64      `json:"id"`
	TagName     string     `json:"tag_name"`
	Name        string     `json:"name"`
	Body        string     `json:"body"`
	Draft       bool       `json:"draft"`
	Prerelease  bool       `json:"prerelease"`
	HTMLURL     string     `json:"html_url"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// WorkflowRun represents a GitHub Actions workflow run.
type WorkflowRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion *string   `json:"conclusion,omitempty"`
	HTMLURL    string    `json:"html_url"`
	WorkflowID int64     `json:"workflow_id"`
	RunNumber  int       `json:"run_number"`
	Event      string    `json:"event"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Comment represents a GitHub comment.
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	User      User      `json:"user"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// User represents a GitHub user.
type User struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	HTMLURL   string `json:"html_url"`
	AvatarURL string `json:"avatar_url"`
	Type      string `json:"type"`
}

// Label represents a GitHub label.
type Label struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// Milestone represents a GitHub milestone.
type Milestone struct {
	ID          int64      `json:"id"`
	Number      int        `json:"number"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	State       string     `json:"state"`
	HTMLURL     string     `json:"html_url"`
	DueOn       *time.Time `json:"due_on,omitempty"`
}

// Branch represents a Git branch.
type Branch struct {
	Ref  string     `json:"ref"`
	SHA  string     `json:"sha"`
	Repo Repository `json:"repo"`
}

// FileContent represents file contents from a repository.
type FileContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	SHA         string `json:"sha"`
	Size        int    `json:"size"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
	DownloadURL string `json:"download_url"`
}

// WorkflowRunsResponse represents the response from listing workflow runs.
type WorkflowRunsResponse struct {
	TotalCount   int           `json:"total_count"`
	WorkflowRuns []WorkflowRun `json:"workflow_runs"`
}
