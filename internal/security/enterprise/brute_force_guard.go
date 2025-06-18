package enterprise

import (
	"sync"
	"time"
)

// BruteForceGuard protects against brute force attacks
type BruteForceGuard struct {
	config        *EnhancedAuthConfig
	userAttempts  map[string]*AttemptsRecord
	ipAttempts    map[string]*AttemptsRecord
	mutex         sync.RWMutex
	cleanupTicker *time.Ticker
}

// AttemptsRecord tracks failed authentication attempts
type AttemptsRecord struct {
	Count        int       `json:"count"`
	FirstAttempt time.Time `json:"first_attempt"`
	LastAttempt  time.Time `json:"last_attempt"`
	BlockedUntil time.Time `json:"blocked_until"`
}

// NewBruteForceGuard creates a new brute force protection guard
func NewBruteForceGuard(config *EnhancedAuthConfig) *BruteForceGuard {
	guard := &BruteForceGuard{
		config:        config,
		userAttempts:  make(map[string]*AttemptsRecord),
		ipAttempts:    make(map[string]*AttemptsRecord),
		cleanupTicker: time.NewTicker(time.Hour), // Cleanup every hour
	}

	// Start cleanup routine
	go guard.cleanupExpiredRecords()

	return guard
}

// IsBlocked checks if a user or IP is currently blocked
func (bg *BruteForceGuard) IsBlocked(userID, ipAddress string) bool {
	bg.mutex.RLock()
	defer bg.mutex.RUnlock()

	now := time.Now()

	// Check user-based blocking
	if userRecord, exists := bg.userAttempts[userID]; exists {
		if userRecord.BlockedUntil.After(now) {
			return true
		}
	}

	// Check IP-based blocking
	if ipRecord, exists := bg.ipAttempts[ipAddress]; exists {
		if ipRecord.BlockedUntil.After(now) {
			return true
		}
	}

	return false
}

// RecordFailedAttempt records a failed authentication attempt
func (bg *BruteForceGuard) RecordFailedAttempt(userID, ipAddress string) {
	bg.mutex.Lock()
	defer bg.mutex.Unlock()

	now := time.Now()
	windowStart := now.Add(-time.Duration(bg.config.BruteForceWindowMinutes) * time.Minute)

	// Record user attempt
	bg.recordUserAttempt(userID, now, windowStart)

	// Record IP attempt
	bg.recordIPAttempt(ipAddress, now, windowStart)
}

// recordUserAttempt records a failed attempt for a specific user
func (bg *BruteForceGuard) recordUserAttempt(userID string, now, windowStart time.Time) {
	userRecord, exists := bg.userAttempts[userID]
	if !exists {
		userRecord = &AttemptsRecord{
			FirstAttempt: now,
		}
		bg.userAttempts[userID] = userRecord
	}

	// Reset count if outside the window
	if userRecord.FirstAttempt.Before(windowStart) {
		userRecord.Count = 0
		userRecord.FirstAttempt = now
	}

	userRecord.Count++
	userRecord.LastAttempt = now

	// Apply blocking if threshold exceeded
	if userRecord.Count >= bg.config.BruteForceThreshold {
		lockoutDuration := time.Duration(bg.config.LockoutDurationMinutes) * time.Minute
		userRecord.BlockedUntil = now.Add(lockoutDuration)
	}
}

// recordIPAttempt records a failed attempt for a specific IP
func (bg *BruteForceGuard) recordIPAttempt(ipAddress string, now, windowStart time.Time) {
	ipRecord, exists := bg.ipAttempts[ipAddress]
	if !exists {
		ipRecord = &AttemptsRecord{
			FirstAttempt: now,
		}
		bg.ipAttempts[ipAddress] = ipRecord
	}

	// Reset count if outside the window
	if ipRecord.FirstAttempt.Before(windowStart) {
		ipRecord.Count = 0
		ipRecord.FirstAttempt = now
	}

	ipRecord.Count++
	ipRecord.LastAttempt = now

	// Apply blocking if threshold exceeded (higher threshold for IP blocking)
	ipThreshold := bg.config.BruteForceThreshold * 3 // 3x threshold for IP blocking
	if ipRecord.Count >= ipThreshold {
		lockoutDuration := time.Duration(bg.config.LockoutDurationMinutes) * time.Minute
		ipRecord.BlockedUntil = now.Add(lockoutDuration)
	}
}

// ResetAttempts resets failed attempts for a user and IP on successful login
func (bg *BruteForceGuard) ResetAttempts(userID, ipAddress string) {
	bg.mutex.Lock()
	defer bg.mutex.Unlock()

	// Reset user attempts
	if userRecord, exists := bg.userAttempts[userID]; exists {
		userRecord.Count = 0
		userRecord.BlockedUntil = time.Time{}
	}

	// Reset IP attempts (only if this was the only user causing issues)
	if ipRecord, exists := bg.ipAttempts[ipAddress]; exists {
		ipRecord.Count = 0
		ipRecord.BlockedUntil = time.Time{}
	}
}

// GetAttemptStats returns current attempt statistics
func (bg *BruteForceGuard) GetAttemptStats(userID, ipAddress string) (userAttempts, ipAttempts int, userBlocked, ipBlocked bool) {
	bg.mutex.RLock()
	defer bg.mutex.RUnlock()

	now := time.Now()

	// Get user stats
	if userRecord, exists := bg.userAttempts[userID]; exists {
		userAttempts = userRecord.Count
		userBlocked = userRecord.BlockedUntil.After(now)
	}

	// Get IP stats
	if ipRecord, exists := bg.ipAttempts[ipAddress]; exists {
		ipAttempts = ipRecord.Count
		ipBlocked = ipRecord.BlockedUntil.After(now)
	}

	return
}

// cleanupExpiredRecords removes old records that are no longer relevant
func (bg *BruteForceGuard) cleanupExpiredRecords() {
	for range bg.cleanupTicker.C {
		bg.mutex.Lock()
		now := time.Now()
		cleanupWindow := time.Duration(bg.config.BruteForceWindowMinutes) * time.Minute * 2 // Keep records for 2x the window

		// Cleanup user attempts
		for userID, record := range bg.userAttempts {
			if record.LastAttempt.Add(cleanupWindow).Before(now) && record.BlockedUntil.Before(now) {
				delete(bg.userAttempts, userID)
			}
		}

		// Cleanup IP attempts
		for ip, record := range bg.ipAttempts {
			if record.LastAttempt.Add(cleanupWindow).Before(now) && record.BlockedUntil.Before(now) {
				delete(bg.ipAttempts, ip)
			}
		}

		bg.mutex.Unlock()
	}
}

// GetBlockedUsers returns a list of currently blocked users
func (bg *BruteForceGuard) GetBlockedUsers() map[string]time.Time {
	bg.mutex.RLock()
	defer bg.mutex.RUnlock()

	blocked := make(map[string]time.Time)
	now := time.Now()

	for userID, record := range bg.userAttempts {
		if record.BlockedUntil.After(now) {
			blocked[userID] = record.BlockedUntil
		}
	}

	return blocked
}

// GetBlockedIPs returns a list of currently blocked IP addresses
func (bg *BruteForceGuard) GetBlockedIPs() map[string]time.Time {
	bg.mutex.RLock()
	defer bg.mutex.RUnlock()

	blocked := make(map[string]time.Time)
	now := time.Now()

	for ip, record := range bg.ipAttempts {
		if record.BlockedUntil.After(now) {
			blocked[ip] = record.BlockedUntil
		}
	}

	return blocked
}

// UnblockUser manually unblocks a user (admin function)
func (bg *BruteForceGuard) UnblockUser(userID string) {
	bg.mutex.Lock()
	defer bg.mutex.Unlock()

	if record, exists := bg.userAttempts[userID]; exists {
		record.BlockedUntil = time.Time{}
		record.Count = 0
	}
}

// UnblockIP manually unblocks an IP address (admin function)
func (bg *BruteForceGuard) UnblockIP(ipAddress string) {
	bg.mutex.Lock()
	defer bg.mutex.Unlock()

	if record, exists := bg.ipAttempts[ipAddress]; exists {
		record.BlockedUntil = time.Time{}
		record.Count = 0
	}
}

// Close stops the cleanup routine
func (bg *BruteForceGuard) Close() {
	if bg.cleanupTicker != nil {
		bg.cleanupTicker.Stop()
	}
}
