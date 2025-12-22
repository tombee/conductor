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

// Package version provides semver parsing and version constraint matching.
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// semverRegex matches semantic versions.
var semverRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:-([a-zA-Z0-9.-]+))?(?:\+([a-zA-Z0-9.-]+))?$`)

// Parse parses a version string into a Version.
func Parse(s string) (*Version, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty version string")
	}

	matches := semverRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid version format: %s", s)
	}

	v := &Version{}

	// Major is required
	major, _ := strconv.Atoi(matches[1])
	v.Major = major

	// Minor is optional
	if matches[2] != "" {
		minor, _ := strconv.Atoi(matches[2])
		v.Minor = minor
	}

	// Patch is optional
	if matches[3] != "" {
		patch, _ := strconv.Atoi(matches[3])
		v.Patch = patch
	}

	v.Prerelease = matches[4]
	v.Build = matches[5]

	return v, nil
}

// String returns the string representation of the version.
func (v *Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Prerelease versions have lower precedence than release versions
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		if v.Prerelease < other.Prerelease {
			return -1
		}
		return 1
	}

	return 0
}

// Constraint represents a version constraint.
type Constraint struct {
	// Operator is the comparison operator (=, >=, >, <, <=, ^, ~)
	Operator string
	// Version is the version to compare against
	Version *Version
	// Raw is the original constraint string
	Raw string
}

// constraintRegex matches version constraints.
var constraintRegex = regexp.MustCompile(`^([=<>^~]?=?)\s*v?(.+)$`)

// ParseConstraint parses a version constraint string.
// Supported formats:
//   - "1.2.3" or "=1.2.3" - exact match
//   - "^1.2.3" - compatible with (same major version)
//   - "~1.2.3" - approximately (same major.minor)
//   - ">=1.2.3" - greater than or equal
//   - ">1.2.3" - greater than
//   - "<=1.2.3" - less than or equal
//   - "<1.2.3" - less than
func ParseConstraint(s string) (*Constraint, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty constraint string")
	}

	// Handle "latest" as any version
	if strings.ToLower(s) == "latest" {
		return &Constraint{
			Operator: ">=",
			Version:  &Version{Major: 0, Minor: 0, Patch: 0},
			Raw:      s,
		}, nil
	}

	matches := constraintRegex.FindStringSubmatch(s)
	if matches == nil {
		return nil, fmt.Errorf("invalid constraint format: %s", s)
	}

	operator := matches[1]
	if operator == "" {
		operator = "="
	}

	version, err := Parse(matches[2])
	if err != nil {
		return nil, fmt.Errorf("invalid version in constraint: %w", err)
	}

	return &Constraint{
		Operator: operator,
		Version:  version,
		Raw:      s,
	}, nil
}

// Match checks if a version satisfies the constraint.
func (c *Constraint) Match(v *Version) bool {
	cmp := v.Compare(c.Version)

	switch c.Operator {
	case "=", "==":
		return cmp == 0
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case "^":
		// Compatible with: same major version
		if v.Major != c.Version.Major {
			return false
		}
		return cmp >= 0
	case "~":
		// Approximately: same major.minor
		if v.Major != c.Version.Major || v.Minor != c.Version.Minor {
			return false
		}
		return cmp >= 0
	default:
		return cmp == 0
	}
}

// String returns the string representation of the constraint.
func (c *Constraint) String() string {
	if c.Raw != "" {
		return c.Raw
	}
	return c.Operator + c.Version.String()
}

// Constraints represents multiple constraints that must all be satisfied.
type Constraints []*Constraint

// ParseConstraints parses a comma-separated list of constraints.
func ParseConstraints(s string) (Constraints, error) {
	parts := strings.Split(s, ",")
	constraints := make(Constraints, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		c, err := ParseConstraint(part)
		if err != nil {
			return nil, err
		}
		constraints = append(constraints, c)
	}

	if len(constraints) == 0 {
		return nil, fmt.Errorf("no valid constraints found")
	}

	return constraints, nil
}

// Match checks if a version satisfies all constraints.
func (cs Constraints) Match(v *Version) bool {
	for _, c := range cs {
		if !c.Match(v) {
			return false
		}
	}
	return true
}

// String returns the string representation of the constraints.
func (cs Constraints) String() string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.String()
	}
	return strings.Join(parts, ", ")
}
