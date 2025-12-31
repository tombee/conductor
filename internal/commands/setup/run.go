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

package setup

import "context"

// WizardRunner is the interface that runs the setup wizard.
// This allows us to avoid import cycles between setup and forms packages.
type WizardRunner interface {
	Run(ctx context.Context, state *SetupState, accessibleMode bool) error
}

// defaultWizardRunner will be set by the forms package during init
var defaultWizardRunner WizardRunner

// SetWizardRunner sets the wizard runner implementation.
// This is called by the forms package during init.
func SetWizardRunner(runner WizardRunner) {
	defaultWizardRunner = runner
}

// RunWizard runs the setup wizard using the configured runner.
func RunWizard(ctx context.Context, state *SetupState, accessibleMode bool) error {
	if defaultWizardRunner == nil {
		// Fallback if runner not set (shouldn't happen in normal use)
		return nil
	}
	return defaultWizardRunner.Run(ctx, state, accessibleMode)
}
