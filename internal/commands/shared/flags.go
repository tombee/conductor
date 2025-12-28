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

// Global flag values - set by root command
var (
	verboseFlag bool
	quietFlag   bool
	jsonFlag    bool
	configFlag  string

	// Build-time version information
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

// RegisterFlagPointers returns pointers to flag variables for binding.
// Called by root command to register flags.
func RegisterFlagPointers() (*bool, *bool, *bool, *string) {
	return &verboseFlag, &quietFlag, &jsonFlag, &configFlag
}

// SetVersion sets the version information (called from main)
func SetVersion(v, c, b string) {
	version = v
	commit = c
	buildDate = b
}

// GetVerbose returns the verbose flag value
func GetVerbose() bool {
	return verboseFlag
}

// GetQuiet returns the quiet flag value
func GetQuiet() bool {
	return quietFlag
}

// GetJSON returns the JSON output flag value
func GetJSON() bool {
	return jsonFlag
}

// GetConfigPath returns the config file path
func GetConfigPath() string {
	return configFlag
}

// GetVersion returns version information
func GetVersion() (string, string, string) {
	return version, commit, buildDate
}

// SetConfigPathForTest sets the config path for testing purposes
func SetConfigPathForTest(path string) {
	configFlag = path
}
