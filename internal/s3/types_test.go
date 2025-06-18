package s3

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestObject(t *testing.T) {
	now := time.Now()
	obj := Object{
		Key:          "test/file.txt",
		Size:         1024,
		LastModified: now,
		ETag:         `"d41d8cd98f00b204e9800998ecf8427e"`,
		StorageClass: "STANDARD",
	}

	assert.Equal(t, "test/file.txt", obj.Key)
	assert.Equal(t, int64(1024), obj.Size)
	assert.Equal(t, now, obj.LastModified)
	assert.Equal(t, `"d41d8cd98f00b204e9800998ecf8427e"`, obj.ETag)
	assert.Equal(t, "STANDARD", obj.StorageClass)
}

func TestBucket(t *testing.T) {
	now := time.Now()
	bucket := Bucket{
		Name:         "test-bucket",
		CreationDate: now,
		Region:       "us-east-1",
	}

	assert.Equal(t, "test-bucket", bucket.Name)
	assert.Equal(t, now, bucket.CreationDate)
	assert.Equal(t, "us-east-1", bucket.Region)
}

func TestUploadRequest(t *testing.T) {
	metadata := map[string]*string{
		"author": stringPtr("test-user"),
		"type":   stringPtr("document"),
	}

	req := UploadRequest{
		Bucket:      "test-bucket",
		Key:         "uploads/test.txt",
		FilePath:    "/local/path/test.txt",
		ContentType: "text/plain",
		Metadata:    metadata,
	}

	assert.Equal(t, "test-bucket", req.Bucket)
	assert.Equal(t, "uploads/test.txt", req.Key)
	assert.Equal(t, "/local/path/test.txt", req.FilePath)
	assert.Equal(t, "text/plain", req.ContentType)
	assert.Equal(t, metadata, req.Metadata)
	assert.Equal(t, "test-user", *req.Metadata["author"])
}

func TestDownloadRequest(t *testing.T) {
	req := DownloadRequest{
		Bucket:   "test-bucket",
		Key:      "downloads/test.txt",
		FilePath: "/local/path/downloaded.txt",
	}

	assert.Equal(t, "test-bucket", req.Bucket)
	assert.Equal(t, "downloads/test.txt", req.Key)
	assert.Equal(t, "/local/path/downloaded.txt", req.FilePath)
}

func TestListRequest(t *testing.T) {
	req := ListRequest{
		Bucket:     "test-bucket",
		Prefix:     "documents/",
		Delimiter:  "/",
		MaxKeys:    100,
		StartAfter: "documents/file1.txt",
	}

	assert.Equal(t, "test-bucket", req.Bucket)
	assert.Equal(t, "documents/", req.Prefix)
	assert.Equal(t, "/", req.Delimiter)
	assert.Equal(t, int64(100), req.MaxKeys)
	assert.Equal(t, "documents/file1.txt", req.StartAfter)
}

func TestProgressCallback(t *testing.T) {
	var capturedTransferred, capturedTotal int64

	callback := func(bytesTransferred, totalBytes int64) {
		capturedTransferred = bytesTransferred
		capturedTotal = totalBytes
	}

	// Test the callback
	callback(512, 1024)

	assert.Equal(t, int64(512), capturedTransferred)
	assert.Equal(t, int64(1024), capturedTotal)
}

func TestProgressCallback_Multiple(t *testing.T) {
	var calls []struct {
		transferred int64
		total       int64
	}

	callback := func(bytesTransferred, totalBytes int64) {
		calls = append(calls, struct {
			transferred int64
			total       int64
		}{bytesTransferred, totalBytes})
	}

	// Test multiple calls
	callback(256, 1024)
	callback(512, 1024)
	callback(1024, 1024)

	assert.Len(t, calls, 3)
	assert.Equal(t, int64(256), calls[0].transferred)
	assert.Equal(t, int64(512), calls[1].transferred)
	assert.Equal(t, int64(1024), calls[2].transferred)

	for _, call := range calls {
		assert.Equal(t, int64(1024), call.total)
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// Benchmark tests
func BenchmarkObjectCreation(b *testing.B) {
	now := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := Object{
			Key:          "test/file.txt",
			Size:         1024,
			LastModified: now,
			ETag:         `"d41d8cd98f00b204e9800998ecf8427e"`,
			StorageClass: "STANDARD",
		}
		_ = obj
	}
}

func BenchmarkUploadRequestCreation(b *testing.B) {
	metadata := map[string]*string{
		"author": stringPtr("test-user"),
		"type":   stringPtr("document"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := UploadRequest{
			Bucket:      "test-bucket",
			Key:         "uploads/test.txt",
			FilePath:    "/local/path/test.txt",
			ContentType: "text/plain",
			Metadata:    metadata,
		}
		_ = req
	}
}
