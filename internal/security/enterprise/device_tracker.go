package enterprise

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// DeviceTracker tracks and manages device registration and recognition
type DeviceTracker struct {
	config             *EnhancedAuthConfig
	knownDevices       map[string]*DeviceInfo // userID -> devices
	deviceFingerprints map[string]*DeviceInfo // fingerprint -> device info
	geoLocation        *GeoLocationService
	mutex              sync.RWMutex
}

// DeviceInfo holds information about a registered device
type DeviceInfo struct {
	Fingerprint string            `json:"fingerprint"`
	UserAgent   string            `json:"user_agent"`
	IPAddress   string            `json:"ip_address"`
	Location    *GeoLocation      `json:"location,omitempty"`
	FirstSeen   time.Time         `json:"first_seen"`
	LastSeen    time.Time         `json:"last_seen"`
	Trusted     bool              `json:"trusted"`
	Approved    bool              `json:"approved"`
	DeviceType  DeviceType        `json:"device_type"`
	OS          string            `json:"os"`
	Browser     string            `json:"browser"`
	Attributes  map[string]string `json:"attributes"`
	RiskScore   float64           `json:"risk_score"`
}

// DeviceType represents the type of device
type DeviceType string

const (
	DeviceTypeDesktop DeviceType = "desktop"
	DeviceTypeMobile  DeviceType = "mobile"
	DeviceTypeTablet  DeviceType = "tablet"
	DeviceTypeServer  DeviceType = "server"
	DeviceTypeUnknown DeviceType = "unknown"
)

// GeoLocation holds geographical location information
type GeoLocation struct {
	Country   string  `json:"country"`
	Region    string  `json:"region"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	ISP       string  `json:"isp"`
	Timezone  string  `json:"timezone"`
}

// GeoLocationService provides IP geolocation services
type GeoLocationService struct {
	// In a real implementation, this would integrate with services like MaxMind GeoIP
	cache map[string]*GeoLocation
	mutex sync.RWMutex
}

// NewDeviceTracker creates a new device tracker
func NewDeviceTracker(config *EnhancedAuthConfig) *DeviceTracker {
	return &DeviceTracker{
		config:             config,
		knownDevices:       make(map[string]*DeviceInfo),
		deviceFingerprints: make(map[string]*DeviceInfo),
		geoLocation:        NewGeoLocationService(),
	}
}

// NewGeoLocationService creates a new geo-location service
func NewGeoLocationService() *GeoLocationService {
	return &GeoLocationService{
		cache: make(map[string]*GeoLocation),
	}
}

// GetOrCreateDevice gets or creates device information based on user agent and IP
func (dt *DeviceTracker) GetOrCreateDevice(userAgent, ipAddress string) *DeviceInfo {
	dt.mutex.Lock()
	defer dt.mutex.Unlock()

	fingerprint := dt.generateDeviceFingerprint(userAgent, ipAddress)

	if device, exists := dt.deviceFingerprints[fingerprint]; exists {
		device.LastSeen = time.Now()
		device.IPAddress = ipAddress // Update IP address
		return device
	}

	// Create new device info
	device := &DeviceInfo{
		Fingerprint: fingerprint,
		UserAgent:   userAgent,
		IPAddress:   ipAddress,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Trusted:     false,
		Approved:    false,
		Attributes:  make(map[string]string),
	}

	// Parse device information from user agent
	dt.parseDeviceInfo(device)

	// Get geo-location information
	device.Location = dt.geoLocation.GetLocation(ipAddress)

	// Calculate initial risk score
	device.RiskScore = dt.calculateRiskScore(device)

	dt.deviceFingerprints[fingerprint] = device

	return device
}

// IsKnownDevice checks if a device is known for a specific user
func (dt *DeviceTracker) IsKnownDevice(userID, fingerprint string) bool {
	dt.mutex.RLock()
	defer dt.mutex.RUnlock()

	userDevices, exists := dt.knownDevices[userID]
	if !exists {
		return false
	}

	return userDevices.Fingerprint == fingerprint
}

// RegisterDevice registers a device for a user
func (dt *DeviceTracker) RegisterDevice(userID string, device *DeviceInfo) {
	dt.mutex.Lock()
	defer dt.mutex.Unlock()

	device.Approved = true
	dt.knownDevices[userID] = device
}

// ApproveDevice approves a device for a user (admin function)
func (dt *DeviceTracker) ApproveDevice(userID, fingerprint string) error {
	dt.mutex.Lock()
	defer dt.mutex.Unlock()

	device, exists := dt.deviceFingerprints[fingerprint]
	if !exists {
		return fmt.Errorf("device with fingerprint %s not found", fingerprint)
	}

	device.Approved = true
	device.Trusted = true
	dt.knownDevices[userID] = device

	return nil
}

// GetUserDevices returns all registered devices for a user
func (dt *DeviceTracker) GetUserDevices(userID string) []*DeviceInfo {
	dt.mutex.RLock()
	defer dt.mutex.RUnlock()

	var devices []*DeviceInfo
	if device, exists := dt.knownDevices[userID]; exists {
		devices = append(devices, device)
	}

	return devices
}

// RevokeDevice revokes trust for a device
func (dt *DeviceTracker) RevokeDevice(userID, fingerprint string) error {
	dt.mutex.Lock()
	defer dt.mutex.Unlock()

	device, exists := dt.knownDevices[userID]
	if !exists || device.Fingerprint != fingerprint {
		return fmt.Errorf("device not found for user %s", userID)
	}

	device.Trusted = false
	device.Approved = false

	return nil
}

// generateDeviceFingerprint creates a unique fingerprint for a device
func (dt *DeviceTracker) generateDeviceFingerprint(userAgent, ipAddress string) string {
	// Create a hash based on user agent and IP network (not exact IP for privacy)
	ip := net.ParseIP(ipAddress)
	var networkBase string

	if ip != nil {
		if ip.To4() != nil {
			// IPv4: Use /24 network
			mask := net.CIDRMask(24, 32)
			network := ip.Mask(mask)
			networkBase = network.String()
		} else {
			// IPv6: Use /64 network
			mask := net.CIDRMask(64, 128)
			network := ip.Mask(mask)
			networkBase = network.String()
		}
	} else {
		networkBase = ipAddress
	}

	// Combine user agent and network base
	combined := fmt.Sprintf("%s|%s", userAgent, networkBase)

	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars for readability
}

// parseDeviceInfo extracts device information from user agent
func (dt *DeviceTracker) parseDeviceInfo(device *DeviceInfo) {
	userAgent := strings.ToLower(device.UserAgent)

	// Detect device type
	if strings.Contains(userAgent, "mobile") || strings.Contains(userAgent, "android") || strings.Contains(userAgent, "iphone") {
		device.DeviceType = DeviceTypeMobile
	} else if strings.Contains(userAgent, "tablet") || strings.Contains(userAgent, "ipad") {
		device.DeviceType = DeviceTypeTablet
	} else if strings.Contains(userAgent, "curl") || strings.Contains(userAgent, "wget") || strings.Contains(userAgent, "bot") {
		device.DeviceType = DeviceTypeServer
	} else {
		device.DeviceType = DeviceTypeDesktop
	}

	// Detect OS
	switch {
	case strings.Contains(userAgent, "windows"):
		device.OS = "Windows"
	case strings.Contains(userAgent, "mac os") || strings.Contains(userAgent, "macos"):
		device.OS = "macOS"
	case strings.Contains(userAgent, "linux"):
		device.OS = "Linux"
	case strings.Contains(userAgent, "android"):
		device.OS = "Android"
	case strings.Contains(userAgent, "ios") || strings.Contains(userAgent, "iphone") || strings.Contains(userAgent, "ipad"):
		device.OS = "iOS"
	default:
		device.OS = "Unknown"
	}

	// Detect browser
	switch {
	case strings.Contains(userAgent, "chrome"):
		device.Browser = "Chrome"
	case strings.Contains(userAgent, "firefox"):
		device.Browser = "Firefox"
	case strings.Contains(userAgent, "safari") && !strings.Contains(userAgent, "chrome"):
		device.Browser = "Safari"
	case strings.Contains(userAgent, "edge"):
		device.Browser = "Edge"
	case strings.Contains(userAgent, "curl"):
		device.Browser = "curl"
	case strings.Contains(userAgent, "wget"):
		device.Browser = "wget"
	default:
		device.Browser = "Unknown"
	}

	// Store raw user agent for detailed analysis
	device.Attributes["raw_user_agent"] = device.UserAgent
}

// calculateRiskScore calculates a risk score for the device
func (dt *DeviceTracker) calculateRiskScore(device *DeviceInfo) float64 {
	score := 0.0

	// Base score for new devices
	score += 0.3

	// Increase score for server-type devices (automated tools)
	if device.DeviceType == DeviceTypeServer {
		score += 0.4
	}

	// Increase score for unknown OS/Browser
	if device.OS == "Unknown" {
		score += 0.2
	}
	if device.Browser == "Unknown" {
		score += 0.2
	}

	// Geo-location based scoring
	if device.Location != nil {
		// Increase score for certain high-risk regions (this would be configurable)
		riskCountries := []string{"tor", "proxy", "vpn"} // Simplified example
		for _, riskCountry := range riskCountries {
			if strings.Contains(strings.ToLower(device.Location.Country), riskCountry) {
				score += 0.3
				break
			}
		}
	}

	// Cap the score at 1.0
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// GetLocation provides geo-location information for an IP address
func (gls *GeoLocationService) GetLocation(ipAddress string) *GeoLocation {
	gls.mutex.Lock()
	defer gls.mutex.Unlock()

	// Check cache first
	if location, exists := gls.cache[ipAddress]; exists {
		return location
	}

	// In a real implementation, this would call an external service
	// For now, return a mock location based on IP analysis
	location := &GeoLocation{
		Country:  "Unknown",
		Region:   "Unknown",
		City:     "Unknown",
		ISP:      "Unknown",
		Timezone: "UTC",
	}

	// Simple IP-based location detection (very basic)
	ip := net.ParseIP(ipAddress)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() {
			location.Country = "Local"
			location.Region = "Private Network"
			location.City = "Local"
		} else {
			// This would typically involve calling a geolocation API
			location.Country = "External"
			location.Region = "Internet"
			location.City = "Remote"
		}
	}

	// Cache the result
	gls.cache[ipAddress] = location

	return location
}

// GetSuspiciousDevices returns devices with high risk scores
func (dt *DeviceTracker) GetSuspiciousDevices(threshold float64) []*DeviceInfo {
	dt.mutex.RLock()
	defer dt.mutex.RUnlock()

	var suspicious []*DeviceInfo
	for _, device := range dt.deviceFingerprints {
		if device.RiskScore >= threshold && !device.Trusted {
			suspicious = append(suspicious, device)
		}
	}

	return suspicious
}

// UpdateDeviceRiskScore updates the risk score for a device
func (dt *DeviceTracker) UpdateDeviceRiskScore(fingerprint string, newScore float64) error {
	dt.mutex.Lock()
	defer dt.mutex.Unlock()

	device, exists := dt.deviceFingerprints[fingerprint]
	if !exists {
		return fmt.Errorf("device with fingerprint %s not found", fingerprint)
	}

	device.RiskScore = newScore
	return nil
}

// CleanupOldDevices removes device records older than the specified duration
func (dt *DeviceTracker) CleanupOldDevices(maxAge time.Duration) int {
	dt.mutex.Lock()
	defer dt.mutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	cleaned := 0

	for fingerprint, device := range dt.deviceFingerprints {
		if device.LastSeen.Before(cutoff) && !device.Trusted {
			delete(dt.deviceFingerprints, fingerprint)
			cleaned++
		}
	}

	return cleaned
}
