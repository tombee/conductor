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

//go:build windows

package client

import (
	"os/exec"
)

// setSysProcAttrPlatform sets Windows-specific process attributes for daemon detachment.
func setSysProcAttrPlatform(cmd *exec.Cmd) {
	// On Windows, the process will be detached by default when started without a console
	// No special SysProcAttr needed
}
