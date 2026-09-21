package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	transferTestSize      = 16 * 1024 * 1024
	transferTestPartSize  = 5 * 1024 * 1024
	transferTestBucket    = "transfer-test-bucket"
	transferTestObjectKey = "large-object.bin"
)

var transferTestPayload = makeTransferTestPayload()

func makeTransferTestPayload() []byte {
	payload := make([]byte, transferTestSize)
	for i := range payload {
		payload[i] = byte((i*31 + 17) % 251)
	}
	return payload
}

func transferTestOptions(opts *Options) {
	opts.PartSize = transferTestPartSize
	opts.Concurrency = 4
}

func makeTransferTestBucket(t *testing.T, client *awss3.Client, bucket string) {
	t.Helper()
	_, err := client.CreateBucket(t.Context(), &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		t.Fatalf("CreateBucket %q: %v", bucket, err)
	}
}

func putTransferTestObject(t *testing.T, client *awss3.Client, bucket, key string, payload []byte) {
	t.Helper()
	_, err := client.PutObject(t.Context(), &awss3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("PutObject %q/%q: %v", bucket, key, err)
	}
}

func writeTransferTestFile(t *testing.T, path string, payload []byte) {
	t.Helper()
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("WriteFile %q: %v", path, err)
	}
}

func assertTransferPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Stat(%q) = %v, want path to be absent", path, err)
	}
}

func assertTransferTempsMissing(t *testing.T, localPath string) {
	t.Helper()
	matches, err := filepath.Glob(localPath + ".s3ry-tmp-*")
	if err != nil {
		t.Fatalf("Glob(%q): %v", localPath+".s3ry-tmp-*", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary download files = %v, want none", matches)
	}
}

func setTransferProgressInterval(t *testing.T, interval time.Duration) {
	t.Helper()
	previous := progressInterval
	progressInterval = interval
	t.Cleanup(func() {
		progressInterval = previous
	})
}

func TestUploadMultipartStatAndETag(t *testing.T) {
	var (
		requestMu    sync.Mutex
		requests     []string
		completeBody []byte
	)
	session, client, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capture := &transferCaptureResponseWriter{ResponseWriter: w}
			next.ServeHTTP(capture, r)
			requestMu.Lock()
			requests = append(requests, r.Method+" "+r.URL.RequestURI())
			if r.Method == http.MethodPost && r.URL.Query().Get("uploadId") != "" {
				completeBody = append([]byte(nil), capture.body.Bytes()...)
			}
			requestMu.Unlock()
		})
	}, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	source := filepath.Join(t.TempDir(), transferTestObjectKey)
	writeTransferTestFile(t, source, transferTestPayload)

	if err := session.Upload(t.Context(), source, transferTestBucket, transferTestObjectKey, UploadOptions{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	head, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
		Bucket: aws.String(transferTestBucket),
		Key:    aws.String(transferTestObjectKey),
	})
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if got := aws.ToInt64(head.ContentLength); got != int64(len(transferTestPayload)) {
		t.Fatalf("HeadObject ContentLength = %d, want %d", got, len(transferTestPayload))
	}
	requestMu.Lock()
	requestSnapshot := append([]string(nil), requests...)
	completeSnapshot := append([]byte(nil), completeBody...)
	requestMu.Unlock()
	if !bytes.Contains(completeSnapshot, []byte("-4")) {
		t.Fatalf("CompleteMultipartUpload response = %q, want ETag ending in -4", completeSnapshot)
	}
	partRequests := 0
	for _, request := range requestSnapshot {
		if strings.Contains(request, "partNumber=") {
			partRequests++
		}
	}
	if partRequests != 4 {
		t.Fatalf("Upload part requests = %d, want 4; requests = %v", partRequests, requestSnapshot)
	}
}

type transferCaptureResponseWriter struct {
	http.ResponseWriter
	body bytes.Buffer
}

func (w *transferCaptureResponseWriter) Write(p []byte) (int, error) {
	_, _ = w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

func TestUploadProgress(t *testing.T) {
	setTransferProgressInterval(t, 0)
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	source := filepath.Join(t.TempDir(), transferTestObjectKey)
	writeTransferTestFile(t, source, transferTestPayload)

	var (
		mu     sync.Mutex
		events []Progress
	)
	progress := func(event Progress) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}

	if err := session.Upload(t.Context(), source, transferTestBucket, transferTestObjectKey, UploadOptions{Progress: progress}); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	mu.Lock()
	snapshots := append([]Progress(nil), events...)
	mu.Unlock()
	nonDone := 0
	done := 0
	var final Progress
	for _, event := range snapshots {
		if event.Done {
			done++
			final = event
		} else {
			nonDone++
		}
	}
	if nonDone < 2 {
		t.Fatalf("Upload progress non-Done events = %d, want at least 2; events = %#v", nonDone, snapshots)
	}
	if done != 1 {
		t.Fatalf("Upload progress Done events = %d, want 1; events = %#v", done, snapshots)
	}
	if final.Transferred != int64(len(transferTestPayload)) {
		t.Fatalf("final Upload Transferred = %d, want %d", final.Transferred, len(transferTestPayload))
	}
	if final.Total != int64(len(transferTestPayload)) {
		t.Fatalf("final Upload Total = %d, want %d", final.Total, len(transferTestPayload))
	}
	if final.Err != nil {
		t.Fatalf("final Upload Err = %v, want nil", final.Err)
	}
}

func TestUploadJSONContentType(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	const key = "document.json"
	source := filepath.Join(t.TempDir(), key)
	writeTransferTestFile(t, source, []byte(`{"ok":true}`))

	if err := session.Upload(t.Context(), source, transferTestBucket, key, UploadOptions{}); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	head, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
		Bucket: aws.String(transferTestBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if got := aws.ToString(head.ContentType); got != "application/json" {
		t.Fatalf("HeadObject ContentType = %q, want application/json", got)
	}
}

func TestUploadExplicitContentTypeAndStorageClass(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	const (
		explicitKey = "explicit.bin"
		storedClass = "STANDARD_IA"
	)
	source := filepath.Join(t.TempDir(), explicitKey)
	writeTransferTestFile(t, source, []byte("explicit options"))
	if err := session.Upload(t.Context(), source, transferTestBucket, explicitKey, UploadOptions{
		ContentType:  "application/x-s3ry-test",
		StorageClass: storedClass,
	}); err != nil {
		t.Fatalf("Upload with explicit options: %v", err)
	}

	head, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
		Bucket: aws.String(transferTestBucket),
		Key:    aws.String(explicitKey),
	})
	if err != nil {
		t.Fatalf("HeadObject explicit options: %v", err)
	}
	if got := aws.ToString(head.ContentType); got != "application/x-s3ry-test" {
		t.Fatalf("HeadObject ContentType = %q, want application/x-s3ry-test", got)
	}
	if got := string(head.StorageClass); got != storedClass {
		t.Fatalf("HeadObject StorageClass = %q, want %q", got, storedClass)
	}

	const emptyStorageClassKey = "empty-storage-class.bin"
	emptySource := filepath.Join(t.TempDir(), emptyStorageClassKey)
	writeTransferTestFile(t, emptySource, []byte("empty storage class"))
	if err := session.Upload(t.Context(), emptySource, transferTestBucket, emptyStorageClassKey, UploadOptions{}); err != nil {
		t.Fatalf("Upload with empty storage class: %v", err)
	}
	emptyHead, err := client.HeadObject(t.Context(), &awss3.HeadObjectInput{
		Bucket: aws.String(transferTestBucket),
		Key:    aws.String(emptyStorageClassKey),
	})
	if err != nil {
		t.Fatalf("HeadObject empty storage class: %v", err)
	}
	if got := string(emptyHead.StorageClass); got != "" {
		t.Fatalf("HeadObject empty StorageClass = %q, want empty", got)
	}
}

func TestDownloadMultipartAndChecksum(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, transferTestObjectKey, transferTestPayload)

	destination := filepath.Join(t.TempDir(), "nested", "downloaded.bin")
	if err := session.Download(t.Context(), transferTestBucket, transferTestObjectKey, destination, DownloadOptions{}); err != nil {
		t.Fatalf("Download: %v", err)
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	wantHash := sha256.Sum256(transferTestPayload)
	gotHash := sha256.Sum256(got)
	if gotHash != wantHash {
		t.Fatalf("download checksum = %x, want %x", gotHash, wantHash)
	}
	assertTransferPathMissing(t, destination+".s3ry-tmp")
	assertTransferTempsMissing(t, destination)
}

func TestDownloadZeroByteObject(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, "empty.bin", nil)

	destination := filepath.Join(t.TempDir(), "empty.bin")
	if err := session.Download(t.Context(), transferTestBucket, "empty.bin", destination, DownloadOptions{}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("Stat downloaded empty file: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("downloaded empty file size = %d, want 0", info.Size())
	}
	assertTransferPathMissing(t, destination+".s3ry-tmp")
	assertTransferTempsMissing(t, destination)
}

func TestDownloadPreservesPreexistingTempSibling(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	const key = "preserve-sibling.bin"
	remote := []byte("downloaded object")
	putTransferTestObject(t, client, transferTestBucket, key, remote)

	destination := filepath.Join(t.TempDir(), key)
	sibling := destination + ".s3ry-tmp"
	siblingContent := []byte("user-owned temporary content")
	writeTransferTestFile(t, sibling, siblingContent)

	if err := session.Download(t.Context(), transferTestBucket, key, destination, DownloadOptions{}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	if got, err := os.ReadFile(destination); err != nil || !bytes.Equal(got, remote) {
		t.Fatalf("downloaded file = %q, err %v, want %q", got, err, remote)
	}
	if got, err := os.ReadFile(sibling); err != nil || !bytes.Equal(got, siblingContent) {
		t.Fatalf("pre-existing sibling = %q, err %v, want %q", got, err, siblingContent)
	}
	assertTransferTempsMissing(t, destination)
}

func TestDownloadConcurrentOverwriteAlways(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	const key = "concurrent-download.bin"
	remote := bytes.Repeat([]byte("concurrent download payload"), 400000)
	putTransferTestObject(t, client, transferTestBucket, key, remote)

	destination := filepath.Join(t.TempDir(), "nested", key)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- session.Download(t.Context(), transferTestBucket, key, destination, DownloadOptions{
				Overwrite: OverwriteAlways,
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if errors.Is(err, ErrExists) {
			t.Fatalf("concurrent Download returned ErrExists: %v", err)
		}
		if err != nil {
			t.Fatalf("concurrent Download: %v", err)
		}
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile concurrent destination: %v", err)
	}
	if !bytes.Equal(got, remote) {
		t.Fatalf("concurrent destination differs from object: got %d bytes, want %d", len(got), len(remote))
	}
	assertTransferPathMissing(t, destination+".s3ry-tmp")
	assertTransferTempsMissing(t, destination)
}

func TestDownloadOverwriteModes(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	const key = "overwrite.txt"
	remote := []byte("remote content")
	localOriginal := []byte("local content")
	putTransferTestObject(t, client, transferTestBucket, key, remote)

	destination := filepath.Join(t.TempDir(), key)
	writeTransferTestFile(t, destination, localOriginal)

	err := session.Download(t.Context(), transferTestBucket, key, destination, DownloadOptions{})
	if !errors.Is(err, ErrExists) {
		t.Fatalf("Download OverwriteFail error = %v, want ErrExists", err)
	}
	if got, readErr := os.ReadFile(destination); readErr != nil || !bytes.Equal(got, localOriginal) {
		t.Fatalf("OverwriteFail changed file: bytes=%q err=%v", got, readErr)
	}

	var skipped Progress
	err = session.Download(t.Context(), transferTestBucket, key, destination, DownloadOptions{
		Overwrite: OverwriteSkip,
		Progress: func(event Progress) {
			if event.Done {
				skipped = event
			}
		},
	})
	if err != nil {
		t.Fatalf("Download OverwriteSkip: %v", err)
	}
	if !skipped.Done || !skipped.Skipped {
		t.Fatalf("OverwriteSkip progress = %#v, want Done and Skipped", skipped)
	}
	if got, readErr := os.ReadFile(destination); readErr != nil || !bytes.Equal(got, localOriginal) {
		t.Fatalf("OverwriteSkip changed file: bytes=%q err=%v", got, readErr)
	}

	if err := session.Download(t.Context(), transferTestBucket, key, destination, DownloadOptions{Overwrite: OverwriteAlways}); err != nil {
		t.Fatalf("Download OverwriteAlways: %v", err)
	}
	if got, readErr := os.ReadFile(destination); readErr != nil || !bytes.Equal(got, remote) {
		t.Fatalf("OverwriteAlways result = %q err=%v, want %q", got, readErr, remote)
	}
}

func TestDownloadOverwriteFailCreatedDuringTransfer(t *testing.T) {
	testDownloadCreatedDuringTransfer(t, OverwriteFail)
}

func TestDownloadOverwriteSkipCreatedDuringTransfer(t *testing.T) {
	testDownloadCreatedDuringTransfer(t, OverwriteSkip)
}

func testDownloadCreatedDuringTransfer(t *testing.T, mode OverwriteMode) {
	setTransferProgressInterval(t, 0)
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	const key = "created-during-transfer.bin"
	putTransferTestObject(t, client, transferTestBucket, key, transferTestPayload)

	destination := filepath.Join(t.TempDir(), key)
	localOriginal := []byte("created while downloading")
	var (
		createOnce sync.Once
		created    atomic.Bool
		createErr  error
		mu         sync.Mutex
		events     []Progress
	)
	progress := func(event Progress) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		if !event.Done {
			createOnce.Do(func() {
				created.Store(true)
				createErr = os.WriteFile(destination, localOriginal, 0o600)
			})
		}
	}

	err := session.Download(t.Context(), transferTestBucket, key, destination, DownloadOptions{
		Overwrite: mode,
		Progress:  progress,
	})
	if !created.Load() {
		t.Fatal("progress callback did not create the destination")
	}
	if createErr != nil {
		t.Fatalf("create destination from progress callback: %v", createErr)
	}

	mu.Lock()
	snapshots := append([]Progress(nil), events...)
	mu.Unlock()
	if mode == OverwriteFail {
		if !errors.Is(err, ErrExists) {
			t.Fatalf("Download OverwriteFail error = %v, want ErrExists", err)
		}
	} else {
		if err != nil {
			t.Fatalf("Download OverwriteSkip: %v", err)
		}
		skipped := 0
		for _, event := range snapshots {
			if event.Done && event.Skipped {
				skipped++
			}
		}
		if skipped != 1 {
			t.Fatalf("Download OverwriteSkip skipped events = %d, want 1; events = %#v", skipped, snapshots)
		}
	}
	if got, readErr := os.ReadFile(destination); readErr != nil || !bytes.Equal(got, localOriginal) {
		t.Fatalf("destination changed during transfer: bytes=%q err=%v", got, readErr)
	}
	assertTransferTempsMissing(t, destination)
}

func TestDownloadProgressAfterRangeRetry(t *testing.T) {
	setTransferProgressInterval(t, 0)
	var (
		firstRangeFailure atomic.Bool
		rangeRequests     atomic.Int64
	)
	session, client, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet && r.Header.Get("Range") != "" {
				rangeRequests.Add(1)
				if firstRangeFailure.CompareAndSwap(false, true) {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, transferTestObjectKey, transferTestPayload)

	var (
		mu     sync.Mutex
		events []Progress
	)
	destination := filepath.Join(t.TempDir(), "retry.bin")
	err := session.Download(t.Context(), transferTestBucket, transferTestObjectKey, destination, DownloadOptions{
		Progress: func(event Progress) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Download after ranged GET retry: %v", err)
	}
	if !firstRangeFailure.Load() {
		t.Fatal("middleware did not fail a ranged GET")
	}
	if got := rangeRequests.Load(); got < 2 {
		t.Fatalf("ranged GET requests = %d, want at least 2", got)
	}

	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("ReadFile retried download: %v", err)
	}
	wantHash := sha256.Sum256(transferTestPayload)
	gotHash := sha256.Sum256(got)
	if gotHash != wantHash {
		t.Fatalf("retried download checksum = %x, want %x", gotHash, wantHash)
	}

	mu.Lock()
	snapshots := append([]Progress(nil), events...)
	mu.Unlock()
	wantTotal := int64(len(transferTestPayload))
	if len(snapshots) == 0 {
		t.Fatal("retried download emitted no progress events")
	}
	done := 0
	var final Progress
	var previous int64
	for _, event := range snapshots {
		if event.Transferred > event.Total {
			t.Fatalf("Download progress exceeds total: transferred=%d total=%d event=%#v", event.Transferred, event.Total, event)
		}
		if event.Total != wantTotal {
			t.Fatalf("Download progress Total = %d, want %d; event = %#v", event.Total, wantTotal, event)
		}
		if event.Transferred < previous {
			t.Fatalf("Download Transferred decreased from %d to %d", previous, event.Transferred)
		}
		previous = event.Transferred
		if event.Done {
			done++
			final = event
		}
	}
	if done != 1 {
		t.Fatalf("Download Done events = %d, want 1; events = %#v", done, snapshots)
	}
	if final.Transferred != wantTotal {
		t.Fatalf("final Download Transferred = %d, want %d", final.Transferred, wantTotal)
	}
}

func TestUploadCancellationAbortsMultipartUpload(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var (
		cancelOnce    sync.Once
		sawUploadPart atomic.Bool
	)
	session, client, _ := newCountingFake(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("partNumber") != "" {
				sawUploadPart.Store(true)
				cancelOnce.Do(cancel)
			}
			next.ServeHTTP(w, r)
		})
	}, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)

	source := filepath.Join(t.TempDir(), transferTestObjectKey)
	writeTransferTestFile(t, source, transferTestPayload)

	err := session.Upload(ctx, source, transferTestBucket, transferTestObjectKey, UploadOptions{})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("Upload cancellation error = %v, want ErrCanceled", err)
	}
	if !sawUploadPart.Load() {
		t.Fatal("Upload cancellation did not observe an UploadPart request")
	}

	out, err := client.ListMultipartUploads(t.Context(), &awss3.ListMultipartUploadsInput{
		Bucket: aws.String(transferTestBucket),
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads: %v", err)
	}
	inProgress := 0
	for _, upload := range out.Uploads {
		if aws.ToString(upload.Key) == transferTestObjectKey {
			inProgress++
		}
	}
	if inProgress != 0 {
		t.Fatalf("in-progress multipart uploads for %q = %d, want 0", transferTestObjectKey, inProgress)
	}
}

func TestDownloadCancellationRemovesDestinationAndTemp(t *testing.T) {
	setTransferProgressInterval(t, 0)
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, transferTestObjectKey, transferTestPayload)

	destination := filepath.Join(t.TempDir(), "canceled.bin")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var cancelOnce sync.Once

	err := session.Download(ctx, transferTestBucket, transferTestObjectKey, destination, DownloadOptions{
		Progress: func(event Progress) {
			if !event.Done {
				cancelOnce.Do(cancel)
			}
		},
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("Download cancellation error = %v, want ErrCanceled", err)
	}
	assertTransferPathMissing(t, destination)
	assertTransferPathMissing(t, destination+".s3ry-tmp")
	assertTransferTempsMissing(t, destination)
}

func TestUploadLocalErrors(t *testing.T) {
	session := &Session{}
	missing := filepath.Join(t.TempDir(), "missing.bin")
	err := session.Upload(t.Context(), missing, transferTestBucket, "missing.bin", UploadOptions{})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Upload missing file error = %v, want ErrNotFound", err)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Upload missing file error = %v, want fs.ErrNotExist", err)
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("Upload missing file error = %v, want local path %q in message", err, missing)
	}

	directory := filepath.Join(t.TempDir(), "directory")
	if err := os.Mkdir(directory, 0o750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	err = session.Upload(t.Context(), directory, transferTestBucket, "directory", UploadOptions{})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Upload directory error = %v, want ErrInvalid", err)
	}

	t.Run("permission denied", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("permission checks are ineffective for root")
		}
		permissionDenied := filepath.Join(t.TempDir(), "permission-denied.bin")
		writeTransferTestFile(t, permissionDenied, []byte("permission denied"))
		if err := os.Chmod(permissionDenied, 0); err != nil {
			t.Fatalf("Chmod: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(permissionDenied, 0o600) })

		err := session.Upload(t.Context(), permissionDenied, transferTestBucket, "permission-denied.bin", UploadOptions{})
		if !errors.Is(err, ErrAccessDenied) {
			t.Fatalf("Upload permission-denied file error = %v, want ErrAccessDenied", err)
		}
	})
}

func TestDownloadDirectoryIsInvalid(t *testing.T) {
	destination := t.TempDir()
	for _, mode := range []OverwriteMode{OverwriteFail, OverwriteSkip, OverwriteAlways} {
		err := (&Session{}).Download(t.Context(), transferTestBucket, "directory-key", destination, DownloadOptions{Overwrite: mode})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Download directory with overwrite mode %d = %v, want ErrInvalid", mode, err)
		}
	}
}

func TestDownloadProgress(t *testing.T) {
	setTransferProgressInterval(t, 0)
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, transferTestObjectKey, transferTestPayload)

	var (
		mu     sync.Mutex
		events []Progress
	)
	destination := filepath.Join(t.TempDir(), "progress.bin")
	err := session.Download(t.Context(), transferTestBucket, transferTestObjectKey, destination, DownloadOptions{
		Progress: func(event Progress) {
			mu.Lock()
			events = append(events, event)
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}

	mu.Lock()
	snapshots := append([]Progress(nil), events...)
	mu.Unlock()
	if len(snapshots) == 0 {
		t.Fatal("Download emitted no progress events")
	}
	wantTotal := int64(len(transferTestPayload))
	done := 0
	var previous int64
	var final Progress
	for _, event := range snapshots {
		if event.Done {
			done++
			final = event
			continue
		}
		if event.Total != wantTotal {
			t.Fatalf("non-Done Download Total = %d, want %d; event = %#v", event.Total, wantTotal, event)
		}
		if event.Transferred < previous {
			t.Fatalf("Download Transferred decreased from %d to %d", previous, event.Transferred)
		}
		previous = event.Transferred
	}
	if done != 1 {
		t.Fatalf("Download Done events = %d, want 1; events = %#v", done, snapshots)
	}
	if final.Transferred != wantTotal || final.Total != wantTotal {
		t.Fatalf("final Download progress = %#v, want transferred and total %d", final, wantTotal)
	}
	if final.Err != nil {
		t.Fatalf("final Download Err = %v, want nil", final.Err)
	}
	if !snapshots[len(snapshots)-1].Done {
		t.Fatalf("last Download progress event = %#v, want Done", snapshots[len(snapshots)-1])
	}
}

func TestMeterFinishStopsProgress(t *testing.T) {
	var events []Progress
	firstErr := errors.New("first")
	m := &meter{
		fn:   func(event Progress) { events = append(events, event) },
		base: Progress{Op: OpDownload, Total: 10},
	}

	m.finish(firstErr)
	m.add(5)
	m.finish(errors.New("second"))

	if len(events) != 1 {
		t.Fatalf("meter events = %d, want 1; events = %#v", len(events), events)
	}
	if !events[0].Done || events[0].Err != firstErr {
		t.Fatalf("meter final event = %#v, want one Done event with first error", events[0])
	}
}

func TestTransferValidationProgressDone(t *testing.T) {
	var events []Progress
	err := (&Session{}).Upload(t.Context(), "", transferTestBucket, "", UploadOptions{
		Progress: func(event Progress) { events = append(events, event) },
	})
	if err == nil {
		t.Fatal("Upload validation returned nil error")
	}
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != KindInvalid {
		t.Fatalf("Upload validation error = %v, want *Error with KindInvalid", err)
	}
	if len(events) != 1 {
		t.Fatalf("validation progress events = %d, want 1; events = %#v", len(events), events)
	}
	if !events[0].Done {
		t.Fatalf("validation progress event = %#v, want Done", events[0])
	}
	var eventErr *Error
	if !errors.As(events[0].Err, &eventErr) || eventErr != typed {
		t.Fatalf("validation progress Err = %#v, want returned *Error %#v", events[0].Err, typed)
	}
}
