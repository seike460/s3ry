package enterprise

import (
	"fmt"
	"sync"
	"time"
)

// SessionManager manages user sessions with enhanced security features
type SessionManager struct {
	config         *EnhancedAuthConfig
	activeSessions map[string]*EnhancedSession
	userSessions   map[string][]string // userID -> sessionIDs
	mutex          sync.RWMutex
	cleanupTicker  *time.Ticker
}

// Session represents a basic user session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Active    bool      `json:"active"`
}

// NewSessionManager creates a new session manager
func NewSessionManager(config *EnhancedAuthConfig) *SessionManager {
	sm := &SessionManager{
		config:         config,
		activeSessions: make(map[string]*EnhancedSession),
		userSessions:   make(map[string][]string),
		cleanupTicker:  time.NewTicker(time.Minute * 10), // Cleanup every 10 minutes
	}

	// Start cleanup routine
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new basic session
func (sm *SessionManager) CreateSession(userID, ipAddress, userAgent string) *Session {
	sessionID := fmt.Sprintf("sess_%s_%d", userID, time.Now().UnixNano())
	expiresAt := time.Now().Add(time.Duration(sm.config.SessionTimeoutMinutes) * time.Minute)

	session := &Session{
		ID:        sessionID,
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Active:    true,
	}

	return session
}

// CreateEnhancedSession creates a new enhanced session with additional security features
func (sm *SessionManager) CreateEnhancedSession(userID, ipAddress, userAgent string, device *DeviceInfo) *EnhancedSession {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sessionID := fmt.Sprintf("sess_%s_%d", userID, time.Now().UnixNano())
	expiresAt := time.Now().Add(time.Duration(sm.config.SessionTimeoutMinutes) * time.Minute)

	session := &EnhancedSession{
		Session: &Session{
			ID:        sessionID,
			UserID:    userID,
			CreatedAt: time.Now(),
			ExpiresAt: expiresAt,
			IPAddress: ipAddress,
			UserAgent: userAgent,
			Active:    true,
		},
		DeviceFingerprint: device.Fingerprint,
		RiskLevel:         RiskLevelLow,
		LastActivity:      time.Now(),
		CreatedFromIP:     ipAddress,
		ActivityLog:       []SessionActivity{},
	}

	// Store session
	sm.activeSessions[sessionID] = session

	// Track user sessions
	if _, exists := sm.userSessions[userID]; !exists {
		sm.userSessions[userID] = []string{}
	}
	sm.userSessions[userID] = append(sm.userSessions[userID], sessionID)

	// Enforce concurrent session limits
	sm.enforceConcurrentSessionLimits(userID)

	return session
}

// ValidateSession validates a session by ID
func (sm *SessionManager) ValidateSession(sessionID string) (*Session, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.activeSessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	if !session.Active {
		return nil, fmt.Errorf("session %s is not active", sessionID)
	}

	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session %s has expired", sessionID)
	}

	return session.Session, nil
}

// UpdateSessionActivity updates session activity timestamp
func (sm *SessionManager) UpdateSessionActivity(sessionID, ipAddress string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.LastActivity = time.Now()
	session.ExpiresAt = time.Now().Add(time.Duration(sm.config.SessionTimeoutMinutes) * time.Minute)

	// Log activity
	activity := SessionActivity{
		Timestamp: time.Now(),
		Action:    "activity_update",
		IPAddress: ipAddress,
	}
	session.ActivityLog = append(session.ActivityLog, activity)

	// Keep only last 10 activities
	if len(session.ActivityLog) > 10 {
		session.ActivityLog = session.ActivityLog[len(session.ActivityLog)-10:]
	}

	return nil
}

// InvalidateSession invalidates a specific session
func (sm *SessionManager) InvalidateSession(sessionID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	session, exists := sm.activeSessions[sessionID]
	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.Active = false
	delete(sm.activeSessions, sessionID)

	// Remove from user sessions
	userSessions := sm.userSessions[session.UserID]
	for i, sid := range userSessions {
		if sid == sessionID {
			sm.userSessions[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
			break
		}
	}

	return nil
}

// GetUserSessions returns all active sessions for a user
func (sm *SessionManager) GetUserSessions(userID string) []*Session {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var sessions []*Session
	sessionIDs, exists := sm.userSessions[userID]
	if !exists {
		return sessions
	}

	for _, sessionID := range sessionIDs {
		if session, exists := sm.activeSessions[sessionID]; exists && session.Active {
			sessions = append(sessions, session.Session)
		}
	}

	return sessions
}

// GetActiveSessionCount returns the number of active sessions for a user
func (sm *SessionManager) GetActiveSessionCount(userID string) int {
	return len(sm.GetUserSessions(userID))
}

// enforceConcurrentSessionLimits enforces maximum concurrent session limits
func (sm *SessionManager) enforceConcurrentSessionLimits(userID string) {
	sessionIDs := sm.userSessions[userID]
	if len(sessionIDs) <= sm.config.MaxConcurrentSessions {
		return
	}

	// Remove oldest sessions
	excessCount := len(sessionIDs) - sm.config.MaxConcurrentSessions
	for i := 0; i < excessCount; i++ {
		oldestSessionID := sessionIDs[i]
		if session, exists := sm.activeSessions[oldestSessionID]; exists {
			session.Active = false
			delete(sm.activeSessions, oldestSessionID)
		}
	}

	// Update user sessions list
	sm.userSessions[userID] = sessionIDs[excessCount:]
}

// cleanupExpiredSessions removes expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	for range sm.cleanupTicker.C {
		sm.mutex.Lock()
		now := time.Now()
		var expiredSessions []string

		for sessionID, session := range sm.activeSessions {
			if now.After(session.ExpiresAt) || !session.Active {
				expiredSessions = append(expiredSessions, sessionID)
			}
		}

		// Remove expired sessions
		for _, sessionID := range expiredSessions {
			session := sm.activeSessions[sessionID]
			delete(sm.activeSessions, sessionID)

			// Remove from user sessions
			userSessions := sm.userSessions[session.UserID]
			for i, sid := range userSessions {
				if sid == sessionID {
					sm.userSessions[session.UserID] = append(userSessions[:i], userSessions[i+1:]...)
					break
				}
			}
		}

		sm.mutex.Unlock()
	}
}

// InvalidateAllUserSessions invalidates all sessions for a user
func (sm *SessionManager) InvalidateAllUserSessions(userID string) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sessionIDs, exists := sm.userSessions[userID]
	if !exists {
		return nil
	}

	for _, sessionID := range sessionIDs {
		if session, exists := sm.activeSessions[sessionID]; exists {
			session.Active = false
			delete(sm.activeSessions, sessionID)
		}
	}

	delete(sm.userSessions, userID)
	return nil
}

// GetSessionInfo returns detailed information about a session
func (sm *SessionManager) GetSessionInfo(sessionID string) (*EnhancedSession, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	session, exists := sm.activeSessions[sessionID]
	if !exists {
		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

// Close stops the session manager cleanup routine
func (sm *SessionManager) Close() {
	if sm.cleanupTicker != nil {
		sm.cleanupTicker.Stop()
	}
}
