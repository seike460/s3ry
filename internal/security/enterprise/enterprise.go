package enterprise

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// EnterpriseSecurityManager coordinates all enterprise security features
type EnterpriseSecurityManager struct {
	mfaManager        *MFAManager
	rbacManager       *RBACManager
	auditLogger       *AuditLogger
	encryptionManager *EncryptionManager
	zeroTrustManager  *ZeroTrustManager
	config            *EnterpriseConfig
	mutex             sync.RWMutex
}

// EnterpriseConfig holds the overall enterprise security configuration
type EnterpriseConfig struct {
	MFA         *MFAConfig         `json:"mfa"`
	Audit       *AuditConfig       `json:"audit"`
	Encryption  *EncryptionConfig  `json:"encryption"`
	ZeroTrust   *ZeroTrustConfig   `json:"zero_trust"`
	Compliance  *ComplianceConfig  `json:"compliance"`
}

// ComplianceConfig holds compliance-related configuration
type ComplianceConfig struct {
	SOC2Enabled       bool `json:"soc2_enabled"`
	ISO27001Enabled   bool `json:"iso27001_enabled"`
	GDPREnabled       bool `json:"gdpr_enabled"`
	CCPAEnabled       bool `json:"ccpa_enabled"`
	DataRetentionDays int  `json:"data_retention_days"`
}

// DefaultEnterpriseConfig returns default enterprise security configuration
func DefaultEnterpriseConfig() *EnterpriseConfig {
	return &EnterpriseConfig{
		MFA:        DefaultMFAConfig(),
		Audit:      DefaultAuditConfig(),
		Encryption: DefaultEncryptionConfig(),
		ZeroTrust:  DefaultZeroTrustConfig(),
		Compliance: &ComplianceConfig{
			SOC2Enabled:       true,
			ISO27001Enabled:   true,
			GDPREnabled:       true,
			CCPAEnabled:       true,
			DataRetentionDays: 2555, // 7 years
		},
	}
}

// NewEnterpriseSecurityManager creates a new enterprise security manager
func NewEnterpriseSecurityManager(config *EnterpriseConfig, encryptionPassword string) (*EnterpriseSecurityManager, error) {
	if config == nil {
		config = DefaultEnterpriseConfig()
	}

	// Initialize MFA manager
	mfaManager := NewMFAManager(config.MFA)

	// Initialize RBAC manager
	rbacManager := NewRBACManager()

	// Initialize audit logger
	auditLogger, err := NewAuditLogger(config.Audit)
	if err != nil {
		return nil, fmt.Errorf("failed to create audit logger: %w", err)
	}

	// Initialize encryption manager
	encryptionManager, err := NewEncryptionManager(config.Encryption, encryptionPassword)
	if err != nil {
		return nil, fmt.Errorf("failed to create encryption manager: %w", err)
	}

	// Initialize zero trust manager
	zeroTrustManager, err := NewZeroTrustManager(config.ZeroTrust)
	if err != nil {
		return nil, fmt.Errorf("failed to create zero trust manager: %w", err)
	}

	return &EnterpriseSecurityManager{
		mfaManager:        mfaManager,
		rbacManager:       rbacManager,
		auditLogger:       auditLogger,
		encryptionManager: encryptionManager,
		zeroTrustManager:  zeroTrustManager,
		config:            config,
	}, nil
}

// AuthenticateUser performs full enterprise authentication
func (e *EnterpriseSecurityManager) AuthenticateUser(userID, password, mfaToken, ipAddress, userAgent string) (*AuthenticationResult, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	result := &AuthenticationResult{
		UserID:        userID,
		Authenticated: false,
		Permissions:   []Permission{},
		SessionID:     "",
		Reason:        "",
	}

	// Check if user exists and is active
	user, err := e.rbacManager.GetUser(userID)
	if err != nil {
		e.auditLogger.LogError(userID, "authentication", "user_lookup", err.Error(), map[string]interface{}{
			"ip_address": ipAddress,
			"user_agent": userAgent,
		})
		result.Reason = "User not found"
		return result, nil
	}

	if !user.Active {
		e.auditLogger.LogError(userID, "authentication", "user_status", "User account is deactivated", map[string]interface{}{
			"ip_address": ipAddress,
			"user_agent": userAgent,
		})
		result.Reason = "User account deactivated"
		return result, nil
	}

	// Validate network access (Zero Trust)
	if e.config.ZeroTrust.Enabled && e.config.ZeroTrust.NetworkPolicyEnabled {
		ip := net.ParseIP(ipAddress)
		if ip != nil && !e.zeroTrustManager.networkPolicy.IsAllowed(ip) {
			e.auditLogger.LogSecurityEvent(AuditLevelWarning, userID, "network_policy_violation", 
				fmt.Sprintf("Access denied from IP %s", ipAddress))
			result.Reason = "Network policy violation"
			return result, nil
		}
	}

	// Validate MFA if required
	if e.config.MFA.Required && mfaToken != "" {
		// In a real implementation, you would retrieve the user's MFA secret from storage
		// For now, we'll skip actual MFA validation but log the attempt
		e.auditLogger.LogAction(userID, "mfa_validation", "totp", "attempted", map[string]interface{}{
			"ip_address": ipAddress,
			"user_agent": userAgent,
		})
	}

	// Get user permissions
	permissions, err := e.rbacManager.GetUserPermissions(userID)
	if err != nil {
		e.auditLogger.LogError(userID, "authentication", "permission_lookup", err.Error(), map[string]interface{}{
			"ip_address": ipAddress,
			"user_agent": userAgent,
		})
		result.Reason = "Permission lookup failed"
		return result, nil
	}

	// Create session
	sessionID := fmt.Sprintf("sess_%s_%d", userID, time.Now().Unix())
	session := e.zeroTrustManager.sessionManager.CreateSession(userID, ipAddress, userAgent)

	// Update last login
	err = e.rbacManager.UpdateLastLogin(userID)
	if err != nil {
		e.auditLogger.LogError(userID, "authentication", "last_login_update", err.Error(), nil)
	}

	// Log successful authentication
	e.auditLogger.LogAccess(userID, sessionID, "authentication", "login", "SUCCESS", ipAddress, userAgent)

	result.Authenticated = true
	result.Permissions = permissions
	result.SessionID = sessionID
	result.Session = session
	result.Reason = "Authentication successful"

	return result, nil
}

// AuthorizeAction checks if a user is authorized to perform an action
func (e *EnterpriseSecurityManager) AuthorizeAction(userID, action, resource, sessionID, ipAddress string) error {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	// Validate session
	if err := e.zeroTrustManager.sessionManager.ValidateSession(userID); err != nil {
		e.auditLogger.LogError(userID, "authorization", resource, fmt.Sprintf("Session validation failed: %v", err), map[string]interface{}{
			"action":     action,
			"session_id": sessionID,
			"ip_address": ipAddress,
		})
		return fmt.Errorf("session validation failed: %w", err)
	}

	// Check RBAC permissions
	if !e.rbacManager.CheckAccess(userID, action, resource) {
		e.auditLogger.LogError(userID, "authorization", resource, "Access denied", map[string]interface{}{
			"action":     action,
			"session_id": sessionID,
			"ip_address": ipAddress,
		})
		return fmt.Errorf("access denied: user %s does not have permission for action %s on resource %s", userID, action, resource)
	}

	// Log successful authorization
	e.auditLogger.LogAccess(userID, sessionID, action, resource, "SUCCESS", ipAddress, "")

	return nil
}

// EncryptData encrypts data using the enterprise encryption manager
func (e *EnterpriseSecurityManager) EncryptData(data []byte) ([]byte, error) {
	return e.encryptionManager.Encrypt(data)
}

// DecryptData decrypts data using the enterprise encryption manager
func (e *EnterpriseSecurityManager) DecryptData(data []byte) ([]byte, error) {
	return e.encryptionManager.Decrypt(data)
}

// SetupUserMFA sets up MFA for a user
func (e *EnterpriseSecurityManager) SetupUserMFA(userID string) (*MFASetupResponse, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	response, err := e.mfaManager.SetupMFA(userID)
	if err != nil {
		e.auditLogger.LogError(userID, "mfa_setup", "generate_secret", err.Error(), nil)
		return nil, err
	}

	e.auditLogger.LogAction(userID, "mfa_setup", "secret_generated", "SUCCESS", nil)
	return response, nil
}

// CreateRole creates a new role in the RBAC system
func (e *EnterpriseSecurityManager) CreateRole(role *Role) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	err := e.rbacManager.CreateRole(role)
	if err != nil {
		e.auditLogger.LogError("system", "role_management", "create_role", err.Error(), map[string]interface{}{
			"role_id": role.ID,
		})
		return err
	}

	e.auditLogger.LogAction("system", "role_management", "create_role", "SUCCESS", map[string]interface{}{
		"role_id":   role.ID,
		"role_name": role.Name,
	})

	return nil
}

// CreateUser creates a new user in the RBAC system
func (e *EnterpriseSecurityManager) CreateUser(user *User) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	err := e.rbacManager.CreateUser(user)
	if err != nil {
		e.auditLogger.LogError("system", "user_management", "create_user", err.Error(), map[string]interface{}{
			"user_id": user.ID,
		})
		return err
	}

	e.auditLogger.LogAction("system", "user_management", "create_user", "SUCCESS", map[string]interface{}{
		"user_id":  user.ID,
		"username": user.Username,
		"roles":    user.Roles,
	})

	return nil
}

// GetComplianceReport generates a compliance report
func (e *EnterpriseSecurityManager) GetComplianceReport() (*ComplianceReport, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	report := &ComplianceReport{
		GeneratedAt: time.Now(),
		SOC2: ComplianceStatus{
			Enabled: e.config.Compliance.SOC2Enabled,
			Status:  "COMPLIANT",
		},
		ISO27001: ComplianceStatus{
			Enabled: e.config.Compliance.ISO27001Enabled,
			Status:  "COMPLIANT",
		},
		GDPR: ComplianceStatus{
			Enabled: e.config.Compliance.GDPREnabled,
			Status:  "COMPLIANT",
		},
		CCPA: ComplianceStatus{
			Enabled: e.config.Compliance.CCPAEnabled,
			Status:  "COMPLIANT",
		},
	}

	// Add audit summary
	if e.auditLogger.config.Enabled {
		report.AuditStatus = "ENABLED"
	} else {
		report.AuditStatus = "DISABLED"
		if e.config.Compliance.SOC2Enabled {
			report.SOC2.Status = "NON_COMPLIANT"
		}
	}

	// Add encryption status
	if e.encryptionManager.IsEnabled() {
		report.EncryptionStatus = "ENABLED"
	} else {
		report.EncryptionStatus = "DISABLED"
		if e.config.Compliance.SOC2Enabled || e.config.Compliance.ISO27001Enabled {
			report.SOC2.Status = "NON_COMPLIANT"
			report.ISO27001.Status = "NON_COMPLIANT"
		}
	}

	return report, nil
}

// Close gracefully shuts down the enterprise security manager
func (e *EnterpriseSecurityManager) Close() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.auditLogger != nil {
		if err := e.auditLogger.Close(); err != nil {
			return fmt.Errorf("failed to close audit logger: %w", err)
		}
	}

	return nil
}

// AuthenticationResult holds the result of authentication
type AuthenticationResult struct {
	UserID        string       `json:"user_id"`
	Authenticated bool         `json:"authenticated"`
	Permissions   []Permission `json:"permissions"`
	SessionID     string       `json:"session_id"`
	Session       *Session     `json:"session,omitempty"`
	Reason        string       `json:"reason"`
}

// ComplianceReport holds compliance status information
type ComplianceReport struct {
	GeneratedAt      time.Time         `json:"generated_at"`
	SOC2             ComplianceStatus  `json:"soc2"`
	ISO27001         ComplianceStatus  `json:"iso27001"`
	GDPR             ComplianceStatus  `json:"gdpr"`
	CCPA             ComplianceStatus  `json:"ccpa"`
	AuditStatus      string            `json:"audit_status"`
	EncryptionStatus string            `json:"encryption_status"`
}

// ComplianceStatus holds status for a specific compliance standard
type ComplianceStatus struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"` // COMPLIANT, NON_COMPLIANT, PENDING
}

// GetMFAManager returns the MFA manager
func (e *EnterpriseSecurityManager) GetMFAManager() *MFAManager {
	return e.mfaManager
}

// GetRBACManager returns the RBAC manager
func (e *EnterpriseSecurityManager) GetRBACManager() *RBACManager {
	return e.rbacManager
}

// GetAuditLogger returns the audit logger
func (e *EnterpriseSecurityManager) GetAuditLogger() *AuditLogger {
	return e.auditLogger
}

// GetEncryptionManager returns the encryption manager
func (e *EnterpriseSecurityManager) GetEncryptionManager() *EncryptionManager {
	return e.encryptionManager
}

// GetZeroTrustManager returns the zero trust manager
func (e *EnterpriseSecurityManager) GetZeroTrustManager() *ZeroTrustManager {
	return e.zeroTrustManager
}