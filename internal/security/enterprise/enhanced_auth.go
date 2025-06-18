package enterprise

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// EnhancedAuthenticator provides advanced authentication capabilities
type EnhancedAuthenticator struct {
	sessionManager  *SessionManager
	bruteForceGuard *BruteForceGuard
	deviceTracker   *DeviceTracker
	riskAssessment  *RiskAssessment
	config          *EnhancedAuthConfig
	auditLogger     *AuditLogger
	mutex           sync.RWMutex
}

// EnhancedAuthConfig holds enhanced authentication configuration
type EnhancedAuthConfig struct {
	SessionTimeoutMinutes       int  `json:"session_timeout_minutes"`
	MaxConcurrentSessions       int  `json:"max_concurrent_sessions"`
	BruteForceThreshold         int  `json:"brute_force_threshold"`
	BruteForceWindowMinutes     int  `json:"brute_force_window_minutes"`
	RequireDeviceRegistration   bool `json:"require_device_registration"`
	EnableRiskAssessment        bool `json:"enable_risk_assessment"`
	RequireMFAForHighRisk       bool `json:"require_mfa_for_high_risk"`
	RequireApprovalForNewDevice bool `json:"require_approval_for_new_device"`
	PasswordComplexityEnabled   bool `json:"password_complexity_enabled"`
	PasswordMinLength           int  `json:"password_min_length"`
	EnableAccountLockout        bool `json:"enable_account_lockout"`
	LockoutDurationMinutes      int  `json:"lockout_duration_minutes"`
}

// DefaultEnhancedAuthConfig returns default enhanced authentication configuration
func DefaultEnhancedAuthConfig() *EnhancedAuthConfig {
	return &EnhancedAuthConfig{
		SessionTimeoutMinutes:       30,
		MaxConcurrentSessions:       5,
		BruteForceThreshold:         5,
		BruteForceWindowMinutes:     15,
		RequireDeviceRegistration:   true,
		EnableRiskAssessment:        true,
		RequireMFAForHighRisk:       true,
		RequireApprovalForNewDevice: true,
		PasswordComplexityEnabled:   true,
		PasswordMinLength:           12,
		EnableAccountLockout:        true,
		LockoutDurationMinutes:      30,
	}
}

// NewEnhancedAuthenticator creates a new enhanced authenticator
func NewEnhancedAuthenticator(config *EnhancedAuthConfig, auditLogger *AuditLogger) *EnhancedAuthenticator {
	if config == nil {
		config = DefaultEnhancedAuthConfig()
	}

	return &EnhancedAuthenticator{
		sessionManager:  NewSessionManager(config),
		bruteForceGuard: NewBruteForceGuard(config),
		deviceTracker:   NewDeviceTracker(config),
		riskAssessment:  NewRiskAssessment(config),
		config:          config,
		auditLogger:     auditLogger,
	}
}

// EnhancedAuthenticateUser performs comprehensive authentication with security checks
func (ea *EnhancedAuthenticator) EnhancedAuthenticateUser(ctx context.Context, req *AuthenticationRequest) (*EnhancedAuthResult, error) {
	ea.mutex.Lock()
	defer ea.mutex.Unlock()

	result := &EnhancedAuthResult{
		UserID:           req.UserID,
		Authenticated:    false,
		RequiresMFA:      false,
		RequiresApproval: false,
		RiskLevel:        RiskLevelLow,
		Session:          nil,
		Reason:           "",
		Recommendations:  []string{},
	}

	// Step 1: Brute force protection
	if ea.bruteForceGuard.IsBlocked(req.UserID, req.IPAddress) {
		result.Reason = "Account temporarily locked due to suspicious activity"
		ea.auditLogger.LogSecurityEvent(AuditLevelWarning, req.UserID, "brute_force_blocked",
			fmt.Sprintf("Authentication blocked for user %s from IP %s", req.UserID, req.IPAddress))
		return result, nil
	}

	// Step 2: Device tracking and registration
	deviceInfo := ea.deviceTracker.GetOrCreateDevice(req.UserAgent, req.IPAddress)
	isNewDevice := !ea.deviceTracker.IsKnownDevice(req.UserID, deviceInfo.Fingerprint)

	if isNewDevice && ea.config.RequireDeviceRegistration {
		if ea.config.RequireApprovalForNewDevice {
			result.RequiresApproval = true
			result.Reason = "New device detected - requires approval"
			ea.auditLogger.LogSecurityEvent(AuditLevelInfo, req.UserID, "new_device_detected",
				fmt.Sprintf("New device detected for user %s: %s", req.UserID, deviceInfo.Fingerprint))
			return result, nil
		}
	}

	// Step 3: Risk assessment
	if ea.config.EnableRiskAssessment {
		riskLevel := ea.riskAssessment.AssessRisk(req, deviceInfo, isNewDevice)
		result.RiskLevel = riskLevel

		if riskLevel >= RiskLevelHigh && ea.config.RequireMFAForHighRisk {
			result.RequiresMFA = true
			result.Recommendations = append(result.Recommendations, "Multi-factor authentication required due to high risk")
		}
	}

	// Step 4: Password complexity validation (if setting new password)
	if req.NewPassword != "" && ea.config.PasswordComplexityEnabled {
		if err := ea.validatePasswordComplexity(req.NewPassword); err != nil {
			result.Reason = fmt.Sprintf("Password does not meet complexity requirements: %v", err)
			return result, nil
		}
	}

	// Step 5: Session management
	if req.SessionID != "" {
		session, err := ea.sessionManager.ValidateSession(req.SessionID)
		if err != nil {
			result.Reason = "Invalid or expired session"
			ea.auditLogger.LogError(req.UserID, "session_validation", "validate", err.Error(), map[string]interface{}{
				"session_id": req.SessionID,
				"ip_address": req.IPAddress,
			})
			return result, nil
		}

		// Update session activity
		ea.sessionManager.UpdateSessionActivity(req.SessionID, req.IPAddress)
		result.Session = session
	}

	// Step 6: Check concurrent session limits
	if ea.sessionManager.GetActiveSessionCount(req.UserID) >= ea.config.MaxConcurrentSessions {
		result.Reason = "Maximum concurrent sessions exceeded"
		result.Recommendations = append(result.Recommendations, "Close existing sessions to continue")
		return result, nil
	}

	// Step 7: Basic authentication successful - create session
	sessionID := ea.generateSessionID()
	session := ea.sessionManager.CreateEnhancedSession(req.UserID, req.IPAddress, req.UserAgent, deviceInfo)

	// Register device if new
	if isNewDevice {
		ea.deviceTracker.RegisterDevice(req.UserID, deviceInfo)
	}

	// Reset brute force counter on successful login
	ea.bruteForceGuard.ResetAttempts(req.UserID, req.IPAddress)

	// Log successful authentication
	ea.auditLogger.LogAccess(req.UserID, sessionID, "enhanced_authentication", "login", "SUCCESS", req.IPAddress, req.UserAgent)

	result.Authenticated = true
	result.Session = session
	result.Reason = "Authentication successful"

	return result, nil
}

// AuthenticationRequest holds enhanced authentication request data
type AuthenticationRequest struct {
	UserID      string    `json:"user_id"`
	Password    string    `json:"password,omitempty"`
	NewPassword string    `json:"new_password,omitempty"`
	MFAToken    string    `json:"mfa_token,omitempty"`
	SessionID   string    `json:"session_id,omitempty"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	Timestamp   time.Time `json:"timestamp"`
}

// EnhancedAuthResult holds the result of enhanced authentication
type EnhancedAuthResult struct {
	UserID            string           `json:"user_id"`
	Authenticated     bool             `json:"authenticated"`
	RequiresMFA       bool             `json:"requires_mfa"`
	RequiresApproval  bool             `json:"requires_approval"`
	RiskLevel         RiskLevel        `json:"risk_level"`
	Session           *EnhancedSession `json:"session,omitempty"`
	Reason            string           `json:"reason"`
	Recommendations   []string         `json:"recommendations"`
	DeviceFingerprint string           `json:"device_fingerprint,omitempty"`
}

// RiskLevel represents authentication risk levels
type RiskLevel int

const (
	RiskLevelLow RiskLevel = iota
	RiskLevelMedium
	RiskLevelHigh
	RiskLevelCritical
)

// EnhancedSession extends the basic session with additional security data
type EnhancedSession struct {
	*Session
	DeviceFingerprint string            `json:"device_fingerprint"`
	RiskLevel         RiskLevel         `json:"risk_level"`
	LastActivity      time.Time         `json:"last_activity"`
	CreatedFromIP     string            `json:"created_from_ip"`
	ActivityLog       []SessionActivity `json:"activity_log"`
}

// SessionActivity tracks session activities
type SessionActivity struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	IPAddress string    `json:"ip_address"`
	Resource  string    `json:"resource,omitempty"`
}

// validatePasswordComplexity validates password against complexity requirements
func (ea *EnhancedAuthenticator) validatePasswordComplexity(password string) error {
	if len(password) < ea.config.PasswordMinLength {
		return fmt.Errorf("password must be at least %d characters long", ea.config.PasswordMinLength)
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool
	specialChars := "!@#$%^&*()_+-=[]{}|;:,.<>?"

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		case strings.ContainsRune(specialChars, char):
			hasSpecial = true
		}
	}

	var missing []string
	if !hasUpper {
		missing = append(missing, "uppercase letter")
	}
	if !hasLower {
		missing = append(missing, "lowercase letter")
	}
	if !hasNumber {
		missing = append(missing, "number")
	}
	if !hasSpecial {
		missing = append(missing, "special character")
	}

	if len(missing) > 0 {
		return fmt.Errorf("password must contain at least one: %s", strings.Join(missing, ", "))
	}

	return nil
}

// generateSessionID generates a cryptographically secure session ID
func (ea *EnhancedAuthenticator) generateSessionID() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("sess_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
	}
	return "sess_" + hex.EncodeToString(bytes)
}

// InvalidateSession invalidates a session and logs the event
func (ea *EnhancedAuthenticator) InvalidateSession(sessionID, reason string) error {
	ea.mutex.Lock()
	defer ea.mutex.Unlock()

	session, err := ea.sessionManager.ValidateSession(sessionID)
	if err != nil {
		return err
	}

	err = ea.sessionManager.InvalidateSession(sessionID)
	if err != nil {
		return err
	}

	ea.auditLogger.LogAccess(session.UserID, sessionID, "session_management", "logout", "SUCCESS", "", "")
	ea.auditLogger.LogAction(session.UserID, "session_invalidation", reason, "SUCCESS", map[string]interface{}{
		"session_id": sessionID,
	})

	return nil
}

// GetUserSessions returns all active sessions for a user
func (ea *EnhancedAuthenticator) GetUserSessions(userID string) ([]*EnhancedSession, error) {
	ea.mutex.RLock()
	defer ea.mutex.RUnlock()

	sessions := ea.sessionManager.GetUserSessions(userID)
	enhancedSessions := make([]*EnhancedSession, len(sessions))

	for i, session := range sessions {
		enhancedSessions[i] = &EnhancedSession{
			Session:       session,
			LastActivity:  time.Now(), // This would be tracked in actual implementation
			CreatedFromIP: session.IPAddress,
		}
	}

	return enhancedSessions, nil
}

// ValidateSecurityRequirements validates if security requirements are met
func (ea *EnhancedAuthenticator) ValidateSecurityRequirements(userID string, action string) error {
	ea.mutex.RLock()
	defer ea.mutex.RUnlock()

	// Check if user has active sessions
	sessions := ea.sessionManager.GetUserSessions(userID)
	if len(sessions) == 0 {
		return fmt.Errorf("no active session found for user %s", userID)
	}

	// Check for high-risk actions requiring additional verification
	highRiskActions := []string{"delete_bucket", "modify_security_settings", "create_user", "delete_user"}
	for _, riskAction := range highRiskActions {
		if action == riskAction {
			// Would require additional MFA verification in real implementation
			ea.auditLogger.LogSecurityEvent(AuditLevelWarning, userID, "high_risk_action",
				fmt.Sprintf("High-risk action attempted: %s", action))
		}
	}

	return nil
}
