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

// Package testing provides a comprehensive testing framework for Conductor workflows.
// It enables developers to test workflows using mocks, fixtures, and assertions without
// incurring API costs or requiring live external services.
//
// The testing framework consists of four main components:
//
//   - mock: Mock implementations for LLM providers and operations
//   - fixture: Fixture loading, pattern matching, and template expansion
//   - record: Recording real API responses as fixtures with credential redaction
//   - assert: Assertion evaluation and result collection
//
// For usage examples, see the conductor test command documentation.
package testing
