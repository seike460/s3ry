package cloud

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/seike460/s3ry/internal/errors"
)

// CloudErrorHandler provides unified error handling across cloud providers
type CloudErrorHandler struct {
	provider    CloudProvider
	logger      Logger
	retryConfig *RetryConfig
}

// RetryConfig defines retry behavior for cloud operations
type RetryConfig struct {
	MaxRetries      int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffMultiple float64
	RetryableErrors []string
}

// NewCloudErrorHandler creates a new cloud error handler
func NewCloudErrorHandler(provider CloudProvider, logger Logger) *CloudErrorHandler {
	return &CloudErrorHandler{
		provider: provider,
		logger:   logger,
		retryConfig: &RetryConfig{
			MaxRetries:      3,
			InitialDelay:    100 * time.Millisecond,
			MaxDelay:        5 * time.Second,
			BackoffMultiple: 2.0,
			RetryableErrors: getRetryableErrorsForProvider(provider),
		},
	}
}

// HandleError converts provider-specific errors to S3ry unified errors
func (h *CloudErrorHandler) HandleError(err error, operation string, context map[string]interface{}) *errors.S3ryError {
	if err == nil {
		return nil
	}

	// Convert to S3ry error with provider context
	s3ryErr := h.convertToS3ryError(err, operation)

	// Add cloud provider context
	if s3ryErr.Context == nil {
		s3ryErr.Context = make(map[string]interface{})
	}
	s3ryErr.Context["provider"] = h.provider.String()
	s3ryErr.Context["operation"] = operation

	// Merge additional context
	for k, v := range context {
		s3ryErr.Context[k] = v
	}

	// Log the error with full context
	h.logger.Error("Cloud operation failed: %s [%s] - %s",
		operation, h.provider.String(), s3ryErr.Error())

	return s3ryErr
}

// ExecuteWithRetry executes an operation with retry logic
func (h *CloudErrorHandler) ExecuteWithRetry(ctx context.Context, operation string, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= h.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff
			delay := h.calculateDelay(attempt)
			h.logger.Debug("Retrying %s operation (attempt %d/%d) after %v",
				operation, attempt+1, h.retryConfig.MaxRetries+1, delay)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				h.logger.Info("Operation %s succeeded after %d retries", operation, attempt)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !h.isRetryableError(err) {
			h.logger.Debug("Error not retryable for operation %s: %v", operation, err)
			break
		}

		// Check if we should continue retrying
		if attempt >= h.retryConfig.MaxRetries {
			h.logger.Warn("Max retries reached for operation %s", operation)
			break
		}
	}

	// Convert to S3ry error with retry context
	s3ryErr := h.convertToS3ryError(lastErr, operation)
	if s3ryErr.Context == nil {
		s3ryErr.Context = make(map[string]interface{})
	}
	s3ryErr.Context["max_retries_reached"] = true
	s3ryErr.Context["retry_attempts"] = h.retryConfig.MaxRetries + 1

	return s3ryErr
}

// convertToS3ryError converts provider-specific errors to S3ry errors
func (h *CloudErrorHandler) convertToS3ryError(err error, operation string) *errors.S3ryError {
	errorStr := err.Error()

	// Determine error type based on provider and error string
	errorType := h.categorizeError(errorStr, operation)

	// Get user-friendly message and help URL
	userMessage, helpURL := h.getUserMessageAndHelp(errorType, errorStr)

	return &errors.S3ryError{
		Code:    errorType,
		Message: err.Error(),
		Cause:   err,
		Context: map[string]interface{}{
			"provider":     h.provider.String(),
			"operation":    operation,
			"user_message": userMessage,
			"help_url":     helpURL,
		},
		Timestamp: time.Now(),
	}
}

// categorizeError categorizes errors based on provider and error content
func (h *CloudErrorHandler) categorizeError(errorStr, operation string) errors.ErrorCode {
	errorLower := strings.ToLower(errorStr)

	// Network-related errors
	if strings.Contains(errorLower, "connection") ||
		strings.Contains(errorLower, "timeout") ||
		strings.Contains(errorLower, "network") ||
		strings.Contains(errorLower, "dial") {
		return errors.ErrCodeNetwork
	}

	// Authentication errors
	if strings.Contains(errorLower, "unauthorized") ||
		strings.Contains(errorLower, "access denied") ||
		strings.Contains(errorLower, "forbidden") ||
		strings.Contains(errorLower, "invalid credentials") ||
		strings.Contains(errorLower, "authentication") {
		return errors.ErrCodeS3Permission
	}

	// Permission errors
	if strings.Contains(errorLower, "permission") ||
		strings.Contains(errorLower, "not allowed") ||
		strings.Contains(errorLower, "insufficient") {
		return errors.ErrCodePermission
	}

	// Provider-specific error categorization
	switch h.provider {
	case ProviderAWS, ProviderAWSBasic:
		return h.categorizeAWSError(errorLower, operation)
	case ProviderAzure:
		return h.categorizeAzureError(errorLower, operation)
	case ProviderGCS:
		return h.categorizeGCSError(errorLower, operation)
	case ProviderMinIO:
		return h.categorizeMinIOError(errorLower, operation)
	}

	// Default to S3 error type
	return errors.ErrCodeS3Connection
}

// categorizeAWSError categorizes AWS-specific errors
func (h *CloudErrorHandler) categorizeAWSError(errorStr, operation string) errors.ErrorCode {
	if strings.Contains(errorStr, "nosuchbucket") ||
		strings.Contains(errorStr, "nosuchkey") ||
		strings.Contains(errorStr, "notfound") {
		return errors.ErrCodeS3NotFound
	}

	if strings.Contains(errorStr, "bucketalreadyexists") ||
		strings.Contains(errorStr, "bucketalreadyownedby") {
		return errors.ErrCodeS3InvalidBucket
	}

	if strings.Contains(errorStr, "invalidaccesskeyid") ||
		strings.Contains(errorStr, "signaturemismatch") {
		return errors.ErrCodeS3Permission
	}

	if strings.Contains(errorStr, "slowdown") ||
		strings.Contains(errorStr, "requesttimeout") {
		return errors.ErrCodeTimeout
	}

	return errors.ErrCodeS3Connection
}

// categorizeAzureError categorizes Azure-specific errors
func (h *CloudErrorHandler) categorizeAzureError(errorStr, operation string) errors.ErrorCode {
	if strings.Contains(errorStr, "containernotfound") ||
		strings.Contains(errorStr, "blobnotfound") {
		return errors.ErrCodeS3NotFound
	}

	if strings.Contains(errorStr, "containeralreadyexists") {
		return errors.ErrCodeS3InvalidBucket
	}

	if strings.Contains(errorStr, "authenticationfailed") {
		return errors.ErrCodeS3Permission
	}

	return errors.ErrCodeS3Connection
}

// categorizeGCSError categorizes Google Cloud Storage errors
func (h *CloudErrorHandler) categorizeGCSError(errorStr, operation string) errors.ErrorCode {
	if strings.Contains(errorStr, "bucket not exist") ||
		strings.Contains(errorStr, "object not exist") {
		return errors.ErrCodeS3NotFound
	}

	if strings.Contains(errorStr, "already exists") {
		return errors.ErrCodeS3InvalidBucket
	}

	if strings.Contains(errorStr, "unauthenticated") {
		return errors.ErrCodeS3Permission
	}

	return errors.ErrCodeS3Connection
}

// categorizeMinIOError categorizes MinIO-specific errors
func (h *CloudErrorHandler) categorizeMinIOError(errorStr, operation string) errors.ErrorCode {
	// MinIO uses S3-compatible error codes
	return h.categorizeAWSError(errorStr, operation)
}

// getUserMessageAndHelp returns user-friendly messages and help URLs
func (h *CloudErrorHandler) getUserMessageAndHelp(errorType errors.ErrorCode, errorStr string) (string, string) {
	providerStr := h.provider.String()

	switch errorType {
	case errors.ErrCodeS3Permission, errors.ErrCodePermission:
		return fmt.Sprintf("%sの認証に失敗しました。認証情報を確認してください。", providerStr),
			"https://docs.s3ry.com/troubleshooting/authentication"

	case errors.ErrCodeNetwork:
		return fmt.Sprintf("%sとの接続に問題があります。ネットワーク設定を確認してください。", providerStr),
			"https://docs.s3ry.com/troubleshooting/network"

	case errors.ErrCodeS3Connection, errors.ErrCodeS3NotFound, errors.ErrCodeS3InvalidBucket, errors.ErrCodeS3InvalidKey:
		if strings.Contains(strings.ToLower(errorStr), "notfound") ||
			strings.Contains(strings.ToLower(errorStr), "nosuchbucket") ||
			strings.Contains(strings.ToLower(errorStr), "nosuchkey") {
			return "指定されたバケットまたはオブジェクトが見つかりません。",
				"https://docs.s3ry.com/troubleshooting/not-found"
		}
		if strings.Contains(strings.ToLower(errorStr), "alreadyexists") {
			return "バケットまたはオブジェクトが既に存在します。",
				"https://docs.s3ry.com/troubleshooting/already-exists"
		}
		return fmt.Sprintf("%sでストレージ操作エラーが発生しました。", providerStr),
			"https://docs.s3ry.com/troubleshooting/storage"

	default:
		return fmt.Sprintf("%sで予期しないエラーが発生しました。", providerStr),
			"https://docs.s3ry.com/troubleshooting/general"
	}
}

// isRetryableError checks if an error should be retried
func (h *CloudErrorHandler) isRetryableError(err error) bool {
	errorStr := strings.ToLower(err.Error())

	// Common retryable errors
	retryablePatterns := []string{
		"timeout", "connection", "network", "temporary",
		"slowdown", "throttl", "rate limit", "server error",
		"internal error", "service unavailable", "bad gateway",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errorStr, pattern) {
			return true
		}
	}

	// Provider-specific retryable errors
	for _, pattern := range h.retryConfig.RetryableErrors {
		if strings.Contains(errorStr, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}

// calculateDelay calculates retry delay with exponential backoff
func (h *CloudErrorHandler) calculateDelay(attempt int) time.Duration {
	delay := float64(h.retryConfig.InitialDelay) *
		pow(h.retryConfig.BackoffMultiple, float64(attempt-1))

	delayDuration := time.Duration(delay)
	if delayDuration > h.retryConfig.MaxDelay {
		delayDuration = h.retryConfig.MaxDelay
	}

	return delayDuration
}

// getRetryableErrorsForProvider returns provider-specific retryable error patterns
func getRetryableErrorsForProvider(provider CloudProvider) []string {
	switch provider {
	case ProviderAWS, ProviderAWSBasic:
		return []string{
			"SlowDown", "RequestTimeout", "ServiceUnavailable",
			"ThrottlingException", "RequestTimeTooSkewed",
		}
	case ProviderAzure:
		return []string{
			"ServerBusy", "InternalError", "OperationTimedOut",
		}
	case ProviderGCS:
		return []string{
			"backendError", "internalError", "rateLimitExceeded",
		}
	case ProviderMinIO:
		return []string{
			"SlowDown", "RequestTimeout", "ServiceUnavailable",
		}
	default:
		return []string{}
	}
}

// pow calculates power since math.Pow requires float64
func pow(base, exp float64) float64 {
	if exp == 0 {
		return 1
	}
	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}
