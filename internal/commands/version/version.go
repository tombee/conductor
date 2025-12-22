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

package version

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tombee/conductor/internal/commands/shared"
)

// VersionInfo contains version metadata
type VersionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// NewVersionCommand creates the version command
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display version, commit hash, and build date for Conductor.`,
		RunE:  runVersion,
	}

	return cmd
}

func runVersion(cmd *cobra.Command, args []string) error {
	v, c, b := shared.GetVersion()

	info := VersionInfo{
		Version:   v,
		Commit:    c,
		BuildDate: b,
	}

	if shared.GetJSON() {
		data, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal version info: %w", err)
		}
		cmd.Println(string(data))
		return nil
	}

	cmd.Printf("conductor version %s\n", info.Version)
	cmd.Printf("  commit:     %s\n", info.Commit)
	cmd.Printf("  build date: %s\n", info.BuildDate)

	return nil
}
