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

package claudecode

import (
	"context"

	"github.com/tombee/conductor/internal/operation"
)

// OperationRegistry defines the interface for executing operations.
// This is currently unused but kept for future MCP server integration.
type OperationRegistry interface {
	Execute(ctx context.Context, reference string, inputs map[string]interface{}) (*operation.Result, error)
	List() []string
}
