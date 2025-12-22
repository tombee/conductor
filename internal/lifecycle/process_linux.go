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

//go:build linux

package lifecycle

import (
	"fmt"
	"os"
	"strings"
)

// isConductorProcess checks if the process is a conductor controller by reading /proc/[pid]/cmdline.
func isConductorProcess(pid int) bool {
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}

	// cmdline is null-separated, convert to space-separated
	cmd := string(cmdline)
	cmd = strings.ReplaceAll(cmd, "\x00", " ")
	cmd = strings.TrimSpace(cmd)

	// Check if command contains "conductor"
	// This catches both "conductor controller start" and the binary path
	return strings.Contains(cmd, "conductor")
}

// getProcessCommand returns the command line of the process.
func getProcessCommand(pid int) (string, error) {
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", fmt.Errorf("failed to read cmdline: %w", err)
	}

	// Convert null-separated to space-separated
	cmd := string(cmdline)
	cmd = strings.ReplaceAll(cmd, "\x00", " ")
	cmd = strings.TrimSpace(cmd)

	return cmd, nil
}
