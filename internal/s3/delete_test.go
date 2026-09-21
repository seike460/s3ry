package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestDeleteKeysBatchesAtMaxDeleteBatch(t *testing.T) {
	var deleteRequests atomic.Int64
	session, client, _ := newCountingFake(t, countDeleteObjects(&deleteRequests))
	bucket := "delete-batches"
	keys := numberedDeleteKeys("object-", 2500)
	putObjects(t, client, bucket, keys...)

	result, err := session.DeleteKeys(t.Context(), bucket, keys, DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteKeys: %v", err)
	}
	if got := deleteRequests.Load(); got != 3 {
		t.Fatalf("DeleteObjects requests = %d, want 3", got)
	}
	if len(result.Deleted) != len(keys) {
		t.Fatalf("Deleted length = %d, want %d", len(result.Deleted), len(keys))
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no failures", result.Failed)
	}

	sort.Strings(result.Deleted)
	if !equalStringSlices(result.Deleted, keys) {
		t.Fatalf("Deleted keys do not match request keys")
	}
}

func TestDeleteKeysReportsMissingKeyAsDeleted(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "delete-missing"
	putObjects(t, client, bucket, "existing")

	result, err := session.DeleteKeys(t.Context(), bucket, []string{"missing"}, DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteKeys: %v", err)
	}
	if !equalStringSlices(result.Deleted, []string{"missing"}) {
		t.Fatalf("Deleted = %v, want missing", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no failures", result.Failed)
	}
}

func TestDeleteKeysDryRunDoesNotMutate(t *testing.T) {
	var mutatingRequests atomic.Int64
	session, _, _ := newCountingFake(t, countDeleteMutations(&mutatingRequests))
	bucket := "delete-dry-run"
	keys := []string{"a", "nested/b", "c"}
	var progress []Progress

	result, err := session.DeleteKeys(t.Context(), bucket, keys, DeleteOptions{
		DryRun: true,
		Progress: func(value Progress) {
			progress = append(progress, value)
		},
	})
	if err != nil {
		t.Fatalf("DeleteKeys dry run: %v", err)
	}
	if got := mutatingRequests.Load(); got != 0 {
		t.Fatalf("mutating requests = %d, want 0", got)
	}
	if !equalStringSlices(result.Deleted, keys) {
		t.Fatalf("Deleted = %v, want %v", result.Deleted, keys)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no failures", result.Failed)
	}
	if len(progress) != len(keys) {
		t.Fatalf("progress callbacks = %d, want %d", len(progress), len(keys))
	}
	for i, value := range progress {
		if value.Op != OpDelete || value.Bucket != bucket || value.Key != keys[i] || !value.Done || value.Err != nil {
			t.Fatalf("progress[%d] = %#v, want completed delete for %q", i, value, keys[i])
		}
	}
}

func TestDeleteKeysSkipsInvalidKey(t *testing.T) {
	var deleteBodyMu sync.Mutex
	var deleteBody string
	session, client, _ := newCountingFake(t, captureDeleteObjectBody(&deleteBodyMu, &deleteBody))
	bucket := "delete-invalid-key"
	putObjects(t, client, bucket, "valid")

	result, err := session.DeleteKeys(t.Context(), bucket, []string{"", "valid"}, DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteKeys: %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].Key != "" {
		t.Fatalf("Failed = %#v, want one failure for the empty key", result.Failed)
	}
	if !errors.Is(result.Failed[0].Err, ErrInvalid) {
		t.Fatalf("invalid key error = %v, want ErrInvalid", result.Failed[0].Err)
	}
	if !equalStringSlices(result.Deleted, []string{"valid"}) {
		t.Fatalf("Deleted = %v, want valid", result.Deleted)
	}

	deleteBodyMu.Lock()
	body := deleteBody
	deleteBodyMu.Unlock()
	if !strings.Contains(body, "<Key>valid</Key>") {
		t.Fatalf("DeleteObjects body = %q, want valid key", body)
	}
	if strings.Contains(body, "<Key></Key>") || strings.Contains(body, "<Key/>") {
		t.Fatalf("DeleteObjects body = %q, contains the invalid empty key", body)
	}
}

func TestDeleteKeysFallsBackToSingleDelete(t *testing.T) {
	var bulkRequests atomic.Int64
	var singleRequests atomic.Int64
	session, client, _ := newCountingFake(t, rejectDeleteObjects(&bulkRequests, &singleRequests), func(options *Options) {
		options.Concurrency = 3
	})
	bucket := "delete-fallback"
	firstKeys := []string{"first-a", "first-b", "first-c", "first-d"}
	putObjects(t, client, bucket, firstKeys...)

	first, err := session.DeleteKeys(t.Context(), bucket, firstKeys, DeleteOptions{})
	if err != nil {
		t.Fatalf("first DeleteKeys: %v", err)
	}
	if len(first.Deleted) != len(firstKeys) || len(first.Failed) != 0 {
		t.Fatalf("first result = %#v, want all keys deleted", first)
	}
	if got := bulkRequests.Load(); got != 1 {
		t.Fatalf("bulk requests after fallback = %d, want 1", got)
	}
	if got := singleRequests.Load(); got != int64(len(firstKeys)) {
		t.Fatalf("single requests after fallback = %d, want %d", got, len(firstKeys))
	}
	if !session.singleDelete.Load() {
		t.Fatal("singleDelete flag is false after unsupported DeleteObjects")
	}

	secondKeys := []string{"second-a", "second-b"}
	second, err := session.DeleteKeys(t.Context(), bucket, secondKeys, DeleteOptions{})
	if err != nil {
		t.Fatalf("second DeleteKeys: %v", err)
	}
	if len(second.Deleted) != len(secondKeys) || len(second.Failed) != 0 {
		t.Fatalf("second result = %#v, want all keys deleted", second)
	}
	if got := bulkRequests.Load(); got != 1 {
		t.Fatalf("bulk requests after second call = %d, want 1", got)
	}
	if got := singleRequests.Load(); got != int64(len(firstKeys)+len(secondKeys)) {
		t.Fatalf("single requests after second call = %d, want %d", got, len(firstKeys)+len(secondKeys))
	}
}

func TestDeleteKeysSingleDeleteHardErrorReturnsPartialResult(t *testing.T) {
	var singleRequests atomic.Int64
	session, client, _ := newCountingFake(t, rejectDeleteObjectRequests(&singleRequests, http.StatusForbidden, "AccessDenied"), func(options *Options) {
		options.Concurrency = 1
	})
	bucket := "delete-single-hard-error"
	keys := []string{"denied", "never-attempted"}
	putObjects(t, client, bucket, keys...)
	session.singleDelete.Store(true)
	var progress []Progress

	result, err := session.DeleteKeys(t.Context(), bucket, keys, DeleteOptions{
		Progress: func(value Progress) {
			progress = append(progress, value)
		},
	})
	if err == nil {
		t.Fatal("DeleteKeys: want hard delete error")
	}
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindAccessDenied {
		t.Fatalf("DeleteKeys error = %v, want KindAccessDenied", err)
	}
	if len(result.Deleted) != 0 {
		t.Fatalf("Deleted = %v, want no deleted keys", result.Deleted)
	}
	if len(result.Failed) != 1 || result.Failed[0].Key != keys[0] {
		t.Fatalf("Failed = %#v, want only %q", result.Failed, keys[0])
	}
	if got := singleRequests.Load(); got != 1 {
		t.Fatalf("DeleteObject requests = %d, want one attempted request", got)
	}
	if len(progress) != 1 || progress[0].Key != keys[0] || progress[0].Err == nil {
		t.Fatalf("progress = %#v, want one failed event for %q", progress, keys[0])
	}
}

func TestDeleteKeysSingleDeleteNotFoundIsDeleted(t *testing.T) {
	var singleRequests atomic.Int64
	session, client, _ := newCountingFake(t, rejectDeleteObjectRequests(&singleRequests, http.StatusNotFound, "NoSuchKey"), func(options *Options) {
		options.Concurrency = 1
	})
	bucket := "delete-single-not-found"
	putObjects(t, client, bucket, "missing")
	session.singleDelete.Store(true)

	result, err := session.DeleteKeys(t.Context(), bucket, []string{"missing"}, DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteKeys: %v", err)
	}
	if !equalStringSlices(result.Deleted, []string{"missing"}) {
		t.Fatalf("Deleted = %v, want missing", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no failures", result.Failed)
	}
	if got := singleRequests.Load(); got != 1 {
		t.Fatalf("DeleteObject requests = %d, want one", got)
	}
}

func TestDeleteKeysDeduplicatesInputKeys(t *testing.T) {
	var deleteRequests atomic.Int64
	session, client, _ := newCountingFake(t, countDeleteObjects(&deleteRequests))
	bucket := "delete-deduplicate"
	putObjects(t, client, bucket, "a")
	var progress []Progress

	result, err := session.DeleteKeys(t.Context(), bucket, []string{"a", "a"}, DeleteOptions{
		Progress: func(value Progress) {
			progress = append(progress, value)
		},
	})
	if err != nil {
		t.Fatalf("DeleteKeys: %v", err)
	}
	if !equalStringSlices(result.Deleted, []string{"a"}) {
		t.Fatalf("Deleted = %v, want one a", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no failures", result.Failed)
	}
	if got := deleteRequests.Load(); got != 1 {
		t.Fatalf("DeleteObjects requests = %d, want one", got)
	}
	if len(progress) != 1 || progress[0].Key != "a" || progress[0].Err != nil {
		t.Fatalf("progress = %#v, want one successful a event", progress)
	}
}

func TestDeleteKeysMethodNotAllowedDoesNotDowngrade(t *testing.T) {
	var bulkRequests atomic.Int64
	var singleRequests atomic.Int64
	session, _, _ := newCountingFake(t, rejectDeleteObjectsAndDeleteObjectRequests(&bulkRequests, &singleRequests, http.StatusMethodNotAllowed, "MethodNotAllowed"))
	bucket := "delete-method-not-allowed"

	result, err := session.DeleteKeys(t.Context(), bucket, []string{"a"}, DeleteOptions{})
	if err == nil {
		t.Fatal("DeleteKeys: want MethodNotAllowed error")
	}
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindUnsupported {
		t.Fatalf("DeleteKeys error = %v, want KindUnsupported", err)
	}
	if result.Deleted != nil || result.Failed != nil {
		t.Fatalf("DeleteKeys result = %#v, want empty result", result)
	}
	if session.singleDelete.Load() {
		t.Fatal("singleDelete flag changed after MethodNotAllowed")
	}
	if got := bulkRequests.Load(); got != 1 {
		t.Fatalf("DeleteObjects requests = %d, want one", got)
	}
	if got := singleRequests.Load(); got != 0 {
		t.Fatalf("DeleteObject requests = %d, want none", got)
	}
}

func TestDeleteKeysSurfacesPerKeyErrors(t *testing.T) {
	session, client, _ := newCountingFake(t, addDeleteObjectError("denied"))
	bucket := "delete-per-key-error"
	putObjects(t, client, bucket, "allowed", "denied")

	result, err := session.DeleteKeys(t.Context(), bucket, []string{"allowed", "denied"}, DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteKeys: %v", err)
	}
	if len(result.Failed) != 1 || result.Failed[0].Key != "denied" {
		t.Fatalf("Failed = %#v, want denied", result.Failed)
	}
	classified, ok := result.Failed[0].Err.(*Error)
	if !ok {
		t.Fatalf("per-key error type = %T, want *Error", result.Failed[0].Err)
	}
	if classified.Kind != KindAccessDenied {
		t.Fatalf("per-key error kind = %v, want %v", classified.Kind, KindAccessDenied)
	}
	if classified.Op != "delete" || classified.Bucket != bucket || classified.Key != "denied" {
		t.Fatalf("per-key error context = %#v, want delete context", classified)
	}
	if !equalStringSlices(result.Deleted, []string{"allowed"}) {
		t.Fatalf("Deleted = %v, want exactly allowed", result.Deleted)
	}
}

func TestDeleteKeysReportsOmittedResponseKeyAsUnknown(t *testing.T) {
	session, client, _ := newCountingFake(t, omitDeleteObjectSuccess("omitted"))
	bucket := "delete-omitted-response-key"
	putObjects(t, client, bucket, "kept", "omitted")
	var progress []Progress

	result, err := session.DeleteKeys(t.Context(), bucket, []string{"kept", "omitted"}, DeleteOptions{
		Progress: func(value Progress) {
			progress = append(progress, value)
		},
	})
	if err != nil {
		t.Fatalf("DeleteKeys: %v", err)
	}
	if !equalStringSlices(result.Deleted, []string{"kept"}) {
		t.Fatalf("Deleted = %v, want kept", result.Deleted)
	}
	if len(result.Failed) != 1 || result.Failed[0].Key != "omitted" {
		t.Fatalf("Failed = %#v, want omitted", result.Failed)
	}
	var unknown *Error
	if !errors.As(result.Failed[0].Err, &unknown) || unknown.Kind != KindUnknown {
		t.Fatalf("omitted response error = %v, want KindUnknown", result.Failed[0].Err)
	}
	if len(progress) != 2 {
		t.Fatalf("progress callbacks = %d, want 2", len(progress))
	}
	for _, value := range progress {
		if value.Key == "omitted" {
			var progressUnknown *Error
			if !errors.As(value.Err, &progressUnknown) || progressUnknown.Kind != KindUnknown {
				t.Fatalf("omitted progress = %#v, want KindUnknown", value)
			}
		}
	}
}

func TestDeleteKeysAccessDeniedDoesNotDowngradeOrHidePartialResult(t *testing.T) {
	keys := numberedDeleteKeys("object-", MaxDeleteBatch+1)
	session, client, _ := newCountingFake(t, rejectDeleteObjectsAfter(2, http.StatusForbidden, "AccessDenied"))
	bucket := "delete-access-denied"
	putObjects(t, client, bucket, keys...)

	result, err := session.DeleteKeys(t.Context(), bucket, keys, DeleteOptions{})
	if err == nil {
		t.Fatal("DeleteKeys: want classified access-denied error")
	}
	var classified *Error
	if !errors.As(err, &classified) {
		t.Fatalf("DeleteKeys error = %v, want *Error in chain", err)
	}
	if classified.Kind != KindAccessDenied {
		t.Fatalf("DeleteKeys error kind = %v, want %v", classified.Kind, KindAccessDenied)
	}
	if !equalStringSlices(result.Deleted, keys[:MaxDeleteBatch]) {
		t.Fatalf("Deleted length or keys = %v, want first complete batch", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no per-key failures for a request error", result.Failed)
	}
	if session.singleDelete.Load() {
		t.Fatal("singleDelete flag is true after access-denied DeleteObjects")
	}
	if _, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(keys[len(keys)-1]),
	}); err != nil {
		t.Fatalf("HeadObject after access-denied batch: %v", err)
	}
}

func TestDeletePrefixRemovesOnlyPrefix(t *testing.T) {
	var deleteRequests atomic.Int64
	session, client, _ := newCountingFake(t, countDeleteObjects(&deleteRequests))
	bucket := "delete-prefix"
	prefixKeys := numberedDeleteKeys("p/", 1200)
	putKeys := append(append([]string(nil), prefixKeys...), "q/untouched")
	putObjects(t, client, bucket, putKeys...)

	result, err := session.DeletePrefix(t.Context(), bucket, "p/", DeleteOptions{})
	if err != nil {
		t.Fatalf("DeletePrefix: %v", err)
	}
	if len(result.Deleted) != len(prefixKeys) || len(result.Failed) != 0 {
		t.Fatalf("DeletePrefix result = %#v, want 1200 deleted keys", result)
	}
	if got := deleteRequests.Load(); got != 2 {
		t.Fatalf("DeleteObjects requests = %d, want 2", got)
	}

	page, err := session.ListPage(t.Context(), bucket, "p/", "", 0)
	if err != nil {
		t.Fatalf("ListPage p/: %v", err)
	}
	if len(page.Objects) != 0 || len(page.Prefixes) != 0 {
		t.Fatalf("p/ after DeletePrefix = %#v, want empty", page)
	}

	sibling, err := session.ListPage(t.Context(), bucket, "q/", "", 0)
	if err != nil {
		t.Fatalf("ListPage q/: %v", err)
	}
	if len(sibling.Objects) != 1 || sibling.Objects[0].Key != "q/untouched" {
		t.Fatalf("q/ after DeletePrefix = %#v, want untouched object", sibling)
	}
}

func TestDeletePrefixRejectsEmptyPrefix(t *testing.T) {
	var mutatingRequests atomic.Int64
	session, _, _ := newCountingFake(t, countAllRequests(&mutatingRequests))
	bucket := "delete-prefix-empty"

	result, err := session.DeletePrefix(t.Context(), bucket, "", DeleteOptions{})
	if err == nil {
		t.Fatal("DeletePrefix: want invalid-prefix error")
	}
	var classified *Error
	if !errors.As(err, &classified) {
		t.Fatalf("DeletePrefix error = %v, want *Error", err)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("DeletePrefix error = %v, want ErrInvalid", err)
	}
	if classified.Kind != KindInvalid || classified.Op != "delete-prefix" || classified.Bucket != bucket {
		t.Fatalf("DeletePrefix error = %#v, want invalid delete-prefix error", classified)
	}
	if classified.Err == nil || classified.Err.Error() != "prefix must not be empty; deleting a whole bucket is not supported" {
		t.Fatalf("DeletePrefix cause = %v, want explicit empty-prefix message", classified.Err)
	}
	if result.Deleted != nil || result.Failed != nil {
		t.Fatalf("DeletePrefix result = %#v, want zero result", result)
	}
	if got := mutatingRequests.Load(); got != 0 {
		t.Fatalf("mutating requests = %d, want 0", got)
	}
}

func TestDeletePrefixRejectsInvalidPrefix(t *testing.T) {
	var requests atomic.Int64
	session, _, _ := newCountingFake(t, countAllRequests(&requests))
	prefix := "p\x00"

	result, err := session.DeletePrefix(t.Context(), "delete-prefix-invalid", prefix, DeleteOptions{})
	if err == nil {
		t.Fatal("DeletePrefix: want invalid-prefix error")
	}
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindInvalid {
		t.Fatalf("DeletePrefix error = %v, want KindInvalid", err)
	}
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("DeletePrefix error = %v, want ErrInvalid", err)
	}
	if result.Deleted != nil || result.Failed != nil {
		t.Fatalf("DeletePrefix result = %#v, want zero result", result)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("requests = %d, want no request for invalid prefix", got)
	}
}

func TestDeletePrefixNormalizesDirectoryPrefix(t *testing.T) {
	session, client, _ := newFakeSession(t)
	bucket := "delete-prefix-directory"
	putObjects(t, client, bucket, "p/x", "p2/x")

	result, err := session.DeletePrefix(t.Context(), bucket, "p", DeleteOptions{})
	if err != nil {
		t.Fatalf("DeletePrefix: %v", err)
	}
	if !equalStringSlices(result.Deleted, []string{"p/x"}) {
		t.Fatalf("Deleted = %v, want only p/x", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no failures", result.Failed)
	}
	if _, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String("p2/x"),
	}); err != nil {
		t.Fatalf("HeadObject p2/x: %v", err)
	}
}

func TestDeletePrefixDeleteErrorStopsWalk(t *testing.T) {
	for _, concurrency := range []int{1, 8} {
		t.Run(fmt.Sprintf("concurrency-%d", concurrency), func(t *testing.T) {
			var listRequests atomic.Int64
			session, client, _ := newCountingFake(t, rejectDeleteObjectsAndCountLists(&listRequests, http.StatusInternalServerError, "InternalError"), func(options *Options) {
				options.Concurrency = concurrency
			})
			bucket := fmt.Sprintf("delete-prefix-delete-error-%d", concurrency)
			keys := numberedDeleteKeys("p/", 2501)
			putObjects(t, client, bucket, keys...)

			result, err := session.DeletePrefix(t.Context(), bucket, "p/", DeleteOptions{})
			if err == nil {
				t.Fatal("DeletePrefix: want delete error")
			}
			var classified *Error
			if !errors.As(err, &classified) || classified.Op != "delete" || classified.Bucket != bucket {
				t.Fatalf("DeletePrefix error = %v, want classified delete error in chain", err)
			}
			if errors.Is(err, ErrCanceled) {
				t.Fatalf("DeletePrefix error = %v, must not report self-cancellation", err)
			}
			totalPages := int64((len(keys) + int(defaultListMaxKeys) - 1) / int(defaultListMaxKeys))
			if got := listRequests.Load(); got >= totalPages {
				t.Fatalf("ListObjectsV2 requests = %d, want fewer than total pages %d after delete failure", got, totalPages)
			}
			if len(result.Deleted) != 0 {
				t.Fatalf("Deleted = %v, want no keys deleted after first request failure", result.Deleted)
			}
			if _, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(keys[len(keys)-1]),
			}); err != nil {
				t.Fatalf("HeadObject after stopped walk: %v", err)
			}
		})
	}
}

func TestDeletePrefixSingleDeleteHardErrorStopsWalk(t *testing.T) {
	var listRequests atomic.Int64
	var singleRequests atomic.Int64
	session, client, _ := newCountingFake(t, rejectDeleteObjectAndCountLists(&listRequests, &singleRequests, http.StatusForbidden, "AccessDenied"), func(options *Options) {
		options.Concurrency = 1
	})
	bucket := "delete-prefix-single-hard-error"
	keys := numberedDeleteKeys("p/", 2501)
	putObjects(t, client, bucket, keys...)
	session.singleDelete.Store(true)

	result, err := session.DeletePrefix(t.Context(), bucket, "p/", DeleteOptions{})
	if err == nil {
		t.Fatal("DeletePrefix: want hard delete error")
	}
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindAccessDenied {
		t.Fatalf("DeletePrefix error = %v, want KindAccessDenied", err)
	}
	if len(result.Deleted) != 0 {
		t.Fatalf("Deleted = %v, want no deleted keys", result.Deleted)
	}
	if len(result.Failed) != 1 || result.Failed[0].Key != keys[0] {
		t.Fatalf("Failed = %#v, want only the attempted key %q", result.Failed, keys[0])
	}
	totalPages := int64((len(keys) + int(defaultListMaxKeys) - 1) / int(defaultListMaxKeys))
	if got := listRequests.Load(); got >= totalPages {
		t.Fatalf("ListObjectsV2 requests = %d, want fewer than total pages %d", got, totalPages)
	}
	if got := singleRequests.Load(); got != 1 {
		t.Fatalf("DeleteObject requests = %d, want one attempted request", got)
	}
}

func TestDeletePrefixParentCancellationDuringDeleteFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	session, client, _ := newCountingFake(t, cancelOnDeleteObjectsAndFail(cancel, http.StatusInternalServerError, "InternalError"), func(options *Options) {
		options.Concurrency = 1
	})
	bucket := "delete-prefix-parent-cancel"
	keys := numberedDeleteKeys("p/", 1001)
	putObjects(t, client, bucket, keys...)

	result, err := session.DeletePrefix(ctx, bucket, "p/", DeleteOptions{})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("DeletePrefix error = %v, want ErrCanceled", err)
	}
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindCanceled {
		t.Fatalf("DeletePrefix error = %v, want KindCanceled", err)
	}
	if len(result.Deleted) != 0 {
		t.Fatalf("Deleted = %v, want no deleted keys", result.Deleted)
	}
}

func TestDeletePrefixStopsOnUnsupportedSingleDelete(t *testing.T) {
	var listRequests atomic.Int64
	var bulkRequests atomic.Int64
	var singleRequests atomic.Int64
	session, client, _ := newCountingFake(t, rejectUnsupportedDeletesAndCountLists(&listRequests, &bulkRequests, &singleRequests), func(options *Options) {
		options.Concurrency = 1
	})
	bucket := "delete-prefix-unsupported"
	keys := numberedDeleteKeys("p/", 2501)
	putObjects(t, client, bucket, keys...)

	result, err := session.DeletePrefix(t.Context(), bucket, "p/", DeleteOptions{})
	if err == nil {
		t.Fatal("DeletePrefix: want unsupported delete error")
	}
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindUnsupported {
		t.Fatalf("DeletePrefix error = %v, want KindUnsupported", err)
	}
	if len(result.Deleted) != 0 {
		t.Fatalf("Deleted = %v, want no deleted keys", result.Deleted)
	}
	if !session.singleDelete.Load() {
		t.Fatal("singleDelete flag is false after DeleteObjects NotImplemented")
	}
	totalPages := int64((len(keys) + int(defaultListMaxKeys) - 1) / int(defaultListMaxKeys))
	if got := listRequests.Load(); got >= totalPages {
		t.Fatalf("ListObjectsV2 requests = %d, want fewer than total pages %d", got, totalPages)
	}
	if got := bulkRequests.Load(); got != 1 {
		t.Fatalf("DeleteObjects requests = %d, want one", got)
	}
	if got := singleRequests.Load(); got != 1 {
		t.Fatalf("DeleteObject requests = %d, want one", got)
	}
}

func TestDeletePrefixDryRunDoesNotMutate(t *testing.T) {
	var mutatingRequests atomic.Int64
	session, client, _ := newCountingFake(t, countDeleteMutations(&mutatingRequests))
	bucket := "delete-prefix-dry-run"
	keys := []string{"p/a", "p/nested/b", "p/c", "p2/not-listed"}
	putObjects(t, client, bucket, keys...)

	result, err := session.DeletePrefix(t.Context(), bucket, "p", DeleteOptions{DryRun: true})
	if err != nil {
		t.Fatalf("DeletePrefix dry run: %v", err)
	}
	deleted := append([]string(nil), result.Deleted...)
	sort.Strings(deleted)
	if !equalStringSlices(deleted, []string{"p/a", "p/c", "p/nested/b"}) {
		t.Fatalf("Deleted = %v, want every key under p/", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no failures", result.Failed)
	}
	if got := mutatingRequests.Load(); got != 0 {
		t.Fatalf("mutating requests = %d, want 0", got)
	}
}

func TestDeletePrefixCancellationStopsFurtherDeleteRequests(t *testing.T) {
	for _, concurrency := range []int{1, 8} {
		t.Run(fmt.Sprintf("concurrency-%d", concurrency), func(t *testing.T) {
			var listRequests atomic.Int64
			var deleteRequests atomic.Int64
			session, client, _ := newCountingFake(t, countDeleteObjectsAndLists(&deleteRequests, &listRequests), func(options *Options) {
				options.Concurrency = concurrency
			})
			bucket := fmt.Sprintf("delete-prefix-cancelled-%d", concurrency)
			keys := numberedDeleteKeys("p/", 2501)
			putObjects(t, client, bucket, keys...)

			ctx, cancel := context.WithCancel(t.Context())
			var progressCount atomic.Int64
			result, err := session.DeletePrefix(ctx, bucket, "p/", DeleteOptions{
				Progress: func(value Progress) {
					if value.Done && progressCount.Add(1) == MaxDeleteBatch {
						cancel()
					}
				},
			})
			if !errors.Is(err, ErrCanceled) {
				cancel()
				t.Fatalf("DeletePrefix error = %v, want ErrCanceled", err)
			}
			if len(result.Deleted) != MaxDeleteBatch {
				cancel()
				t.Fatalf("Deleted length = %d, want %d", len(result.Deleted), MaxDeleteBatch)
			}
			totalPages := int64((len(keys) + int(defaultListMaxKeys) - 1) / int(defaultListMaxKeys))
			if got := listRequests.Load(); got >= totalPages {
				cancel()
				t.Fatalf("ListObjectsV2 requests = %d, want fewer than total pages %d after cancellation", got, totalPages)
			}
			totalDeleteBatches := int64((len(keys) + MaxDeleteBatch - 1) / MaxDeleteBatch)
			if got := deleteRequests.Load(); got >= totalDeleteBatches {
				cancel()
				t.Fatalf("DeleteObjects requests = %d, want fewer than total batches %d after cancellation", got, totalDeleteBatches)
			}
			if _, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
				Bucket: aws.String(bucket),
				Key:    aws.String(keys[len(keys)-1]),
			}); err != nil {
				cancel()
				t.Fatalf("HeadObject after cancellation: %v", err)
			}
			cancel()
		})
	}
}

func TestDeleteKeysSingleDeleteCancellationProgressMatchesResult(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	session, client, _ := newFakeSession(t, withCancelAfterDeleteObjectRequest(4, cancel), func(options *Options) {
		options.Concurrency = 1
	})
	bucket := "delete-single-cancel-progress"
	keys := []string{"a", "b", "c", "d"}
	putObjects(t, client, bucket, keys...)
	session.singleDelete.Store(true)
	var progress []Progress

	result, err := session.DeleteKeys(ctx, bucket, keys, DeleteOptions{
		Progress: func(value Progress) {
			progress = append(progress, value)
		},
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("DeleteKeys error = %v, want ErrCanceled", err)
	}
	if len(result.Deleted) != 3 {
		t.Fatalf("Deleted = %v, want three keys", result.Deleted)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("Failed = %#v, want no false failures", result.Failed)
	}
	if len(progress) != 3 {
		t.Fatalf("progress callbacks = %#v, want three attempted successes", progress)
	}
	for _, value := range progress {
		if !value.Done || value.Err != nil {
			t.Fatalf("progress = %#v, want only successful completion events", progress)
		}
	}
}

func TestDeletePrefixCancelledContext(t *testing.T) {
	session, _, _ := newFakeSession(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result, err := session.DeletePrefix(ctx, "delete-cancelled", "p/", DeleteOptions{})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("DeletePrefix error = %v, want ErrCanceled", err)
	}
	if len(result.Deleted) != 0 || len(result.Failed) != 0 {
		t.Fatalf("cancelled result = %#v, want empty result", result)
	}
}

func countDeleteObjects(requests *atomic.Int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				requests.Add(1)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func countDeleteObjectsAndLists(deleteRequests, listRequests *atomic.Int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				deleteRequests.Add(1)
			}
			if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
				listRequests.Add(1)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func countAllRequests(requests *atomic.Int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			next.ServeHTTP(w, r)
		})
	}
}

func countDeleteMutations(requests *atomic.Int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost || r.Method == http.MethodDelete {
				requests.Add(1)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func captureDeleteObjectBody(mu *sync.Mutex, body *string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				value, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(value))
				mu.Lock()
				*body = string(value)
				mu.Unlock()
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectDeleteObjects(bulkRequests, singleRequests *atomic.Int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				bulkRequests.Add(1)
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = io.WriteString(w, "<Error><Code>NotImplemented</Code><Message>x</Message></Error>")
				return
			}
			if r.Method == http.MethodDelete {
				singleRequests.Add(1)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectDeleteObjectRequests(singleRequests *atomic.Int64, status int, code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete {
				singleRequests.Add(1)
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "<Error><Code>"+code+"</Code><Message>delete failed</Message></Error>")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectDeleteObjectAndCountLists(listRequests, singleRequests *atomic.Int64, status int, code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
				listRequests.Add(1)
			}
			if r.Method == http.MethodDelete {
				singleRequests.Add(1)
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "<Error><Code>"+code+"</Code><Message>delete failed</Message></Error>")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectDeleteObjectsAndDeleteObjectRequests(bulkRequests, singleRequests *atomic.Int64, status int, code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				bulkRequests.Add(1)
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "<Error><Code>"+code+"</Code><Message>delete failed</Message></Error>")
				return
			}
			if r.Method == http.MethodDelete {
				singleRequests.Add(1)
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "<Error><Code>"+code+"</Code><Message>delete failed</Message></Error>")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func cancelOnDeleteObjectsAndFail(cancel context.CancelFunc, status int, code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				cancel()
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "<Error><Code>"+code+"</Code><Message>delete failed</Message></Error>")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectUnsupportedDeletesAndCountLists(listRequests, bulkRequests, singleRequests *atomic.Int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
				listRequests.Add(1)
			}
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				bulkRequests.Add(1)
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = io.WriteString(w, "<Error><Code>NotImplemented</Code><Message>delete unsupported</Message></Error>")
				return
			}
			if r.Method == http.MethodDelete {
				singleRequests.Add(1)
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(http.StatusNotImplemented)
				_, _ = io.WriteString(w, "<Error><Code>NotImplemented</Code><Message>delete unsupported</Message></Error>")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func addDeleteObjectError(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || !r.URL.Query().Has("delete") {
				next.ServeHTTP(w, r)
				return
			}

			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, r)
			body := append([]byte(nil), recorder.Body.Bytes()...)
			body = removeDeleteObjectSuccess(body, key)
			marker := []byte("</DeleteResult>")
			insertion := []byte("<Error><Key>" + key + "</Key><Code>AccessDenied</Code><Message>denied</Message></Error>")
			if index := bytes.Index(body, marker); index >= 0 {
				rewritten := make([]byte, 0, len(body)+len(insertion))
				rewritten = append(rewritten, body[:index]...)
				rewritten = append(rewritten, insertion...)
				rewritten = append(rewritten, body[index:]...)
				body = rewritten
			}

			for name, values := range recorder.Header() {
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}
			w.Header().Del("Content-Length")
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(body)
		})
	}
}

func omitDeleteObjectSuccess(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || !r.URL.Query().Has("delete") {
				next.ServeHTTP(w, r)
				return
			}

			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, r)
			body := removeDeleteObjectSuccess(append([]byte(nil), recorder.Body.Bytes()...), key)
			for name, values := range recorder.Header() {
				for _, value := range values {
					w.Header().Add(name, value)
				}
			}
			w.Header().Del("Content-Length")
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(body)
		})
	}
}

func removeDeleteObjectSuccess(body []byte, key string) []byte {
	const (
		openTag  = "<Deleted>"
		closeTag = "</Deleted>"
	)
	wantedKey := []byte("<Key>" + key + "</Key>")
	searchStart := 0
	for {
		startOffset := bytes.Index(body[searchStart:], []byte(openTag))
		if startOffset < 0 {
			return body
		}
		start := searchStart + startOffset
		closeOffset := bytes.Index(body[start+len(openTag):], []byte(closeTag))
		if closeOffset < 0 {
			return body
		}
		end := start + len(openTag) + closeOffset + len(closeTag)
		if bytes.Contains(body[start:end], wantedKey) {
			return append(body[:start], body[end:]...)
		}
		searchStart = end
	}
}

func rejectDeleteObjectsAfter(requestNumber int64, status int, code string) func(http.Handler) http.Handler {
	var requests atomic.Int64
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				if requests.Add(1) >= requestNumber {
					w.Header().Set("Content-Type", "application/xml")
					w.WriteHeader(status)
					_, _ = io.WriteString(w, "<Error><Code>"+code+"</Code><Message>delete failed</Message></Error>")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func rejectDeleteObjectsAndCountLists(listRequests *atomic.Int64, status int, code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.URL.Query().Get("list-type") == "2" {
				listRequests.Add(1)
			}
			if r.Method == http.MethodPost && r.URL.Query().Has("delete") {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "<Error><Code>"+code+"</Code><Message>delete failed</Message></Error>")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type cancelAfterDeleteObjectRequestTransport struct {
	transport http.RoundTripper
	requests  *atomic.Int64
	cancelAt  int64
	cancel    context.CancelFunc
}

func (t cancelAfterDeleteObjectRequestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Method == http.MethodDelete && t.requests.Add(1) == t.cancelAt {
		t.cancel()
		return nil, r.Context().Err()
	}
	return t.transport.RoundTrip(r)
}

func withCancelAfterDeleteObjectRequest(cancelAt int64, cancel context.CancelFunc) func(*Options) {
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
		wrapped.Transport = cancelAfterDeleteObjectRequestTransport{
			transport: transport,
			requests:  new(atomic.Int64),
			cancelAt:  cancelAt,
			cancel:    cancel,
		}
		opts.HTTPClient = &wrapped
	}
}

func numberedDeleteKeys(prefix string, count int) []string {
	keys := make([]string, count)
	for i := range keys {
		keys[i] = fmt.Sprintf("%s%04d", prefix, i)
	}
	return keys
}
