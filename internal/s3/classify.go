package s3

import (
	"context"
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
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
	return &Error{Kind: classifyKind(err), Op: op, Bucket: bucket, Key: key, Err: err}
}

// classifyKind maps err to the most specific Kind it recognises.
func classifyKind(err error) Kind {
	for _, match := range kindMatchers {
		if kind, ok := match(err); ok {
			return kind
		}
	}
	return KindUnknown
}

// kindMatchers runs in order; the first match wins.
var kindMatchers = []func(error) (Kind, bool){
	classifyContext,
	classifyToken,
	classifyNotFoundType,
	classifyAPICode,
	classifyThrottle,
	classifyStatus,
	classifyCredentialService,
}

func classifyContext(err error) (Kind, bool) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return KindTimeout, true
	case errors.Is(err, context.Canceled):
		return KindCanceled, true
	default:
		return 0, false
	}
}

func classifyToken(err error) (Kind, bool) {
	var invalidToken *ssocreds.InvalidTokenError
	if errors.As(err, &invalidToken) && invalidToken != nil {
		return KindNoCredentials, true
	}
	return 0, false
}

func classifyNotFoundType(err error) (Kind, bool) {
	var noSuchKey *s3types.NoSuchKey
	var noSuchBucket *s3types.NoSuchBucket
	var notFound *s3types.NotFound
	if (errors.As(err, &noSuchKey) && noSuchKey != nil) ||
		(errors.As(err, &noSuchBucket) && noSuchBucket != nil) ||
		(errors.As(err, &notFound) && notFound != nil) {
		return KindNotFound, true
	}
	return 0, false
}

// apiErrorCodes maps S3 error codes to their Kind.
var apiErrorCodes = map[string]Kind{
	"AccessDenied":          KindAccessDenied,
	"AllAccessDisabled":     KindAccessDenied,
	"InvalidAccessKeyId":    KindAccessDenied,
	"SignatureDoesNotMatch": KindAccessDenied,
	"ExpiredToken":          KindAccessDenied,
	"InvalidToken":          KindAccessDenied,
	"AccountProblem":        KindAccessDenied,
	"NotImplemented":        KindUnsupported,
	"MethodNotAllowed":      KindUnsupported,
	"NoSuchBucket":          KindNotFound,
	"NoSuchKey":             KindNotFound,
	"NotFound":              KindNotFound,
}

func classifyAPICode(err error) (Kind, bool) {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		if kind, ok := apiErrorCodes[apiErr.ErrorCode()]; ok {
			return kind, true
		}
	}
	return 0, false
}

func classifyThrottle(err error) (Kind, bool) {
	if retry.IsErrorThrottles(retry.DefaultThrottles).IsErrorThrottle(err) == aws.TrueTernary {
		return KindThrottled, true
	}
	return 0, false
}

// statusCodes maps HTTP statuses to their Kind.
var statusCodes = map[int]Kind{
	http.StatusNotFound:           KindNotFound,
	http.StatusUnauthorized:       KindAccessDenied,
	http.StatusForbidden:          KindAccessDenied,
	http.StatusTooManyRequests:    KindThrottled,
	http.StatusServiceUnavailable: KindThrottled,
	http.StatusNotImplemented:     KindUnsupported,
}

func classifyStatus(err error) (Kind, bool) {
	status, ok := responseStatus(err)
	if !ok {
		return 0, false
	}
	kind, ok := statusCodes[status]
	return kind, ok
}

func classifyCredentialService(err error) (Kind, bool) {
	var operationErr *smithy.OperationError
	if errors.As(err, &operationErr) && operationErr != nil && isCredentialService(operationErr.ServiceID) {
		return KindNoCredentials, true
	}
	return 0, false
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
