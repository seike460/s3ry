//go:build integration

package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const (
	integrationListPageMaxKeys         = 7
	integrationWalkObjectCount         = 300
	integrationDeleteObjectCount       = 1500
	integrationTransferSize            = 20 * 1024 * 1024
	integrationTransferPartSize  int64 = 5 * 1024 * 1024
)

type integrationFixture struct {
	session       *Session
	client        *awss3.Client
	bucket        string
	root          string
	createdBucket bool // cleanup deletes the bucket only when this fixture created it.
}

type integrationProgressRecorder struct {
	mu     sync.Mutex
	events []Progress
}

func skipUnlessIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("S3RY_TEST_ENDPOINT") == "" {
		t.Skip("S3RY_TEST_ENDPOINT is unset")
	}
}

func TestS3Integration(t *testing.T) {
	var fixture *integrationFixture
	if os.Getenv("S3RY_TEST_ENDPOINT") != "" {
		fixture = newIntegrationFixture(t)
	}

	tests := []struct {
		name string
		run  func(*testing.T, *integrationFixture)
	}{
		{name: "ListPagePaginationIntegration", run: testIntegrationListPagePagination},
		{name: "WalkConcurrencyIntegration", run: testIntegrationWalkConcurrency},
		{name: "TransferProgressIntegration", run: testIntegrationTransferProgress},
		{name: "DeleteKeysIntegration", run: testIntegrationDeleteKeys},
		{name: "PresignGetIntegration", run: testIntegrationPresignGet},
		{name: "BucketRegionIntegration", run: testIntegrationBucketRegion},
		{name: "UploadCancellationIntegration", run: testIntegrationUploadCancellation},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			skipUnlessIntegration(t)
			test.run(t, fixture)
		})
	}
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()

	endpoint := os.Getenv("S3RY_TEST_ENDPOINT")
	region := os.Getenv("S3RY_TEST_REGION")
	if region == "" {
		region = "us-east-1"
	}
	bucket := os.Getenv("S3RY_TEST_BUCKET")
	if bucket == "" {
		bucket = fmt.Sprintf("s3ry-it-%d", time.Now().UnixNano())
	}

	session, err := NewSession(t.Context(), Options{
		EndpointURL: endpoint,
		PathStyle:   true,
		Region:      region,
		Concurrency: 8,
		PartSize:    integrationTransferPartSize,
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}

	client := session.cache.clientForRegion(session.Region())
	createdBucket := false
	if _, err := client.HeadBucket(t.Context(), &awss3.HeadBucketInput{Bucket: aws.String(bucket)}); err != nil {
		if !integrationBucketMissing(err) {
			t.Fatalf("HeadBucket %q: %v", bucket, err)
		}
		if _, err := client.CreateBucket(t.Context(), &awss3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
			if !integrationBucketAlreadyExists(err) {
				t.Fatalf("CreateBucket %q: %v", bucket, err)
			}
		} else {
			createdBucket = true
		}
	}

	fixture := &integrationFixture{
		session:       session,
		client:        client,
		bucket:        bucket,
		root:          fmt.Sprintf("s3ry-integration-%d/", time.Now().UnixNano()),
		createdBucket: createdBucket,
	}
	t.Cleanup(func() { fixture.cleanup(t) })
	return fixture
}

func (f *integrationFixture) cleanup(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := f.session.DeletePrefix(ctx, f.bucket, f.root, DeleteOptions{})
	if err != nil {
		t.Errorf("DeletePrefix %q: %v", f.root, err)
	}
	if len(result.Failed) != 0 {
		t.Errorf("DeletePrefix %q failed keys: %#v", f.root, result.Failed)
	}
	if f.createdBucket {
		if _, err := f.client.DeleteBucket(ctx, &awss3.DeleteBucketInput{Bucket: aws.String(f.bucket)}); err != nil {
			t.Errorf("DeleteBucket %q: %v", f.bucket, err)
		}
	}
}

func integrationBucketMissing(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		switch apiErr.ErrorCode() {
		case "NoSuchBucket", "NotFound":
			return true
		}
	}

	var responseErr *awshttp.ResponseError
	return errors.As(err, &responseErr) && responseErr != nil && responseErr.HTTPStatusCode() == http.StatusNotFound
}

func integrationBucketAlreadyExists(err error) bool {
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

func (f *integrationFixture) prefix(name string) string {
	return f.root + name + "/"
}

func (f *integrationFixture) putObject(t *testing.T, key string, body []byte) {
	t.Helper()
	if _, err := f.client.PutObject(t.Context(), &awss3.PutObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	}); err != nil {
		t.Fatalf("PutObject %q: %v", key, err)
	}
}

func (f *integrationFixture) putKeys(t *testing.T, keys []string) {
	t.Helper()
	for _, key := range keys {
		f.putObject(t, key, []byte(key))
	}
}

func integrationKeys(prefix, name string, count int) []string {
	keys := make([]string, count)
	for i := range keys {
		keys[i] = fmt.Sprintf("%s%s-%04d", prefix, name, i)
	}
	return keys
}

func integrationPayload(size int) []byte {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = byte((i*31 + 17) % 251)
	}
	return payload
}

func (r *integrationProgressRecorder) callback(event Progress) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *integrationProgressRecorder) snapshot() []Progress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Progress(nil), r.events...)
}

func assertOneIntegrationDone(t *testing.T, operation string, events []Progress) Progress {
	t.Helper()
	done := 0
	var final Progress
	for _, event := range events {
		if event.Done {
			done++
			final = event
		}
	}
	if done != 1 {
		t.Fatalf("%s Done progress events = %d, want 1; events = %#v", operation, done, events)
	}
	return final
}

func testIntegrationListPagePagination(t *testing.T, f *integrationFixture) {
	prefix := f.prefix("list-page")
	wantObjects := make(map[string]struct{}, 20)
	wantPrefixes := make(map[string]struct{}, 3)
	keys := make([]string, 0, 23)
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("%sobject-%02d", prefix, i)
		keys = append(keys, key)
		wantObjects[key] = struct{}{}
	}
	for i := 0; i < 3; i++ {
		commonPrefix := fmt.Sprintf("%scommon-%02d/", prefix, i)
		keys = append(keys, commonPrefix+"object")
		wantPrefixes[commonPrefix] = struct{}{}
	}
	f.putKeys(t, keys)

	gotObjects := make(map[string]struct{}, len(wantObjects))
	gotPrefixes := make(map[string]struct{}, len(wantPrefixes))
	token := ""
	pages := 0
	for {
		page, err := f.session.ListPage(t.Context(), f.bucket, prefix, token, integrationListPageMaxKeys)
		if err != nil {
			t.Fatalf("ListPage token %q: %v", token, err)
		}
		pages++
		for _, object := range page.Objects {
			gotObjects[object.Key] = struct{}{}
		}
		for _, commonPrefix := range page.Prefixes {
			gotPrefixes[commonPrefix] = struct{}{}
		}
		if page.NextToken == "" {
			if page.IsTruncated {
				t.Fatal("final ListPage has IsTruncated=true with an empty NextToken")
			}
			break
		}
		if !page.IsTruncated {
			t.Fatal("ListPage has a NextToken with IsTruncated=false")
		}
		if page.NextToken == token {
			t.Fatalf("ListPage repeated token %q", token)
		}
		token = page.NextToken
	}

	if pages != 4 {
		t.Fatalf("ListPage pages = %d, want 4", pages)
	}
	if len(gotObjects) != len(wantObjects) || !integrationStringSetEqual(gotObjects, wantObjects) {
		t.Fatalf("ListPage objects = %#v, want exact set %#v", gotObjects, wantObjects)
	}
	if len(gotPrefixes) != len(wantPrefixes) || !integrationStringSetEqual(gotPrefixes, wantPrefixes) {
		t.Fatalf("ListPage common prefixes = %#v, want exact set %#v", gotPrefixes, wantPrefixes)
	}
}

func testIntegrationWalkConcurrency(t *testing.T, f *integrationFixture) {
	prefix := f.prefix("walk")
	wantKeys := integrationKeys(prefix, "object", integrationWalkObjectCount)
	f.putKeys(t, wantKeys)

	sequential, sequentialCount := walkIntegrationKeys(t, f, prefix, 1)
	parallel, parallelCount := walkIntegrationKeys(t, f, prefix, 8)
	if sequentialCount != integrationWalkObjectCount || parallelCount != integrationWalkObjectCount {
		t.Fatalf("Walk callback counts = %d and %d, want %d", sequentialCount, parallelCount, integrationWalkObjectCount)
	}
	want := make(map[string]struct{}, len(wantKeys))
	for _, key := range wantKeys {
		want[key] = struct{}{}
	}
	if !integrationStringSetEqual(sequential, want) {
		t.Fatalf("Walk concurrency 1 keys do not match the expected set")
	}
	if !integrationStringSetEqual(parallel, want) {
		t.Fatalf("Walk concurrency 8 keys do not match the expected set")
	}
	if !integrationStringSetEqual(sequential, parallel) {
		t.Fatal("Walk concurrency 1 and 8 returned different key sets")
	}
}

func walkIntegrationKeys(t *testing.T, f *integrationFixture, prefix string, concurrency int) (map[string]struct{}, int) {
	t.Helper()
	seen := make(map[string]struct{})
	count := 0
	err := f.session.Walk(t.Context(), f.bucket, prefix, WalkOptions{
		Concurrency: concurrency,
		MaxKeys:     37,
	}, func(object Object) error {
		seen[object.Key] = struct{}{}
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("Walk concurrency %d: %v", concurrency, err)
	}
	return seen, count
}

func testIntegrationTransferProgress(t *testing.T, f *integrationFixture) {
	payload := integrationPayload(integrationTransferSize)
	source := filepath.Join(t.TempDir(), "upload.bin")
	if err := os.WriteFile(source, payload, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	prefix := f.prefix("transfer")
	key := prefix + "object.bin"

	uploadProgress := &integrationProgressRecorder{}
	if err := f.session.Upload(t.Context(), source, f.bucket, key, UploadOptions{Progress: uploadProgress.callback}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	uploadFinal := assertOneIntegrationDone(t, "Upload", uploadProgress.snapshot())
	if uploadFinal.Err != nil {
		t.Fatalf("Upload final progress error = %v, want nil", uploadFinal.Err)
	}

	destination := filepath.Join(t.TempDir(), "download.bin")
	downloadProgress := &integrationProgressRecorder{}
	if err := f.session.Download(t.Context(), f.bucket, key, destination, DownloadOptions{
		Overwrite: OverwriteAlways,
		Progress:  downloadProgress.callback,
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	downloadFinal := assertOneIntegrationDone(t, "Download", downloadProgress.snapshot())
	if downloadFinal.Err != nil {
		t.Fatalf("Download final progress error = %v, want nil", downloadFinal.Err)
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	wantHash := sha256.Sum256(payload)
	gotHash := sha256.Sum256(got)
	if gotHash != wantHash {
		t.Fatalf("download sha256 = %x, want %x", gotHash, wantHash)
	}
}

func testIntegrationDeleteKeys(t *testing.T, f *integrationFixture) {
	prefix := f.prefix("delete")
	keys := integrationKeys(prefix, "object", integrationDeleteObjectCount)
	f.putKeys(t, keys)

	result, err := f.session.DeleteKeys(t.Context(), f.bucket, keys, DeleteOptions{})
	if err != nil {
		t.Fatalf("DeleteKeys: %v", err)
	}
	if len(result.Deleted) != integrationDeleteObjectCount {
		t.Fatalf("DeleteKeys deleted keys = %d, want %d", len(result.Deleted), integrationDeleteObjectCount)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("DeleteKeys failed keys = %#v, want none", result.Failed)
	}

	page, err := f.session.ListPage(t.Context(), f.bucket, prefix, "", integrationListPageMaxKeys)
	if err != nil {
		t.Fatalf("ListPage after DeleteKeys: %v", err)
	}
	if len(page.Objects) != 0 || len(page.Prefixes) != 0 || page.NextToken != "" {
		t.Fatalf("ListPage after DeleteKeys = %#v, want empty page", page)
	}
}

func testIntegrationPresignGet(t *testing.T, f *integrationFixture) {
	prefix := f.prefix("presign")
	key := prefix + "object.txt"
	want := []byte("presigned object body")
	f.putObject(t, key, want)

	presignedURL, err := f.session.PresignGet(t.Context(), f.bucket, key, time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	response, err := http.Get(presignedURL)
	if err != nil {
		t.Fatalf("http.Get: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("http.Get status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("ReadAll presigned response: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("presigned body = %q, want %q", got, want)
	}
}

func testIntegrationBucketRegion(t *testing.T, f *integrationFixture) {
	region, err := f.session.BucketRegion(t.Context(), f.bucket)
	if err != nil {
		t.Fatalf("BucketRegion: %v", err)
	}
	if region == "" {
		t.Fatal("BucketRegion returned an empty region")
	}
}

func testIntegrationUploadCancellation(t *testing.T, f *integrationFixture) {
	payload := integrationPayload(integrationTransferSize)
	source := filepath.Join(t.TempDir(), "canceled-upload.bin")
	if err := os.WriteFile(source, payload, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	prefix := f.prefix("cancel")
	key := prefix + "object.bin"

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	progress := &integrationProgressRecorder{}
	var first sync.Once
	err := f.session.Upload(ctx, source, f.bucket, key, UploadOptions{
		ProgressInterval: time.Nanosecond,
		Progress: func(event Progress) {
			progress.callback(event)
			first.Do(cancel)
		},
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("canceled Upload error = %v, want ErrCanceled", err)
	}
	events := progress.snapshot()
	if len(events) == 0 {
		t.Fatal("canceled Upload emitted no progress callback")
	}
	if events[0].Done {
		t.Fatalf("first canceled Upload progress event = %#v, want an in-progress event", events[0])
	}

	paginator := awss3.NewListMultipartUploadsPaginator(f.client, &awss3.ListMultipartUploadsInput{
		Bucket: aws.String(f.bucket),
		Prefix: aws.String(key),
	})
	for paginator.HasMorePages() {
		page, listErr := paginator.NextPage(t.Context())
		if listErr != nil {
			t.Fatalf("ListMultipartUploads: %v", listErr)
		}
		for _, upload := range page.Uploads {
			if aws.ToString(upload.Key) == key {
				t.Fatalf("ListMultipartUploads contains canceled key %q", key)
			}
		}
	}
}

func integrationStringSetEqual(got, want map[string]struct{}) bool {
	if len(got) != len(want) {
		return false
	}
	for key := range want {
		if _, ok := got[key]; !ok {
			return false
		}
	}
	return true
}
