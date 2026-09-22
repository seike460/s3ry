package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestListPagePaginatesOneRequestPerCall(t *testing.T) {
	var listRequests atomic.Int64
	session, client, _ := newCountingFake(t, countListRequests(&listRequests))
	bucket := "list-page-bucket"
	putObjects(t, client, bucket, "a/1", "a/2", "b/1", "c", "d", "e")

	prefixes := make(map[string]bool)
	objects := make(map[string]bool)
	token := ""
	for pageNumber := 0; ; pageNumber++ {
		if pageNumber >= 20 {
			t.Fatal("ListPage did not terminate within 20 pages")
		}
		before := listRequests.Load()
		page, err := session.ListPage(t.Context(), bucket, "", token, 2)
		if err != nil {
			t.Fatalf("ListPage page %d: %v", pageNumber, err)
		}
		if got := listRequests.Load() - before; got != 1 {
			t.Fatalf("ListPage page %d requests = %d, want exactly 1", pageNumber, got)
		}
		if page.Bucket != bucket || page.Prefix != "" {
			t.Fatalf("ListPage page identity = %#v, want bucket %q and empty prefix", page, bucket)
		}
		for _, value := range page.Prefixes {
			prefixes[value] = true
		}
		for _, object := range page.Objects {
			objects[object.Key] = true
		}
		token = page.NextToken
		if token == "" {
			break
		}
	}

	if !equalStringSet(prefixes, map[string]bool{"a/": true, "b/": true}) {
		t.Fatalf("ListPage prefixes = %#v, want a/ and b/", prefixes)
	}
	if !equalStringSet(objects, map[string]bool{"c": true, "d": true, "e": true}) {
		t.Fatalf("ListPage objects = %#v, want c, d, and e", objects)
	}
}

func TestListPageUsesPrefixAndDefaultMaxKeys(t *testing.T) {
	var sawPrefix atomic.Bool
	var sawDefaultMaxKeys atomic.Bool
	session, client, _ := newCountingFake(t, inspectListQuery("a/", &sawPrefix, &sawDefaultMaxKeys))
	bucket := "list-page-options"
	putObjects(t, client, bucket, "a/1", "a/2", "b/1")

	page, err := session.ListPage(t.Context(), bucket, "a/", "", 0)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if page.Bucket != bucket || page.Prefix != "a/" {
		t.Fatalf("ListPage page identity = %#v, want bucket %q and prefix a/", page, bucket)
	}
	if len(page.Objects) != 2 {
		t.Fatalf("ListPage objects = %#v, want two objects", page.Objects)
	}
	gotObjects := map[string]bool{}
	for _, object := range page.Objects {
		gotObjects[object.Key] = true
	}
	if !equalStringSet(gotObjects, map[string]bool{"a/1": true, "a/2": true}) {
		t.Fatalf("ListPage objects = %#v, want a/1 and a/2", page.Objects)
	}
	if !sawPrefix.Load() {
		t.Fatal("server did not receive prefix=a/")
	}
	if !sawDefaultMaxKeys.Load() {
		t.Fatal("server did not receive max-keys=1000")
	}
}

func TestListPageNilObjectFieldsAreSafe(t *testing.T) {
	var sawStorageClass atomic.Bool
	var sawETag atomic.Bool
	session, client, _ := newCountingFake(t, dropListObjectFields(&sawStorageClass, &sawETag))
	bucket := "list-nil-fields"
	putObjects(t, client, bucket, "one")

	page, err := session.ListPage(t.Context(), bucket, "", "", 10)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if len(page.Objects) != 1 {
		t.Fatalf("ListPage objects = %#v, want one object", page.Objects)
	}
	if page.Objects[0].StorageClass != "" {
		t.Fatalf("StorageClass = %q after XML field removal, want empty", page.Objects[0].StorageClass)
	}
	if page.Objects[0].ETag != "" {
		t.Fatalf("ETag = %q after XML field removal, want empty", page.Objects[0].ETag)
	}
	if !sawStorageClass.Load() {
		t.Log("gofakes3 omitted StorageClass from the real response")
	}
	if !sawETag.Load() {
		t.Log("gofakes3 omitted ETag from the real response")
	}
}

func TestWalkSequentialUsesLexicalOrder(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "walk-sequential"
	keys := []string{"z/2", "a/2", "m", "a/1", "z/1", "n"}
	putObjects(t, client, bucket, keys...)

	var got []string
	err := session.Walk(t.Context(), bucket, "", WalkOptions{Concurrency: 1, MaxKeys: 2}, func(object Object) error {
		got = append(got, object.Key)
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	want := append([]string(nil), keys...)
	sort.Strings(want)
	if !equalStringSlices(got, want) {
		t.Fatalf("Walk order = %v, want %v", got, want)
	}
}

func TestWalkParallelCrawlsAllPrefixes(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "walk-parallel"
	keys := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		keys = append(keys, fmt.Sprintf("l%02d/b%02d/c%02d/d%02d", i/20, (i/5)%4, i%5, i))
	}
	putObjects(t, client, bucket, keys...)

	want := make(map[string]int, len(keys))
	for _, key := range keys {
		want[key] = 1
	}

	sequential := collectWalkObjects(t, session, bucket, WalkOptions{Concurrency: 1, MaxKeys: 3})
	if !equalStringCounts(sequential, want) {
		t.Fatalf("sequential Walk keys = %#v, want 60 keys", sequential)
	}
	parallel := collectWalkObjects(t, session, bucket, WalkOptions{Concurrency: 8, MaxKeys: 3})
	if !equalStringCounts(parallel, sequential) {
		t.Fatalf("parallel Walk keys = %#v, sequential keys = %#v", parallel, sequential)
	}
}

func TestWalkSkipsInvalidCommonPrefixes(t *testing.T) {
	session, client, _ := newCountingFake(t, injectInvalidCommonPrefixes("tree/"))
	bucket := "walk-invalid-prefixes"
	putObjects(t, client, bucket, "tree/root", "tree/branch/one", "tree/branch/two")

	want := map[string]int{
		"tree/branch/one": 1,
		"tree/branch/two": 1,
		"tree/root":       1,
	}
	got := collectWalkObjectsWithPrefix(t, session, bucket, "tree/", WalkOptions{Concurrency: 8, MaxKeys: 3})
	if !equalStringCounts(got, want) {
		t.Fatalf("Walk keys and counts = %#v, want %#v", got, want)
	}
}

func TestWalkNonEmptyPrefixBothModes(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "walk-non-empty-prefix"
	putObjects(t, client, bucket,
		"tree/a/one",
		"tree/a/two",
		"tree/b/one",
		"tree/root",
	)

	want := map[string]int{
		"tree/a/one": 1,
		"tree/a/two": 1,
	}
	for _, concurrency := range []int{1, 8} {
		got := collectWalkObjectsWithPrefix(t, session, bucket, "tree/a/", WalkOptions{Concurrency: concurrency, MaxKeys: 1})
		if !equalStringCounts(got, want) {
			t.Fatalf("Walk concurrency %d keys and counts = %#v, want %#v", concurrency, got, want)
		}
	}
}

func TestWalkCallbackErrorStopsSequentialRequests(t *testing.T) {
	var listRequests atomic.Int64
	session, client, _ := newFakeSession(t, withClientListRequestCounter(&listRequests))
	bucket := "walk-callback-error"
	keys := make([]string, 0, 30)
	for i := 0; i < 30; i++ {
		keys = append(keys, fmt.Sprintf("object-%02d", i))
	}
	putObjects(t, client, bucket, keys...)

	wantErr := errors.New("stop at tenth object")
	var seen atomic.Int64
	delivered := make(map[string]int)
	var deliveredMu sync.Mutex
	err := session.Walk(t.Context(), bucket, "", WalkOptions{Concurrency: 1, MaxKeys: 3}, func(object Object) error {
		deliveredMu.Lock()
		delivered[object.Key]++
		deliveredMu.Unlock()
		if seen.Add(1) == 10 {
			return wantErr
		}
		return nil
	})
	if err != wantErr {
		t.Fatalf("Walk error = %v, want exact callback error %v", err, wantErr)
	}
	deliveredMu.Lock()
	for key, count := range delivered {
		if count > 1 {
			deliveredMu.Unlock()
			t.Fatalf("object %q delivered more than once (%d callbacks)", key, count)
		}
	}
	distinctDelivered := len(delivered)
	deliveredMu.Unlock()
	if distinctDelivered < 10 {
		t.Fatalf("distinct objects delivered = %d, want at least 10 before callback error", distinctDelivered)
	}
	countAfterReturn := listRequests.Load()
	assertListRequestsStable(t, &listRequests, countAfterReturn)
	if countAfterReturn != 4 {
		t.Fatalf("ListObjectsV2 requests = %d, want four pages through the tenth object", countAfterReturn)
	}
}

func TestWalkCallbackErrorStopsParallelRequests(t *testing.T) {
	var listRequests atomic.Int64
	session, client, _ := newFakeSession(t, withClientListRequestCounter(&listRequests))
	bucket := "walk-parallel-callback-error"
	keys := makeWalkTreeKeys()
	putObjects(t, client, bucket, keys...)

	wantErr := errors.New("stop at tenth object in parallel Walk")
	var seen atomic.Int64
	delivered := make(map[string]int)
	var deliveredMu sync.Mutex
	err := walkWithTimeout(t.Context(), t, session, bucket, WalkOptions{Concurrency: 8, MaxKeys: 3}, func(object Object) error {
		deliveredMu.Lock()
		delivered[object.Key]++
		deliveredMu.Unlock()
		if seen.Add(1) == 10 {
			return wantErr
		}
		return nil
	})
	if err != wantErr {
		t.Fatalf("Walk error = %v, want exact callback error %v", err, wantErr)
	}
	deliveredMu.Lock()
	for key, count := range delivered {
		if count > 1 {
			deliveredMu.Unlock()
			t.Fatalf("object %q delivered more than once (%d callbacks)", key, count)
		}
	}
	distinctDelivered := len(delivered)
	deliveredMu.Unlock()
	if distinctDelivered < 10 {
		t.Fatalf("distinct objects delivered = %d, want at least 10 before callback error", distinctDelivered)
	}
	countAfterReturn := listRequests.Load()
	assertListRequestsStable(t, &listRequests, countAfterReturn)
}

func TestWalkParallelCancellationStopsRequests(t *testing.T) {
	var listRequests atomic.Int64
	session, client, _ := newFakeSession(t, withClientListRequestCounter(&listRequests))
	bucket := "walk-parallel-cancellation"
	putObjects(t, client, bucket, makeWalkTreeKeys()...)

	ctx, cancel := contextWithCancel(t)
	defer cancel()
	var seen atomic.Int64
	delivered := make(map[string]int)
	var deliveredMu sync.Mutex
	err := walkWithTimeout(ctx, t, session, bucket, WalkOptions{Concurrency: 8, MaxKeys: 3}, func(object Object) error {
		deliveredMu.Lock()
		delivered[object.Key]++
		deliveredMu.Unlock()
		if seen.Add(1) == 5 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("Walk error = %v, want ErrCanceled", err)
	}
	deliveredMu.Lock()
	for key, count := range delivered {
		if count > 1 {
			deliveredMu.Unlock()
			t.Fatalf("object %q delivered more than once (%d callbacks)", key, count)
		}
	}
	distinctDelivered := len(delivered)
	deliveredMu.Unlock()
	if distinctDelivered < 5 {
		t.Fatalf("distinct objects delivered = %d, want at least 5 before cancellation", distinctDelivered)
	}
	countAfterReturn := listRequests.Load()
	assertListRequestsStable(t, &listRequests, countAfterReturn)
}

func TestWalkSuccessfulCompletionWinsParentCancellation(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "walk-successful-empty"
	putObjects(t, client, bucket)

	ctx, cancel := contextWithCancel(t)
	err := session.Walk(ctx, bucket, "", WalkOptions{Concurrency: 8, MaxKeys: 3}, func(Object) error {
		t.Fatal("Walk callback called for an empty bucket")
		return nil
	})
	cancel()
	if err != nil {
		t.Fatalf("Walk error = %v, want nil after successful completion", err)
	}
}

func TestWalkParallelCallbackPanicIsRepanicked(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "walk-callback-panic"
	putObjects(t, client, bucket, "object")

	panicValue := "callback panic"
	defer func() {
		if got := recover(); got != panicValue {
			t.Fatalf("Walk panic = %#v, want %#v", got, panicValue)
		}
	}()
	_ = session.Walk(t.Context(), bucket, "", WalkOptions{Concurrency: 8, MaxKeys: 3}, func(Object) error {
		panic(panicValue)
	})
	t.Fatal("Walk returned after callback panic")
}

func TestWalkAlreadyCanceledContext(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "walk-canceled"
	putObjects(t, client, bucket, "object")

	ctx, cancel := contextWithCancel(t)
	cancel()
	err := session.Walk(ctx, bucket, "", WalkOptions{Concurrency: 8, MaxKeys: 3}, func(Object) error {
		t.Fatal("Walk callback called for canceled context")
		return nil
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("Walk error = %v, want ErrCanceled", err)
	}
}

func TestStat(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "stat-bucket"
	key := "existing"
	putObjects(t, client, bucket, key)
	payload := "hello"
	_, err := client.PutObject(t.Context(), &awss3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        strings.NewReader(payload),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	info, err := session.Stat(t.Context(), bucket, key)
	if err != nil {
		t.Fatalf("Stat existing object: %v", err)
	}
	if info.Size != int64(len(payload)) {
		t.Fatalf("Stat Size = %d, want %d", info.Size, len(payload))
	}
	if info.ContentType != "text/plain" {
		t.Fatalf("Stat ContentType = %q, want text/plain", info.ContentType)
	}
	if strings.HasPrefix(info.ETag, `"`) || strings.HasSuffix(info.ETag, `"`) {
		t.Fatalf("Stat ETag = %q, want surrounding quotes removed", info.ETag)
	}

	_, err = session.Stat(t.Context(), bucket, "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat missing error = %v, want ErrNotFound", err)
	}
}

func TestOpenRange(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "open-range-bucket"
	key := "object"
	payload := "0123456789abcdefghij"
	putObjects(t, client, bucket)
	_, err := client.PutObject(t.Context(), &awss3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	body, err := session.Open(t.Context(), bucket, key, "bytes=0-9")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = body.Close() }()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != payload[:10] || len(got) != 10 {
		t.Fatalf("Open range = %q (%d bytes), want %q (10 bytes)", got, len(got), payload[:10])
	}
}

func TestOpenWithoutRangeReturnsFullBody(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "open-full-body"
	key := "object"
	payload := "full object payload"
	putObjects(t, client, bucket)
	_, err := client.PutObject(t.Context(), &awss3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}

	body, err := session.Open(t.Context(), bucket, key, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = body.Close() }()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("Open without range = %q, want %q", got, payload)
	}
}

func TestOpenMissingKey(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "open-missing-key"
	putObjects(t, client, bucket)

	body, err := session.Open(t.Context(), bucket, "missing", "")
	if body != nil {
		_ = body.Close()
		t.Fatalf("Open missing body = %#v, want nil", body)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open missing error = %v, want ErrNotFound", err)
	}
}

func countListRequests(requests *atomic.Int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
				requests.Add(1)
			}
			next.ServeHTTP(w, r)
		})
	}
}

type listRequestCountingTransport struct {
	transport http.RoundTripper
	requests  *atomic.Int64
}

func (t listRequestCountingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
		t.requests.Add(1)
	}
	return t.transport.RoundTrip(r)
}

func withClientListRequestCounter(requests *atomic.Int64) func(*Options) {
	return func(opts *Options) {
		client, ok := opts.HTTPClient.(*http.Client)
		if !ok {
			panic(fmt.Sprintf("fake HTTPClient = %T, want *http.Client", opts.HTTPClient))
		}
		wrapped := *client
		transport := wrapped.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		wrapped.Transport = listRequestCountingTransport{
			transport: transport,
			requests:  requests,
		}
		opts.HTTPClient = &wrapped
	}
}

func inspectListQuery(wantPrefix string, sawPrefix, sawDefaultMaxKeys *atomic.Bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
				sawPrefix.Store(r.URL.Query().Get("prefix") == wantPrefix)
				sawDefaultMaxKeys.Store(r.URL.Query().Get("max-keys") == "1000")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func injectInvalidCommonPrefixes(prefix string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Query().Get("list-type") != "2" {
				next.ServeHTTP(w, r)
				return
			}

			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, r)
			body := recorder.Body.Bytes()
			marker := []byte("</ListBucketResult>")
			insertion := []byte("<CommonPrefixes><Prefix>" + prefix + "</Prefix></CommonPrefixes><CommonPrefixes><Prefix></Prefix></CommonPrefixes>")
			if index := bytes.Index(body, marker); index >= 0 {
				rewritten := make([]byte, 0, len(body)+len(insertion))
				rewritten = append(rewritten, body[:index]...)
				rewritten = append(rewritten, insertion...)
				rewritten = append(rewritten, body[index:]...)
				body = rewritten
			}

			for key, values := range recorder.Header() {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.Header().Del("Content-Length")
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(body)
		})
	}
}

func dropListObjectFields(sawStorageClass, sawETag *atomic.Bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet || r.URL.Query().Get("list-type") != "2" {
				next.ServeHTTP(w, r)
				return
			}

			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, r)
			body := recorder.Body.Bytes()
			var changed bool
			body, found := removeXMLElement(body, "StorageClass")
			if found {
				sawStorageClass.Store(true)
				changed = true
			}
			body, found = removeXMLElement(body, "ETag")
			if found {
				sawETag.Store(true)
				changed = true
			}

			for key, values := range recorder.Header() {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			if changed {
				w.Header().Del("Content-Length")
			}
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(body)
		})
	}
}

func removeXMLElement(body []byte, element string) ([]byte, bool) {
	open := []byte("<" + element + ">")
	closing := []byte("</" + element + ">")
	changed := false
	for {
		start := bytes.Index(body, open)
		if start < 0 {
			return body, changed
		}
		contentStart := start + len(open)
		end := bytes.Index(body[contentStart:], closing)
		if end < 0 {
			return body, changed
		}
		end += contentStart + len(closing)
		rewritten := make([]byte, 0, len(body)-(end-start))
		rewritten = append(rewritten, body[:start]...)
		rewritten = append(rewritten, body[end:]...)
		body = rewritten
		changed = true
	}
}

func collectWalkObjects(t *testing.T, session *Session, bucket string, options WalkOptions) map[string]int {
	t.Helper()
	return collectWalkObjectsWithPrefix(t, session, bucket, "", options)
}

func collectWalkObjectsWithPrefix(t *testing.T, session *Session, bucket, prefix string, options WalkOptions) map[string]int {
	t.Helper()
	objects := make(map[string]int)
	var mu sync.Mutex
	err := session.Walk(t.Context(), bucket, prefix, options, func(object Object) error {
		mu.Lock()
		objects[object.Key]++
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	return objects
}

func equalStringCounts(got, want map[string]int) bool {
	if len(got) != len(want) {
		return false
	}
	for key, wantCount := range want {
		if got[key] != wantCount {
			return false
		}
	}
	return true
}

func makeWalkTreeKeys() []string {
	keys := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		keys = append(keys, fmt.Sprintf("l%02d/b%02d/c%02d/d%02d", i/20, (i/5)%4, i%5, i))
	}
	return keys
}

func walkWithTimeout(ctx context.Context, t *testing.T, session *Session, bucket string, options WalkOptions, fn func(Object) error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- session.Walk(ctx, bucket, "", options, fn)
	}()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		t.Fatal("Walk did not return within the test timeout")
		return nil
	}
}

func assertListRequestsStable(t *testing.T, requests *atomic.Int64, want int64) {
	t.Helper()
	deadline := time.Now().Add(200 * time.Millisecond)
	for {
		if got := requests.Load(); got != want {
			t.Fatalf("ListObjectsV2 requests changed after Walk returned: got %d, want stable count %d", got, want)
		}
		if !time.Now().Before(deadline) {
			return
		}
		runtime.Gosched()
	}
}

func equalStringSet(got, want map[string]bool) bool {
	if len(got) != len(want) {
		return false
	}
	for key := range want {
		if !got[key] {
			return false
		}
	}
	return true
}

func equalStringSlices(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func contextWithCancel(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()
	return context.WithCancel(t.Context())
}
