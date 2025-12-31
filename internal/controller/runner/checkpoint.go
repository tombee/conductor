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

// Checkpoint save and resume logic.
// Handles persisting workflow execution state to enable recovery from interruptions,
// and resuming interrupted runs from saved checkpoints.
package runner

import "context"

// ResumeInterrupted attempts to resume any interrupted runs from checkpoints.
// Delegates to LifecycleManager.
func (r *Runner) ResumeInterrupted(ctx context.Context) error {
	return r.lifecycle.ResumeInterrupted(ctx)
}
