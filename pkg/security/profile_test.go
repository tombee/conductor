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

package security

import (
	"testing"
	"time"
)

func TestLoadProfile(t *testing.T) {
	tests := []struct {
		name          string
		profileName   string
		customProfile map[string]*SecurityProfile
		wantError     bool
	}{
		{
			name:        "load unrestricted profile",
			profileName: ProfileUnrestricted,
			wantError:   false,
		},
		{
			name:        "load standard profile",
			profileName: ProfileStandard,
			wantError:   false,
		},
		{
			name:        "load non-existent profile",
			profileName: "non-existent",
			wantError:   true,
		},
		{
			name:        "load custom profile",
			profileName: "custom",
			customProfile: map[string]*SecurityProfile{
				"custom": {
					Name:      "custom",
					Isolation: IsolationNone,
					Limits: ResourceLimits{
						TimeoutPerTool: 30 * time.Second,
					},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := LoadProfile(tt.profileName, tt.customProfile)
			if (err != nil) != tt.wantError {
				t.Errorf("LoadProfile() error = %v, wantError %v", err, tt.wantError)
				return
			}
			if !tt.wantError && profile == nil {
				t.Error("LoadProfile() returned nil profile")
			}
			if !tt.wantError && profile.Name != tt.profileName {
				t.Errorf("LoadProfile() profile.Name = %v, want %v", profile.Name, tt.profileName)
			}
		})
	}
}

func TestValidateProfile(t *testing.T) {
	tests := []struct {
		name      string
		profile   *SecurityProfile
		wantError bool
	}{
		{
			name:      "nil profile",
			profile:   nil,
			wantError: true,
		},
		{
			name: "empty name",
			profile: &SecurityProfile{
				Name:      "",
				Isolation: IsolationNone,
				Limits: ResourceLimits{
					TimeoutPerTool: 30 * time.Second,
				},
			},
			wantError: true,
		},
		{
			name: "invalid isolation level",
			profile: &SecurityProfile{
				Name:      "test",
				Isolation: "invalid",
				Limits: ResourceLimits{
					TimeoutPerTool: 30 * time.Second,
				},
			},
			wantError: true,
		},
		{
			name: "zero timeout",
			profile: &SecurityProfile{
				Name:      "test",
				Isolation: IsolationNone,
				Limits: ResourceLimits{
					TimeoutPerTool: 0,
				},
			},
			wantError: true,
		},
		{
			name: "negative memory",
			profile: &SecurityProfile{
				Name:      "test",
				Isolation: IsolationNone,
				Limits: ResourceLimits{
					TimeoutPerTool: 30 * time.Second,
					MaxMemory:      -1,
				},
			},
			wantError: true,
		},
		{
			name: "valid profile",
			profile: &SecurityProfile{
				Name:      "test",
				Isolation: IsolationNone,
				Limits: ResourceLimits{
					TimeoutPerTool: 30 * time.Second,
					MaxMemory:      512 * 1024 * 1024,
					MaxProcesses:   10,
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProfile(tt.profile)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateProfile() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestGetBuiltinProfiles(t *testing.T) {
	profiles := GetBuiltinProfiles()
	expected := []string{
		ProfileUnrestricted,
		ProfileStandard,
	}

	if len(profiles) != len(expected) {
		t.Errorf("GetBuiltinProfiles() returned %d profiles, want %d", len(profiles), len(expected))
	}

	for _, name := range expected {
		found := false
		for _, p := range profiles {
			if p == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetBuiltinProfiles() missing profile: %s", name)
		}
	}
}
