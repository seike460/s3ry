package s3

import (
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

func TestListBuckets(t *testing.T) {
	session, client, _ := newFakeSession(t)
	putObjects(t, client, "alpha-bucket")
	putObjects(t, client, "beta-bucket")

	buckets, err := session.ListBuckets(t.Context())
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}

	found := make(map[string]bool, len(buckets))
	for _, bucket := range buckets {
		found[bucket.Name] = true
	}
	if len(found) != 2 || !found["alpha-bucket"] || !found["beta-bucket"] {
		t.Fatalf("ListBuckets = %#v, want alpha-bucket and beta-bucket", buckets)
	}
}

func TestListBucketsRejectsRepeatedContinuationToken(t *testing.T) {
	var requests atomic.Int64
	session, _, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			if r.Method == http.MethodGet && r.URL.Path == "/" {
				w.Header().Set("Content-Type", "application/xml")
				_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Buckets/>
  <ContinuationToken>fixed-token</ContinuationToken>
</ListAllMyBucketsResult>`))
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	_, err := session.ListBuckets(t.Context())
	if err == nil {
		t.Fatal("ListBuckets succeeded, want repeated-token error")
	}
	var listErr *Error
	if !errors.As(err, &listErr) {
		t.Fatalf("ListBuckets error = %T %v, want *Error", err, err)
	}
	if listErr.Kind != KindUnsupported {
		t.Fatalf("ListBuckets error kind = %v, want %v", listErr.Kind, KindUnsupported)
	}
	if listErr.Op != "list-buckets" {
		t.Fatalf("ListBuckets error op = %q, want list-buckets", listErr.Op)
	}
	if !strings.Contains(err.Error(), "same continuation token") {
		t.Fatalf("ListBuckets error = %q, want repeated-token message", err)
	}
	if got := requests.Load(); got > 2 {
		t.Fatalf("HTTP requests = %d, want at most 2", got)
	}
}
