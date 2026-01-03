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
	"sync"
	"time"

	"golang.org/x/term"
)

// spinnerFrames defines the animation frames for the spinner
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner displays an animated spinner with elapsed time during long operations.
// It updates in-place using ANSI escape codes and respects TTY/color settings.
type Spinner struct {
	mu        sync.Mutex
	message   string
	startTime time.Time
	active    bool
	done      chan struct{}
	frameIdx  int
	isTTY     bool
}

// NewSpinner creates a new spinner instance.
func NewSpinner() *Spinner {
	return &Spinner{
		isTTY: term.IsTerminal(int(os.Stdout.Fd())),
	}
}

// Start begins the spinner animation with the given message.
// The message is displayed alongside the spinner and elapsed time.
// If not running in a TTY, it prints the message once without animation.
func (s *Spinner) Start(message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active {
		return
	}

	s.message = message
	s.startTime = time.Now()
	s.active = true
	s.done = make(chan struct{})
	s.frameIdx = 0

	if !s.isTTY {
		// Non-TTY: just print the message once
		fmt.Printf("%s\n", message)
		return
	}

	// Print initial state
	s.render()

	// Start animation goroutine
	go s.animate()
}

// Stop stops the spinner and clears the line.
// Returns the elapsed duration since Start was called.
func (s *Spinner) Stop() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.active {
		return 0
	}

	elapsed := time.Since(s.startTime)
	s.active = false
	close(s.done)

	if s.isTTY {
		// Clear the spinner line
		fmt.Print("\r\033[K")
	}

	return elapsed
}

// animate runs the spinner animation loop
func (s *Spinner) animate() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.mu.Lock()
			if s.active {
				s.frameIdx = (s.frameIdx + 1) % len(spinnerFrames)
				s.render()
			}
			s.mu.Unlock()
		}
	}
}

// render draws the current spinner state (must be called with mu held)
func (s *Spinner) render() {
	elapsed := time.Since(s.startTime)
	elapsedStr := formatElapsed(elapsed)

	frame := spinnerFrames[s.frameIdx]
	if !ColorEnabled() {
		frame = "..."
	}

	// Move to start of line and clear, then print
	fmt.Printf("\r\033[K%s %s %s",
		s.message,
		Muted.Render(frame),
		Muted.Render("("+elapsedStr+")"))
}

// formatElapsed formats a duration for display (e.g., "12s", "1m 23s")
func formatElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	if seconds == 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}
