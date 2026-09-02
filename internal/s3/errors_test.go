package s3

import (
	"context"
	"errors"
	"fmt"
	stdhttp "net/http"
	"strings"
	"testing"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestKindString(t *testing.T) {
	tests := []struct {
		kind Kind
		want string
	}{
		{KindUnknown, "unknown"},
		{KindNotFound, "not_found"},
		{KindAccessDenied, "access_denied"},
		{KindNoCredentials, "no_credentials"},
		{KindThrottled, "throttled"},
		{KindCanceled, "canceled"},
		{KindInvalid, "invalid"},
		{KindExists, "exists"},
		{KindUnsupported, "unsupported"},
		{Kind(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

func TestErrorFormattingAndUnwrap(t *testing.T) {
	cause := errors.New("object is missing")
	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "full context",
			err:  &Error{Kind: KindNotFound, Op: "download", Bucket: "bucket", Key: "key", Err: cause},
			want: "download s3://bucket/key: not found: object is missing",
		},
		{
			name: "operation only",
			err:  &Error{Kind: KindExists, Op: "upload"},
			want: "upload: already exists",
		},
		{
			name: "resource only",
			err:  &Error{Kind: KindUnsupported, Bucket: "bucket", Key: "key"},
			want: "s3://bucket/key: not supported by this endpoint",
		},
		{
			name: "key without bucket",
			err:  &Error{Kind: KindInvalid, Op: "get", Key: "key"},
			want: "get key: invalid input",
		},
		{
			name: "sentence only",
			err:  &Error{Kind: KindNoCredentials},
			want: "no AWS credentials found (configure a profile or run aws sso login)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Fatalf("Error() = %q, want %q", got, tt.want)
			}
		})
	}

	wrapped := &Error{Kind: KindInvalid, Err: cause}
	if !errors.Is(wrapped, ErrInvalid) {
		t.Fatal("same kind typed errors should match")
	}
	if errors.Is(wrapped, ErrNotFound) {
		t.Fatal("different kind typed errors should not match")
	}
	if !errors.Is(wrapped, cause) {
		t.Fatal("Error.Unwrap() did not expose the cause")
	}
	var nilError *Error
	if nilError.Error() != "<nil>" || nilError.Unwrap() != nil {
		t.Fatal("nil Error receiver behavior is not stable")
	}
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name string
		err  error
		kind Kind
	}{
		{name: "no such key", err: &s3types.NoSuchKey{}, kind: KindNotFound},
		{name: "no such bucket", err: &s3types.NoSuchBucket{}, kind: KindNotFound},
		{name: "typed not found", err: &s3types.NotFound{}, kind: KindNotFound},
		{name: "access denied", err: &smithy.GenericAPIError{Code: "AccessDenied"}, kind: KindAccessDenied},
		{name: "slow down", err: &smithy.GenericAPIError{Code: "SlowDown"}, kind: KindThrottled},
		{name: "not implemented", err: &smithy.GenericAPIError{Code: "NotImplemented"}, kind: KindUnsupported},
		{name: "HTTP forbidden", err: responseError(403), kind: KindAccessDenied},
		{name: "context canceled", err: context.Canceled, kind: KindCanceled},
		{name: "SSO invalid token", err: &ssocreds.InvalidTokenError{}, kind: KindNoCredentials},
		{name: "wrapped unknown", err: fmt.Errorf("outer: %w", errors.New("opaque")), kind: KindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause := tt.err
			got := Classify("download", "bucket", "key", cause)
			if got == nil {
				t.Fatal("Classify returned nil for a non-nil error")
			}
			typed, ok := got.(*Error)
			if !ok {
				t.Fatalf("Classify returned %T, want *Error", got)
			}
			if typed.Kind != tt.kind {
				t.Fatalf("Classify kind = %v, want %v (%v)", typed.Kind, tt.kind, typed)
			}
			if typed.Op != "download" || typed.Bucket != "bucket" || typed.Key != "key" || typed.Err != cause {
				t.Fatalf("Classify did not preserve context/cause: %#v", typed)
			}
		})
	}

	if Classify("", "", "", nil) != nil {
		t.Fatal("Classify(nil) must return nil")
	}
	existing := &Error{Kind: KindExists, Op: "upload"}
	if got := Classify("download", "other", "key", existing); got != existing {
		t.Fatal("existing *Error was not passed through")
	}
	if got := Classify("download", "bucket", "key", context.DeadlineExceeded).(*Error).Kind; got != KindCanceled {
		t.Fatalf("deadline exceeded kind = %v, want canceled", got)
	}
	if !errors.Is(Classify("download", "bucket", "key", &s3types.NoSuchKey{}), ErrNotFound) {
		t.Fatal("NoSuchKey classification should match ErrNotFound")
	}
}

func TestClassifyWrappedTypedErrorKeepsContext(t *testing.T) {
	original := &Error{Kind: KindInvalid, Op: "validate"}
	wrapped := fmt.Errorf("checksum mismatch: %w", original)

	classified := Classify("get", "b", "k", wrapped)
	typed, ok := classified.(*Error)
	if !ok {
		t.Fatalf("Classify returned %T, want *Error", classified)
	}
	if typed.Op != "get" || typed.Bucket != "b" || typed.Key != "k" {
		t.Fatalf("Classify context = %#v, want get b k", typed)
	}
	if typed.Kind != KindUnknown {
		t.Fatalf("Classify kind = %v, want %v", typed.Kind, KindUnknown)
	}
	if typed.Err != wrapped || !errors.Is(classified, wrapped) {
		t.Fatal("Classify did not preserve the wrapped error chain")
	}
}

func TestClassifyAPICodes(t *testing.T) {
	accessCodes := []string{"AccessDenied", "AllAccessDisabled", "InvalidAccessKeyId", "SignatureDoesNotMatch", "ExpiredToken", "InvalidToken", "AccountProblem"}
	for _, code := range accessCodes {
		t.Run(code, func(t *testing.T) {
			if got := Classify("op", "bucket", "key", &smithy.GenericAPIError{Code: code}).(*Error).Kind; got != KindAccessDenied {
				t.Fatalf("code %q classified as %v", code, got)
			}
		})
	}
	for _, code := range []string{"NotImplemented", "MethodNotAllowed"} {
		t.Run(code, func(t *testing.T) {
			if got := Classify("op", "bucket", "key", &smithy.GenericAPIError{Code: code}).(*Error).Kind; got != KindUnsupported {
				t.Fatalf("code %q classified as %v", code, got)
			}
		})
	}
	for _, code := range []string{"NoSuchBucket", "NoSuchKey", "NotFound"} {
		t.Run(code, func(t *testing.T) {
			if got := Classify("op", "bucket", "key", &smithy.GenericAPIError{Code: code}).(*Error).Kind; got != KindNotFound {
				t.Fatalf("code %q classified as %v", code, got)
			}
		})
	}
}

func TestClassifyHTTPStatuses(t *testing.T) {
	tests := []struct {
		status int
		kind   Kind
	}{
		{404, KindNotFound},
		{401, KindAccessDenied},
		{403, KindAccessDenied},
		{429, KindThrottled},
		{503, KindThrottled},
		{501, KindUnsupported},
		{500, KindUnknown},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status-%d", tt.status), func(t *testing.T) {
			if got := Classify("op", "bucket", "key", responseError(tt.status)).(*Error).Kind; got != tt.kind {
				t.Fatalf("status %d classified as %v, want %v", tt.status, got, tt.kind)
			}
		})
	}
}

func TestClassifyCredentialOperationErrors(t *testing.T) {
	for _, serviceID := range []string{"ec2imds", "SSO", "SSO OIDC", "STS"} {
		t.Run(serviceID, func(t *testing.T) {
			cause := &smithy.OperationError{ServiceID: serviceID, OperationName: "load", Err: errors.New("credentials unavailable")}
			wrapped := fmt.Errorf("credential chain: %w", cause)
			if got := Classify("op", "bucket", "key", wrapped).(*Error).Kind; got != KindNoCredentials {
				t.Fatalf("service %q classified as %v", serviceID, got)
			}
		})
	}
	unknownService := &smithy.OperationError{ServiceID: "S3", OperationName: "GetObject", Err: errors.New("failure")}
	if got := Classify("op", "bucket", "key", unknownService).(*Error).Kind; got != KindUnknown {
		t.Fatalf("unknown service classified as %v, want unknown", got)
	}
}

func TestBulkError(t *testing.T) {
	err := &BulkError{Errors: []KeyError{
		{Key: "a", Err: errors.New("one")},
		{Key: "b", Err: errors.New("two")},
		{Key: "c", Err: errors.New("three")},
		{Key: "d", Err: errors.New("four")},
	}}
	message := err.Error()
	for _, want := range []string{"4 errors", "a: one", "b: two", "c: three", "... and 1 more"} {
		if !strings.Contains(message, want) {
			t.Errorf("BulkError message %q does not contain %q", message, want)
		}
	}
	if strings.Contains(message, "d: four") {
		t.Fatal("BulkError summary must stop after the first three errors")
	}
	if (&BulkError{}).Error() != "0 errors" || (*BulkError)(nil).Error() != "0 errors" {
		t.Fatal("empty BulkError message is not stable")
	}
	if got := (&BulkError{Errors: []KeyError{{Key: "key"}}}).Error(); got != "1 error: key" {
		t.Fatalf("nil cause BulkError = %q", got)
	}
	notFound := &Error{Kind: KindNotFound, Op: "get"}
	bulk := &BulkError{Errors: []KeyError{{Key: "missing", Err: notFound}}}
	if !errors.Is(bulk, ErrNotFound) {
		t.Fatal("BulkError should match ErrNotFound from a contained error")
	}
}

func responseError(status int) error {
	return &awshttp.ResponseError{
		ResponseError: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &stdhttp.Response{StatusCode: status}},
			Err:      errors.New("HTTP response"),
		},
	}
}
