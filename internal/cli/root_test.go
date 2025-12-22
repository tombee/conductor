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

package cli

import (
	"testing"
)

func TestNewRootCommand(t *testing.T) {
	cmd := NewRootCommand()

	if cmd.Use != "conductor" {
		t.Errorf("expected use 'conductor', got %q", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected short description to be set")
	}

	if cmd.Long == "" {
		t.Error("expected long description to be set")
	}
}

func TestGlobalFlags(t *testing.T) {
	cmd := NewRootCommand()

	// Check that flags are registered
	if cmd.PersistentFlags().Lookup("verbose") == nil {
		t.Error("verbose flag not registered")
	}

	if cmd.PersistentFlags().Lookup("quiet") == nil {
		t.Error("quiet flag not registered")
	}

	if cmd.PersistentFlags().Lookup("json") == nil {
		t.Error("json flag not registered")
	}

	if cmd.PersistentFlags().Lookup("config") == nil {
		t.Error("config flag not registered")
	}
}

func TestSetVersion(t *testing.T) {
	// Test setting version
	SetVersion("1.2.3", "abc123", "2025-12-22")

	v, c, b := GetVersion()
	if v != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %q", v)
	}
	if c != "abc123" {
		t.Errorf("expected commit 'abc123', got %q", c)
	}
	if b != "2025-12-22" {
		t.Errorf("expected build date '2025-12-22', got %q", b)
	}
}
