package analytics

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// ErrorTracker collects and analyzes error patterns
type ErrorTracker struct {
	mu            sync.RWMutex
	errors        []ErrorEvent
	errorPatterns map[string]*ErrorPattern
	config        *ErrorTrackerConfig
}

// ErrorEvent represents a single error occurrence
type ErrorEvent struct {
	ID          string            `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	ErrorType   string            `json:"error_type"`
	Message     string            `json:"message"`
	Command     string            `json:"command"`
	StackTrace  string            `json:"stack_trace,omitempty"`
	Context     map[string]string `json:"context"`
	Severity    ErrorSeverity     `json:"severity"`
	UserID      string            `json:"user_id,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
	Version     string            `json:"version"`
	Fingerprint string            `json:"fingerprint"`
}

// ErrorPattern represents patterns of similar errors
type ErrorPattern struct {
	Fingerprint     string            `json:"fingerprint"`
	FirstSeen       time.Time         `json:"first_seen"`
	LastSeen        time.Time         `json:"last_seen"`
	Count           int64             `json:"count"`
	ErrorType       string            `json:"error_type"`
	CommonMessage   string            `json:"common_message"`
	CommonContext   map[string]string `json:"common_context"`
	Severity        ErrorSeverity     `json:"severity"`
	AffectedCommands []string         `json:"affected_commands"`
	TrendDirection  TrendDirection    `json:"trend_direction"`
	RecentCount     int64             `json:"recent_count"` // Last 24 hours
}

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// TrendDirection represents the trend of error occurrence
type TrendDirection string

const (
	TrendIncreasing TrendDirection = "increasing"
	TrendStable     TrendDirection = "stable"
	TrendDecreasing TrendDirection = "decreasing"
)

// ErrorTrackerConfig configures the error tracker
type ErrorTrackerConfig struct {
	MaxErrors           int           `json:"max_errors"`
	RetentionPeriod     time.Duration `json:"retention_period"`
	AlertThresholds     AlertThresholds `json:"alert_thresholds"`
	IgnorePatterns      []string      `json:"ignore_patterns"`
	EnableStackTraces   bool          `json:"enable_stack_traces"`
	SamplingRate        float64       `json:"sampling_rate"`
}

// AlertThresholds defines when to trigger alerts
type AlertThresholds struct {
	CriticalErrorRate   float64 `json:"critical_error_rate"`   // Errors per minute
	HighErrorCount      int64   `json:"high_error_count"`      // Total errors in window
	NewErrorThreshold   int64   `json:"new_error_threshold"`   // New unique errors
	SpikeMultiplier     float64 `json:"spike_multiplier"`      // Multiplier for spike detection
}

// ErrorSummary provides aggregated error information
type ErrorSummary struct {
	TotalErrors       int64                      `json:"total_errors"`
	UniqueErrors      int64                      `json:"unique_errors"`
	ErrorRate         float64                    `json:"error_rate_per_minute"`
	TopErrors         []*ErrorPattern            `json:"top_errors"`
	RecentErrors      []ErrorEvent               `json:"recent_errors"`
	ErrorsByType      map[string]int64           `json:"errors_by_type"`
	ErrorsByCommand   map[string]int64           `json:"errors_by_command"`
	ErrorsBySeverity  map[ErrorSeverity]int64    `json:"errors_by_severity"`
	TrendAnalysis     *TrendAnalysis             `json:"trend_analysis"`
	Alerts            []Alert                    `json:"alerts"`
}

// TrendAnalysis provides trend information
type TrendAnalysis struct {
	OverallTrend      TrendDirection `json:"overall_trend"`
	HourlyTrend       []int64        `json:"hourly_trend"`
	DailyTrend        []int64        `json:"daily_trend"`
	WorstPerformingCommand string    `json:"worst_performing_command"`
	MostCommonError   string         `json:"most_common_error"`
}

// Alert represents an error alert
type Alert struct {
	ID          string        `json:"id"`
	Type        AlertType     `json:"type"`
	Severity    ErrorSeverity `json:"severity"`
	Message     string        `json:"message"`
	Timestamp   time.Time     `json:"timestamp"`
	Pattern     *ErrorPattern `json:"pattern,omitempty"`
	Context     map[string]interface{} `json:"context"`
}

// AlertType represents different types of alerts
type AlertType string

const (
	AlertTypeErrorSpike    AlertType = "error_spike"
	AlertTypeNewError      AlertType = "new_error"
	AlertTypeCriticalError AlertType = "critical_error"
	AlertTypeHighRate      AlertType = "high_rate"
)

// NewErrorTracker creates a new error tracker
func NewErrorTracker(config *ErrorTrackerConfig) *ErrorTracker {
	if config == nil {
		config = &ErrorTrackerConfig{
			MaxErrors:       10000,
			RetentionPeriod: 30 * 24 * time.Hour,
			AlertThresholds: AlertThresholds{
				CriticalErrorRate:  10.0,  // 10 errors per minute
				HighErrorCount:     100,   // 100 errors in window
				NewErrorThreshold:  5,     // 5 new unique errors
				SpikeMultiplier:    3.0,   // 3x normal rate
			},
			IgnorePatterns:    []string{},
			EnableStackTraces: true,
			SamplingRate:      1.0, // 100% sampling
		}
	}

	return &ErrorTracker{
		errors:        make([]ErrorEvent, 0),
		errorPatterns: make(map[string]*ErrorPattern),
		config:        config,
	}
}

// TrackError records a new error event
func (et *ErrorTracker) TrackError(ctx context.Context, errorType, message, command string, err error, context map[string]string, userID, sessionID, version string) {
	et.mu.Lock()
	defer et.mu.Unlock()

	// Apply sampling rate
	if et.config.SamplingRate < 1.0 {
		// Simple sampling implementation
		if time.Now().UnixNano()%100 >= int64(et.config.SamplingRate*100) {
			return
		}
	}

	event := ErrorEvent{
		ID:        generateEventID(),
		Timestamp: time.Now().UTC(),
		ErrorType: errorType,
		Message:   message,
		Command:   command,
		Context:   context,
		Severity:  classifyErrorSeverity(errorType, message),
		UserID:    userID,
		SessionID: sessionID,
		Version:   version,
	}

	// Generate fingerprint for pattern matching
	event.Fingerprint = generateErrorFingerprint(errorType, message, command)

	// Add stack trace if enabled and available
	if et.config.EnableStackTraces && err != nil {
		event.StackTrace = captureStackTrace()
	}

	// Add to events list
	et.errors = append(et.errors, event)

	// Maintain max errors limit
	if len(et.errors) > et.config.MaxErrors {
		et.errors = et.errors[len(et.errors)-et.config.MaxErrors:]
	}

	// Update error patterns
	et.updateErrorPattern(event)

	// Clean old errors
	et.cleanOldErrors()
}

// GetErrorSummary returns a comprehensive error summary
func (et *ErrorTracker) GetErrorSummary() *ErrorSummary {
	et.mu.RLock()
	defer et.mu.RUnlock()

	now := time.Now()

	summary := &ErrorSummary{
		TotalErrors:      int64(len(et.errors)),
		UniqueErrors:     int64(len(et.errorPatterns)),
		ErrorsByType:     make(map[string]int64),
		ErrorsByCommand:  make(map[string]int64),
		ErrorsBySeverity: make(map[ErrorSeverity]int64),
	}

	// Calculate error rate (last hour)
	hourAgo := now.Add(-time.Hour)
	recentErrors := 0
	for _, event := range et.errors {
		if event.Timestamp.After(hourAgo) {
			recentErrors++
		}
		
		// Aggregate by type
		summary.ErrorsByType[event.ErrorType]++
		
		// Aggregate by command
		summary.ErrorsByCommand[event.Command]++
		
		// Aggregate by severity
		summary.ErrorsBySeverity[event.Severity]++
	}
	summary.ErrorRate = float64(recentErrors) / 60.0 // errors per minute

	// Get top error patterns
	patterns := make([]*ErrorPattern, 0, len(et.errorPatterns))
	for _, pattern := range et.errorPatterns {
		patterns = append(patterns, pattern)
	}
	
	// Sort by count (descending)
	for i := 0; i < len(patterns)-1; i++ {
		for j := i + 1; j < len(patterns); j++ {
			if patterns[i].Count < patterns[j].Count {
				patterns[i], patterns[j] = patterns[j], patterns[i]
			}
		}
	}
	
	// Take top 10
	if len(patterns) > 10 {
		patterns = patterns[:10]
	}
	summary.TopErrors = patterns

	// Get recent errors (last 50)
	recentErrorsCount := 50
	if len(et.errors) < recentErrorsCount {
		recentErrorsCount = len(et.errors)
	}
	summary.RecentErrors = et.errors[len(et.errors)-recentErrorsCount:]

	// Generate trend analysis
	summary.TrendAnalysis = et.generateTrendAnalysis()

	// Generate alerts
	summary.Alerts = et.generateAlerts()

	return summary
}

// updateErrorPattern updates or creates error patterns
func (et *ErrorTracker) updateErrorPattern(event ErrorEvent) {
	pattern, exists := et.errorPatterns[event.Fingerprint]
	
	if !exists {
		pattern = &ErrorPattern{
			Fingerprint:      event.Fingerprint,
			FirstSeen:        event.Timestamp,
			LastSeen:         event.Timestamp,
			Count:            1,
			ErrorType:        event.ErrorType,
			CommonMessage:    event.Message,
			CommonContext:    event.Context,
			Severity:         event.Severity,
			AffectedCommands: []string{event.Command},
			TrendDirection:   TrendStable,
			RecentCount:      1,
		}
		et.errorPatterns[event.Fingerprint] = pattern
	} else {
		pattern.LastSeen = event.Timestamp
		pattern.Count++
		
		// Update recent count (last 24 hours)
		recentCutoff := time.Now().Add(-24 * time.Hour)
		if event.Timestamp.After(recentCutoff) {
			pattern.RecentCount++
		}
		
		// Add command if not already present
		commandExists := false
		for _, cmd := range pattern.AffectedCommands {
			if cmd == event.Command {
				commandExists = true
				break
			}
		}
		if !commandExists {
			pattern.AffectedCommands = append(pattern.AffectedCommands, event.Command)
		}
		
		// Update severity if higher
		if event.Severity == SeverityCritical || 
		   (event.Severity == SeverityHigh && pattern.Severity != SeverityCritical) ||
		   (event.Severity == SeverityMedium && pattern.Severity == SeverityLow) {
			pattern.Severity = event.Severity
		}
	}
}

// generateTrendAnalysis creates trend analysis
func (et *ErrorTracker) generateTrendAnalysis() *TrendAnalysis {
	now := time.Now()
	
	// Calculate hourly trend (last 24 hours)
	hourlyTrend := make([]int64, 24)
	for _, event := range et.errors {
		if event.Timestamp.After(now.Add(-24 * time.Hour)) {
			hour := int(now.Sub(event.Timestamp).Hours())
			if hour >= 0 && hour < 24 {
				hourlyTrend[23-hour]++
			}
		}
	}
	
	// Calculate daily trend (last 7 days)
	dailyTrend := make([]int64, 7)
	for _, event := range et.errors {
		if event.Timestamp.After(now.Add(-7 * 24 * time.Hour)) {
			day := int(now.Sub(event.Timestamp).Hours() / 24)
			if day >= 0 && day < 7 {
				dailyTrend[6-day]++
			}
		}
	}
	
	// Determine overall trend
	overallTrend := TrendStable
	if len(dailyTrend) >= 3 {
		recent := dailyTrend[len(dailyTrend)-1] + dailyTrend[len(dailyTrend)-2]
		older := dailyTrend[0] + dailyTrend[1]
		if recent > older*2 {
			overallTrend = TrendIncreasing
		} else if older > recent*2 {
			overallTrend = TrendDecreasing
		}
	}
	
	// Find worst performing command
	commandErrors := make(map[string]int64)
	for _, event := range et.errors {
		commandErrors[event.Command]++
	}
	
	worstCommand := ""
	maxErrors := int64(0)
	for cmd, count := range commandErrors {
		if count > maxErrors {
			maxErrors = count
			worstCommand = cmd
		}
	}
	
	// Find most common error
	errorTypes := make(map[string]int64)
	for _, event := range et.errors {
		errorTypes[event.ErrorType]++
	}
	
	mostCommonError := ""
	maxErrorCount := int64(0)
	for errorType, count := range errorTypes {
		if count > maxErrorCount {
			maxErrorCount = count
			mostCommonError = errorType
		}
	}
	
	return &TrendAnalysis{
		OverallTrend:           overallTrend,
		HourlyTrend:            hourlyTrend,
		DailyTrend:             dailyTrend,
		WorstPerformingCommand: worstCommand,
		MostCommonError:        mostCommonError,
	}
}

// generateAlerts creates alerts based on error patterns
func (et *ErrorTracker) generateAlerts() []Alert {
	var alerts []Alert
	now := time.Now()
	
	// Check for error rate spike
	hourAgo := now.Add(-time.Hour)
	recentErrors := 0
	for _, event := range et.errors {
		if event.Timestamp.After(hourAgo) {
			recentErrors++
		}
	}
	
	errorRate := float64(recentErrors) / 60.0 // errors per minute
	if errorRate > et.config.AlertThresholds.CriticalErrorRate {
		alerts = append(alerts, Alert{
			ID:        generateEventID(),
			Type:      AlertTypeHighRate,
			Severity:  SeverityCritical,
			Message:   fmt.Sprintf("High error rate detected: %.2f errors/minute", errorRate),
			Timestamp: now,
			Context: map[string]interface{}{
				"error_rate": errorRate,
				"threshold":  et.config.AlertThresholds.CriticalErrorRate,
			},
		})
	}
	
	// Check for new error patterns
	dayAgo := now.Add(-24 * time.Hour)
	newPatterns := 0
	for _, pattern := range et.errorPatterns {
		if pattern.FirstSeen.After(dayAgo) {
			newPatterns++
		}
	}
	
	if int64(newPatterns) > et.config.AlertThresholds.NewErrorThreshold {
		alerts = append(alerts, Alert{
			ID:        generateEventID(),
			Type:      AlertTypeNewError,
			Severity:  SeverityHigh,
			Message:   fmt.Sprintf("%d new error patterns detected in the last 24 hours", newPatterns),
			Timestamp: now,
			Context: map[string]interface{}{
				"new_patterns": newPatterns,
				"threshold":    et.config.AlertThresholds.NewErrorThreshold,
			},
		})
	}
	
	// Check for critical errors
	for _, pattern := range et.errorPatterns {
		if pattern.Severity == SeverityCritical && pattern.RecentCount > 0 {
			alerts = append(alerts, Alert{
				ID:        generateEventID(),
				Type:      AlertTypeCriticalError,
				Severity:  SeverityCritical,
				Message:   fmt.Sprintf("Critical error pattern active: %s", pattern.CommonMessage),
				Timestamp: now,
				Pattern:   pattern,
				Context: map[string]interface{}{
					"pattern_count": pattern.Count,
					"recent_count":  pattern.RecentCount,
				},
			})
		}
	}
	
	return alerts
}

// cleanOldErrors removes errors older than retention period
func (et *ErrorTracker) cleanOldErrors() {
	cutoff := time.Now().Add(-et.config.RetentionPeriod)
	
	var validErrors []ErrorEvent
	for _, event := range et.errors {
		if event.Timestamp.After(cutoff) {
			validErrors = append(validErrors, event)
		}
	}
	
	et.errors = validErrors
	
	// Clean old patterns (no recent errors)
	recentCutoff := time.Now().Add(-24 * time.Hour)
	for fingerprint, pattern := range et.errorPatterns {
		if pattern.LastSeen.Before(recentCutoff) {
			pattern.RecentCount = 0
		}
		
		// Remove patterns with no errors in retention period
		if pattern.LastSeen.Before(cutoff) {
			delete(et.errorPatterns, fingerprint)
		}
	}
}

// Helper functions

func generateEventID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func generateErrorFingerprint(errorType, message, command string) string {
	// Simple fingerprinting - combine error type and first part of message
	messagePrefix := message
	if len(message) > 100 {
		messagePrefix = message[:100]
	}
	return fmt.Sprintf("%s:%s:%s", errorType, command, messagePrefix)
}

func classifyErrorSeverity(errorType, message string) ErrorSeverity {
	// Simple classification logic
	switch errorType {
	case "panic", "fatal", "critical":
		return SeverityCritical
	case "auth", "permission", "access_denied":
		return SeverityHigh
	case "network", "timeout", "connection":
		return SeverityMedium
	default:
		return SeverityLow
	}
}

func captureStackTrace() string {
	// Capture stack trace
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}