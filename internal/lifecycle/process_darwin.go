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

//go:build darwin

package lifecycle

import (
	"fmt"
	"os/exec"
	"strings"
)

// isConductorProcess checks if the process is a conductor controller using ps command.
func isConductorProcess(pid int) bool {
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "command=")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	command := strings.TrimSpace(string(output))

	// Check if command contains "conductor"
	return strings.Contains(command, "conductor")
}

// getProcessCommand returns the command line of the process using ps.
func getProcessCommand(pid int) (string, error) {
	cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid), "-o", "command=")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ps command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
