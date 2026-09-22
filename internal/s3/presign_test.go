package s3

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPresignGetReturnsRoutableURL(t *testing.T) {
	const (
		bucket = "presign-bucket"
		key    = "dir/object.txt"
	)

	session, client, server := newFakeSession(t)
	putObjects(t, client, bucket, key)

	presigned, err := session.PresignGet(t.Context(), bucket, key, 15*time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	parsed, err := url.Parse(presigned)
	if err != nil {
		t.Fatalf("parse presigned URL: %v", err)
	}
	query := parsed.Query()
	if got := query.Get("X-Amz-Expires"); got != "900" {
		t.Fatalf("X-Amz-Expires = %q, want 900", got)
	}
	if query.Get("X-Amz-Signature") == "" {
		t.Fatal("X-Amz-Signature is empty")
	}
	if !strings.Contains(parsed.Path, key) {
		t.Fatalf("URL path = %q, want it to contain %q", parsed.Path, key)
	}

	defaultTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = server.Client().Transport
	t.Cleanup(func() {
		http.DefaultClient.Transport = defaultTransport
	})
	//nolint:gosec // G107: fetching the generated presigned URL is the test's purpose
	response, err := http.Get(presigned)
	if err != nil {
		t.Fatalf("GET presigned URL: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET presigned URL status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read presigned object: %v", err)
	}
	if got := string(body); got != key {
		t.Fatalf("presigned object body = %q, want %q", got, key)
	}
}

func TestPresignGetRejectsNoSignRequest(t *testing.T) {
	var requests atomic.Int64
	session, _, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			next.ServeHTTP(w, r)
		})
	}, func(opts *Options) {
		opts.NoSignRequest = true
	})

	_, err := session.PresignGet(t.Context(), "presign-bucket", "object.txt", 15*time.Minute)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("PresignGet error = %v, want ErrInvalid", err)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("HTTP requests = %d, want 0", got)
	}
}

func TestPresignGetRejectsInvalidExpiry(t *testing.T) {
	session, _, _ := newFakeSession(t)
	for _, expires := range []time.Duration{0, 8 * 24 * time.Hour} {
		_, err := session.PresignGet(t.Context(), "presign-bucket", "object.txt", expires)
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("PresignGet(%s) error = %v, want ErrInvalid", expires, err)
		}
	}
}
