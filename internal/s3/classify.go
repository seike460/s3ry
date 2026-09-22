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

	if errors.Is(err, context.DeadlineExceeded) {
		return &Error{Kind: KindTimeout, Op: op, Bucket: bucket, Key: key, Err: err}
	}
	if errors.Is(err, context.Canceled) {
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
		case http.StatusNotFound:
			return &Error{Kind: KindNotFound, Op: op, Bucket: bucket, Key: key, Err: err}
		case http.StatusUnauthorized, http.StatusForbidden:
			return &Error{Kind: KindAccessDenied, Op: op, Bucket: bucket, Key: key, Err: err}
		case http.StatusTooManyRequests, http.StatusServiceUnavailable:
			return &Error{Kind: KindThrottled, Op: op, Bucket: bucket, Key: key, Err: err}
		case http.StatusNotImplemented:
			return &Error{Kind: KindUnsupported, Op: op, Bucket: bucket, Key: key, Err: err}
		}
	}

	var operationErr *smithy.OperationError
	if errors.As(err, &operationErr) && operationErr != nil && isCredentialService(operationErr.ServiceID) {
		return &Error{Kind: KindNoCredentials, Op: op, Bucket: bucket, Key: key, Err: err}
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
