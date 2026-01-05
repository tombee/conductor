// Copyright 2025 Tom Barlow
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shared

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
)

// ProgressDisplay manages interactive workflow progress output.
// It provides animated spinners for running steps and formatted completion output.
// Falls back to static output when not running in a TTY or when disabled.
type ProgressDisplay struct {
	mu         sync.Mutex
	isTTY      bool
	noProgress bool
	verbose    bool

	workflowName string
	runID        string

	// Current step tracking
	currentStepID   string
	currentStepName string
	stepStartTime   time.Time
	stepIndex       int
	totalSteps      int

	// Log messages for current step (verbose mode)
	currentLogs []string

	// Completed steps
	completedSteps []CompletedStep

	// Animation state
	spinnerFrames []string
	frameIdx      int
	done          chan struct{}
	running       bool
}

// CompletedStep tracks information about a completed step.
type CompletedStep struct {
	Name          string
	Status        string // "success", "error", "skipped"
	Cost          float64
	Accuracy      string
	Duration      time.Duration
	TokensIn      int
	TokensOut     int
	CacheCreation int
	CacheRead     int
}

// NewProgressDisplay creates a new ProgressDisplay.
func NewProgressDisplay(noProgress, verbose bool) *ProgressDisplay {
	return &ProgressDisplay{
		isTTY:         term.IsTerminal(int(os.Stdout.Fd())),
		noProgress:    noProgress,
		verbose:       verbose,
		spinnerFrames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Start begins the progress display with workflow info.
func (p *ProgressDisplay) Start(workflowName, runID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.workflowName = workflowName
	p.runID = runID

	// Print header
	header := fmt.Sprintf("Running workflow: %s", workflowName)
	if runID != "" {
		header += fmt.Sprintf(" %s", Muted.Render("("+runID+")"))
	}
	fmt.Println(header)
	fmt.Println()
}

// StepStarted is called when a step begins execution.
func (p *ProgressDisplay) StepStarted(stepID, stepName string, index, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Store step info
	p.currentStepID = stepID
	p.currentStepName = stepName
	p.stepStartTime = time.Now()
	p.stepIndex = index
	p.totalSteps = total
	p.currentLogs = nil

	if p.isInteractive() {
		// Start spinner animation
		p.startSpinner()
	} else {
		// Static mode: print step started
		fmt.Printf("  %s %s...\n", Muted.Render(SymbolInfo), stepName)
	}
}

// StepCompleted is called when a step finishes execution.
func (p *ProgressDisplay) StepCompleted(stepID, stepName, status string, cost float64, accuracy string, durationMs int64, tokensIn, tokensOut, cacheCreation, cacheRead int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Calculate duration
	duration := time.Duration(durationMs) * time.Millisecond

	// Store completed step
	p.completedSteps = append(p.completedSteps, CompletedStep{
		Name:          stepName,
		Status:        status,
		Cost:          cost,
		Accuracy:      accuracy,
		Duration:      duration,
		TokensIn:      tokensIn,
		TokensOut:     tokensOut,
		CacheCreation: cacheCreation,
		CacheRead:     cacheRead,
	})

	if p.isInteractive() {
		// Stop spinner
		p.stopSpinner()

		// Clear the spinner line and any log lines
		p.clearCurrentLines()
	}

	// Print completed step line
	p.printCompletedStep(stepName, status, cost, accuracy, duration)

	// Reset current step
	p.currentStepID = ""
	p.currentStepName = ""
	p.currentLogs = nil
}

// LogMessage adds a log message (for verbose mode).
func (p *ProgressDisplay) LogMessage(message string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.verbose {
		return
	}

	if p.isInteractive() && p.currentStepName != "" {
		// Store log for redraw
		p.currentLogs = append(p.currentLogs, message)
		// Redraw with new log
		p.redrawSpinnerLine()
	} else {
		// Static mode: print log directly
		fmt.Printf("    %s %s\n", Muted.Render("│"), message)
	}
}

// Finish completes the progress display with final status.
func (p *ProgressDisplay) Finish(status string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop any running spinner
	p.stopSpinner()

	fmt.Println()

	// Print final status
	switch status {
	case "completed":
		fmt.Printf("%s Workflow completed\n", StatusOK.Render(SymbolOK))
	case "failed":
		fmt.Printf("%s Workflow failed\n", StatusError.Render(SymbolError))
	case "cancelled":
		fmt.Printf("%s Workflow cancelled\n", StatusWarn.Render(SymbolWarn))
	default:
		fmt.Printf("Workflow %s\n", status)
	}
}

// isInteractive returns true if we should use interactive mode.
func (p *ProgressDisplay) isInteractive() bool {
	return p.isTTY && !p.noProgress
}

// startSpinner begins the spinner animation goroutine.
func (p *ProgressDisplay) startSpinner() {
	if p.running {
		return
	}
	p.running = true
	p.done = make(chan struct{})
	p.frameIdx = 0

	// Print initial state
	p.renderSpinnerLine()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-p.done:
				return
			case <-ticker.C:
				p.mu.Lock()
				if p.running {
					p.frameIdx = (p.frameIdx + 1) % len(p.spinnerFrames)
					p.redrawSpinnerLine()
				}
				p.mu.Unlock()
			}
		}
	}()
}

// stopSpinner stops the spinner animation.
func (p *ProgressDisplay) stopSpinner() {
	if !p.running {
		return
	}
	p.running = false
	close(p.done)
}

// clearCurrentLines clears the spinner line and any log lines below it.
func (p *ProgressDisplay) clearCurrentLines() {
	if !p.isTTY {
		return
	}
	// Clear current line
	fmt.Print("\r\033[K")
	// Clear log lines (move up and clear for each log line)
	for i := 0; i < len(p.currentLogs); i++ {
		fmt.Print("\033[A\033[K") // move up and clear
	}
}

// renderSpinnerLine renders the current spinner state.
func (p *ProgressDisplay) renderSpinnerLine() {
	elapsed := time.Since(p.stepStartTime)
	elapsedStr := formatDuration(elapsed)

	frame := p.spinnerFrames[p.frameIdx]
	if !ColorEnabled() {
		frame = "..."
	}

	// Format: "  ⠋ Step Name...                          (3s)"
	stepDisplay := p.currentStepName + "..."
	line := fmt.Sprintf("  %s %s", StatusInfo.Render(frame), stepDisplay)

	// Right-align the elapsed time
	timeStr := Muted.Render("(" + elapsedStr + ")")
	padding := 60 - len(stepDisplay) - 4 // 4 = "  " + frame + " "
	if padding < 2 {
		padding = 2
	}
	line += strings.Repeat(" ", padding) + timeStr

	fmt.Print(line)
}

// redrawSpinnerLine redraws the spinner line (and logs in verbose mode).
func (p *ProgressDisplay) redrawSpinnerLine() {
	if !p.isTTY {
		return
	}

	// Move to start of line and clear everything below
	fmt.Print("\r\033[K")
	for i := 0; i < len(p.currentLogs); i++ {
		fmt.Print("\033[A\033[K")
	}

	// Render spinner line
	p.renderSpinnerLine()

	// Render log lines in verbose mode
	for _, log := range p.currentLogs {
		fmt.Printf("\n    %s %s", Muted.Render("│"), log)
	}
}

// printCompletedStep prints a completed step line.
func (p *ProgressDisplay) printCompletedStep(stepName, status string, cost float64, accuracy string, duration time.Duration) {
	// Choose symbol based on status
	var symbol string
	switch status {
	case "success":
		symbol = StatusOK.Render(SymbolOK)
	case "error", "failed":
		symbol = StatusError.Render(SymbolError)
	case "skipped":
		symbol = Muted.Render("-")
	default:
		symbol = StatusOK.Render(SymbolOK)
	}

	// Format cost
	costStr := formatCostValue(cost, accuracy)

	// Format duration
	durationStr := formatDuration(duration)

	// Right-aligned format: "  ✓ Step Name               $0.03  (12.4s)"
	// Calculate padding for alignment
	maxNameLen := 35
	nameLen := len(stepName)
	if nameLen > maxNameLen {
		stepName = stepName[:maxNameLen-3] + "..."
		nameLen = maxNameLen
	}
	padding := maxNameLen - nameLen
	if padding < 1 {
		padding = 1
	}

	fmt.Printf("  %s %s%s%s  %s\n",
		symbol,
		stepName,
		strings.Repeat(" ", padding),
		costStr,
		Muted.Render("("+durationStr+")"),
	)
}

// formatCostValue formats a cost value with accuracy indicator.
func formatCostValue(cost float64, accuracy string) string {
	if accuracy == "unavailable" || cost == 0 {
		return Muted.Render("--")
	}

	prefix := ""
	if accuracy == "estimated" {
		prefix = "~"
	}

	return fmt.Sprintf("%s$%.2f", prefix, cost)
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	d = d.Round(100 * time.Millisecond)
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := d.Seconds() - float64(minutes*60)
	return fmt.Sprintf("%dm %.0fs", minutes, seconds)
}
