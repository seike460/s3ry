package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
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
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile %q: %v", path, err)
	}
}

func assertTransferPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Stat(%q) = %v, want path to be absent", path, err)
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
		mu                sync.Mutex
		events            []Progress
		returned          atomic.Bool
		calledAfterReturn atomic.Bool
	)
	progress := func(event Progress) {
		if returned.Load() {
			calledAfterReturn.Store(true)
		}
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	}

	if err := session.Upload(t.Context(), source, transferTestBucket, transferTestObjectKey, UploadOptions{Progress: progress}); err != nil {
		t.Fatalf("Upload: %v", err)
	}
	returned.Store(true)
	time.Sleep(25 * time.Millisecond)
	if calledAfterReturn.Load() {
		t.Fatal("Upload progress callback ran after Upload returned")
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

func TestDownloadMultipartAndChecksum(t *testing.T) {
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, transferTestObjectKey, transferTestPayload)

	destination := filepath.Join(t.TempDir(), "downloaded.bin")
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

func TestUploadCancellationAbortsMultipartUpload(t *testing.T) {
	setTransferProgressInterval(t, 0)
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	seed, err := client.CreateMultipartUpload(t.Context(), &awss3.CreateMultipartUploadInput{
		Bucket: aws.String(transferTestBucket),
		Key:    aws.String("seed-upload"),
	})
	if err != nil {
		t.Fatalf("CreateMultipartUpload seed: %v", err)
	}
	if _, err := client.AbortMultipartUpload(t.Context(), &awss3.AbortMultipartUploadInput{
		Bucket:   aws.String(transferTestBucket),
		Key:      aws.String("seed-upload"),
		UploadId: seed.UploadId,
	}); err != nil {
		t.Fatalf("AbortMultipartUpload seed: %v", err)
	}

	source := filepath.Join(t.TempDir(), transferTestObjectKey)
	writeTransferTestFile(t, source, transferTestPayload)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var cancelOnce sync.Once
	err = session.Upload(ctx, source, transferTestBucket, transferTestObjectKey, UploadOptions{
		Progress: func(event Progress) {
			if !event.Done {
				cancelOnce.Do(cancel)
			}
		},
	})
	if !errors.Is(err, ErrCanceled) {
		t.Fatalf("Upload cancellation error = %v, want ErrCanceled", err)
	}

	out, err := client.ListMultipartUploads(t.Context(), &awss3.ListMultipartUploadsInput{
		Bucket: aws.String(transferTestBucket),
	})
	if err != nil {
		t.Fatalf("ListMultipartUploads: %v", err)
	}
	if got := len(out.Uploads); got != 0 {
		t.Fatalf("in-progress multipart uploads = %d, want 0", got)
	}
}

func TestDownloadCancellationRemovesDestinationAndTemp(t *testing.T) {
	setTransferProgressInterval(t, 0)
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, transferTestObjectKey, transferTestPayload)

	destination := filepath.Join(t.TempDir(), "canceled.bin")
	tmpPath := destination + ".s3ry-tmp"
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
	assertTransferPathMissing(t, tmpPath)
}

func TestUploadMissingFileAndDirectory(t *testing.T) {
	session, client, _ := newFakeSession(t)
	makeTransferTestBucket(t, client, transferTestBucket)

	missing := filepath.Join(t.TempDir(), "missing.bin")
	if err := session.Upload(t.Context(), missing, transferTestBucket, "missing.bin", UploadOptions{}); err == nil {
		t.Fatal("Upload missing file returned nil error")
	}

	directory := filepath.Join(t.TempDir(), "directory")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	err := session.Upload(t.Context(), directory, transferTestBucket, "directory", UploadOptions{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != KindInvalid {
		t.Fatalf("Upload directory error = %v, want KindInvalid", err)
	}
}

func TestDownloadProgressStopsAfterReturn(t *testing.T) {
	setTransferProgressInterval(t, 0)
	session, client, _ := newFakeSession(t, transferTestOptions)
	makeTransferTestBucket(t, client, transferTestBucket)
	putTransferTestObject(t, client, transferTestBucket, "progress.bin", []byte("progress"))

	var calledAfterReturn atomic.Bool
	returned := atomic.Bool{}
	destination := filepath.Join(t.TempDir(), "progress.bin")
	if err := session.Download(t.Context(), transferTestBucket, "progress.bin", destination, DownloadOptions{
		Progress: func(Progress) {
			if returned.Load() {
				calledAfterReturn.Store(true)
			}
		},
	}); err != nil {
		t.Fatalf("Download: %v", err)
	}
	returned.Store(true)
	time.Sleep(25 * time.Millisecond)
	if calledAfterReturn.Load() {
		t.Fatal("Download progress callback ran after Download returned")
	}
}
