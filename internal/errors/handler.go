package errors

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/logging"
)

// Use types from s3ry_errors.go to avoid conflicts

// Methods for S3ryError are defined in s3ry_errors.go

// ErrorHandler manages error handling and recovery
type ErrorHandler struct {
	mu           sync.RWMutex
	config       *config.Config
	logger       *logging.Logger
	tracker      *AdvancedErrorTracker
	patterns     map[string]*ErrorPattern
	recoverers   map[ErrorCode]RecoveryFunc
	interceptors []ErrorInterceptor
	metrics      *ErrorMetrics
	ctx          context.Context
	cancel       context.CancelFunc
}

// RecoveryFunc represents a function that can attempt to recover from an error
type RecoveryFunc func(ctx context.Context, err *S3ryError) error

// ErrorInterceptor represents a function that can intercept and modify errors
type ErrorInterceptor func(err *S3ryError) *S3ryError

// ErrorPattern represents a pattern for matching and handling errors
type ErrorPattern struct {
	Type        ErrorCode     `json:"type"`
	CodePattern string        `json:"code_pattern"`
	Severity    string        `json:"severity"`
	Retryable   bool          `json:"retryable"`
	RetryDelay  time.Duration `json:"retry_delay"`
	Suggestions []string      `json:"suggestions"`
	HelpURL     string        `json:"help_url"`
}

// ErrorMetrics tracks error statistics
type ErrorMetrics struct {
	mu               sync.RWMutex
	TotalErrors      int64                   `json:"total_errors"`
	ErrorsByType     map[ErrorCode]int64     `json:"errors_by_type"`
	ErrorsBySeverity map[string]int64        `json:"errors_by_severity"`
	RecoveryRate     float64                 `json:"recovery_rate"`
	LastError        time.Time               `json:"last_error"`
	MostCommonError  string                  `json:"most_common_error"`
	ErrorTrends      map[string][]ErrorTrend `json:"error_trends"`
}

// ErrorTrend represents error trend data
type ErrorTrend struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(cfg *config.Config, logger *logging.Logger) *ErrorHandler {
	ctx, cancel := context.WithCancel(context.Background())

	handler := &ErrorHandler{
		config:       cfg,
		logger:       logger,
		tracker:      NewAdvancedErrorTracker(cfg),
		patterns:     make(map[string]*ErrorPattern),
		recoverers:   make(map[ErrorCode]RecoveryFunc),
		interceptors: make([]ErrorInterceptor, 0),
		metrics: &ErrorMetrics{
			ErrorsByType:     make(map[ErrorCode]int64),
			ErrorsBySeverity: make(map[string]int64),
			ErrorTrends:      make(map[string][]ErrorTrend),
		},
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize default patterns
	handler.initializeDefaultPatterns()

	// Initialize default recoverers
	handler.initializeDefaultRecoverers()

	// Start error tracker
	if err := handler.tracker.Start(); err != nil {
		logger.Warn("Failed to start error tracker", "error", err)
	}

	return handler
}

// initializeDefaultPatterns sets up default error patterns
func (h *ErrorHandler) initializeDefaultPatterns() {
	patterns := []*ErrorPattern{
		{
			Type:        ErrCodeNetwork,
			CodePattern: "NetworkError|ConnectionError|TimeoutError",
			Severity:    "medium",
			Retryable:   true,
			RetryDelay:  2 * time.Second,
			Suggestions: []string{
				"Check your internet connection",
				"Verify AWS service status",
				"Try again in a few moments",
			},
			HelpURL: "https://docs.aws.amazon.com/general/latest/gr/api-retries.html",
		},
		{
			Type:        ErrCodeS3Permission,
			CodePattern: "InvalidAccessKeyId|SignatureDoesNotMatch|TokenRefreshRequired",
			Severity:    "high",
			Retryable:   false,
			Suggestions: []string{
				"Check your AWS credentials",
				"Verify your AWS profile configuration",
				"Ensure your access keys are valid",
			},
			HelpURL: "https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html",
		},
		{
			Type:        ErrCodePermission,
			CodePattern: "AccessDenied|Forbidden|UnauthorizedOperation",
			Severity:    "high",
			Retryable:   false,
			Suggestions: []string{
				"Check your IAM permissions",
				"Verify bucket policies",
				"Contact your AWS administrator",
			},
			HelpURL: "https://docs.aws.amazon.com/s3/latest/userguide/access-control-overview.html",
		},
		{
			Type:        ErrCodeTimeout,
			CodePattern: "SlowDown|RequestLimitExceeded|TooManyRequests",
			Severity:    "medium",
			Retryable:   true,
			RetryDelay:  5 * time.Second,
			Suggestions: []string{
				"Reduce request rate",
				"Implement exponential backoff",
				"Consider using request batching",
			},
			HelpURL: "https://docs.aws.amazon.com/s3/latest/userguide/optimizing-performance.html",
		},
		{
			Type:        ErrCodeS3NotFound,
			CodePattern: "NoSuchBucket|NoSuchKey|NotFound",
			Severity:    "medium",
			Retryable:   false,
			Suggestions: []string{
				"Verify the resource name",
				"Check if the resource exists",
				"Ensure you're using the correct region",
			},
		},
	}

	for _, pattern := range patterns {
		h.patterns[string(pattern.Type)] = pattern
	}
}

// initializeDefaultRecoverers sets up default recovery functions
func (h *ErrorHandler) initializeDefaultRecoverers() {
	// Network error recovery
	h.recoverers[ErrCodeNetwork] = func(ctx context.Context, err *S3ryError) error {
		h.logger.Info("Attempting network error recovery", "error_id", err.Code)

		// Simple retry with exponential backoff
		for i := 0; i < 3; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i+1) * time.Second):
				// In a real implementation, this would retry the original operation
				h.logger.Debug("Network recovery attempt", "attempt", i+1)
			}
		}

		return fmt.Errorf("network recovery failed after 3 attempts")
	}

	// Rate limit recovery
	h.recoverers[ErrCodeTimeout] = func(ctx context.Context, err *S3ryError) error {
		h.logger.Info("Attempting timeout recovery", "error_code", err.Code)

		delay := 5 * time.Second // Default delay

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			h.logger.Debug("Timeout recovery completed", "delay", delay)
			return nil
		}
	}
}

// Handle processes an error and returns an enhanced S3ryError
func (h *ErrorHandler) Handle(err error, operation string, context map[string]interface{}) *S3ryError {
	if err == nil {
		return nil
	}

	// Check if it's already an S3ryError
	if s3ryErr, ok := err.(*S3ryError); ok {
		return s3ryErr
	}

	// Create new S3ryError
	s3ryErr := &S3ryError{
		Code:      ErrorCode(h.generateErrorCode(operation, err.Error())),
		Message:   err.Error(),
		Cause:     err,
		Context:   context,
		Timestamp: time.Now(),
		Operation: operation,
	}

	// Classify error
	h.classifyError(s3ryErr)

	// Capture stack trace (store in context)
	if s3ryErr.Context == nil {
		s3ryErr.Context = make(map[string]interface{})
	}
	s3ryErr.Context["stack_trace"] = h.captureStackTrace(2)

	// Apply patterns
	h.applyPatterns(s3ryErr)

	// Apply interceptors
	for _, interceptor := range h.interceptors {
		s3ryErr = interceptor(s3ryErr)
	}

	// Track error
	h.trackError(s3ryErr)

	// Log error
	h.logError(s3ryErr)

	return s3ryErr
}

// classifyError determines the error type and severity
func (h *ErrorHandler) classifyError(err *S3ryError) {
	message := strings.ToLower(err.Message)

	switch {
	case strings.Contains(message, "network") || strings.Contains(message, "connection") || strings.Contains(message, "timeout"):
		err.Code = ErrCodeNetwork
	case strings.Contains(message, "access") && strings.Contains(message, "denied"):
		err.Code = ErrCodePermission
	case strings.Contains(message, "invalid") && strings.Contains(message, "credentials"):
		err.Code = ErrCodeS3Permission
	case strings.Contains(message, "not found") || strings.Contains(message, "does not exist"):
		err.Code = ErrCodeS3NotFound
	case strings.Contains(message, "rate") || strings.Contains(message, "throttle"):
		err.Code = ErrCodeTimeout
	case strings.Contains(message, "validation") || strings.Contains(message, "invalid"):
		err.Code = ErrCodeValidation
	default:
		err.Code = ErrCodeUnknown
	}
}

// applyPatterns applies error patterns to enhance the error
func (h *ErrorHandler) applyPatterns(err *S3ryError) {
	if pattern, exists := h.patterns[string(err.Code)]; exists {
		// Apply pattern suggestions and help
		if len(pattern.Suggestions) > 0 {
			if err.Context == nil {
				err.Context = make(map[string]interface{})
			}
			err.Context["suggestions"] = pattern.Suggestions
		}
		if pattern.HelpURL != "" {
			if err.Context == nil {
				err.Context = make(map[string]interface{})
			}
			err.Context["help_url"] = pattern.HelpURL
		}
	}
}

// captureStackTrace captures the current stack trace
func (h *ErrorHandler) captureStackTrace(skip int) []StackFrame {
	var frames []StackFrame

	for i := skip; i < skip+10; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		var funcName string
		if fn != nil {
			funcName = fn.Name()
		}

		frames = append(frames, StackFrame{
			Function: funcName,
			File:     file,
			Line:     line,
		})
	}

	return frames
}

// trackError tracks the error for analytics
func (h *ErrorHandler) trackError(err *S3ryError) {
	// Update metrics
	h.updateMetrics(err)

	// Track with advanced tracker
	context := make(map[string]interface{})
	if err.Context != nil {
		context = err.Context
	}
	context["error_code"] = string(err.Code)
	if severity, ok := err.Context["severity"].(string); ok {
		context["severity"] = severity
	}

	h.tracker.TrackError(
		err.Operation,
		string(err.Code),
		err.Message,
		string(err.Code),
		context,
	)
}

// updateMetrics updates error metrics
func (h *ErrorHandler) updateMetrics(err *S3ryError) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()

	h.metrics.TotalErrors++
	h.metrics.ErrorsByType[err.Code]++
	if severity, ok := err.Context["severity"].(string); ok {
		h.metrics.ErrorsBySeverity[severity]++
	}
	h.metrics.LastError = err.Timestamp

	// Update trends
	today := time.Now().Format("2006-01-02")
	if trends, exists := h.metrics.ErrorTrends[today]; exists {
		// Update last entry or add new one
		if len(trends) > 0 && time.Since(trends[len(trends)-1].Timestamp) < time.Hour {
			trends[len(trends)-1].Count++
		} else {
			trends = append(trends, ErrorTrend{
				Timestamp: time.Now(),
				Count:     1,
			})
		}
		h.metrics.ErrorTrends[today] = trends
	} else {
		h.metrics.ErrorTrends[today] = []ErrorTrend{{
			Timestamp: time.Now(),
			Count:     1,
		}}
	}
}

// logError logs the error with appropriate level
func (h *ErrorHandler) logError(err *S3ryError) {
	fields := []interface{}{
		"error_code", string(err.Code),
		"operation", err.Operation,
		"timestamp", err.Timestamp,
	}

	// Determine severity from context or default to info
	severity := "info"
	if s, ok := err.Context["severity"].(string); ok {
		severity = s
	}

	switch severity {
	case "critical":
		h.logger.Error(err.Message, err.Cause, fields...)
	case "high":
		h.logger.Error(err.Message, err.Cause, fields...)
	case "medium":
		h.logger.Warn(err.Message, fields...)
	default:
		h.logger.Info(err.Message, fields...)
	}
}

// Recover attempts to recover from an error
func (h *ErrorHandler) Recover(ctx context.Context, err *S3ryError) error {
	// Check if recoverable based on context
	retryable := false
	if r, ok := err.Context["retryable"].(bool); ok {
		retryable = r
	}

	if !retryable {
		return fmt.Errorf("error is not recoverable: %w", err)
	}

	if recoverer, exists := h.recoverers[err.Code]; exists {
		h.logger.Info("Attempting error recovery", "error_code", string(err.Code))

		if recoverErr := recoverer(ctx, err); recoverErr != nil {
			h.logger.Error("Error recovery failed", recoverErr, "error_code", string(err.Code))
			return recoverErr
		}

		h.logger.Info("Error recovery successful", "error_code", string(err.Code))
		h.updateRecoveryMetrics(true)
		return nil
	}

	return fmt.Errorf("no recovery function available for error code: %s", err.Code)
}

// updateRecoveryMetrics updates recovery success metrics
func (h *ErrorHandler) updateRecoveryMetrics(success bool) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()

	// This would track recovery attempts and success rate
	// Implementation depends on specific requirements
}

// AddRecoverer adds a custom recovery function
func (h *ErrorHandler) AddRecoverer(errorCode ErrorCode, recoverer RecoveryFunc) {
	h.mu.Lock()
	h.recoverers[errorCode] = recoverer
	h.mu.Unlock()
}

// AddInterceptor adds an error interceptor
func (h *ErrorHandler) AddInterceptor(interceptor ErrorInterceptor) {
	h.mu.Lock()
	h.interceptors = append(h.interceptors, interceptor)
	h.mu.Unlock()
}

// GetMetrics returns current error metrics
func (h *ErrorHandler) GetMetrics() *ErrorMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	// Create a deep copy
	metrics := &ErrorMetrics{
		TotalErrors:      h.metrics.TotalErrors,
		ErrorsByType:     make(map[ErrorCode]int64),
		ErrorsBySeverity: make(map[string]int64),
		RecoveryRate:     h.metrics.RecoveryRate,
		LastError:        h.metrics.LastError,
		MostCommonError:  h.metrics.MostCommonError,
		ErrorTrends:      make(map[string][]ErrorTrend),
	}

	for k, v := range h.metrics.ErrorsByType {
		metrics.ErrorsByType[k] = v
	}

	for k, v := range h.metrics.ErrorsBySeverity {
		metrics.ErrorsBySeverity[k] = v
	}

	for k, v := range h.metrics.ErrorTrends {
		trends := make([]ErrorTrend, len(v))
		copy(trends, v)
		metrics.ErrorTrends[k] = trends
	}

	return metrics
}

// Close closes the error handler
func (h *ErrorHandler) Close() error {
	h.cancel()
	return h.tracker.Stop()
}

// Helper functions
func (h *ErrorHandler) generateErrorCode(operation, message string) string {
	// Generate a consistent error code based on operation and message
	hash := 0
	for _, c := range message {
		hash = hash*31 + int(c)
	}
	return fmt.Sprintf("%s_%04X", strings.ToUpper(operation), hash&0xFFFF)
}

// Note: Convenience functions moved to s3ry_errors.go to avoid conflicts
