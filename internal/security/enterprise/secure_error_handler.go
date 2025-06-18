package enterprise

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// SecureErrorHandler provides security-aware error handling
type SecureErrorHandler struct {
	config          *SecureErrorConfig
	sanitizer       *ErrorSanitizer
	securityMonitor *SecurityMonitor
}

// SecureErrorConfig holds secure error handling configuration
type SecureErrorConfig struct {
	ProductionMode      bool     `json:"production_mode"`
	SanitizeStackTraces bool     `json:"sanitize_stack_traces"`
	FilterSensitiveInfo bool     `json:"filter_sensitive_info"`
	MaxStackTraceLines  int      `json:"max_stack_trace_lines"`
	AllowedInfoPatterns []string `json:"allowed_info_patterns"`
	SensitivePatterns   []string `json:"sensitive_patterns"`
}

// NewSecureErrorHandler creates a new secure error handler
func NewSecureErrorHandler(config *SecureErrorConfig, monitor *SecurityMonitor) *SecureErrorHandler {
	if config == nil {
		config = DefaultSecureErrorConfig()
	}

	return &SecureErrorHandler{
		config:          config,
		sanitizer:       NewErrorSanitizer(config),
		securityMonitor: monitor,
	}
}

// DefaultSecureErrorConfig returns default secure error configuration
func DefaultSecureErrorConfig() *SecureErrorConfig {
	return &SecureErrorConfig{
		ProductionMode:      true,
		SanitizeStackTraces: true,
		FilterSensitiveInfo: true,
		MaxStackTraceLines:  5,
		AllowedInfoPatterns: []string{
			"^(INFO|WARN|ERROR):",
			"^Operation:",
			"^Status:",
		},
		SensitivePatterns: []string{
			`AKIA[0-9A-Z]{16}`,  // AWS Access Keys
			`[0-9a-zA-Z/+]{40}`, // AWS Secret Keys
			`eyJ[0-9a-zA-Z_-]*\.eyJ[0-9a-zA-Z_-]*\.[0-9a-zA-Z_-]*`,             // JWT tokens
			`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`,                         // Private keys
			`(?i)(password|passwd|pwd|secret|token|key)\s*[:=]\s*[^\s]+`,       // Passwords/secrets
			`(?i)(mysql|postgres|mongodb)://[^\\s]+`,                           // Database URLs
			`\b(?:[0-9]{1,3}\.){3}[0-9]{1,3}\b`,                                // IP addresses
			`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,              // Email addresses
			`/[a-zA-Z0-9/_-]*/(home|users?|Documents|Desktop)/[a-zA-Z0-9/_-]*`, // File paths
		},
	}
}

// SecureHandleError handles errors with security considerations
func (seh *SecureErrorHandler) SecureHandleError(ctx context.Context, err error, operation string) *SecureError {
	if err == nil {
		return nil
	}

	// Create secure error
	secureErr := &SecureError{
		ID:          generateErrorID(),
		Timestamp:   time.Now(),
		Operation:   operation,
		SafeMessage: seh.sanitizer.SanitizeMessage(err.Error()),
		Code:        seh.extractErrorCode(err),
		Severity:    seh.determineSeverity(err),
	}

	// Add safe context information
	if ctx != nil {
		secureErr.Context = seh.extractSafeContext(ctx)
	}

	// Generate safe stack trace if needed
	if !seh.config.ProductionMode || seh.shouldIncludeStackTrace(err) {
		secureErr.StackTrace = seh.sanitizer.SanitizeStackTrace(seh.captureStackTrace(2))
	}

	// Record security metrics
	if seh.securityMonitor != nil {
		seh.recordSecurityMetrics(secureErr, err)
	}

	// Check for security-related errors
	if seh.isSecurityRelatedError(err) {
		seh.handleSecurityError(secureErr, err)
	}

	return secureErr
}

// SecureError represents a security-sanitized error
type SecureError struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Operation   string                 `json:"operation"`
	SafeMessage string                 `json:"safe_message"`
	Code        string                 `json:"code"`
	Severity    ErrorSeverity          `json:"severity"`
	Context     map[string]interface{} `json:"context,omitempty"`
	StackTrace  string                 `json:"stack_trace,omitempty"`
	Remediation string                 `json:"remediation,omitempty"`
}

// ErrorSeverity represents error severity levels
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "LOW"
	SeverityMedium   ErrorSeverity = "MEDIUM"
	SeverityHigh     ErrorSeverity = "HIGH"
	SeverityCritical ErrorSeverity = "CRITICAL"
)

// Error implements the error interface
func (se *SecureError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", se.Severity, se.Code, se.SafeMessage)
}

// ErrorSanitizer sanitizes error information
type ErrorSanitizer struct {
	config            *SecureErrorConfig
	sensitivePatterns []*regexp.Regexp
	allowedPatterns   []*regexp.Regexp
}

// NewErrorSanitizer creates a new error sanitizer
func NewErrorSanitizer(config *SecureErrorConfig) *ErrorSanitizer {
	sensitivePatterns := make([]*regexp.Regexp, len(config.SensitivePatterns))
	for i, pattern := range config.SensitivePatterns {
		sensitivePatterns[i] = regexp.MustCompile(pattern)
	}

	allowedPatterns := make([]*regexp.Regexp, len(config.AllowedInfoPatterns))
	for i, pattern := range config.AllowedInfoPatterns {
		allowedPatterns[i] = regexp.MustCompile(pattern)
	}

	return &ErrorSanitizer{
		config:            config,
		sensitivePatterns: sensitivePatterns,
		allowedPatterns:   allowedPatterns,
	}
}

// SanitizeMessage sanitizes error messages
func (es *ErrorSanitizer) SanitizeMessage(message string) string {
	if !es.config.FilterSensitiveInfo {
		return message
	}

	sanitized := message

	// Replace sensitive patterns
	for _, pattern := range es.sensitivePatterns {
		sanitized = pattern.ReplaceAllString(sanitized, "[REDACTED]")
	}

	// If in production mode, provide generic messages for certain errors
	if es.config.ProductionMode {
		sanitized = es.genericizeMessage(sanitized)
	}

	return sanitized
}

// SanitizeStackTrace sanitizes stack traces
func (es *ErrorSanitizer) SanitizeStackTrace(stackTrace string) string {
	if !es.config.SanitizeStackTraces {
		return stackTrace
	}

	lines := strings.Split(stackTrace, "\n")
	sanitizedLines := make([]string, 0)

	lineCount := 0
	for _, line := range lines {
		if lineCount >= es.config.MaxStackTraceLines {
			break
		}

		// Filter out sensitive paths
		if es.containsSensitiveInfo(line) {
			sanitizedLines = append(sanitizedLines, "[REDACTED]")
		} else {
			// Remove absolute paths, keep only relative paths
			sanitizedLine := es.sanitizePaths(line)
			sanitizedLines = append(sanitizedLines, sanitizedLine)
		}

		if strings.TrimSpace(line) != "" {
			lineCount++
		}
	}

	return strings.Join(sanitizedLines, "\n")
}

// containsSensitiveInfo checks if a string contains sensitive information
func (es *ErrorSanitizer) containsSensitiveInfo(text string) bool {
	for _, pattern := range es.sensitivePatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

// sanitizePaths removes absolute paths and keeps only function names
func (es *ErrorSanitizer) sanitizePaths(line string) string {
	// Remove absolute paths, keep only package.function format
	pathRegex := regexp.MustCompile(`(/[^:]+):(\d+)`)
	return pathRegex.ReplaceAllStringFunc(line, func(match string) string {
		parts := strings.Split(match, "/")
		if len(parts) > 0 {
			// Keep only the last part (filename)
			return "/" + parts[len(parts)-1]
		}
		return "[PATH]"
	})
}

// genericizeMessage provides generic error messages for production
func (es *ErrorSanitizer) genericizeMessage(message string) string {
	// Map specific errors to generic messages
	genericMessages := map[string]string{
		"connection":     "Connection error occurred",
		"authentication": "Authentication failed",
		"authorization":  "Access denied",
		"timeout":        "Operation timed out",
		"invalid":        "Invalid input provided",
		"not found":      "Resource not found",
		"internal":       "Internal server error",
	}

	lowerMessage := strings.ToLower(message)
	for keyword, generic := range genericMessages {
		if strings.Contains(lowerMessage, keyword) {
			return generic
		}
	}

	return "An error occurred while processing your request"
}

// extractErrorCode extracts error code from error
func (seh *SecureErrorHandler) extractErrorCode(err error) string {
	// Try to extract error code from known error types
	switch e := err.(type) {
	case *SecurityError:
		return e.Code
	default:
		// Extract from error message patterns
		return seh.extractCodeFromMessage(err.Error())
	}
}

// extractCodeFromMessage extracts error code from error message
func (seh *SecureErrorHandler) extractCodeFromMessage(message string) string {
	// Define patterns for common error codes
	patterns := map[string]*regexp.Regexp{
		"AWS_ERROR":        regexp.MustCompile(`(?i)(aws|s3).*error`),
		"AUTH_ERROR":       regexp.MustCompile(`(?i)(auth|login|permission).*error`),
		"NETWORK_ERROR":    regexp.MustCompile(`(?i)(network|connection|timeout).*error`),
		"VALIDATION_ERROR": regexp.MustCompile(`(?i)(validation|invalid|malformed).*error`),
		"RESOURCE_ERROR":   regexp.MustCompile(`(?i)(resource|not found|missing).*error`),
	}

	for code, pattern := range patterns {
		if pattern.MatchString(message) {
			return code
		}
	}

	return "GENERAL_ERROR"
}

// determineSeverity determines error severity
func (seh *SecureErrorHandler) determineSeverity(err error) ErrorSeverity {
	message := strings.ToLower(err.Error())

	// Critical errors
	criticalPatterns := []string{"panic", "fatal", "critical", "security", "breach"}
	for _, pattern := range criticalPatterns {
		if strings.Contains(message, pattern) {
			return SeverityCritical
		}
	}

	// High severity errors
	highPatterns := []string{"error", "failed", "denied", "unauthorized", "forbidden"}
	for _, pattern := range highPatterns {
		if strings.Contains(message, pattern) {
			return SeverityHigh
		}
	}

	// Medium severity errors
	mediumPatterns := []string{"warning", "timeout", "retry", "degraded"}
	for _, pattern := range mediumPatterns {
		if strings.Contains(message, pattern) {
			return SeverityMedium
		}
	}

	return SeverityLow
}

// extractSafeContext extracts safe context information
func (seh *SecureErrorHandler) extractSafeContext(ctx context.Context) map[string]interface{} {
	safeContext := make(map[string]interface{})

	// Extract only safe context values
	if userID := ctx.Value("user_id"); userID != nil {
		if str, ok := userID.(string); ok && !seh.sanitizer.containsSensitiveInfo(str) {
			safeContext["user_id"] = str
		}
	}

	if requestID := ctx.Value("request_id"); requestID != nil {
		if str, ok := requestID.(string); ok && !seh.sanitizer.containsSensitiveInfo(str) {
			safeContext["request_id"] = str
		}
	}

	if operation := ctx.Value("operation"); operation != nil {
		if str, ok := operation.(string); ok && !seh.sanitizer.containsSensitiveInfo(str) {
			safeContext["operation"] = str
		}
	}

	return safeContext
}

// captureStackTrace captures stack trace
func (seh *SecureErrorHandler) captureStackTrace(skip int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// shouldIncludeStackTrace determines if stack trace should be included
func (seh *SecureErrorHandler) shouldIncludeStackTrace(err error) bool {
	// Include stack trace for critical errors even in production
	severity := seh.determineSeverity(err)
	return severity == SeverityCritical || !seh.config.ProductionMode
}

// isSecurityRelatedError checks if error is security-related
func (seh *SecureErrorHandler) isSecurityRelatedError(err error) bool {
	message := strings.ToLower(err.Error())
	securityKeywords := []string{
		"security", "auth", "permission", "unauthorized", "forbidden",
		"breach", "attack", "malicious", "suspicious", "violation",
	}

	for _, keyword := range securityKeywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}

	return false
}

// handleSecurityError handles security-related errors
func (seh *SecureErrorHandler) handleSecurityError(secureErr *SecureError, originalErr error) {
	if seh.securityMonitor != nil {
		// Escalate threat level for security errors
		seh.securityMonitor.escalateThreatLevel(ThreatLevelHigh)

		// Record security event
		seh.securityMonitor.RecordErrorSpike(100.0) // Max error rate for security events
	}

	// Add security-specific remediation advice
	secureErr.Remediation = seh.generateSecurityRemediation(originalErr)
}

// generateSecurityRemediation generates security remediation advice
func (seh *SecureErrorHandler) generateSecurityRemediation(err error) string {
	message := strings.ToLower(err.Error())

	remediations := map[string]string{
		"auth":         "Please verify your authentication credentials and try again",
		"permission":   "Contact your administrator to request appropriate permissions",
		"unauthorized": "Please ensure you are properly authenticated",
		"forbidden":    "You do not have permission to access this resource",
		"security":     "This appears to be a security-related issue. Please contact support",
		"breach":       "Security breach detected. Immediate attention required",
	}

	for keyword, remediation := range remediations {
		if strings.Contains(message, keyword) {
			return remediation
		}
	}

	return "Please contact support if this error persists"
}

// recordSecurityMetrics records security metrics
func (seh *SecureErrorHandler) recordSecurityMetrics(secureErr *SecureError, originalErr error) {
	if seh.securityMonitor == nil {
		return
	}

	// Record based on severity
	switch secureErr.Severity {
	case SeverityCritical:
		seh.securityMonitor.escalateThreatLevel(ThreatLevelCritical)
	case SeverityHigh:
		seh.securityMonitor.escalateThreatLevel(ThreatLevelHigh)
	case SeverityMedium:
		seh.securityMonitor.escalateThreatLevel(ThreatLevelMedium)
	}

	// Check for specific error patterns that indicate attacks
	if seh.isAttackPattern(originalErr) {
		seh.securityMonitor.RecordErrorSpike(50.0)
	}
}

// isAttackPattern checks if error indicates potential attack
func (seh *SecureErrorHandler) isAttackPattern(err error) bool {
	message := strings.ToLower(err.Error())
	attackPatterns := []string{
		"too many", "rate limit", "flood", "ddos", "injection",
		"overflow", "traversal", "malformed", "suspicious",
	}

	for _, pattern := range attackPatterns {
		if strings.Contains(message, pattern) {
			return true
		}
	}

	return false
}

// generateErrorID generates a unique error ID
func generateErrorID() string {
	return fmt.Sprintf("ERR_%d", time.Now().UnixNano())
}
