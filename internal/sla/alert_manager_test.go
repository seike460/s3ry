package sla

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimpleSLAAlertManager_SendAlert(t *testing.T) {
	config := DefaultAlertManagerConfig()
	config.RateLimiting.Enabled = false // Disable for testing

	manager, err := NewSimpleSLAAlertManager(config)
	require.NoError(t, err)

	alert := SLAAlert{
		Type:        AlertTypeViolation,
		Severity:    SeverityCritical,
		SLAID:       "test-sla",
		ViolationID: "test-violation",
		Title:       "Test Alert",
		Message:     "This is a test alert",
		Channels:    []ChannelType{ChannelTypeEmail},
		Tags:        map[string]string{"test": "true"},
	}

	err = manager.SendAlert(alert)
	assert.NoError(t, err)

	// Check alert was stored - we need to get all active alerts since ID was generated
	activeAlerts, err := manager.GetActiveAlerts()
	assert.NoError(t, err)
	assert.Len(t, activeAlerts, 1)

	storedAlert := activeAlerts[0]
	assert.NotEmpty(t, storedAlert.ID)
	assert.False(t, storedAlert.Timestamp.IsZero())
	assert.Equal(t, AlertStatusActive, storedAlert.Status)
	assert.Equal(t, alert.Title, storedAlert.Title)
}

func TestSimpleSLAAlertManager_RateLimiting(t *testing.T) {
	config := DefaultAlertManagerConfig()
	config.RateLimiting.Enabled = true
	config.RateLimiting.MaxPerMinute = 2

	manager, err := NewSimpleSLAAlertManager(config)
	require.NoError(t, err)

	alert := SLAAlert{
		Type:        AlertTypeViolation,
		Severity:    SeverityCritical,
		SLAID:       "test-sla",
		ViolationID: "test-violation",
		Title:       "Test Alert",
		Message:     "This is a test alert",
		Channels:    []ChannelType{ChannelTypeEmail},
	}

	// Send first two alerts - should succeed
	err = manager.SendAlert(alert)
	assert.NoError(t, err)

	alert.ID = "" // Reset ID for new alert
	err = manager.SendAlert(alert)
	assert.NoError(t, err)

	// Third alert should be rate limited
	alert.ID = "" // Reset ID for new alert
	err = manager.SendAlert(alert)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestSimpleSLAAlertManager_AcknowledgeAlert(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	alert := SLAAlert{
		ID:       "test-alert",
		Type:     AlertTypeViolation,
		Severity: SeverityCritical,
		SLAID:    "test-sla",
		Title:    "Test Alert",
		Message:  "This is a test alert",
		Status:   AlertStatusActive,
		Tags:     make(map[string]string),
	}

	// Store alert first
	manager.alerts[alert.ID] = alert

	// Acknowledge the alert
	err = manager.AcknowledgeAlert(alert.ID, "test-user")
	assert.NoError(t, err)

	// Check alert status changed
	acknowledgedAlert := manager.alerts[alert.ID]
	assert.Equal(t, AlertStatusAcknowledged, acknowledgedAlert.Status)
	assert.Equal(t, "test-user", acknowledgedAlert.Tags["acknowledged_by"])
	assert.NotEmpty(t, acknowledgedAlert.Tags["acknowledged_at"])
}

func TestSimpleSLAAlertManager_ResolveAlert(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	alert := SLAAlert{
		ID:       "test-alert",
		Type:     AlertTypeViolation,
		Severity: SeverityCritical,
		SLAID:    "test-sla",
		Title:    "Test Alert",
		Message:  "This is a test alert",
		Status:   AlertStatusActive,
		Tags:     make(map[string]string),
	}

	// Store alert first
	manager.alerts[alert.ID] = alert

	// Resolve the alert
	err = manager.ResolveAlert(alert.ID, "test-user")
	assert.NoError(t, err)

	// Check alert status changed
	resolvedAlert := manager.alerts[alert.ID]
	assert.Equal(t, AlertStatusResolved, resolvedAlert.Status)
	assert.Equal(t, "test-user", resolvedAlert.Tags["resolved_by"])
	assert.NotEmpty(t, resolvedAlert.Tags["resolved_at"])
}

func TestSimpleSLAAlertManager_ChannelConfiguration(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	// Test configuring a channel
	config := map[string]string{
		"webhook_url": "https://hooks.slack.com/test",
		"channel":     "#alerts",
	}

	err = manager.ConfigureChannel(ChannelTypeSlack, config)
	assert.NoError(t, err)

	// Check channel was configured
	channel := manager.channels[ChannelTypeSlack]
	assert.Equal(t, "https://hooks.slack.com/test", channel.Config["webhook_url"])
	assert.Equal(t, "#alerts", channel.Config["channel"])

	// Test enabling/disabling channels
	err = manager.EnableChannel(ChannelTypeSlack)
	assert.NoError(t, err)
	assert.True(t, manager.channels[ChannelTypeSlack].Enabled)

	err = manager.DisableChannel(ChannelTypeSlack)
	assert.NoError(t, err)
	assert.False(t, manager.channels[ChannelTypeSlack].Enabled)
}

func TestSimpleSLAAlertManager_ChannelSeverityHandling(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	// Create a channel that only handles critical and error alerts
	channel := AlertChannel{
		Type:     ChannelTypeEmail,
		Enabled:  true,
		Severity: []Severity{SeverityCritical, SeverityError},
	}

	// Test severity matching
	assert.True(t, manager.channelHandlesSeverity(channel, SeverityCritical))
	assert.True(t, manager.channelHandlesSeverity(channel, SeverityError))
	assert.False(t, manager.channelHandlesSeverity(channel, SeverityWarning))
	assert.False(t, manager.channelHandlesSeverity(channel, SeverityInfo))
}

func TestSimpleSLAAlertManager_AlertHistory(t *testing.T) {
	config := DefaultAlertManagerConfig()
	config.MaxHistorySize = 3 // Small size for testing

	manager, err := NewSimpleSLAAlertManager(config)
	require.NoError(t, err)

	// Send multiple alerts
	for i := 0; i < 5; i++ {
		alert := SLAAlert{
			Type:     AlertTypeViolation,
			Severity: SeverityCritical,
			SLAID:    "test-sla",
			Title:    "Test Alert",
			Message:  "This is a test alert",
		}

		err = manager.SendAlert(alert)
		assert.NoError(t, err)
	}

	// Check history size is limited
	history, err := manager.GetAlertHistory(nil)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(history)) // Should be limited to MaxHistorySize

	// Test filtering history
	filters := map[string]interface{}{
		"sla_id": "test-sla",
	}
	filteredHistory, err := manager.GetAlertHistory(filters)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(filteredHistory))

	// Test filtering by severity
	filters = map[string]interface{}{
		"severity": string(SeverityWarning),
	}
	filteredHistory, err = manager.GetAlertHistory(filters)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(filteredHistory)) // No warning alerts sent
}

func TestSimpleSLAAlertManager_AlertStatistics(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	// Send various types of alerts
	alerts := []SLAAlert{
		{Type: AlertTypeViolation, Severity: SeverityCritical, Status: AlertStatusActive},
		{Type: AlertTypeViolation, Severity: SeverityError, Status: AlertStatusResolved},
		{Type: AlertTypeWarning, Severity: SeverityWarning, Status: AlertStatusActive},
		{Type: AlertTypeRecovery, Severity: SeverityInfo, Status: AlertStatusResolved},
	}

	for _, alert := range alerts {
		alert.SLAID = "test-sla"
		alert.Title = "Test Alert"
		alert.Message = "Test message"
		err = manager.SendAlert(alert)
		assert.NoError(t, err)
	}

	// Get statistics
	stats := manager.GetAlertStatistics()

	assert.Equal(t, 4, stats["total_alerts"])
	assert.Equal(t, 2, stats["active_alerts"])
	assert.Equal(t, 2, stats["resolved_alerts"])

	severityStats := stats["by_severity"].(map[string]int)
	assert.Equal(t, 1, severityStats["critical"])
	assert.Equal(t, 1, severityStats["error"])
	assert.Equal(t, 1, severityStats["warning"])
	assert.Equal(t, 1, severityStats["info"])

	typeStats := stats["by_type"].(map[string]int)
	assert.Equal(t, 2, typeStats["violation"])
	assert.Equal(t, 1, typeStats["warning"])
	assert.Equal(t, 1, typeStats["recovery"])
}

func TestSimpleSLAAlertManager_EmailFormatting(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	alert := SLAAlert{
		ID:          "test-alert-123",
		Type:        AlertTypeViolation,
		Severity:    SeverityCritical,
		SLAID:       "test-sla-456",
		ViolationID: "violation-789",
		Title:       "Critical Service Down",
		Message:     "Service is experiencing downtime",
		Timestamp:   time.Now(),
		Tags: map[string]string{
			"service": "s3",
			"region":  "us-east-1",
		},
	}

	emailBody := manager.formatEmailAlert(alert)

	// Check that key information is included
	assert.Contains(t, emailBody, "Critical Service Down")
	assert.Contains(t, emailBody, "CRITICAL")
	assert.Contains(t, emailBody, "test-alert-123")
	assert.Contains(t, emailBody, "test-sla-456")
	assert.Contains(t, emailBody, "violation-789")
	assert.Contains(t, emailBody, "Service is experiencing downtime")
	assert.Contains(t, emailBody, "service: s3")
	assert.Contains(t, emailBody, "region: us-east-1")
}

func TestSimpleSLAAlertManager_SeverityEmoji(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	testCases := []struct {
		severity      Severity
		expectedEmoji string
	}{
		{SeverityCritical, "🚨"},
		{SeverityError, "❌"},
		{SeverityWarning, "⚠️"},
		{SeverityInfo, "ℹ️"},
	}

	for _, tc := range testCases {
		emoji := manager.getSeverityEmoji(tc.severity)
		assert.Equal(t, tc.expectedEmoji, emoji)
	}
}

func TestSimpleSLAAlertManager_TagFormatting(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	// Test with tags
	tags := map[string]string{
		"service": "s3",
		"region":  "us-east-1",
		"env":     "production",
	}
	formatted := manager.formatTags(tags)
	assert.Contains(t, formatted, "service: s3")
	assert.Contains(t, formatted, "region: us-east-1")
	assert.Contains(t, formatted, "env: production")

	// Test with no tags
	emptyFormatted := manager.formatTags(map[string]string{})
	assert.Equal(t, "  (none)", emptyFormatted)
}

func TestSimpleSLAAlertManager_AlertFiltering(t *testing.T) {
	manager, err := NewSimpleSLAAlertManager(nil)
	require.NoError(t, err)

	alert1 := SLAAlert{
		SLAID:    "sla-1",
		Severity: SeverityCritical,
		Type:     AlertTypeViolation,
		Status:   AlertStatusActive,
	}

	alert2 := SLAAlert{
		SLAID:    "sla-2",
		Severity: SeverityWarning,
		Type:     AlertTypeWarning,
		Status:   AlertStatusResolved,
	}

	// Test SLA ID filter
	filters := map[string]interface{}{"sla_id": "sla-1"}
	assert.True(t, manager.matchesAlertFilters(alert1, filters))
	assert.False(t, manager.matchesAlertFilters(alert2, filters))

	// Test severity filter
	filters = map[string]interface{}{"severity": string(SeverityWarning)}
	assert.False(t, manager.matchesAlertFilters(alert1, filters))
	assert.True(t, manager.matchesAlertFilters(alert2, filters))

	// Test type filter
	filters = map[string]interface{}{"type": string(AlertTypeViolation)}
	assert.True(t, manager.matchesAlertFilters(alert1, filters))
	assert.False(t, manager.matchesAlertFilters(alert2, filters))

	// Test status filter
	filters = map[string]interface{}{"status": string(AlertStatusActive)}
	assert.True(t, manager.matchesAlertFilters(alert1, filters))
	assert.False(t, manager.matchesAlertFilters(alert2, filters))

	// Test nil filters (should match all)
	assert.True(t, manager.matchesAlertFilters(alert1, nil))
	assert.True(t, manager.matchesAlertFilters(alert2, nil))
}

func TestFileAlertStorage_SaveAndLoad(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	storage := NewFileAlertStorage(tempDir)

	alert := SLAAlert{
		ID:        "test-alert",
		Type:      AlertTypeViolation,
		Severity:  SeverityCritical,
		SLAID:     "test-sla",
		Title:     "Test Alert",
		Message:   "Test message",
		Timestamp: time.Now(),
		Status:    AlertStatusActive,
	}

	// Test save
	err := storage.SaveAlert(alert)
	assert.NoError(t, err)

	// Test load
	loadedAlert, err := storage.LoadAlert(alert.ID)
	assert.NoError(t, err)
	assert.Equal(t, alert.ID, loadedAlert.ID)
	assert.Equal(t, alert.Title, loadedAlert.Title)
	assert.Equal(t, alert.Severity, loadedAlert.Severity)

	// Test list
	alerts, err := storage.ListAlerts(nil)
	assert.NoError(t, err)
	assert.Len(t, alerts, 1)
	assert.Equal(t, alert.ID, alerts[0].ID)

	// Test delete
	err = storage.DeleteAlert(alert.ID)
	assert.NoError(t, err)

	// Verify deletion
	_, err = storage.LoadAlert(alert.ID)
	assert.Error(t, err)
}
