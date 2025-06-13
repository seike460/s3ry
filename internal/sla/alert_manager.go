package sla

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SimpleSLAAlertManager implements AlertManager interface
type SimpleSLAAlertManager struct {
	config       *AlertManagerConfig
	alerts       map[string]SLAAlert
	alertHistory []SLAAlert
	channels     map[ChannelType]AlertChannel
	storage      AlertStorage
	mutex        sync.RWMutex
}

// AlertManagerConfig holds alert manager configuration
type AlertManagerConfig struct {
	Enabled          bool          `json:"enabled"`
	ConfigDir        string        `json:"config_dir"`
	HistoryRetention time.Duration `json:"history_retention"`
	MaxHistorySize   int           `json:"max_history_size"`
	DefaultChannels  []ChannelType `json:"default_channels"`
	RateLimiting     RateLimitConfig `json:"rate_limiting"`
}

// DefaultAlertManagerConfig returns default alert manager configuration
func DefaultAlertManagerConfig() *AlertManagerConfig {
	return &AlertManagerConfig{
		Enabled:          true,
		ConfigDir:        "config/sla/alerts",
		HistoryRetention: time.Hour * 24 * 30, // 30 days
		MaxHistorySize:   10000,
		DefaultChannels:  []ChannelType{ChannelTypeEmail},
		RateLimiting: RateLimitConfig{
			Enabled:        true,
			MaxPerMinute:   10,
			MaxPerHour:     60,
			CooldownPeriod: time.Minute * 5,
		},
	}
}

// RateLimitConfig defines rate limiting for alerts
type RateLimitConfig struct {
	Enabled        bool          `json:"enabled"`
	MaxPerMinute   int           `json:"max_per_minute"`
	MaxPerHour     int           `json:"max_per_hour"`
	CooldownPeriod time.Duration `json:"cooldown_period"`
}

// AlertStorage interface for storing alert data
type AlertStorage interface {
	SaveAlert(alert SLAAlert) error
	LoadAlert(id string) (*SLAAlert, error)
	ListAlerts(filters map[string]interface{}) ([]SLAAlert, error)
	DeleteAlert(id string) error
	Cleanup(retentionPeriod time.Duration) error
}

// FileAlertStorage implements AlertStorage using files
type FileAlertStorage struct {
	configDir string
}

// NewFileAlertStorage creates a file-based alert storage
func NewFileAlertStorage(configDir string) *FileAlertStorage {
	os.MkdirAll(configDir, 0755)
	return &FileAlertStorage{configDir: configDir}
}

// SaveAlert saves an alert to file
func (f *FileAlertStorage) SaveAlert(alert SLAAlert) error {
	data, err := json.MarshalIndent(alert, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	filename := filepath.Join(f.configDir, fmt.Sprintf("%s.json", alert.ID))
	return os.WriteFile(filename, data, 0644)
}

// LoadAlert loads an alert from file
func (f *FileAlertStorage) LoadAlert(id string) (*SLAAlert, error) {
	filename := filepath.Join(f.configDir, fmt.Sprintf("%s.json", id))
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read alert file: %w", err)
	}

	var alert SLAAlert
	if err := json.Unmarshal(data, &alert); err != nil {
		return nil, fmt.Errorf("failed to unmarshal alert: %w", err)
	}

	return &alert, nil
}

// ListAlerts lists alerts with filters
func (f *FileAlertStorage) ListAlerts(filters map[string]interface{}) ([]SLAAlert, error) {
	entries, err := os.ReadDir(f.configDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []SLAAlert{}, nil
		}
		return nil, fmt.Errorf("failed to read alert directory: %w", err)
	}

	var alerts []SLAAlert
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		id := entry.Name()[:len(entry.Name())-5] // Remove .json extension
		alert, err := f.LoadAlert(id)
		if err != nil {
			fmt.Printf("Failed to load alert %s: %v\n", id, err)
			continue
		}

		alerts = append(alerts, *alert)
	}

	return alerts, nil
}

// DeleteAlert deletes an alert
func (f *FileAlertStorage) DeleteAlert(id string) error {
	filename := filepath.Join(f.configDir, fmt.Sprintf("%s.json", id))
	return os.Remove(filename)
}

// Cleanup removes old alerts based on retention period
func (f *FileAlertStorage) Cleanup(retentionPeriod time.Duration) error {
	entries, err := os.ReadDir(f.configDir)
	if err != nil {
		return nil // Directory might not exist
	}

	cutoff := time.Now().Add(-retentionPeriod)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(f.configDir, entry.Name()))
		}
	}

	return nil
}

// NewSimpleSLAAlertManager creates a new simple SLA alert manager
func NewSimpleSLAAlertManager(config *AlertManagerConfig) (*SimpleSLAAlertManager, error) {
	if config == nil {
		config = DefaultAlertManagerConfig()
	}

	storage := NewFileAlertStorage(config.ConfigDir)

	manager := &SimpleSLAAlertManager{
		config:       config,
		alerts:       make(map[string]SLAAlert),
		alertHistory: make([]SLAAlert, 0),
		channels:     make(map[ChannelType]AlertChannel),
		storage:      storage,
	}

	// Initialize default channels
	manager.initializeChannels()

	return manager, nil
}

// initializeChannels initializes default alert channels
func (s *SimpleSLAAlertManager) initializeChannels() {
	// Email channel
	s.channels[ChannelTypeEmail] = AlertChannel{
		Type:    ChannelTypeEmail,
		Enabled: true,
		Config: map[string]string{
			"smtp_server": "localhost",
			"port":        "587",
			"from_email":  "alerts@s3ry.com",
		},
		Severity: []Severity{SeverityCritical, SeverityError, SeverityWarning},
	}

	// Slack channel (example configuration)
	s.channels[ChannelTypeSlack] = AlertChannel{
		Type:    ChannelTypeSlack,
		Enabled: false, // Disabled by default
		Config: map[string]string{
			"webhook_url": "https://hooks.slack.com/services/...",
			"channel":     "#alerts",
			"username":    "S3ry-Bot",
		},
		Severity: []Severity{SeverityCritical, SeverityError},
	}

	// Webhook channel
	s.channels[ChannelTypeWebhook] = AlertChannel{
		Type:    ChannelTypeWebhook,
		Enabled: false, // Disabled by default
		Config: map[string]string{
			"url":     "https://api.example.com/alerts",
			"method":  "POST",
			"timeout": "30s",
		},
		Severity: []Severity{SeverityCritical, SeverityError, SeverityWarning, SeverityInfo},
	}
}

// SendAlert sends an SLA alert through configured channels
func (s *SimpleSLAAlertManager) SendAlert(alert SLAAlert) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.config.Enabled {
		return fmt.Errorf("alert manager is disabled")
	}

	// Check rate limiting
	if s.config.RateLimiting.Enabled {
		if s.isRateLimited(alert) {
			fmt.Printf("Alert rate limited: %s\n", alert.ID)
			return fmt.Errorf("alert rate limited")
		}
	}

	// Generate unique ID if not provided
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("alert_%d", time.Now().UnixNano())
	}

	// Set timestamp if not provided
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now()
	}

	// Set default status if not provided
	if alert.Status == "" {
		alert.Status = AlertStatusActive
	}

	// Store alert
	s.alerts[alert.ID] = alert

	// Add to history
	s.alertHistory = append(s.alertHistory, alert)
	if len(s.alertHistory) > s.config.MaxHistorySize {
		s.alertHistory = s.alertHistory[1:]
	}

	// Save to storage
	if err := s.storage.SaveAlert(alert); err != nil {
		fmt.Printf("Failed to save alert to storage: %v\n", err)
	}

	// Send through configured channels
	for _, channelType := range alert.Channels {
		channel, exists := s.channels[channelType]
		if !exists || !channel.Enabled {
			continue
		}

		// Check if channel handles this severity
		if !s.channelHandlesSeverity(channel, alert.Severity) {
			continue
		}

		if err := s.sendThroughChannel(alert, channel); err != nil {
			fmt.Printf("Failed to send alert through %s: %v\n", channelType, err)
		}
	}

	// If no specific channels defined, use default channels
	if len(alert.Channels) == 0 {
		for _, channelType := range s.config.DefaultChannels {
			channel, exists := s.channels[channelType]
			if exists && channel.Enabled && s.channelHandlesSeverity(channel, alert.Severity) {
				if err := s.sendThroughChannel(alert, channel); err != nil {
					fmt.Printf("Failed to send alert through default %s: %v\n", channelType, err)
				}
			}
		}
	}

	fmt.Printf("SLA Alert sent: %s - %s\n", alert.Severity, alert.Title)
	return nil
}

// isRateLimited checks if an alert should be rate limited
func (s *SimpleSLAAlertManager) isRateLimited(alert SLAAlert) bool {
	now := time.Now()
	
	// Count recent alerts
	recentAlerts := 0
	hourlyAlerts := 0
	
	for _, histAlert := range s.alertHistory {
		if histAlert.SLAID == alert.SLAID {
			if now.Sub(histAlert.Timestamp) < time.Minute {
				recentAlerts++
			}
			if now.Sub(histAlert.Timestamp) < time.Hour {
				hourlyAlerts++
			}
		}
	}

	return recentAlerts >= s.config.RateLimiting.MaxPerMinute ||
		   hourlyAlerts >= s.config.RateLimiting.MaxPerHour
}

// channelHandlesSeverity checks if a channel handles the given severity
func (s *SimpleSLAAlertManager) channelHandlesSeverity(channel AlertChannel, severity Severity) bool {
	for _, s := range channel.Severity {
		if s == severity {
			return true
		}
	}
	return false
}

// sendThroughChannel sends an alert through a specific channel
func (s *SimpleSLAAlertManager) sendThroughChannel(alert SLAAlert, channel AlertChannel) error {
	switch channel.Type {
	case ChannelTypeEmail:
		return s.sendEmailAlert(alert, channel)
	case ChannelTypeSlack:
		return s.sendSlackAlert(alert, channel)
	case ChannelTypeWebhook:
		return s.sendWebhookAlert(alert, channel)
	case ChannelTypeSMS:
		return s.sendSMSAlert(alert, channel)
	default:
		return fmt.Errorf("unsupported channel type: %s", channel.Type)
	}
}

// sendEmailAlert sends an alert via email
func (s *SimpleSLAAlertManager) sendEmailAlert(alert SLAAlert, channel AlertChannel) error {
	// In a real implementation, this would use SMTP to send emails
	fmt.Printf("📧 EMAIL ALERT: [%s] %s - %s\n", alert.Severity, alert.Title, alert.Message)
	
	emailBody := s.formatEmailAlert(alert)
	fmt.Printf("Email Body:\n%s\n", emailBody)
	
	return nil // Simulated success
}

// sendSlackAlert sends an alert to Slack
func (s *SimpleSLAAlertManager) sendSlackAlert(alert SLAAlert, channel AlertChannel) error {
	// In a real implementation, this would send to Slack webhook
	emoji := s.getSeverityEmoji(alert.Severity)
	fmt.Printf("💬 SLACK ALERT: %s [%s] %s - %s\n", emoji, alert.Severity, alert.Title, alert.Message)
	
	return nil // Simulated success
}

// sendWebhookAlert sends an alert via webhook
func (s *SimpleSLAAlertManager) sendWebhookAlert(alert SLAAlert, channel AlertChannel) error {
	// In a real implementation, this would make HTTP POST to webhook URL
	webhookPayload := map[string]interface{}{
		"alert_id":   alert.ID,
		"severity":   alert.Severity,
		"title":      alert.Title,
		"message":    alert.Message,
		"timestamp":  alert.Timestamp,
		"sla_id":     alert.SLAID,
		"tags":       alert.Tags,
	}
	
	payload, _ := json.MarshalIndent(webhookPayload, "", "  ")
	fmt.Printf("🔗 WEBHOOK ALERT:\n%s\n", string(payload))
	
	return nil // Simulated success
}

// sendSMSAlert sends an alert via SMS
func (s *SimpleSLAAlertManager) sendSMSAlert(alert SLAAlert, channel AlertChannel) error {
	// In a real implementation, this would use SMS service API
	smsText := fmt.Sprintf("[%s] %s: %s", alert.Severity, alert.Title, alert.Message)
	fmt.Printf("📱 SMS ALERT: %s\n", smsText)
	
	return nil // Simulated success
}

// formatEmailAlert formats an alert for email
func (s *SimpleSLAAlertManager) formatEmailAlert(alert SLAAlert) string {
	return fmt.Sprintf(`
Subject: [SLA Alert] %s - %s

SLA Alert Details:
==================

Alert ID: %s
Severity: %s
SLA ID: %s
Type: %s
Timestamp: %s

Title: %s
Message: %s

Violation ID: %s

Tags:
%s

This is an automated alert from S3ry SLA Monitoring System.
`, alert.Severity, alert.Title,
		alert.ID, alert.Severity, alert.SLAID, alert.Type, alert.Timestamp.Format(time.RFC3339),
		alert.Title, alert.Message, alert.ViolationID, s.formatTags(alert.Tags))
}

// formatTags formats tags for display
func (s *SimpleSLAAlertManager) formatTags(tags map[string]string) string {
	if len(tags) == 0 {
		return "  (none)"
	}
	
	result := ""
	for key, value := range tags {
		result += fmt.Sprintf("  %s: %s\n", key, value)
	}
	return result
}

// getSeverityEmoji returns an emoji for the given severity
func (s *SimpleSLAAlertManager) getSeverityEmoji(severity Severity) string {
	switch severity {
	case SeverityCritical:
		return "🚨"
	case SeverityError:
		return "❌"
	case SeverityWarning:
		return "⚠️"
	case SeverityInfo:
		return "ℹ️"
	default:
		return "📢"
	}
}

// GetActiveAlerts returns all active alerts
func (s *SimpleSLAAlertManager) GetActiveAlerts() ([]SLAAlert, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	alerts := make([]SLAAlert, 0)
	for _, alert := range s.alerts {
		if alert.Status == AlertStatusActive {
			alerts = append(alerts, alert)
		}
	}

	return alerts, nil
}

// AcknowledgeAlert acknowledges an alert
func (s *SimpleSLAAlertManager) AcknowledgeAlert(alertID, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	alert, exists := s.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}

	alert.Status = AlertStatusAcknowledged
	alert.Tags["acknowledged_by"] = userID
	alert.Tags["acknowledged_at"] = time.Now().Format(time.RFC3339)

	s.alerts[alertID] = alert

	// Save updated alert
	if err := s.storage.SaveAlert(alert); err != nil {
		fmt.Printf("Failed to save acknowledged alert: %v\n", err)
	}

	fmt.Printf("Alert %s acknowledged by %s\n", alertID, userID)
	return nil
}

// ResolveAlert resolves an alert
func (s *SimpleSLAAlertManager) ResolveAlert(alertID, userID string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	alert, exists := s.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert %s not found", alertID)
	}

	alert.Status = AlertStatusResolved
	alert.Tags["resolved_by"] = userID
	alert.Tags["resolved_at"] = time.Now().Format(time.RFC3339)

	s.alerts[alertID] = alert

	// Save updated alert
	if err := s.storage.SaveAlert(alert); err != nil {
		fmt.Printf("Failed to save resolved alert: %v\n", err)
	}

	fmt.Printf("Alert %s resolved by %s\n", alertID, userID)
	return nil
}

// GetAlertHistory returns alert history with optional filters
func (s *SimpleSLAAlertManager) GetAlertHistory(filters map[string]interface{}) ([]SLAAlert, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if filters == nil {
		return append([]SLAAlert{}, s.alertHistory...), nil
	}

	filtered := make([]SLAAlert, 0)
	for _, alert := range s.alertHistory {
		if s.matchesAlertFilters(alert, filters) {
			filtered = append(filtered, alert)
		}
	}

	return filtered, nil
}

// matchesAlertFilters checks if an alert matches the given filters
func (s *SimpleSLAAlertManager) matchesAlertFilters(alert SLAAlert, filters map[string]interface{}) bool {
	if slaID, exists := filters["sla_id"]; exists {
		if alert.SLAID != slaID.(string) {
			return false
		}
	}

	if severity, exists := filters["severity"]; exists {
		if alert.Severity != Severity(severity.(string)) {
			return false
		}
	}

	if alertType, exists := filters["type"]; exists {
		if alert.Type != AlertType(alertType.(string)) {
			return false
		}
	}

	if status, exists := filters["status"]; exists {
		if alert.Status != AlertStatus(status.(string)) {
			return false
		}
	}

	return true
}

// GetAlertStatistics returns alert statistics
func (s *SimpleSLAAlertManager) GetAlertStatistics() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_alerts":    len(s.alertHistory),
		"active_alerts":   0,
		"resolved_alerts": 0,
		"by_severity": map[string]int{
			"critical": 0,
			"error":    0,
			"warning":  0,
			"info":     0,
		},
		"by_type": map[string]int{
			"violation": 0,
			"warning":   0,
			"recovery":  0,
			"flapping":  0,
		},
	}

	for _, alert := range s.alertHistory {
		switch alert.Status {
		case AlertStatusActive:
			stats["active_alerts"] = stats["active_alerts"].(int) + 1
		case AlertStatusResolved:
			stats["resolved_alerts"] = stats["resolved_alerts"].(int) + 1
		}

		severityMap := stats["by_severity"].(map[string]int)
		switch alert.Severity {
		case SeverityCritical:
			severityMap["critical"]++
		case SeverityError:
			severityMap["error"]++
		case SeverityWarning:
			severityMap["warning"]++
		case SeverityInfo:
			severityMap["info"]++
		}

		typeMap := stats["by_type"].(map[string]int)
		switch alert.Type {
		case AlertTypeViolation:
			typeMap["violation"]++
		case AlertTypeWarning:
			typeMap["warning"]++
		case AlertTypeRecovery:
			typeMap["recovery"]++
		case AlertTypeFlapping:
			typeMap["flapping"]++
		}
	}

	return stats
}

// ConfigureChannel configures an alert channel
func (s *SimpleSLAAlertManager) ConfigureChannel(channelType ChannelType, config map[string]string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	channel, exists := s.channels[channelType]
	if !exists {
		channel = AlertChannel{
			Type:     channelType,
			Enabled:  true,
			Config:   make(map[string]string),
			Severity: []Severity{SeverityCritical, SeverityError, SeverityWarning},
		}
	}

	for key, value := range config {
		channel.Config[key] = value
	}

	s.channels[channelType] = channel
	fmt.Printf("Configured %s channel\n", channelType)
	return nil
}

// EnableChannel enables an alert channel
func (s *SimpleSLAAlertManager) EnableChannel(channelType ChannelType) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	channel, exists := s.channels[channelType]
	if !exists {
		return fmt.Errorf("channel %s not found", channelType)
	}

	channel.Enabled = true
	s.channels[channelType] = channel
	fmt.Printf("Enabled %s channel\n", channelType)
	return nil
}

// DisableChannel disables an alert channel
func (s *SimpleSLAAlertManager) DisableChannel(channelType ChannelType) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	channel, exists := s.channels[channelType]
	if !exists {
		return fmt.Errorf("channel %s not found", channelType)
	}

	channel.Enabled = false
	s.channels[channelType] = channel
	fmt.Printf("Disabled %s channel\n", channelType)
	return nil
}

// Cleanup performs periodic cleanup of old alerts
func (s *SimpleSLAAlertManager) Cleanup() error {
	// Clean storage
	if err := s.storage.Cleanup(s.config.HistoryRetention); err != nil {
		return fmt.Errorf("failed to cleanup alert storage: %w", err)
	}

	// Clean in-memory history
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cutoff := time.Now().Add(-s.config.HistoryRetention)
	filtered := make([]SLAAlert, 0)
	
	for _, alert := range s.alertHistory {
		if alert.Timestamp.After(cutoff) {
			filtered = append(filtered, alert)
		}
	}
	
	s.alertHistory = filtered
	
	fmt.Printf("Alert cleanup completed\n")
	return nil
}