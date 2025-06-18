package errors

import (
	"context"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
)

// Use types from s3ry_errors.go to avoid conflicts

// Methods for S3ryError are defined in s3ry_errors.go

// EnhancedErrorHandler は強化されたエラーハンドラー
type EnhancedErrorHandler struct {
	mu                 sync.RWMutex
	errorPatterns      map[string]*EnhancedErrorPattern
	recoveryStrategies map[ErrorCode]*RecoveryStrategy
	retryPolicies      map[ErrorCode]*RetryPolicy
	userMessageMap     map[string]string
	logger             Logger
	tracker            *AdvancedErrorTracker
	metrics            *EnhancedErrorMetrics
}

// EnhancedErrorPattern はエラーパターンの定義
type EnhancedErrorPattern struct {
	Pattern     string    `json:"pattern"`
	Type        ErrorCode `json:"type"`
	Severity    string    `json:"severity"`
	Recoverable bool      `json:"recoverable"`
	UserMessage string    `json:"user_message"`
	HelpURL     string    `json:"help_url"`
	Suggestions []string  `json:"suggestions"`
}

// RecoveryStrategy はリカバリ戦略
type RecoveryStrategy struct {
	Type        ErrorCode                                                    `json:"type"`
	MaxRetries  int                                                          `json:"max_retries"`
	BackoffFunc func(attempt int) time.Duration                              `json:"-"`
	RecoverFunc func(ctx context.Context, err *S3ryError) (*S3ryError, bool) `json:"-"`
	Conditions  []string                                                     `json:"conditions"`
}

// RetryPolicy はリトライポリシー
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
	Jitter        bool          `json:"jitter"`
}

// EnhancedErrorMetrics はエラーメトリクス
type EnhancedErrorMetrics struct {
	mu                   sync.RWMutex
	totalErrors          int64
	errorsByType         map[ErrorCode]int64
	errorsBySeverity     map[string]int64
	errorsByOperation    map[string]int64
	recoveryAttempts     int64
	successfulRecoveries int64
}

// Logger はログインターフェース
type Logger interface {
	Error(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
	WithError(err error) Logger
}

// NewEnhancedErrorHandler は新しいエラーハンドラーを作成
func NewEnhancedErrorHandler(logger Logger, tracker *AdvancedErrorTracker) *EnhancedErrorHandler {
	handler := &EnhancedErrorHandler{
		errorPatterns:      make(map[string]*EnhancedErrorPattern),
		recoveryStrategies: make(map[ErrorCode]*RecoveryStrategy),
		retryPolicies:      make(map[ErrorCode]*RetryPolicy),
		userMessageMap:     make(map[string]string),
		logger:             logger,
		tracker:            tracker,
		metrics: &EnhancedErrorMetrics{
			errorsByType:      make(map[ErrorCode]int64),
			errorsBySeverity:  make(map[string]int64),
			errorsByOperation: make(map[string]int64),
		},
	}

	// デフォルトパターンを初期化
	handler.initializeDefaultPatterns()

	// デフォルトリカバリ戦略を初期化
	handler.initializeRecoveryStrategies()

	// デフォルトリトライポリシーを初期化
	handler.initializeRetryPolicies()

	return handler
}

// HandleError はエラーを処理
func (h *EnhancedErrorHandler) HandleError(ctx context.Context, err error, operation, component string) *S3ryError {
	if err == nil {
		return nil
	}

	// S3ryErrorの場合はそのまま返す
	if s3ryErr, ok := err.(*S3ryError); ok {
		h.updateMetrics(s3ryErr)
		h.logError(s3ryErr)
		return s3ryErr
	}

	// 新しいS3ryErrorを作成
	s3ryErr := h.createS3ryError(err, operation, component)

	// パターンマッチング
	h.matchPattern(s3ryErr)

	// メトリクス更新
	h.updateMetrics(s3ryErr)

	// ログ出力
	h.logError(s3ryErr)

	// エラー追跡
	if h.tracker != nil {
		h.tracker.TrackError(
			s3ryErr.Operation,
			string(s3ryErr.Code),
			s3ryErr.Message,
			string(s3ryErr.Code),
			s3ryErr.Context,
		)
	}

	return s3ryErr
}

// HandleAWSError はAWSエラーを処理
func (h *EnhancedErrorHandler) HandleAWSError(ctx context.Context, err error, operation, component string) *S3ryError {
	if err == nil {
		return nil
	}

	s3ryErr := h.HandleError(ctx, err, operation, component)

	// AWS固有のエラー処理
	if awsErr, ok := err.(awserr.Error); ok {
		s3ryErr.Code = h.classifyAWSError(awsErr)
		if s3ryErr.Context == nil {
			s3ryErr.Context = make(map[string]interface{})
		}
		s3ryErr.Context["aws_code"] = awsErr.Code()
		s3ryErr.Context["severity"] = h.determineSeverity(awsErr)

		// AWS固有のコンテキスト情報を追加
		if s3ryErr.Context == nil {
			s3ryErr.Context = make(map[string]interface{})
		}
		s3ryErr.Context["aws_error_code"] = awsErr.Code()
		s3ryErr.Context["aws_error_message"] = awsErr.Message()

		if reqErr, ok := err.(awserr.RequestFailure); ok {
			s3ryErr.Context["aws_status_code"] = reqErr.StatusCode()
			s3ryErr.Context["aws_request_id"] = reqErr.RequestID()
		}

		// ユーザーフレンドリーなメッセージを設定
		s3ryErr.Context["user_message"] = h.getAWSUserMessage(awsErr)
		s3ryErr.Context["suggestions"] = h.getAWSSuggestions(awsErr)
		s3ryErr.Context["help_url"] = h.getAWSHelpURL(awsErr)
	}

	return s3ryErr
}

// RecoverError はエラーからの回復を試行
func (h *EnhancedErrorHandler) RecoverError(ctx context.Context, err *S3ryError) (*S3ryError, bool) {
	if err == nil {
		return err, false
	}

	// Check if recoverable from context
	recoverable := false
	if r, ok := err.Context["recoverable"].(bool); ok {
		recoverable = r
	}

	if !recoverable {
		return err, false
	}

	h.mu.RLock()
	strategy, exists := h.recoveryStrategies[err.Code]
	h.mu.RUnlock()

	if !exists || strategy.RecoverFunc == nil {
		return err, false
	}

	h.metrics.mu.Lock()
	h.metrics.recoveryAttempts++
	h.metrics.mu.Unlock()

	h.logger.WithFields(map[string]interface{}{
		"error_code": string(err.Code),
		"operation":  err.Operation,
	}).Info("Attempting error recovery")

	recoveredErr, success := strategy.RecoverFunc(ctx, err)

	if success {
		h.metrics.mu.Lock()
		h.metrics.successfulRecoveries++
		h.metrics.mu.Unlock()

		h.logger.WithField("error_code", string(err.Code)).Info("Error recovery successful")
	} else {
		h.logger.WithField("error_code", string(err.Code)).Warn("Error recovery failed")
	}

	return recoveredErr, success
}

// RetryWithPolicy はポリシーに基づいてリトライ
func (h *EnhancedErrorHandler) RetryWithPolicy(ctx context.Context, errorCode ErrorCode, fn func() error) error {
	h.mu.RLock()
	policy, exists := h.retryPolicies[errorCode]
	h.mu.RUnlock()

	if !exists {
		return fn()
	}

	var lastErr error
	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := h.calculateBackoff(policy, attempt)
			h.logger.WithFields(map[string]interface{}{
				"attempt": attempt,
				"delay":   delay,
				"type":    string(errorCode),
			}).Debug("Retrying operation")

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		if err := fn(); err != nil {
			lastErr = err

			// リトライ可能かチェック
			if s3ryErr, ok := err.(*S3ryError); ok {
				if r, ok := s3ryErr.Context["recoverable"].(bool); ok && !r {
					return err
				}
			}

			continue
		}

		return nil
	}

	return lastErr
}

// createS3ryError は基本的なS3ryErrorを作成
func (h *EnhancedErrorHandler) createS3ryError(err error, operation, component string) *S3ryError {
	// スタックトレースを取得
	stackTrace := h.captureStackTrace(3)

	s3ryErr := &S3ryError{
		Code:      ErrCodeUnknown,
		Message:   err.Error(),
		Operation: operation,
		Timestamp: time.Now(),
		Context:   make(map[string]interface{}),
		Cause:     err,
	}

	s3ryErr.Context["component"] = component
	s3ryErr.Context["stack_trace"] = stackTrace
	s3ryErr.Context["recoverable"] = false
	s3ryErr.Context["severity"] = "medium"

	// コンテキスト情報を追加
	s3ryErr.Context["go_version"] = runtime.Version()
	s3ryErr.Context["goroutines"] = runtime.NumGoroutine()

	return s3ryErr
}

// matchPattern はエラーパターンをマッチング
func (h *EnhancedErrorHandler) matchPattern(err *S3ryError) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for patternStr, pattern := range h.errorPatterns {
		if strings.Contains(err.Message, patternStr) || strings.Contains(string(err.Code), patternStr) {
			err.Code = pattern.Type
			err.Context["severity"] = pattern.Severity
			err.Context["recoverable"] = pattern.Recoverable
			if pattern.UserMessage != "" {
				err.Context["user_message"] = pattern.UserMessage
			}
			if pattern.HelpURL != "" {
				err.Context["help_url"] = pattern.HelpURL
			}
			if len(pattern.Suggestions) > 0 {
				err.Context["suggestions"] = pattern.Suggestions
			}
			break
		}
	}
}

// classifyAWSError はAWSエラーを分類
func (h *EnhancedErrorHandler) classifyAWSError(awsErr awserr.Error) ErrorCode {
	code := awsErr.Code()

	switch {
	case strings.Contains(code, "NoSuchBucket") || strings.Contains(code, "NoSuchKey"):
		return ErrCodeValidation
	case strings.Contains(code, "AccessDenied") || strings.Contains(code, "Forbidden"):
		return ErrCodePermission
	case strings.Contains(code, "InvalidAccessKeyId") || strings.Contains(code, "SignatureDoesNotMatch"):
		return ErrCodeS3Permission
	case strings.Contains(code, "RequestTimeout") || strings.Contains(code, "Timeout"):
		return ErrCodeTimeout
	case strings.Contains(code, "Throttling") || strings.Contains(code, "SlowDown"):
		return ErrCodeTimeout
	case strings.Contains(code, "ServiceUnavailable") || strings.Contains(code, "InternalError"):
		return ErrCodeNetwork
	case strings.Contains(code, "QuotaExceeded") || strings.Contains(code, "LimitExceeded"):
		return ErrCodeS3Connection
	default:
		return ErrCodeS3Connection
	}
}

// determineSeverity は重要度を決定
func (h *EnhancedErrorHandler) determineSeverity(awsErr awserr.Error) string {
	code := awsErr.Code()

	switch {
	case strings.Contains(code, "InternalError") || strings.Contains(code, "ServiceUnavailable"):
		return "critical"
	case strings.Contains(code, "AccessDenied") || strings.Contains(code, "InvalidAccessKeyId"):
		return "high"
	case strings.Contains(code, "NoSuchBucket") || strings.Contains(code, "NoSuchKey"):
		return "medium"
	default:
		return "low"
	}
}

// updateMetrics はメトリクスを更新
func (h *EnhancedErrorHandler) updateMetrics(err *S3ryError) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()

	h.metrics.totalErrors++
	h.metrics.errorsByType[err.Code]++
	if severity, ok := err.Context["severity"].(string); ok {
		h.metrics.errorsBySeverity[severity]++
	}
	h.metrics.errorsByOperation[err.Operation]++
}

// logError はエラーをログ出力
func (h *EnhancedErrorHandler) logError(err *S3ryError) {
	fields := map[string]interface{}{
		"error_code": string(err.Code),
		"operation":  err.Operation,
		"timestamp":  err.Timestamp,
	}

	// Add context fields
	for key, value := range err.Context {
		fields[key] = value
	}

	logger := h.logger.WithFields(fields).WithError(err.Cause)

	severity := "info"
	if s, ok := err.Context["severity"].(string); ok {
		severity = s
	}

	switch severity {
	case "critical":
		logger.Error("Critical error occurred: %s", err.Message)
	case "high":
		logger.Error("High severity error occurred: %s", err.Message)
	case "medium":
		logger.Warn("Medium severity error occurred: %s", err.Message)
	case "low":
		logger.Info("Low severity error occurred: %s", err.Message)
	default:
		logger.Info("Error occurred: %s", err.Message)
	}
}

// captureStackTrace はスタックトレースを取得
func (h *EnhancedErrorHandler) captureStackTrace(skip int) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// calculateBackoff はバックオフ時間を計算
func (h *EnhancedErrorHandler) calculateBackoff(policy *RetryPolicy, attempt int) time.Duration {
	delay := time.Duration(float64(policy.InitialDelay) *
		(policy.BackoffFactor * float64(attempt)))

	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	// ジッターを追加
	if policy.Jitter {
		jitter := time.Duration(float64(delay) * 0.1)
		delay += time.Duration(float64(jitter) * (2.0*float64(time.Now().UnixNano()%1000)/1000.0 - 1.0))
	}

	return delay
}

// GetMetrics はエラーメトリクスを取得
func (h *EnhancedErrorHandler) GetMetrics() *EnhancedErrorMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	metrics := &EnhancedErrorMetrics{
		totalErrors:          h.metrics.totalErrors,
		errorsByType:         make(map[ErrorCode]int64),
		errorsBySeverity:     make(map[string]int64),
		errorsByOperation:    make(map[string]int64),
		recoveryAttempts:     h.metrics.recoveryAttempts,
		successfulRecoveries: h.metrics.successfulRecoveries,
	}

	for k, v := range h.metrics.errorsByType {
		metrics.errorsByType[k] = v
	}
	for k, v := range h.metrics.errorsBySeverity {
		metrics.errorsBySeverity[k] = v
	}
	for k, v := range h.metrics.errorsByOperation {
		metrics.errorsByOperation[k] = v
	}

	return metrics
}
