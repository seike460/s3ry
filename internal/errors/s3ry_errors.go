package errors

import (
	"fmt"
	"runtime"
	"time"
)

// ErrorCode represents different types of errors in S3ry
type ErrorCode string

const (
	// S3 related errors
	ErrCodeS3Connection    ErrorCode = "S3_CONNECTION"
	ErrCodeS3Permission    ErrorCode = "S3_PERMISSION"
	ErrCodeS3NotFound      ErrorCode = "S3_NOT_FOUND"
	ErrCodeS3InvalidBucket ErrorCode = "S3_INVALID_BUCKET"
	ErrCodeS3InvalidKey    ErrorCode = "S3_INVALID_KEY"

	// Validation errors
	ErrCodeValidation    ErrorCode = "VALIDATION"
	ErrCodeInvalidInput  ErrorCode = "INVALID_INPUT"
	ErrCodeInvalidConfig ErrorCode = "INVALID_CONFIG"

	// Worker/Concurrency errors
	ErrCodeWorkerPool   ErrorCode = "WORKER_POOL"
	ErrCodeJobExecution ErrorCode = "JOB_EXECUTION"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeCancelled    ErrorCode = "CANCELLED"

	// File system errors
	ErrCodeFileSystem   ErrorCode = "FILE_SYSTEM"
	ErrCodeFileNotFound ErrorCode = "FILE_NOT_FOUND"
	ErrCodePermission   ErrorCode = "PERMISSION"

	// Network errors
	ErrCodeNetwork ErrorCode = "NETWORK"
	ErrCodeDNS     ErrorCode = "DNS"
	ErrCodeTLS     ErrorCode = "TLS"

	// Internal errors
	ErrCodeInternal ErrorCode = "INTERNAL"
	ErrCodeUnknown  ErrorCode = "UNKNOWN"
)

// S3ryError represents a structured error in S3ry
type S3ryError struct {
	Code      ErrorCode              `json:"code"`
	Operation string                 `json:"operation"`
	Message   string                 `json:"message"`
	Cause     error                  `json:"cause,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Stack     []StackFrame           `json:"stack,omitempty"`
}

// StackFrame represents a single frame in the call stack
type StackFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// Error implements the error interface
func (e *S3ryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("s3ry %s [%s]: %s: %v", e.Operation, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("s3ry %s [%s]: %s", e.Operation, e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *S3ryError) Unwrap() error {
	return e.Cause
}

// Is checks if the error matches the target error code
func (e *S3ryError) Is(target error) bool {
	if t, ok := target.(*S3ryError); ok {
		return e.Code == t.Code
	}
	return false
}

// WithContext adds context information to the error
func (e *S3ryError) WithContext(key string, value interface{}) *S3ryError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// WithStack captures the current call stack
func (e *S3ryError) WithStack() *S3ryError {
	e.Stack = captureStack(2) // Skip this function and the caller
	return e
}

// New creates a new S3ryError
func New(code ErrorCode, operation, message string) *S3ryError {
	return &S3ryError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// Wrap wraps an existing error with S3ry error information
func Wrap(err error, code ErrorCode, operation, message string) *S3ryError {
	return &S3ryError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Cause:     err,
		Timestamp: time.Now(),
	}
}

// captureStack captures the current call stack
func captureStack(skip int) []StackFrame {
	var frames []StackFrame

	for i := skip; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}

		frames = append(frames, StackFrame{
			Function: fn.Name(),
			File:     file,
			Line:     line,
		})

		// Limit stack depth to prevent excessive memory usage
		if len(frames) >= 10 {
			break
		}
	}

	return frames
}

// Convenience functions for common error types

// NewS3Error creates a new S3-related error
func NewS3Error(operation, message string, cause error) *S3ryError {
	return Wrap(cause, ErrCodeS3Connection, operation, message)
}

// NewValidationError creates a new validation error
func NewValidationError(operation, message string) *S3ryError {
	return New(ErrCodeValidation, operation, message)
}

// NewFileSystemError creates a new file system error
func NewFileSystemError(operation, message string, cause error) *S3ryError {
	return Wrap(cause, ErrCodeFileSystem, operation, message)
}

// NewWorkerError creates a new worker pool error
func NewWorkerError(operation, message string, cause error) *S3ryError {
	return Wrap(cause, ErrCodeWorkerPool, operation, message)
}

// NewTimeoutError creates a new timeout error
func NewTimeoutError(operation, message string) *S3ryError {
	return New(ErrCodeTimeout, operation, message)
}

// NewNetworkError creates a new network error
func NewNetworkError(operation, message string, cause error) *S3ryError {
	return Wrap(cause, ErrCodeNetwork, operation, message)
}

// IsS3Error checks if the error is an S3-related error
func IsS3Error(err error) bool {
	if s3ryErr, ok := err.(*S3ryError); ok {
		switch s3ryErr.Code {
		case ErrCodeS3Connection, ErrCodeS3Permission, ErrCodeS3NotFound,
			ErrCodeS3InvalidBucket, ErrCodeS3InvalidKey:
			return true
		}
	}
	return false
}

// IsValidationError checks if the error is a validation error
func IsValidationError(err error) bool {
	if s3ryErr, ok := err.(*S3ryError); ok {
		return s3ryErr.Code == ErrCodeValidation || s3ryErr.Code == ErrCodeInvalidInput
	}
	return false
}

// IsTimeoutError checks if the error is a timeout error
func IsTimeoutError(err error) bool {
	if s3ryErr, ok := err.(*S3ryError); ok {
		return s3ryErr.Code == ErrCodeTimeout
	}
	return false
}

// IsNetworkError checks if the error is a network error
func IsNetworkError(err error) bool {
	if s3ryErr, ok := err.(*S3ryError); ok {
		switch s3ryErr.Code {
		case ErrCodeNetwork, ErrCodeDNS, ErrCodeTLS:
			return true
		}
	}
	return false
}

// ErrorCollector collects multiple errors
type ErrorCollector struct {
	errors []error
}

// NewErrorCollector creates a new error collector
func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{}
}

// Add adds an error to the collector
func (ec *ErrorCollector) Add(err error) {
	if err != nil {
		ec.errors = append(ec.errors, err)
	}
}

// HasErrors returns true if there are any errors
func (ec *ErrorCollector) HasErrors() bool {
	return len(ec.errors) > 0
}

// Errors returns all collected errors
func (ec *ErrorCollector) Errors() []error {
	return ec.errors
}

// Error returns a combined error message
func (ec *ErrorCollector) Error() error {
	if len(ec.errors) == 0 {
		return nil
	}

	if len(ec.errors) == 1 {
		return ec.errors[0]
	}

	var messages []string
	for _, err := range ec.errors {
		messages = append(messages, err.Error())
	}

	return New(ErrCodeInternal, "multiple_errors",
		fmt.Sprintf("multiple errors occurred: %v", messages))
}

// Clear clears all collected errors
func (ec *ErrorCollector) Clear() {
	ec.errors = nil
}
