package enterprise

import (
	"math"
	"net"
	"strings"
	"time"
)

// RiskAssessment provides comprehensive risk analysis for authentication attempts
type RiskAssessment struct {
	config          *EnhancedAuthConfig
	behaviorTracker *BehaviorTracker
	threatIntel     *ThreatIntelligence
}

// BehaviorTracker tracks user behavior patterns
type BehaviorTracker struct {
	userPatterns map[string]*UserBehaviorPattern
}

// UserBehaviorPattern holds behavioral patterns for a user
type UserBehaviorPattern struct {
	TypicalLoginHours    []int          `json:"typical_login_hours"` // Hours 0-23
	TypicalDaysOfWeek    []time.Weekday `json:"typical_days_of_week"`
	CommonLocations      []*GeoLocation `json:"common_locations"`
	CommonDeviceTypes    []DeviceType   `json:"common_device_types"`
	AverageSessionLength time.Duration  `json:"average_session_length"`
	LoginFrequency       float64        `json:"login_frequency"` // Logins per day
	LastLoginTime        time.Time      `json:"last_login_time"`
	VelocityThreshold    int            `json:"velocity_threshold"` // Max logins per hour
}

// ThreatIntelligence provides threat intelligence data
type ThreatIntelligence struct {
	knownMaliciousIPs    map[string]ThreatLevel
	suspiciousUserAgents []string
	blockedCountries     []string
	vpnDetection         bool
}

// ThreatLevel represents the threat level of an entity
type ThreatLevel int

const (
	ThreatLevelNone ThreatLevel = iota
	ThreatLevelLow
	ThreatLevelMedium
	ThreatLevelHigh
	ThreatLevelCritical
)

// NewRiskAssessment creates a new risk assessment engine
func NewRiskAssessment(config *EnhancedAuthConfig) *RiskAssessment {
	return &RiskAssessment{
		config:          config,
		behaviorTracker: NewBehaviorTracker(),
		threatIntel:     NewThreatIntelligence(),
	}
}

// NewBehaviorTracker creates a new behavior tracker
func NewBehaviorTracker() *BehaviorTracker {
	return &BehaviorTracker{
		userPatterns: make(map[string]*UserBehaviorPattern),
	}
}

// NewThreatIntelligence creates a new threat intelligence service
func NewThreatIntelligence() *ThreatIntelligence {
	return &ThreatIntelligence{
		knownMaliciousIPs: make(map[string]ThreatLevel),
		suspiciousUserAgents: []string{
			"curl", "wget", "python-requests", "bot", "crawler", "scanner",
		},
		blockedCountries: []string{
			// This would be configurable based on organizational policy
		},
		vpnDetection: true,
	}
}

// AssessRisk performs comprehensive risk assessment for an authentication attempt
func (ra *RiskAssessment) AssessRisk(req *AuthenticationRequest, device *DeviceInfo, isNewDevice bool) RiskLevel {
	if !ra.config.EnableRiskAssessment {
		return RiskLevelLow
	}

	riskScore := 0.0

	// 1. Device-based risk assessment
	deviceRisk := ra.assessDeviceRisk(device, isNewDevice)
	riskScore += deviceRisk * 0.3 // 30% weight

	// 2. Behavioral risk assessment
	behaviorRisk := ra.assessBehaviorRisk(req)
	riskScore += behaviorRisk * 0.25 // 25% weight

	// 3. Geographical risk assessment
	geoRisk := ra.assessGeographicalRisk(req.IPAddress, req.UserID)
	riskScore += geoRisk * 0.2 // 20% weight

	// 4. Temporal risk assessment
	temporalRisk := ra.assessTemporalRisk(req)
	riskScore += temporalRisk * 0.15 // 15% weight

	// 5. Threat intelligence risk assessment
	threatRisk := ra.assessThreatIntelligenceRisk(req, device)
	riskScore += threatRisk * 0.1 // 10% weight

	// Convert score to risk level
	return ra.scoreToRiskLevel(riskScore)
}

// assessDeviceRisk evaluates risk based on device characteristics
func (ra *RiskAssessment) assessDeviceRisk(device *DeviceInfo, isNewDevice bool) float64 {
	risk := 0.0

	// New device penalty
	if isNewDevice {
		risk += 0.4
	}

	// Device type risk
	switch device.DeviceType {
	case DeviceTypeServer:
		risk += 0.6 // Automated tools are higher risk
	case DeviceTypeMobile:
		risk += 0.1 // Mobile devices are slightly lower risk
	case DeviceTypeDesktop:
		risk += 0.2 // Desktop baseline
	case DeviceTypeTablet:
		risk += 0.15
	default:
		risk += 0.3 // Unknown device type
	}

	// Browser/OS risk
	if device.OS == "Unknown" || device.Browser == "Unknown" {
		risk += 0.2
	}

	// Use device's own risk score
	risk += device.RiskScore * 0.5

	return math.Min(risk, 1.0)
}

// assessBehaviorRisk evaluates risk based on user behavior patterns
func (ra *RiskAssessment) assessBehaviorRisk(req *AuthenticationRequest) float64 {
	pattern := ra.behaviorTracker.getUserPattern(req.UserID)
	if pattern == nil {
		// No established pattern - moderate risk for new users
		return 0.3
	}

	risk := 0.0

	// Time-based analysis
	currentHour := req.Timestamp.Hour()
	if !ra.isTypicalLoginHour(pattern, currentHour) {
		risk += 0.2
	}

	currentDay := req.Timestamp.Weekday()
	if !ra.isTypicalLoginDay(pattern, currentDay) {
		risk += 0.15
	}

	// Velocity analysis
	if ra.isHighVelocityLogin(pattern, req.Timestamp) {
		risk += 0.4
	}

	// Session frequency analysis
	timeSinceLastLogin := req.Timestamp.Sub(pattern.LastLoginTime)
	if timeSinceLastLogin < time.Minute*5 {
		risk += 0.3 // Very rapid re-authentication
	}

	return math.Min(risk, 1.0)
}

// assessGeographicalRisk evaluates risk based on location
func (ra *RiskAssessment) assessGeographicalRisk(ipAddress, userID string) float64 {
	risk := 0.0

	// Get user's typical locations
	pattern := ra.behaviorTracker.getUserPattern(userID)
	if pattern != nil && len(pattern.CommonLocations) > 0 {
		// Check if current location is unusual
		// This would involve geo-IP lookup and comparison
		// For now, simplified implementation
		if ra.isUnusualLocation(ipAddress, pattern.CommonLocations) {
			risk += 0.5
		}
	}

	// Check for VPN/Proxy indicators
	if ra.threatIntel.vpnDetection && ra.isVPNOrProxy(ipAddress) {
		risk += 0.3
	}

	// Check blocked countries
	location := NewGeoLocationService().GetLocation(ipAddress)
	if location != nil && ra.isBlockedCountry(location.Country) {
		risk += 0.7
	}

	return math.Min(risk, 1.0)
}

// assessTemporalRisk evaluates risk based on timing patterns
func (ra *RiskAssessment) assessTemporalRisk(req *AuthenticationRequest) float64 {
	risk := 0.0

	// Check for unusual login times (very early morning, late night)
	hour := req.Timestamp.Hour()
	if hour >= 2 && hour <= 5 {
		risk += 0.2 // Early morning logins are slightly suspicious
	}

	// Check for weekend logins (depends on organization policy)
	if req.Timestamp.Weekday() == time.Saturday || req.Timestamp.Weekday() == time.Sunday {
		risk += 0.1
	}

	// Check for holiday logins (would require holiday calendar integration)
	// if ra.isHoliday(req.Timestamp) {
	//     risk += 0.1
	// }

	return math.Min(risk, 1.0)
}

// assessThreatIntelligenceRisk evaluates risk based on threat intelligence
func (ra *RiskAssessment) assessThreatIntelligenceRisk(req *AuthenticationRequest, device *DeviceInfo) float64 {
	risk := 0.0

	// Check IP against threat intelligence
	if threatLevel, exists := ra.threatIntel.knownMaliciousIPs[req.IPAddress]; exists {
		switch threatLevel {
		case ThreatLevelCritical:
			risk += 1.0
		case ThreatLevelHigh:
			risk += 0.8
		case ThreatLevelMedium:
			risk += 0.5
		case ThreatLevelLow:
			risk += 0.2
		}
	}

	// Check user agent against suspicious patterns
	userAgent := strings.ToLower(device.UserAgent)
	for _, suspicious := range ra.threatIntel.suspiciousUserAgents {
		if strings.Contains(userAgent, suspicious) {
			risk += 0.3
			break
		}
	}

	return math.Min(risk, 1.0)
}

// scoreToRiskLevel converts a numerical risk score to a risk level
func (ra *RiskAssessment) scoreToRiskLevel(score float64) RiskLevel {
	switch {
	case score >= 0.8:
		return RiskLevelCritical
	case score >= 0.6:
		return RiskLevelHigh
	case score >= 0.4:
		return RiskLevelMedium
	case score >= 0.2:
		return RiskLevelLow
	default:
		return RiskLevelLow
	}
}

// Helper methods for behavior analysis

func (bt *BehaviorTracker) getUserPattern(userID string) *UserBehaviorPattern {
	return bt.userPatterns[userID]
}

func (ra *RiskAssessment) isTypicalLoginHour(pattern *UserBehaviorPattern, hour int) bool {
	for _, typicalHour := range pattern.TypicalLoginHours {
		if typicalHour == hour {
			return true
		}
	}
	return false
}

func (ra *RiskAssessment) isTypicalLoginDay(pattern *UserBehaviorPattern, day time.Weekday) bool {
	for _, typicalDay := range pattern.TypicalDaysOfWeek {
		if typicalDay == day {
			return true
		}
	}
	return false
}

func (ra *RiskAssessment) isHighVelocityLogin(pattern *UserBehaviorPattern, timestamp time.Time) bool {
	// Check if login frequency exceeds normal pattern
	// This would involve tracking recent login attempts
	return false // Simplified for now
}

func (ra *RiskAssessment) isUnusualLocation(ipAddress string, commonLocations []*GeoLocation) bool {
	// Compare current IP location with user's typical locations
	// This would involve geo-IP lookup and distance calculation
	return false // Simplified for now
}

func (ra *RiskAssessment) isVPNOrProxy(ipAddress string) bool {
	// Check if IP is from known VPN/proxy providers
	// This would involve checking against VPN/proxy databases
	ip := net.ParseIP(ipAddress)
	if ip == nil {
		return false
	}

	// Simple check for common VPN/proxy indicators
	// In practice, this would use a comprehensive database
	vpnRanges := []string{
		"10.0.0.0/8",     // Private networks (often VPN)
		"172.16.0.0/12",  // Private networks
		"192.168.0.0/16", // Private networks
	}

	for _, cidr := range vpnRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil && network.Contains(ip) {
			return true
		}
	}

	return false
}

func (ra *RiskAssessment) isBlockedCountry(country string) bool {
	for _, blocked := range ra.threatIntel.blockedCountries {
		if strings.EqualFold(country, blocked) {
			return true
		}
	}
	return false
}

// UpdateUserBehavior updates behavioral patterns for a user
func (bt *BehaviorTracker) UpdateUserBehavior(userID string, loginTime time.Time, location *GeoLocation, deviceType DeviceType) {
	pattern, exists := bt.userPatterns[userID]
	if !exists {
		pattern = &UserBehaviorPattern{
			TypicalLoginHours: []int{},
			TypicalDaysOfWeek: []time.Weekday{},
			CommonLocations:   []*GeoLocation{},
			CommonDeviceTypes: []DeviceType{},
		}
		bt.userPatterns[userID] = pattern
	}

	// Update login hour pattern
	hour := loginTime.Hour()
	if !contains(pattern.TypicalLoginHours, hour) {
		pattern.TypicalLoginHours = append(pattern.TypicalLoginHours, hour)
	}

	// Update day of week pattern
	day := loginTime.Weekday()
	if !containsWeekday(pattern.TypicalDaysOfWeek, day) {
		pattern.TypicalDaysOfWeek = append(pattern.TypicalDaysOfWeek, day)
	}

	// Update device type pattern
	if !containsDeviceType(pattern.CommonDeviceTypes, deviceType) {
		pattern.CommonDeviceTypes = append(pattern.CommonDeviceTypes, deviceType)
	}

	// Update location pattern (if location is provided)
	if location != nil {
		// Add location if not already present (simplified)
		pattern.CommonLocations = append(pattern.CommonLocations, location)
	}

	pattern.LastLoginTime = loginTime
}

// AddThreatIntelligence adds threat intelligence data
func (ti *ThreatIntelligence) AddThreatIntelligence(ipAddress string, level ThreatLevel) {
	ti.knownMaliciousIPs[ipAddress] = level
}

// Helper functions
func contains(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsWeekday(slice []time.Weekday, item time.Weekday) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func containsDeviceType(slice []DeviceType, item DeviceType) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
