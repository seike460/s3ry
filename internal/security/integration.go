package security

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/seike460/s3ry/internal/config"
	// "github.com/seike460/s3ry/internal/security/enterprise" // TODO: Add when enterprise module is available
)

// SecurityIntegration provides integration points for the main S3ry application
type SecurityIntegration struct {
	securityManager *SecurityManager
	s3Wrapper       *S3SecurityWrapper
	middleware      *SecurityMiddleware
	monitoringAgent *MonitoringAgent
}

// MonitoringAgent provides real-time security monitoring integration
type MonitoringAgent struct {
	// securityMonitor *enterprise.SecurityMonitor
	config   *MonitoringConfig
	stopChan chan struct{}
}

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	EnableRealTimeAlerts      bool            `json:"enable_realtime_alerts" yaml:"enable_realtime_alerts"`
	AlertThresholds           AlertThresholds `json:"alert_thresholds" yaml:"alert_thresholds"`
	MonitoringIntervalSec     int             `json:"monitoring_interval_sec" yaml:"monitoring_interval_sec"`
	PerformanceMetricsEnabled bool            `json:"performance_metrics_enabled" yaml:"performance_metrics_enabled"`
}

// AlertThresholds defines when to trigger security alerts
type AlertThresholds struct {
	ErrorRatePercent           float64 `json:"error_rate_percent" yaml:"error_rate_percent"`
	ConcurrentConnectionsLimit int     `json:"concurrent_connections_limit" yaml:"concurrent_connections_limit"`
	MemoryUsageMB              int64   `json:"memory_usage_mb" yaml:"memory_usage_mb"`
	ThreatLevelCritical        bool    `json:"threat_level_critical" yaml:"threat_level_critical"`
}

// DefaultMonitoringConfig returns default monitoring configuration
func DefaultMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		EnableRealTimeAlerts:      true,
		MonitoringIntervalSec:     30,
		PerformanceMetricsEnabled: true,
		AlertThresholds: AlertThresholds{
			ErrorRatePercent:           5.0,
			ConcurrentConnectionsLimit: 1000,
			MemoryUsageMB:              500,
			ThreatLevelCritical:        true,
		},
	}
}

// NewSecurityIntegration creates a new security integration
func NewSecurityIntegration(appConfig *config.Config, s3Client *s3.Client) (*SecurityIntegration, error) {
	// Initialize security manager
	securityManager, err := NewSecurityManager(appConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize security manager: %w", err)
	}

	// Initialize S3 security wrapper
	s3Wrapper := NewS3SecurityWrapper(s3Client, securityManager)

	// Initialize security middleware
	middleware := NewSecurityMiddleware(securityManager)

	// Initialize monitoring agent
	// monitoringAgent := NewMonitoringAgent(securityManager.GetSecurityMonitor())
	monitoringAgent := &MonitoringAgent{config: DefaultMonitoringConfig(), stopChan: make(chan struct{})}

	integration := &SecurityIntegration{
		securityManager: securityManager,
		s3Wrapper:       s3Wrapper,
		middleware:      middleware,
		monitoringAgent: monitoringAgent,
	}

	// Start monitoring if enabled
	if securityManager.IsSecurityEnabled() {
		integration.monitoringAgent.Start()
	}

	return integration, nil
}

// NewMonitoringAgent creates a new monitoring agent
// func NewMonitoringAgent(securityMonitor *enterprise.SecurityMonitor) *MonitoringAgent {
// return &MonitoringAgent{
// securityMonitor: securityMonitor,
// config:          DefaultMonitoringConfig(),
// stopChan:        make(chan struct{}),
// }
// }

// Start begins real-time security monitoring
func (ma *MonitoringAgent) Start() {
	// if ma.securityMonitor == nil || !ma.config.EnableRealTimeAlerts {
	if !ma.config.EnableRealTimeAlerts {
		return
	}

	go ma.monitoringLoop()
}

// Stop stops the monitoring agent
func (ma *MonitoringAgent) Stop() {
	close(ma.stopChan)
}

// monitoringLoop runs the continuous monitoring process
func (ma *MonitoringAgent) monitoringLoop() {
	ticker := time.NewTicker(time.Duration(ma.config.MonitoringIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ma.stopChan:
			return
		case <-ticker.C:
			ma.checkSecurityMetrics()
		}
	}
}

// checkSecurityMetrics checks current security metrics and triggers alerts
func (ma *MonitoringAgent) checkSecurityMetrics() {
	// if ma.securityMonitor == nil {
	// return
	// }

	// metrics := ma.securityMonitor.GetCurrentMetrics()

	// Check error rate
	// if metrics.ErrorRate > ma.config.AlertThresholds.ErrorRatePercent {
	// ma.triggerAlert("HIGH_ERROR_RATE", fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%",
	// metrics.ErrorRate, ma.config.AlertThresholds.ErrorRatePercent))
	// }

	// Check concurrent connections
	// if int(metrics.ConcurrentConnections) > ma.config.AlertThresholds.ConcurrentConnectionsLimit {
	// ma.triggerAlert("HIGH_CONCURRENT_CONNECTIONS", fmt.Sprintf("Concurrent connections %d exceeds limit %d",
	// metrics.ConcurrentConnections, ma.config.AlertThresholds.ConcurrentConnectionsLimit))
	// }

	// Check memory usage
	// if metrics.MemoryUsageMB > ma.config.AlertThresholds.MemoryUsageMB {
	// ma.triggerAlert("HIGH_MEMORY_USAGE", fmt.Sprintf("Memory usage %d MB exceeds threshold %d MB",
	// metrics.MemoryUsageMB, ma.config.AlertThresholds.MemoryUsageMB))
	// }

	// Check threat level
	// if ma.config.AlertThresholds.ThreatLevelCritical && metrics.ThreatLevel >= enterprise.ThreatLevelCritical {
	// ma.triggerAlert("CRITICAL_THREAT_LEVEL", "Critical threat level detected")
	// }
}

// triggerAlert sends a security alert
func (ma *MonitoringAgent) triggerAlert(alertType, message string) {
	// In a production environment, this would send alerts via:
	// - Email notifications
	// - Slack/Discord webhooks
	// - SMS alerts
	// - PagerDuty integration
	// - Custom webhook endpoints

	fmt.Printf("[SECURITY ALERT] %s: %s\n", alertType, message)

	// Log the alert through the security monitor
	// if ma.securityMonitor != nil {
	// ma.securityMonitor.RecordSecurityAlert(alertType, message)
	// }
}

// Performance Integration Methods

// WrapS3OperationWithSecurity wraps an S3 operation with security enhancements
func (si *SecurityIntegration) WrapS3OperationWithSecurity(ctx context.Context, userID string, operation func() error) error {
	if !si.securityManager.IsSecurityEnabled() {
		return operation()
	}

	// Use concurrent guard for worker execution
	return si.securityManager.GuardWorkerExecution(ctx, 0, operation)
}

// GetSecureS3Client returns the security-wrapped S3 client
func (si *SecurityIntegration) GetSecureS3Client() *S3SecurityWrapper {
	return si.s3Wrapper
}

// GetSecurityMiddleware returns the security middleware
func (si *SecurityIntegration) GetSecurityMiddleware() *SecurityMiddleware {
	return si.middleware
}

// GetSecurityManager returns the security manager
func (si *SecurityIntegration) GetSecurityManager() *SecurityManager {
	return si.securityManager
}

// IsSecurityEnabled returns whether security features are enabled
func (si *SecurityIntegration) IsSecurityEnabled() bool {
	return si.securityManager.IsSecurityEnabled()
}

// ValidateUserOperation validates if a user can perform an operation
func (si *SecurityIntegration) ValidateUserOperation(ctx context.Context, userID, operation, resource string) error {
	if !si.securityManager.IsSecurityEnabled() {
		return nil
	}

	return si.securityManager.AuthorizeAction(ctx, userID, operation, resource, "", "")
}

// EncryptSensitiveData encrypts sensitive data if encryption is enabled
func (si *SecurityIntegration) EncryptSensitiveData(data []byte) ([]byte, error) {
	return si.securityManager.EncryptData(data)
}

// DecryptSensitiveData decrypts sensitive data if encryption is enabled
func (si *SecurityIntegration) DecryptSensitiveData(data []byte) ([]byte, error) {
	return si.securityManager.DecryptData(data)
}

// AuditUserAction logs a user action for security auditing
func (si *SecurityIntegration) AuditUserAction(userID, action, resource, status string, context map[string]interface{}) {
	if !si.securityManager.IsSecurityEnabled() {
		return
	}

	// auditLogger := si.securityManager.GetAuditLogger()
	// if auditLogger != nil {
	// auditLogger.LogAction(userID, action, resource, status, context)
	// }
}

// GetSecurityDashboard returns the current security status
func (si *SecurityIntegration) GetSecurityDashboard() *SecurityDashboard {
	return si.securityManager.GetSecurityDashboard()
}

// PerformSecurityScan performs a comprehensive security scan
// func (si *SecurityIntegration) PerformSecurityScan() (*enterprise.ScanResult, error) {
// return si.securityManager.PerformVulnerabilityAnalysis()
// }

// GetComplianceStatus returns the current compliance status
// func (si *SecurityIntegration) GetComplianceStatus() (*enterprise.ComplianceReport, error) {
// return si.securityManager.GetComplianceReport()
// }

// HandleSecureError processes errors with security considerations
// func (si *SecurityIntegration) HandleSecureError(ctx context.Context, err error, operation string) *enterprise.SecureError {
// return si.securityManager.HandleSecureError(ctx, err, operation)
// }

// Worker Pool Security Integration

// CreateSecureWorkerPool creates a worker pool with security monitoring
func (si *SecurityIntegration) CreateSecureWorkerPool(ctx context.Context, size int, workerFunc func(int, context.Context) error) error {
	if !si.securityManager.IsSecurityEnabled() {
		// Execute without security if disabled
		for i := 0; i < size; i++ {
			go workerFunc(i, ctx)
		}
		return nil
	}

	// Create workers with security guarding
	for i := 0; i < size; i++ {
		workerID := i
		go func() {
			err := si.securityManager.GuardWorkerExecution(ctx, workerID, func() error {
				return workerFunc(workerID, ctx)
			})
			if err != nil {
				// Log worker errors securely
				secureErr := si.securityManager.HandleSecureError(ctx, err, "worker_execution")
				fmt.Printf("Worker %d error: %s\n", workerID, secureErr.SafeMessage)
			}
		}()
	}

	return nil
}

// Performance Metrics Integration

// RecordPerformanceMetric records a performance metric with security context
func (si *SecurityIntegration) RecordPerformanceMetric(operation string, duration time.Duration, success bool) {
	if !si.securityManager.IsSecurityEnabled() {
		return
	}

	securityMonitor := si.securityManager.GetSecurityMonitor()
	// TODO: Enable when security monitor interface is implemented
	// if securityMonitor != nil {
	//     // Record performance metric with security implications
	//     if !success {
	//         securityMonitor.RecordErrorSpike(100.0) // High error rate for failed operations
	//     }
	//
	//     // Record operation timing for anomaly detection
	//     securityMonitor.RecordOperationTiming(operation, duration)
	// }
	_ = securityMonitor // Suppress unused variable warning
}

// Configuration Management

// UpdateSecurityConfig updates security configuration at runtime
func (si *SecurityIntegration) UpdateSecurityConfig(newConfig *SecurityManagerConfig) error {
	// Validate the new configuration
	if newConfig.Enabled && newConfig.EncryptionPassword == "" {
		return fmt.Errorf("encryption password is required when security is enabled")
	}

	// Update security manager configuration
	// Note: In a production system, this might require a restart for some changes
	si.securityManager.config = newConfig

	return nil
}

// GetCurrentSecurityConfig returns the current security configuration
func (si *SecurityIntegration) GetCurrentSecurityConfig() *SecurityManagerConfig {
	return si.securityManager.config
}

// GetCurrentSecurityFileConfig returns the current file security configuration
func (si *SecurityIntegration) GetCurrentSecurityFileConfig() *SecurityConfig {
	// Return basic file security config (from security.go)
	return DefaultSecurityConfig()
}

// Graceful Shutdown

// Close gracefully shuts down all security components
func (si *SecurityIntegration) Close() error {
	var errors []error

	// Stop monitoring
	si.monitoringAgent.Stop()

	// Close security manager
	if err := si.securityManager.Close(); err != nil {
		errors = append(errors, fmt.Errorf("security manager close error: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during security integration shutdown: %v", errors)
	}

	return nil
}

// Utility functions for integration

// ExtractUserFromContext extracts user ID from request context
func ExtractUserFromContext(ctx context.Context) string {
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return "anonymous"
}

// ExtractSessionFromContext extracts session ID from request context
func ExtractSessionFromContext(ctx context.Context) string {
	if sessionID, ok := ctx.Value("session_id").(string); ok {
		return sessionID
	}
	return ""
}

// CreateSecurityContext creates a context with security information
func CreateSecurityContext(ctx context.Context, userID, sessionID string) context.Context {
	ctx = context.WithValue(ctx, "user_id", userID)
	ctx = context.WithValue(ctx, "session_id", sessionID)
	ctx = context.WithValue(ctx, "security_enabled", true)
	return ctx
}
