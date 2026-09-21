package s3

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
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
	ErrInvalid       = &Error{Kind: KindInvalid}
	ErrExists        = &Error{Kind: KindExists}
	ErrUnsupported   = &Error{Kind: KindUnsupported}
)

// Classify wraps err in an *Error with the best matching Kind. Errors that
// are already *Error values pass through unchanged.
func Classify(op, bucket, key string, err error) error {
	if err == nil {
		return nil
	}

	if e, ok := err.(*Error); ok {
		return e
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &Error{Kind: KindCanceled, Op: op, Bucket: bucket, Key: key, Err: err}
	}

	var invalidToken *ssocreds.InvalidTokenError
	if errors.As(err, &invalidToken) && invalidToken != nil {
		return &Error{Kind: KindNoCredentials, Op: op, Bucket: bucket, Key: key, Err: err}
	}

	var noSuchKey *s3types.NoSuchKey
	var noSuchBucket *s3types.NoSuchBucket
	var notFound *s3types.NotFound
	if (errors.As(err, &noSuchKey) && noSuchKey != nil) ||
		(errors.As(err, &noSuchBucket) && noSuchBucket != nil) ||
		(errors.As(err, &notFound) && notFound != nil) {
		return &Error{Kind: KindNotFound, Op: op, Bucket: bucket, Key: key, Err: err}
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		switch apiErr.ErrorCode() {
		case "AccessDenied", "AllAccessDisabled", "InvalidAccessKeyId", "SignatureDoesNotMatch", "ExpiredToken", "InvalidToken", "AccountProblem":
			return &Error{Kind: KindAccessDenied, Op: op, Bucket: bucket, Key: key, Err: err}
		case "NotImplemented", "MethodNotAllowed":
			return &Error{Kind: KindUnsupported, Op: op, Bucket: bucket, Key: key, Err: err}
		case "NoSuchBucket", "NoSuchKey", "NotFound":
			return &Error{Kind: KindNotFound, Op: op, Bucket: bucket, Key: key, Err: err}
		}
	}

	if retry.IsErrorThrottles(retry.DefaultThrottles).IsErrorThrottle(err) == aws.TrueTernary {
		return &Error{Kind: KindThrottled, Op: op, Bucket: bucket, Key: key, Err: err}
	}

	if status, ok := responseStatus(err); ok {
		switch status {
		case 404:
			return &Error{Kind: KindNotFound, Op: op, Bucket: bucket, Key: key, Err: err}
		case 401, 403:
			return &Error{Kind: KindAccessDenied, Op: op, Bucket: bucket, Key: key, Err: err}
		case 429, 503:
			return &Error{Kind: KindThrottled, Op: op, Bucket: bucket, Key: key, Err: err}
		case 501:
			return &Error{Kind: KindUnsupported, Op: op, Bucket: bucket, Key: key, Err: err}
		}
	}

	for current := err; current != nil; current = errors.Unwrap(current) {
		operationErr, ok := current.(*smithy.OperationError)
		if ok && isCredentialService(operationErr.ServiceID) {
			return &Error{Kind: KindNoCredentials, Op: op, Bucket: bucket, Key: key, Err: err}
		}
	}

	return &Error{Kind: KindUnknown, Op: op, Bucket: bucket, Key: key, Err: err}
}

func deleteNotImplemented(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr != nil && apiErr.ErrorCode() == "NotImplemented" {
		return true
	}

	status, ok := responseStatus(err)
	return ok && status == http.StatusNotImplemented
}

// responseStatus extracts the HTTP status code from an error chain. The
// embedded *smithyhttp.ResponseError may be nil, and promoted field access
// through a nil embedded pointer panics, so each link is checked directly.
func responseStatus(err error) (int, bool) {
	var responseErr *awshttp.ResponseError
	if !errors.As(err, &responseErr) || responseErr == nil {
		return 0, false
	}
	embedded := responseErr.ResponseError
	if embedded == nil || embedded.Response == nil || embedded.Response.Response == nil {
		return 0, false
	}
	return responseErr.HTTPStatusCode(), true
}

func isCredentialService(serviceID string) bool {
	switch serviceID {
	case "ec2imds", "SSO", "SSO OIDC", "STS":
		return true
	default:
		return false
	}
}

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
