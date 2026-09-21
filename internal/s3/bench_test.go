package s3

import (
	"bytes"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

const (
	benchmarkListObjectCount   = 1000
	benchmarkWalkPrefixCount   = 50
	benchmarkWalkKeysPerPrefix = 100
	benchmarkWalkObjectCount   = benchmarkWalkPrefixCount * benchmarkWalkKeysPerPrefix
	benchmarkTransferSize      = 16 * 1024 * 1024
	benchmarkTransferPartSize  = 5 * 1024 * 1024
)

func benchmarkFakeSession(b *testing.B, mods ...func(*Options)) (*Session, *awss3.Client) {
	b.Helper()

	b.Setenv("AWS_ACCESS_KEY_ID", "test")
	b.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	b.Setenv("AWS_SESSION_TOKEN", "")
	b.Setenv("AWS_REGION", "us-east-1")
	b.Setenv("AWS_CONFIG_FILE", "/dev/null")
	b.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
	b.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	b.Setenv("AWS_ENDPOINT_URL", "")
	b.Setenv("AWS_ENDPOINT_URL_S3", "")

	fake := gofakes3.New(s3mem.New())
	ts, serverErr := tryLocalTestServer(func() *httptest.Server {
		server := httptest.NewServer(fake.Server())
		b.Cleanup(server.Close)
		return server
	})
	if ts == nil {
		b.Logf("loopback listener unavailable, using in-memory test server: %v", serverErr)
		ts = httptest.NewTestServer(b, fake.Server())
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

	session, err := NewSession(b.Context(), opts)
	if err != nil {
		b.Fatalf("NewSession: %v", err)
	}
	return session, session.clientForRegion(session.Region())
}

func benchmarkPutObjects(b *testing.B, client *awss3.Client, bucket string, keys ...string) {
	b.Helper()
	_, err := client.CreateBucket(b.Context(), &awss3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		b.Fatalf("CreateBucket %q: %v", bucket, err)
	}
	for _, key := range keys {
		_, err := client.PutObject(b.Context(), &awss3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   strings.NewReader(key),
		})
		if err != nil {
			b.Fatalf("PutObject %q/%q: %v", bucket, key, err)
		}
	}
}

func BenchmarkListPage1000(b *testing.B) {
	const bucket = "benchmark-list-page-1000"
	keys := make([]string, benchmarkListObjectCount)
	for i := range keys {
		keys[i] = fmt.Sprintf("object-%04d", i)
	}

	session, client := benchmarkFakeSession(b)
	benchmarkPutObjects(b, client, bucket, keys...)
	ctx := b.Context()

	warmPage, err := session.ListPage(ctx, bucket, "", "", benchmarkListObjectCount)
	if err != nil {
		b.Fatalf("warm ListPage: %v", err)
	}
	if len(warmPage.Objects) != benchmarkListObjectCount {
		b.Fatalf("warm ListPage objects = %d, want %d", len(warmPage.Objects), benchmarkListObjectCount)
	}

	runtime.GC()
	const measurementRuns = 4
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < measurementRuns; i++ {
		page, listErr := session.ListPage(ctx, bucket, "", "", benchmarkListObjectCount)
		if listErr != nil {
			b.Fatalf("measure ListPage: %v", listErr)
		}
		if len(page.Objects) != benchmarkListObjectCount {
			b.Fatalf("measure ListPage objects = %d, want %d", len(page.Objects), benchmarkListObjectCount)
		}
	}
	runtime.ReadMemStats(&after)
	allocatedBytes := (after.TotalAlloc - before.TotalAlloc) / measurementRuns

	b.ResetTimer()
	b.ReportAllocs()
	b.ReportMetric(float64(allocatedBytes)/1000, "B/key")
	for i := 0; i < b.N; i++ {
		if _, err := session.ListPage(ctx, bucket, "", "", benchmarkListObjectCount); err != nil {
			b.Fatalf("ListPage: %v", err)
		}
	}
}

func BenchmarkWalkSequential(b *testing.B) {
	const bucket = "benchmark-walk-sequential"
	keys := benchmarkWalkKeys()
	session, client := benchmarkFakeSession(b)
	benchmarkPutObjects(b, client, bucket, keys...)
	ctx := b.Context()
	options := WalkOptions{Concurrency: 1, MaxKeys: benchmarkListObjectCount}

	count := 0
	if err := session.Walk(ctx, bucket, "", options, func(Object) error {
		count++
		return nil
	}); err != nil {
		b.Fatalf("warm Walk: %v", err)
	}
	if count != benchmarkWalkObjectCount {
		b.Fatalf("warm Walk objects = %d, want %d", count, benchmarkWalkObjectCount)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count = 0
		if err := session.Walk(ctx, bucket, "", options, func(Object) error {
			count++
			return nil
		}); err != nil {
			b.Fatalf("Walk: %v", err)
		}
		if count != benchmarkWalkObjectCount {
			b.Fatalf("Walk objects = %d, want %d", count, benchmarkWalkObjectCount)
		}
	}
}

func BenchmarkWalkFanout8(b *testing.B) {
	const bucket = "benchmark-walk-fanout-8"
	keys := benchmarkWalkKeys()
	session, client := benchmarkFakeSession(b)
	benchmarkPutObjects(b, client, bucket, keys...)
	ctx := b.Context()
	options := WalkOptions{Concurrency: 8, MaxKeys: benchmarkListObjectCount}

	count := 0
	if err := session.Walk(ctx, bucket, "", options, func(Object) error {
		count++
		return nil
	}); err != nil {
		b.Fatalf("warm Walk: %v", err)
	}
	if count != benchmarkWalkObjectCount {
		b.Fatalf("warm Walk objects = %d, want %d", count, benchmarkWalkObjectCount)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count = 0
		if err := session.Walk(ctx, bucket, "", options, func(Object) error {
			count++
			return nil
		}); err != nil {
			b.Fatalf("Walk: %v", err)
		}
		if count != benchmarkWalkObjectCount {
			b.Fatalf("Walk objects = %d, want %d", count, benchmarkWalkObjectCount)
		}
	}
}

func BenchmarkUpload16MiB(b *testing.B) {
	const bucket = "benchmark-upload-16mib"
	payload := benchmarkTransferPayload()
	source := filepath.Join(b.TempDir(), "upload.bin")
	if err := os.WriteFile(source, payload, 0o600); err != nil {
		b.Fatalf("WriteFile: %v", err)
	}

	session, client := benchmarkFakeSession(b, benchmarkMultipartOptions)
	benchmarkPutObjects(b, client, bucket)
	ctx := b.Context()
	if err := session.Upload(ctx, source, bucket, "warmup.bin", UploadOptions{}); err != nil {
		b.Fatalf("warm Upload: %v", err)
	}

	b.ResetTimer()
	b.SetBytes(benchmarkTransferSize)
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("upload-%06d.bin", i)
		if err := session.Upload(ctx, source, bucket, key, UploadOptions{}); err != nil {
			b.Fatalf("Upload: %v", err)
		}
	}
}

func BenchmarkDownload16MiB(b *testing.B) {
	const (
		bucket = "benchmark-download-16mib"
		key    = "download-source.bin"
	)
	payload := benchmarkTransferPayload()
	session, client := benchmarkFakeSession(b, benchmarkMultipartOptions)
	benchmarkPutObjects(b, client, bucket)
	_, err := client.PutObject(b.Context(), &awss3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(payload),
	})
	if err != nil {
		b.Fatalf("PutObject: %v", err)
	}
	ctx := b.Context()
	destinationDir := b.TempDir()
	warmDestination := filepath.Join(destinationDir, "warmup.bin")
	if err := session.Download(ctx, bucket, key, warmDestination, DownloadOptions{Overwrite: OverwriteAlways}); err != nil {
		b.Fatalf("warm Download: %v", err)
	}
	if info, err := os.Stat(warmDestination); err != nil {
		b.Fatalf("Stat warm Download: %v", err)
	} else if info.Size() != int64(len(payload)) {
		b.Fatalf("warm Download size = %d, want %d", info.Size(), len(payload))
	}
	if err := os.Remove(warmDestination); err != nil {
		b.Fatalf("Remove warm Download: %v", err)
	}

	b.ResetTimer()
	b.SetBytes(benchmarkTransferSize)
	for i := 0; i < b.N; i++ {
		destination := filepath.Join(destinationDir, fmt.Sprintf("download-%06d.bin", i))
		if err := session.Download(ctx, bucket, key, destination, DownloadOptions{Overwrite: OverwriteAlways}); err != nil {
			b.Fatalf("Download: %v", err)
		}
		b.StopTimer()
		removeErr := os.Remove(destination)
		b.StartTimer()
		if removeErr != nil {
			b.Fatalf("Remove Download: %v", removeErr)
		}
	}
}

func benchmarkWalkKeys() []string {
	keys := make([]string, 0, benchmarkWalkObjectCount)
	for prefix := 0; prefix < benchmarkWalkPrefixCount; prefix++ {
		for key := 0; key < benchmarkWalkKeysPerPrefix; key++ {
			keys = append(keys, fmt.Sprintf("prefix-%02d/object-%03d", prefix, key))
		}
	}
	return keys
}

func benchmarkMultipartOptions(opts *Options) {
	opts.PartSize = benchmarkTransferPartSize
	opts.Concurrency = 4
}

func benchmarkTransferPayload() []byte {
	return bytes.Repeat([]byte{'s'}, benchmarkTransferSize)
}
