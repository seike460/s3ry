package s3

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

func newFakeSession(t *testing.T, mods ...func(*Options)) (*Session, *awss3.Client, *httptest.Server) {
	t.Helper()
	return newFakeSessionWithMiddleware(t, nil, mods...)
}

func newCountingFake(t *testing.T, mw func(http.Handler) http.Handler, mods ...func(*Options)) (*Session, *awss3.Client, *httptest.Server) {
	t.Helper()
	return newFakeSessionWithMiddleware(t, mw, mods...)
}

func newFakeSessionWithMiddleware(t *testing.T, mw func(http.Handler) http.Handler, mods ...func(*Options)) (*Session, *awss3.Client, *httptest.Server) {
	t.Helper()

	setFakeEnvironment(t)
	fake := gofakes3.New(s3mem.New())
	var (
		ts        *httptest.Server
		serverErr error
	)
	// Prefer the requested loopback server APIs. The managed sandbox may
	// reject loopback listeners, so use Go 1.27's in-memory server there.
	if mw == nil {
		ts, serverErr = tryLocalTestServer(func() *httptest.Server {
			server := httptest.NewServer(fake.Server())
			t.Cleanup(server.Close)
			return server
		})
	} else {
		ts, serverErr = tryLocalTestServer(func() *httptest.Server {
			server := httptest.NewUnstartedServer(fake.Server())
			server.Config.Handler = mw(server.Config.Handler)
			server.Start()
			t.Cleanup(server.Close)
			return server
		})
	}
	if ts == nil {
		t.Logf("loopback listener unavailable, using in-memory test server: %v", serverErr)
		ts = httptest.NewTestServer(t, fake.Server())
		if mw != nil {
			ts.Config.Handler = mw(ts.Config.Handler)
		}
		_ = ts.Client()
		if ts.URL == "" {
			t.Fatalf("fallback test server URL is empty")
		}
	}

	opts := Options{
		Region:      "us-east-1",
		EndpointURL: ts.URL,
		PathStyle:   true,
		HTTPClient:  ts.Client(),
	}
	for _, mod := range mods {
		mod(&opts)
	}

	session, err := NewSession(t.Context(), opts)
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	return session, session.clientForRegion(session.Region()), ts
}

func tryLocalTestServer(create func() *httptest.Server) (server *httptest.Server, err error) {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			server = nil
			switch value := panicValue.(type) {
			case error:
				err = value
			default:
				err = fmt.Errorf("%v", value)
			}
		}
	}()
	return create(), nil
}

func setFakeEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_SESSION_TOKEN", "")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_ENDPOINT_URL", "")
	t.Setenv("AWS_ENDPOINT_URL_S3", "")
}

func putObjects(t *testing.T, client *awss3.Client, bucket string, keys ...string) {
	t.Helper()

	_, err := client.CreateBucket(t.Context(), &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil && !bucketAlreadyExists(err) {
		t.Fatalf("CreateBucket %q: %v", bucket, err)
	}
	for _, key := range keys {
		_, err := client.PutObject(t.Context(), &awss3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   strings.NewReader(key),
		})
		if err != nil {
			t.Fatalf("PutObject %q/%q: %v", bucket, key, err)
		}
	}
}

func bucketAlreadyExists(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr == nil {
		return false
	}
	switch apiErr.ErrorCode() {
	case "BucketAlreadyExists", "BucketAlreadyOwnedByYou":
		return true
	default:
		return false
	}
}
