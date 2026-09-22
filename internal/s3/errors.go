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

// kindNames maps each Kind to its stable machine name.
var kindNames = map[Kind]string{
	KindNotFound:      "not_found",
	KindAccessDenied:  "access_denied",
	KindNoCredentials: "no_credentials",
	KindThrottled:     "throttled",
	KindCanceled:      "canceled",
	KindTimeout:       "timeout",
	KindInvalid:       "invalid",
	KindExists:        "exists",
	KindUnsupported:   "unsupported",
}

func (k Kind) String() string {
	if name, ok := kindNames[k]; ok {
		return name
	}
	return "unknown"
}

// kindSentences maps each Kind to its human-readable sentence.
// #nosec G101 -- these are user-facing error messages, not credentials.
var kindSentences = map[Kind]string{
	KindNotFound:      "not found",
	KindAccessDenied:  "access denied",
	KindNoCredentials: "no AWS credentials found (configure a profile or run aws sso login)",
	KindThrottled:     "throttled by S3",
	KindCanceled:      "canceled",
	KindTimeout:       "timed out",
	KindInvalid:       "invalid input",
	KindExists:        "already exists",
	KindUnsupported:   "not supported by this endpoint",
}

func (k Kind) sentence() string {
	if sentence, ok := kindSentences[k]; ok {
		return sentence
	}
	return "unknown error"
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
	countName := "errors"
	if len(b.Errors) == 1 {
		countName = "error"
	}
	return fmt.Sprintf("%d %s: %s", len(b.Errors), countName, strings.Join(b.errorSummary(), "; "))
}

// errorSummary renders up to three item messages plus an overflow marker.
func (b *BulkError) errorSummary() []string {
	limit := min(len(b.Errors), 3)
	summary := make([]string, 0, limit+1)
	for _, item := range b.Errors[:limit] {
		summary = append(summary, item.message())
	}
	if len(b.Errors) > limit {
		summary = append(summary, fmt.Sprintf("... and %d more", len(b.Errors)-limit))
	}
	return summary
}

// message formats the item as "key: err", tolerating empty parts.
func (item KeyError) message() string {
	if item.Err == nil {
		return item.Key
	}
	if item.Key == "" {
		return item.Err.Error()
	}
	return fmt.Sprintf("%s: %v", item.Key, item.Err)
}
