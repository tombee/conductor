package jenkins

import "time"

// Build represents a Jenkins build.
type Build struct {
	ID              string    `json:"id"`
	Number          int       `json:"number"`
	URL             string    `json:"url"`
	Result          string    `json:"result"` // SUCCESS, FAILURE, UNSTABLE, ABORTED, null (in progress)
	Building        bool      `json:"building"`
	Duration        int64     `json:"duration"`  // milliseconds
	Timestamp       int64     `json:"timestamp"` // Unix timestamp in milliseconds
	DisplayName     string    `json:"displayName"`
	FullDisplayName string    `json:"fullDisplayName"`
	Description     string    `json:"description"`
	QueueID         int       `json:"queueId"`
	Actions         []Action  `json:"actions"`
	ChangeSet       ChangeSet `json:"changeSet"`
}

// Job represents a Jenkins job.
type Job struct {
	Name                string         `json:"name"`
	URL                 string         `json:"url"`
	Description         string         `json:"description"`
	Color               string         `json:"color"` // blue, red, yellow, etc.
	Buildable           bool           `json:"buildable"`
	InQueue             bool           `json:"inQueue"`
	LastBuild           *BuildRef      `json:"lastBuild"`
	LastSuccessfulBuild *BuildRef      `json:"lastSuccessfulBuild"`
	LastFailedBuild     *BuildRef      `json:"lastFailedBuild"`
	NextBuildNumber     int            `json:"nextBuildNumber"`
	Jobs                []Job          `json:"jobs"` // For folders
	Property            []Property     `json:"property"`
	HealthReport        []HealthReport `json:"healthReport"`
}

// BuildRef represents a reference to a build.
type BuildRef struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// QueueItem represents an item in the Jenkins build queue.
type QueueItem struct {
	ID           int         `json:"id"`
	Blocked      bool        `json:"blocked"`
	Buildable    bool        `json:"buildable"`
	InQueueSince int64       `json:"inQueueSince"` // Unix timestamp in milliseconds
	Params       string      `json:"params"`
	Stuck        bool        `json:"stuck"`
	Task         Task        `json:"task"`
	Why          string      `json:"why"`
	URL          string      `json:"url"`
	Cancelled    bool        `json:"cancelled"`
	Executable   *Executable `json:"executable,omitempty"` // Present when build starts
}

// Task represents a Jenkins task (job) reference.
type Task struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`
}

// Executable represents the actual build that started from a queue item.
type Executable struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// TestReport represents test results for a build.
type TestReport struct {
	Duration  float64     `json:"duration"`
	Empty     bool        `json:"empty"`
	FailCount int         `json:"failCount"`
	PassCount int         `json:"passCount"`
	SkipCount int         `json:"skipCount"`
	Suites    []TestSuite `json:"suites"`
}

// TestSuite represents a test suite within a test report.
type TestSuite struct {
	Name      string     `json:"name"`
	Duration  float64    `json:"duration"`
	Timestamp time.Time  `json:"timestamp"`
	Cases     []TestCase `json:"cases"`
}

// TestCase represents an individual test case.
type TestCase struct {
	ClassName       string  `json:"className"`
	Name            string  `json:"name"`
	Duration        float64 `json:"duration"`
	Status          string  `json:"status"` // PASSED, FAILED, SKIPPED
	ErrorDetails    string  `json:"errorDetails,omitempty"`
	ErrorStackTrace string  `json:"errorStackTrace,omitempty"`
}

// Node represents a Jenkins build agent/node.
type Node struct {
	DisplayName        string        `json:"displayName"`
	Description        string        `json:"description"`
	NumExecutors       int           `json:"numExecutors"`
	Mode               string        `json:"mode"` // NORMAL, EXCLUSIVE
	Offline            bool          `json:"offline"`
	OfflineCause       *OfflineCause `json:"offlineCause,omitempty"`
	TemporarilyOffline bool          `json:"temporarilyOffline"`
	MonitorData        MonitorData   `json:"monitorData"`
}

// OfflineCause represents why a node is offline.
type OfflineCause struct {
	Description string `json:"description"`
	Timestamp   int64  `json:"timestamp"`
}

// MonitorData contains node monitoring information.
type MonitorData struct {
	Architecture   *ArchitectureMonitor `json:"hudson.node_monitors.ArchitectureMonitor,omitempty"`
	Clock          *ClockMonitor        `json:"hudson.node_monitors.ClockMonitor,omitempty"`
	DiskSpace      *DiskSpaceMonitor    `json:"hudson.node_monitors.DiskSpaceMonitor,omitempty"`
	ResponseTime   *ResponseTimeMonitor `json:"hudson.node_monitors.ResponseTimeMonitor,omitempty"`
	SwapSpace      *SwapSpaceMonitor    `json:"hudson.node_monitors.SwapSpaceMonitor,omitempty"`
	TemporarySpace *DiskSpaceMonitor    `json:"hudson.node_monitors.TemporarySpaceMonitor,omitempty"`
}

// ArchitectureMonitor contains architecture information.
type ArchitectureMonitor struct {
	Value string `json:"value"`
}

// ClockMonitor contains clock difference information.
type ClockMonitor struct {
	Diff int64 `json:"diff"` // milliseconds
}

// DiskSpaceMonitor contains disk space information.
type DiskSpaceMonitor struct {
	Size      int64  `json:"size"` // bytes
	Path      string `json:"path"`
	Timestamp int64  `json:"timestamp"`
}

// ResponseTimeMonitor contains response time information.
type ResponseTimeMonitor struct {
	Average int64 `json:"average"` // milliseconds
}

// SwapSpaceMonitor contains swap space information.
type SwapSpaceMonitor struct {
	AvailablePhysicalMemory int64 `json:"availablePhysicalMemory"` // bytes
	AvailableSwapSpace      int64 `json:"availableSwapSpace"`      // bytes
	TotalPhysicalMemory     int64 `json:"totalPhysicalMemory"`     // bytes
	TotalSwapSpace          int64 `json:"totalSwapSpace"`          // bytes
}

// Action represents a build action (parameters, causes, etc.).
type Action struct {
	Class      string      `json:"_class"`
	Parameters []Parameter `json:"parameters,omitempty"`
	Causes     []Cause     `json:"causes,omitempty"`
}

// Parameter represents a build parameter.
type Parameter struct {
	Class string      `json:"_class"`
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// Cause represents why a build was triggered.
type Cause struct {
	Class            string `json:"_class"`
	ShortDescription string `json:"shortDescription"`
	UserID           string `json:"userId,omitempty"`
	UserName         string `json:"userName,omitempty"`
}

// ChangeSet represents changes in a build.
type ChangeSet struct {
	Kind  string   `json:"kind"`
	Items []Change `json:"items"`
}

// Change represents a single source code change.
type Change struct {
	CommitID      string   `json:"commitId"`
	Message       string   `json:"msg"`
	Author        Author   `json:"author"`
	Timestamp     int64    `json:"timestamp"`
	AffectedPaths []string `json:"affectedPaths"`
}

// Author represents a commit author.
type Author struct {
	FullName    string `json:"fullName"`
	AbsoluteURL string `json:"absoluteUrl"`
}

// Property represents a job property.
type Property struct {
	Class string `json:"_class"`
}

// HealthReport represents job health status.
type HealthReport struct {
	Description   string `json:"description"`
	IconClassName string `json:"iconClassName"`
	IconURL       string `json:"iconUrl"`
	Score         int    `json:"score"` // 0-100
}

// Crumb represents a CSRF protection token.
type Crumb struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

// JobListResponse represents the response from listing jobs.
type JobListResponse struct {
	Jobs []Job `json:"jobs"`
}

// NodeListResponse represents the response from listing nodes.
type NodeListResponse struct {
	Computer []Node `json:"computer"`
}
