package errors

import "fmt"

// ErrorCode represents different types of errors in S3ry
type ErrorCode string

const (
	ErrCodeS3Connection ErrorCode = "S3_CONNECTION"
	ErrCodeJobExecution ErrorCode = "JOB_EXECUTION"
	ErrCodeTimeout      ErrorCode = "TIMEOUT"
	ErrCodeCancelled    ErrorCode = "CANCELLED"
	ErrCodeFileSystem   ErrorCode = "FILE_SYSTEM"
	ErrCodeUnknown      ErrorCode = "UNKNOWN"
)

// S3ryError represents a structured error in S3ry
type S3ryError struct {
	Code      ErrorCode
	Operation string
	Message   string
	Cause     error
	Context   map[string]interface{}
}

// Error implements the error interface
func (e *S3ryError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("s3ry %s [%s]: %s: %v", e.Operation, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("s3ry %s [%s]: %s", e.Operation, e.Code, e.Message)
}

// WithContext adds context information to the error
func (e *S3ryError) WithContext(key string, value interface{}) *S3ryError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// Wrap wraps an existing error with S3ry error information
func Wrap(err error, code ErrorCode, operation, message string) *S3ryError {
	return &S3ryError{
		Code:      code,
		Operation: operation,
		Message:   message,
		Cause:     err,
	}
}
