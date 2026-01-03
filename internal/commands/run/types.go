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

package run

import (
	"github.com/tombee/conductor/pkg/workflow"
)

// RunStats contains execution statistics
type RunStats struct {
	CostUSD      float64
	TokensIn     int
	TokensOut    int
	DurationMs   int64
	StepCosts    map[string]StepCost
	RunningTotal float64
	Accuracy     string
}

// StepCost tracks cost for a single step
type StepCost struct {
	CostUSD   float64
	TokensIn  int
	TokensOut int
	Accuracy  string
}

// ExecutionPlan represents a resolved execution plan
type ExecutionPlan struct {
	Steps    []ResolvedStep
	Warnings []string
}

// ResolvedStep represents a workflow step with resolved provider
type ResolvedStep struct {
	ID            string
	Type          workflow.StepType
	ProviderName  string
	ProviderType  string
	ModelTier     string
	ResolvedModel string
}
