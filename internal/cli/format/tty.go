package format

import (
	"os"

	"golang.org/x/term"
)

// IsTTY determines if output should use terminal formatting.
// Returns true if stdout is a TTY with color support.
// Returns false if stdout is piped, NO_COLOR is set, or TERM is "dumb" or empty.
func IsTTY() bool {
	// Check NO_COLOR environment variable
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	// Check TERM environment variable
	termEnv := os.Getenv("TERM")
	if termEnv == "dumb" || termEnv == "" {
		return false
	}

	// Check if stdout is a terminal
	return term.IsTerminal(int(os.Stdout.Fd()))
}
