package enterprise

import (
	"testing"
)

func TestNewRBACManager(t *testing.T) {
	rbac := NewRBACManager()
	if rbac == nil {
		t.Fatal("NewRBACManager returned nil")
	}

	// Check default roles are created
	defaultRoles := []string{"admin", "s3-user", "s3-readonly", "auditor"}
	for _, roleID := range defaultRoles {
		role, err := rbac.GetRole(roleID)
		if err != nil {
			t.Errorf("Default role %s not found: %v", roleID, err)
		}
		if role == nil {
			t.Errorf("Default role %s is nil", roleID)
		}
	}
}

func TestCreateRole(t *testing.T) {
	rbac := NewRBACManager()

	role := &Role{
		ID:          "test-role",
		Name:        "Test Role",
		Description: "A test role",
		Permissions: []Permission{PermissionS3Read, PermissionS3Write},
	}

	err := rbac.CreateRole(role)
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	// Verify role was created
	retrieved, err := rbac.GetRole("test-role")
	if err != nil {
		t.Fatalf("GetRole failed: %v", err)
	}

	if retrieved.ID != "test-role" {
		t.Errorf("Expected role ID 'test-role', got '%s'", retrieved.ID)
	}

	if retrieved.Name != "Test Role" {
		t.Errorf("Expected role name 'Test Role', got '%s'", retrieved.Name)
	}

	if len(retrieved.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(retrieved.Permissions))
	}
}

func TestCreateRoleDuplicate(t *testing.T) {
	rbac := NewRBACManager()

	role := &Role{
		ID:   "duplicate-role",
		Name: "Duplicate Role",
	}

	// Create role first time
	err := rbac.CreateRole(role)
	if err != nil {
		t.Fatalf("First CreateRole failed: %v", err)
	}

	// Try to create same role again
	err = rbac.CreateRole(role)
	if err == nil {
		t.Error("CreateRole should fail for duplicate role ID")
	}
}

func TestCreateRoleEmptyID(t *testing.T) {
	rbac := NewRBACManager()

	role := &Role{
		Name: "No ID Role",
	}

	err := rbac.CreateRole(role)
	if err == nil {
		t.Error("CreateRole should fail for empty role ID")
	}
}

func TestUpdateRole(t *testing.T) {
	rbac := NewRBACManager()

	// Create initial role
	role := &Role{
		ID:          "update-role",
		Name:        "Original Name",
		Permissions: []Permission{PermissionS3Read},
	}

	err := rbac.CreateRole(role)
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	// Update role
	updatedRole := &Role{
		Name:        "Updated Name",
		Permissions: []Permission{PermissionS3Read, PermissionS3Write},
	}

	err = rbac.UpdateRole("update-role", updatedRole)
	if err != nil {
		t.Fatalf("UpdateRole failed: %v", err)
	}

	// Verify update
	retrieved, err := rbac.GetRole("update-role")
	if err != nil {
		t.Fatalf("GetRole failed: %v", err)
	}

	if retrieved.Name != "Updated Name" {
		t.Errorf("Expected updated name 'Updated Name', got '%s'", retrieved.Name)
	}

	if len(retrieved.Permissions) != 2 {
		t.Errorf("Expected 2 permissions after update, got %d", len(retrieved.Permissions))
	}
}

func TestDeleteRole(t *testing.T) {
	rbac := NewRBACManager()

	// Create role
	role := &Role{
		ID:   "delete-role",
		Name: "To Be Deleted",
	}

	err := rbac.CreateRole(role)
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	// Delete role
	err = rbac.DeleteRole("delete-role")
	if err != nil {
		t.Fatalf("DeleteRole failed: %v", err)
	}

	// Verify role is deleted
	_, err = rbac.GetRole("delete-role")
	if err == nil {
		t.Error("GetRole should fail for deleted role")
	}
}

func TestDeleteRoleWithUsers(t *testing.T) {
	rbac := NewRBACManager()

	// Create role
	role := &Role{
		ID:   "assigned-role",
		Name: "Assigned Role",
	}

	err := rbac.CreateRole(role)
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	// Create user with role
	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Roles:    []string{"assigned-role"},
	}

	err = rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Try to delete role
	err = rbac.DeleteRole("assigned-role")
	if err == nil {
		t.Error("DeleteRole should fail when role is assigned to users")
	}
}

func TestCreateUser(t *testing.T) {
	rbac := NewRBACManager()

	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Email:    "test@example.com",
		Roles:    []string{"s3-user"},
	}

	err := rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Verify user was created
	retrieved, err := rbac.GetUser("test-user")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if retrieved.Username != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", retrieved.Username)
	}

	if !retrieved.Active {
		t.Error("New user should be active by default")
	}
}

func TestCreateUserInvalidRole(t *testing.T) {
	rbac := NewRBACManager()

	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Roles:    []string{"nonexistent-role"},
	}

	err := rbac.CreateUser(user)
	if err == nil {
		t.Error("CreateUser should fail with nonexistent role")
	}
}

func TestHasPermission(t *testing.T) {
	rbac := NewRBACManager()

	// Create user with s3-user role
	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Roles:    []string{"s3-user"},
	}

	err := rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Test permission that s3-user should have
	if !rbac.HasPermission("test-user", PermissionS3Read) {
		t.Error("s3-user should have S3 read permission")
	}

	// Test permission that s3-user should not have
	if rbac.HasPermission("test-user", PermissionS3Admin) {
		t.Error("s3-user should not have S3 admin permission")
	}

	// Test nonexistent user
	if rbac.HasPermission("nonexistent-user", PermissionS3Read) {
		t.Error("Nonexistent user should not have any permissions")
	}
}

func TestGetUserPermissions(t *testing.T) {
	rbac := NewRBACManager()

	// Create user with admin role
	user := &User{
		ID:       "admin-user",
		Username: "adminuser",
		Roles:    []string{"admin"},
	}

	err := rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	permissions, err := rbac.GetUserPermissions("admin-user")
	if err != nil {
		t.Fatalf("GetUserPermissions failed: %v", err)
	}

	// Admin should have many permissions
	if len(permissions) == 0 {
		t.Error("Admin user should have permissions")
	}

	// Check for specific admin permissions
	hasAdminPerm := false
	for _, perm := range permissions {
		if perm == PermissionSystemAdmin {
			hasAdminPerm = true
			break
		}
	}
	if !hasAdminPerm {
		t.Error("Admin user should have system admin permission")
	}
}

func TestAssignRole(t *testing.T) {
	rbac := NewRBACManager()

	// Create user
	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Roles:    []string{},
	}

	err := rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Assign role
	err = rbac.AssignRole("test-user", "s3-user")
	if err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	// Verify role was assigned
	retrieved, err := rbac.GetUser("test-user")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if len(retrieved.Roles) != 1 || retrieved.Roles[0] != "s3-user" {
		t.Error("Role was not assigned correctly")
	}
}

func TestRevokeRole(t *testing.T) {
	rbac := NewRBACManager()

	// Create user with role
	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Roles:    []string{"s3-user", "auditor"},
	}

	err := rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Revoke one role
	err = rbac.RevokeRole("test-user", "auditor")
	if err != nil {
		t.Fatalf("RevokeRole failed: %v", err)
	}

	// Verify role was revoked
	retrieved, err := rbac.GetUser("test-user")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if len(retrieved.Roles) != 1 || retrieved.Roles[0] != "s3-user" {
		t.Error("Role was not revoked correctly")
	}
}

func TestCheckAccess(t *testing.T) {
	rbac := NewRBACManager()

	// Create user with s3-user role
	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Roles:    []string{"s3-user"},
	}

	err := rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Test access for allowed action
	if !rbac.CheckAccess("test-user", "s3:read", "bucket/object") {
		t.Error("s3-user should have s3:read access")
	}

	// Test access for denied action
	if rbac.CheckAccess("test-user", "system:admin", "config") {
		t.Error("s3-user should not have system:admin access")
	}
}

func TestDeactivateUser(t *testing.T) {
	rbac := NewRBACManager()

	// Create user
	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Roles:    []string{"s3-user"},
	}

	err := rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Deactivate user
	err = rbac.DeactivateUser("test-user")
	if err != nil {
		t.Fatalf("DeactivateUser failed: %v", err)
	}

	// Verify user is deactivated
	retrieved, err := rbac.GetUser("test-user")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if retrieved.Active {
		t.Error("User should be deactivated")
	}

	// Verify deactivated user has no permissions
	if rbac.HasPermission("test-user", PermissionS3Read) {
		t.Error("Deactivated user should not have permissions")
	}
}

func TestUpdateLastLogin(t *testing.T) {
	rbac := NewRBACManager()

	// Create user
	user := &User{
		ID:       "test-user",
		Username: "testuser",
		Roles:    []string{"s3-user"},
	}

	err := rbac.CreateUser(user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Record initial state
	before, err := rbac.GetUser("test-user")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	initialLogin := before.LastLogin

	// Update last login
	err = rbac.UpdateLastLogin("test-user")
	if err != nil {
		t.Fatalf("UpdateLastLogin failed: %v", err)
	}

	// Verify last login was updated
	after, err := rbac.GetUser("test-user")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	// The update should set LastLogin to a time after the initial creation
	if !after.LastLogin.After(initialLogin) || after.LastLogin.Equal(initialLogin) {
		t.Errorf("LastLogin should be updated to a later time. Initial: %v, After: %v", initialLogin, after.LastLogin)
	}
}

func TestPermissionConstants(t *testing.T) {
	// Test that permission constants are defined correctly
	permissions := []Permission{
		PermissionS3Read,
		PermissionS3Write,
		PermissionS3Delete,
		PermissionS3List,
		PermissionS3Admin,
		PermissionSystemAdmin,
		PermissionSystemConfig,
		PermissionSystemUser,
		PermissionAuditRead,
		PermissionAuditWrite,
	}

	for _, perm := range permissions {
		if string(perm) == "" {
			t.Errorf("Permission constant is empty: %v", perm)
		}
	}
}
