package enterprise

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Permission represents a specific permission
type Permission string

const (
	// S3 Permissions
	PermissionS3Read   Permission = "s3:read"
	PermissionS3Write  Permission = "s3:write"
	PermissionS3Delete Permission = "s3:delete"
	PermissionS3List   Permission = "s3:list"
	PermissionS3Admin  Permission = "s3:admin"

	// System Permissions
	PermissionSystemAdmin  Permission = "system:admin"
	PermissionSystemConfig Permission = "system:config"
	PermissionSystemUser   Permission = "system:user"

	// Audit Permissions
	PermissionAuditRead  Permission = "audit:read"
	PermissionAuditWrite Permission = "audit:write"

	// Security Permissions
	PermissionSecurityRead  Permission = "security:read"
	PermissionSecurityAdmin Permission = "security:admin"

	// MFA Permissions
	PermissionMFASetup    Permission = "mfa:setup"
	PermissionMFAValidate Permission = "mfa:validate"
)

// Role represents a role with a set of permissions
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// User represents a user with roles
type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	LastLogin time.Time `json:"last_login"`
}

// RBACManager manages role-based access control
type RBACManager struct {
	roles map[string]*Role
	users map[string]*User
	mutex sync.RWMutex
}

// NewRBACManager creates a new RBAC manager
func NewRBACManager() *RBACManager {
	rbac := &RBACManager{
		roles: make(map[string]*Role),
		users: make(map[string]*User),
	}

	// Initialize default roles
	rbac.initializeDefaultRoles()

	return rbac
}

// initializeDefaultRoles creates default system roles
func (r *RBACManager) initializeDefaultRoles() {
	defaultRoles := []*Role{
		{
			ID:          "admin",
			Name:        "Administrator",
			Description: "Full system administrator with all permissions",
			Permissions: []Permission{
				PermissionS3Read, PermissionS3Write, PermissionS3Delete,
				PermissionS3List, PermissionS3Admin,
				PermissionSystemAdmin, PermissionSystemConfig, PermissionSystemUser,
				PermissionAuditRead, PermissionAuditWrite,
				PermissionSecurityRead, PermissionSecurityAdmin,
				PermissionMFASetup, PermissionMFAValidate,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "s3-user",
			Name:        "S3 User",
			Description: "Standard S3 operations user",
			Permissions: []Permission{
				PermissionS3Read, PermissionS3Write, PermissionS3List,
				PermissionSystemUser,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "s3-readonly",
			Name:        "S3 Read-Only",
			Description: "Read-only access to S3 operations",
			Permissions: []Permission{
				PermissionS3Read, PermissionS3List,
				PermissionSystemUser,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "auditor",
			Name:        "Auditor",
			Description: "Audit log access and system monitoring",
			Permissions: []Permission{
				PermissionAuditRead,
				PermissionSystemUser,
				PermissionSecurityRead,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "security-admin",
			Name:        "Security Administrator",
			Description: "Security administration and MFA management",
			Permissions: []Permission{
				PermissionSecurityRead, PermissionSecurityAdmin,
				PermissionMFASetup, PermissionMFAValidate,
				PermissionAuditRead,
				PermissionSystemUser,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, role := range defaultRoles {
		r.roles[role.ID] = role
	}
}

// CreateRole creates a new role
func (r *RBACManager) CreateRole(role *Role) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if role.ID == "" {
		return fmt.Errorf("role ID cannot be empty")
	}

	if _, exists := r.roles[role.ID]; exists {
		return fmt.Errorf("role with ID %s already exists", role.ID)
	}

	role.CreatedAt = time.Now()
	role.UpdatedAt = time.Now()
	r.roles[role.ID] = role

	return nil
}

// UpdateRole updates an existing role
func (r *RBACManager) UpdateRole(roleID string, updatedRole *Role) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	role, exists := r.roles[roleID]
	if !exists {
		return fmt.Errorf("role with ID %s not found", roleID)
	}

	// Preserve creation time
	updatedRole.CreatedAt = role.CreatedAt
	updatedRole.UpdatedAt = time.Now()
	updatedRole.ID = roleID
	r.roles[roleID] = updatedRole

	return nil
}

// DeleteRole deletes a role
func (r *RBACManager) DeleteRole(roleID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.roles[roleID]; !exists {
		return fmt.Errorf("role with ID %s not found", roleID)
	}

	// Check if any users have this role
	for _, user := range r.users {
		for _, userRole := range user.Roles {
			if userRole == roleID {
				return fmt.Errorf("cannot delete role %s: still assigned to user %s", roleID, user.Username)
			}
		}
	}

	delete(r.roles, roleID)
	return nil
}

// GetRole retrieves a role by ID
func (r *RBACManager) GetRole(roleID string) (*Role, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	role, exists := r.roles[roleID]
	if !exists {
		return nil, fmt.Errorf("role with ID %s not found", roleID)
	}

	return role, nil
}

// ListRoles returns all roles
func (r *RBACManager) ListRoles() []*Role {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	roles := make([]*Role, 0, len(r.roles))
	for _, role := range r.roles {
		roles = append(roles, role)
	}

	return roles
}

// CreateUser creates a new user
func (r *RBACManager) CreateUser(user *User) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if user.ID == "" {
		return fmt.Errorf("user ID cannot be empty")
	}

	if _, exists := r.users[user.ID]; exists {
		return fmt.Errorf("user with ID %s already exists", user.ID)
	}

	// Validate roles exist
	for _, roleID := range user.Roles {
		if _, exists := r.roles[roleID]; !exists {
			return fmt.Errorf("role %s does not exist", roleID)
		}
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	user.Active = true
	r.users[user.ID] = user

	return nil
}

// UpdateUser updates an existing user
func (r *RBACManager) UpdateUser(userID string, updatedUser *User) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	// Validate roles exist
	for _, roleID := range updatedUser.Roles {
		if _, exists := r.roles[roleID]; !exists {
			return fmt.Errorf("role %s does not exist", roleID)
		}
	}

	// Preserve creation time and ID
	updatedUser.CreatedAt = user.CreatedAt
	updatedUser.UpdatedAt = time.Now()
	updatedUser.ID = userID
	r.users[userID] = updatedUser

	return nil
}

// DeleteUser deletes a user
func (r *RBACManager) DeleteUser(userID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.users[userID]; !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	delete(r.users, userID)
	return nil
}

// GetUser retrieves a user by ID
func (r *RBACManager) GetUser(userID string) (*User, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	user, exists := r.users[userID]
	if !exists {
		return nil, fmt.Errorf("user with ID %s not found", userID)
	}

	return user, nil
}

// ListUsers returns all users
func (r *RBACManager) ListUsers() []*User {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	users := make([]*User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}

	return users
}

// HasPermission checks if a user has a specific permission
func (r *RBACManager) HasPermission(userID string, permission Permission) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	user, exists := r.users[userID]
	if !exists || !user.Active {
		return false
	}

	// Check all user roles for the permission
	for _, roleID := range user.Roles {
		role, exists := r.roles[roleID]
		if !exists {
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

// GetUserPermissions returns all permissions for a user
func (r *RBACManager) GetUserPermissions(userID string) ([]Permission, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	user, exists := r.users[userID]
	if !exists {
		return nil, fmt.Errorf("user with ID %s not found", userID)
	}

	if !user.Active {
		return []Permission{}, nil
	}

	permissionSet := make(map[Permission]bool)

	// Collect permissions from all user roles
	for _, roleID := range user.Roles {
		role, exists := r.roles[roleID]
		if !exists {
			continue
		}

		for _, perm := range role.Permissions {
			permissionSet[perm] = true
		}
	}

	// Convert to slice
	permissions := make([]Permission, 0, len(permissionSet))
	for perm := range permissionSet {
		permissions = append(permissions, perm)
	}

	return permissions, nil
}

// AssignRole assigns a role to a user
func (r *RBACManager) AssignRole(userID, roleID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	if _, exists := r.roles[roleID]; !exists {
		return fmt.Errorf("role with ID %s not found", roleID)
	}

	// Check if user already has the role
	for _, existingRole := range user.Roles {
		if existingRole == roleID {
			return fmt.Errorf("user %s already has role %s", userID, roleID)
		}
	}

	user.Roles = append(user.Roles, roleID)
	user.UpdatedAt = time.Now()

	return nil
}

// RevokeRole revokes a role from a user
func (r *RBACManager) RevokeRole(userID, roleID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	// Find and remove the role
	for i, existingRole := range user.Roles {
		if existingRole == roleID {
			user.Roles = append(user.Roles[:i], user.Roles[i+1:]...)
			user.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("user %s does not have role %s", userID, roleID)
}

// CheckAccess checks if a user can perform an action on a resource
func (r *RBACManager) CheckAccess(userID string, action string, resource string) bool {
	// Convert action to permission
	var permission Permission

	// Parse action (e.g., "s3:read", "system:admin")
	parts := strings.Split(action, ":")
	if len(parts) != 2 {
		return false
	}

	permission = Permission(action)
	return r.HasPermission(userID, permission)
}

// UpdateLastLogin updates a user's last login time
func (r *RBACManager) UpdateLastLogin(userID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	user.LastLogin = time.Now()
	user.UpdatedAt = time.Now()

	return nil
}

// DeactivateUser deactivates a user
func (r *RBACManager) DeactivateUser(userID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	user.Active = false
	user.UpdatedAt = time.Now()

	return nil
}

// ActivateUser activates a user
func (r *RBACManager) ActivateUser(userID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	user, exists := r.users[userID]
	if !exists {
		return fmt.Errorf("user with ID %s not found", userID)
	}

	user.Active = true
	user.UpdatedAt = time.Now()

	return nil
}
