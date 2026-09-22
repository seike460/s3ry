package s3

import (
	"fmt"
	"strings"
)

// Kind classifies an Error so callers can switch on the failure mode
// instead of matching message text.
type Kind int

const (
	// KindUnknown is an error that did not match a known category.
	KindUnknown Kind = iota
	// KindNotFound means the bucket or object does not exist.
	KindNotFound
	// KindAccessDenied means the request was authenticated but rejected.
	KindAccessDenied
	// KindNoCredentials means no usable AWS credentials were found.
	KindNoCredentials
	// KindThrottled means the service asked the client to slow down.
	KindThrottled
	// KindCanceled means the caller canceled the operation.
	KindCanceled
	// KindTimeout means the operation exceeded its deadline.
	KindTimeout
	// KindInvalid means the request arguments failed validation.
	KindInvalid
	// KindExists means the local destination already exists.
	KindExists
	// KindUnsupported means the endpoint does not implement the operation.
	KindUnsupported
)

func (k Kind) String() string {
	switch k {
	case KindNotFound:
		return "not_found"
	case KindAccessDenied:
		return "access_denied"
	case KindNoCredentials:
		return "no_credentials"
	case KindThrottled:
		return "throttled"
	case KindCanceled:
		return "canceled"
	case KindTimeout:
		return "timeout"
	case KindInvalid:
		return "invalid"
	case KindExists:
		return "exists"
	case KindUnsupported:
		return "unsupported"
	default:
		return "unknown"
	}
}

func (k Kind) sentence() string {
	switch k {
	case KindNotFound:
		return "not found"
	case KindAccessDenied:
		return "access denied"
	case KindNoCredentials:
		return "no AWS credentials found (configure a profile or run aws sso login)"
	case KindThrottled:
		return "throttled by S3"
	case KindCanceled:
		return "canceled"
	case KindTimeout:
		return "timed out"
	case KindInvalid:
		return "invalid input"
	case KindExists:
		return "already exists"
	case KindUnsupported:
		return "not supported by this endpoint"
	default:
		return "unknown error"
	}
}

// Error is the package's classified error. Kind is the machine-readable
// category; Op, Bucket, and Key locate the failed operation.
type Error struct {
	Kind   Kind
	Op     string
	Bucket string
	Key    string
	Err    error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}

	prefix := e.Op
	if e.Bucket != "" {
		resource := (URL{Bucket: e.Bucket, Key: e.Key}).String()
		if prefix != "" {
			prefix += " "
		}
		prefix += resource
	} else if e.Key != "" {
		if prefix != "" {
			prefix += " "
		}
		prefix += e.Key
	}

	message := e.Kind.sentence()
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	if prefix != "" {
		return prefix + ": " + message
	}
	return message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Is reports whether target is an Error of the same Kind, enabling
// errors.Is(err, s3.ErrNotFound) style checks.
func (e *Error) Is(target error) bool {
	targetErr, ok := target.(*Error)
	return ok && e != nil && targetErr != nil && e.Kind == targetErr.Kind
}

// Sentinel errors for errors.Is checks against a classified Kind.
var (
	ErrNotFound      = &Error{Kind: KindNotFound}
	ErrAccessDenied  = &Error{Kind: KindAccessDenied}
	ErrNoCredentials = &Error{Kind: KindNoCredentials}
	ErrThrottled     = &Error{Kind: KindThrottled}
	ErrCanceled      = &Error{Kind: KindCanceled}
	ErrTimeout       = &Error{Kind: KindTimeout}
	ErrInvalid       = &Error{Kind: KindInvalid}
	ErrExists        = &Error{Kind: KindExists}
	ErrUnsupported   = &Error{Kind: KindUnsupported}
)

// KeyError pairs a failed object key with its error in bulk operations.
type KeyError struct {
	Key string
	Err error
}

// BulkError aggregates the per-key failures of a multi-object operation.
type BulkError struct {
	Errors []KeyError
}

func (b *BulkError) Unwrap() []error {
	if b == nil {
		return nil
	}

	errs := make([]error, 0, len(b.Errors))
	for _, item := range b.Errors {
		errs = append(errs, item.Err)
	}
	return errs
}

func (b *BulkError) Error() string {
	if b == nil || len(b.Errors) == 0 {
		return "0 errors"
	}

	limit := len(b.Errors)
	if limit > 3 {
		limit = 3
	}
	summary := make([]string, 0, limit+1)
	for _, item := range b.Errors[:limit] {
		if item.Err == nil {
			summary = append(summary, item.Key)
			continue
		}
		if item.Key == "" {
			summary = append(summary, item.Err.Error())
			continue
		}
		summary = append(summary, fmt.Sprintf("%s: %v", item.Key, item.Err))
	}
	if len(b.Errors) > limit {
		summary = append(summary, fmt.Sprintf("... and %d more", len(b.Errors)-limit))
	}
	countName := "errors"
	if len(b.Errors) == 1 {
		countName = "error"
	}
	return fmt.Sprintf("%d %s: %s", len(b.Errors), countName, strings.Join(summary, "; "))
}
