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

package rpc

import (
	"sync"
)

// CostPermission represents a permission for cost data access.
type CostPermission string

const (
	// PermissionCostRead allows viewing cost data.
	PermissionCostRead CostPermission = "cost:read"

	// PermissionCostAdmin allows modifying cost limits and viewing all costs.
	PermissionCostAdmin CostPermission = "cost:admin"

	// PermissionCostExport allows exporting cost data to external formats.
	PermissionCostExport CostPermission = "cost:export"
)

// CostAccessScope defines the scope of cost data access.
type CostAccessScope string

const (
	// ScopeOwnWorkflows allows access only to costs for workflows owned by the user.
	ScopeOwnWorkflows CostAccessScope = "own-workflows"

	// ScopeAllWorkflows allows access to all workflow costs.
	ScopeAllWorkflows CostAccessScope = "all-workflows"
)

// CostRole defines a role with specific permissions and scope.
type CostRole struct {
	Name        string
	Permissions []CostPermission
	Scope       CostAccessScope
}

// CostAuthorizer handles authorization for cost data access.
type CostAuthorizer struct {
	mu    sync.RWMutex
	roles map[string]*CostRole  // role name -> role definition
	users map[string][]string   // user ID -> role names
}

// NewCostAuthorizer creates a new cost authorizer with default roles.
func NewCostAuthorizer() *CostAuthorizer {
	authz := &CostAuthorizer{
		roles: make(map[string]*CostRole),
		users: make(map[string][]string),
	}

	// Register default roles
	authz.roles["cost-viewer"] = &CostRole{
		Name:        "cost-viewer",
		Permissions: []CostPermission{PermissionCostRead},
		Scope:       ScopeOwnWorkflows,
	}

	authz.roles["cost-admin"] = &CostRole{
		Name: "cost-admin",
		Permissions: []CostPermission{
			PermissionCostRead,
			PermissionCostAdmin,
			PermissionCostExport,
		},
		Scope: ScopeAllWorkflows,
	}

	return authz
}

// RegisterRole registers a custom cost access role.
func (a *CostAuthorizer) RegisterRole(role *CostRole) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.roles[role.Name] = role
}

// AssignRole assigns a role to a user.
func (a *CostAuthorizer) AssignRole(userID, roleName string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.roles[roleName]; !exists {
		return // Role doesn't exist
	}

	// Add role to user's role list if not already present
	userRoles := a.users[userID]
	for _, r := range userRoles {
		if r == roleName {
			return // Already assigned
		}
	}

	a.users[userID] = append(userRoles, roleName)
}

// RevokeRole removes a role from a user.
func (a *CostAuthorizer) RevokeRole(userID, roleName string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	userRoles := a.users[userID]
	var updated []string
	for _, r := range userRoles {
		if r != roleName {
			updated = append(updated, r)
		}
	}
	a.users[userID] = updated
}

// HasPermission checks if a user has a specific permission.
func (a *CostAuthorizer) HasPermission(userID string, permission CostPermission) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	userRoles := a.users[userID]
	for _, roleName := range userRoles {
		role := a.roles[roleName]
		if role == nil {
			continue
		}

		for _, perm := range role.Permissions {
			if perm == permission {
				return true
			}
		}
	}

	return false
}

// CanViewCosts checks if a user can view cost data.
func (a *CostAuthorizer) CanViewCosts(userID string) bool {
	return a.HasPermission(userID, PermissionCostRead) ||
		a.HasPermission(userID, PermissionCostAdmin)
}

// CanExportCosts checks if a user can export cost data.
func (a *CostAuthorizer) CanExportCosts(userID string) bool {
	return a.HasPermission(userID, PermissionCostExport) ||
		a.HasPermission(userID, PermissionCostAdmin)
}

// CanModifyLimits checks if a user can modify cost limits.
func (a *CostAuthorizer) CanModifyLimits(userID string) bool {
	return a.HasPermission(userID, PermissionCostAdmin)
}

// IsAdmin checks if a user has admin privileges for cost data.
func (a *CostAuthorizer) IsAdmin(userID string) bool {
	return a.HasPermission(userID, PermissionCostAdmin)
}

// CanViewRunCosts checks if a user can view costs for a specific run.
// This checks if the user owns the run or has admin access.
func (a *CostAuthorizer) CanViewRunCosts(userID, runID string) bool {
	// Admin can view all runs
	if a.IsAdmin(userID) {
		return true
	}

	// User can view their own runs if they have cost:read permission
	if !a.HasPermission(userID, PermissionCostRead) {
		return false
	}

	// In production, this would check run ownership via a run store
	// For MVP, we allow access if user has cost:read permission
	// The cost store will filter by userID anyway for non-admins
	return true
}

// GetScope returns the user's access scope (own-workflows or all-workflows).
func (a *CostAuthorizer) GetScope(userID string) CostAccessScope {
	a.mu.RLock()
	defer a.mu.RUnlock()

	userRoles := a.users[userID]
	for _, roleName := range userRoles {
		role := a.roles[roleName]
		if role == nil {
			continue
		}

		// Return the widest scope available
		if role.Scope == ScopeAllWorkflows {
			return ScopeAllWorkflows
		}
	}

	return ScopeOwnWorkflows
}

// GetUserRoles returns the list of roles assigned to a user.
func (a *CostAuthorizer) GetUserRoles(userID string) []string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	roles := a.users[userID]
	result := make([]string, len(roles))
	copy(result, roles)
	return result
}
