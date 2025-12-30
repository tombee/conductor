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
	"strings"
	"testing"
)

func TestValidateSecurityMode(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty mode is valid",
			mode:    "",
			wantErr: false,
		},
		{
			name:    "unrestricted profile",
			mode:    "unrestricted",
			wantErr: false,
		},
		{
			name:    "standard profile",
			mode:    "standard",
			wantErr: false,
		},
		{
			name:    "invalid profile",
			mode:    "invalid-profile",
			wantErr: true,
			errMsg:  "invalid security profile 'invalid-profile', valid: unrestricted, standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecurityMode(tt.mode)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSecurityMode() expected error but got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSecurityMode() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateSecurityMode() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidateHosts(t *testing.T) {
	tests := []struct {
		name    string
		hosts   []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty list",
			hosts:   []string{},
			wantErr: false,
		},
		{
			name:    "valid hostname",
			hosts:   []string{"api.example.com"},
			wantErr: false,
		},
		{
			name:    "valid IP address",
			hosts:   []string{"192.168.1.1"},
			wantErr: false,
		},
		{
			name:    "hostname with port",
			hosts:   []string{"api.example.com:443"},
			wantErr: false,
		},
		{
			name:    "wildcard subdomain",
			hosts:   []string{"*.example.com"},
			wantErr: false,
		},
		{
			name:    "multiple valid hosts",
			hosts:   []string{"api.example.com", "*.cdn.example.com", "192.168.1.1"},
			wantErr: false,
		},
		{
			name:    "empty host",
			hosts:   []string{""},
			wantErr: true,
			errMsg:  "host cannot be empty",
		},
		{
			name:    "host with protocol",
			hosts:   []string{"https://api.example.com"},
			wantErr: true,
			errMsg:  "should not include protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHosts(tt.hosts)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateHosts() expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateHosts() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateHosts() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestValidatePaths(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty list",
			paths:   []string{},
			wantErr: false,
		},
		{
			name:    "absolute path",
			paths:   []string{"/tmp/output"},
			wantErr: false,
		},
		{
			name:    "relative path",
			paths:   []string{"./output"},
			wantErr: false,
		},
		{
			name:    "current directory",
			paths:   []string{"."},
			wantErr: false,
		},
		{
			name:    "home directory shorthand",
			paths:   []string{"~/documents"},
			wantErr: false,
		},
		{
			name:    "multiple valid paths",
			paths:   []string{"/tmp", "./output", "~/data"},
			wantErr: false,
		},
		{
			name:    "empty path",
			paths:   []string{""},
			wantErr: true,
			errMsg:  "path cannot be empty",
		},
		{
			name:    "path with null byte",
			paths:   []string{"/tmp/test\x00file"},
			wantErr: true,
			errMsg:  "contains null byte",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePaths(tt.paths)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidatePaths() expected error but got nil")
				} else if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidatePaths() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidatePaths() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestBuildSecurityProfile(t *testing.T) {
	tests := []struct {
		name    string
		opts    SecurityOptions
		wantNil bool
		wantErr bool
		check   func(t *testing.T, opts SecurityOptions)
	}{
		{
			name: "empty mode returns nil",
			opts: SecurityOptions{
				Mode: "",
			},
			wantNil: true,
			wantErr: false,
		},
		{
			name: "standard profile",
			opts: SecurityOptions{
				Mode: "standard",
			},
			wantNil: false,
			wantErr: false,
			check: func(t *testing.T, opts SecurityOptions) {
				profile, _ := buildSecurityProfile(opts)
				if profile.Name != "standard" {
					t.Errorf("expected profile name 'standard', got '%s'", profile.Name)
				}
			},
		},
		{
			name: "profile with additional hosts",
			opts: SecurityOptions{
				Mode:       "standard",
				AllowHosts: []string{"api.test.com"},
			},
			wantNil: false,
			wantErr: false,
			check: func(t *testing.T, opts SecurityOptions) {
				profile, _ := buildSecurityProfile(opts)
				found := false
				for _, host := range profile.Network.Allow {
					if host == "api.test.com" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected 'api.test.com' in allowed hosts, got %v", profile.Network.Allow)
				}
			},
		},
		{
			name: "profile with additional paths",
			opts: SecurityOptions{
				Mode:       "standard",
				AllowPaths: []string{"/tmp/custom"},
			},
			wantNil: false,
			wantErr: false,
			check: func(t *testing.T, opts SecurityOptions) {
				profile, _ := buildSecurityProfile(opts)
				found := false
				for _, path := range profile.Filesystem.Read {
					if path == "/tmp/custom" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected '/tmp/custom' in allowed read paths, got %v", profile.Filesystem.Read)
				}
			},
		},
		{
			name: "invalid profile name",
			opts: SecurityOptions{
				Mode: "invalid",
			},
			wantNil: false,
			wantErr: true,
		},
		{
			name: "invalid host",
			opts: SecurityOptions{
				Mode:       "standard",
				AllowHosts: []string{"https://invalid"},
			},
			wantNil: false,
			wantErr: true,
		},
		{
			name: "invalid path",
			opts: SecurityOptions{
				Mode:       "standard",
				AllowPaths: []string{""},
			},
			wantNil: false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile, err := buildSecurityProfile(tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Errorf("buildSecurityProfile() expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("buildSecurityProfile() unexpected error = %v", err)
				return
			}

			if tt.wantNil {
				if profile != nil {
					t.Errorf("buildSecurityProfile() expected nil profile, got %v", profile)
				}
			} else {
				if profile == nil {
					t.Errorf("buildSecurityProfile() expected profile, got nil")
				}
			}

			if tt.check != nil && profile != nil {
				tt.check(t, tt.opts)
			}
		})
	}
}
