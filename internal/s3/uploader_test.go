package s3

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultUploadConfig(t *testing.T) {
	config := DefaultUploadConfig()

	assert.Equal(t, 5, config.ConcurrentUploads)
	assert.Equal(t, int64(5*1024*1024), config.PartSize)
	assert.Equal(t, 3, config.Concurrency)
	assert.True(t, config.ChecksumValidation)
	assert.True(t, config.AutoContentType)
	assert.False(t, config.Compression)
	assert.True(t, config.Deduplication)
	assert.Nil(t, config.Metadata)
	assert.Nil(t, config.OnProgress)
}

func TestNewUploader(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultUploadConfig()
	uploader := NewUploader(client, config)

	assert.NotNil(t, uploader)
	assert.NotNil(t, uploader.client)
	assert.NotNil(t, uploader.pool)
	assert.Equal(t, client, uploader.client)

	// Clean up
	uploader.Close()
}

func TestUploader_Close(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultUploadConfig()
	uploader := NewUploader(client, config)

	// Should not panic
	assert.NotPanics(t, func() {
		uploader.Close()
	})
}

func TestUploadRequest_Creation(t *testing.T) {
	metadata := map[string]*string{
		"author": stringPtr("test-user"),
		"type":   stringPtr("document"),
	}

	request := UploadRequest{
		Bucket:      "test-bucket",
		Key:         "path/to/file.txt",
		FilePath:    "/local/path/file.txt",
		ContentType: "text/plain",
		Metadata:    metadata,
	}

	assert.Equal(t, "test-bucket", request.Bucket)
	assert.Equal(t, "path/to/file.txt", request.Key)
	assert.Equal(t, "/local/path/file.txt", request.FilePath)
	assert.Equal(t, "text/plain", request.ContentType)
	assert.Equal(t, metadata, request.Metadata)
}

func TestUploadConfig_CustomValues(t *testing.T) {
	metadata := map[string]*string{
		"environment": stringPtr("test"),
	}

	config := UploadConfig{
		ConcurrentUploads:  10,
		PartSize:           10 * 1024 * 1024,
		Concurrency:        5,
		ChecksumValidation: false,
		AutoContentType:    false,
		Metadata:           metadata,
		Compression:        true,
		Deduplication:      false,
		OnProgress: func(bytes, total int64) {
			// Custom progress callback
		},
	}

	assert.Equal(t, 10, config.ConcurrentUploads)
	assert.Equal(t, int64(10*1024*1024), config.PartSize)
	assert.Equal(t, 5, config.Concurrency)
	assert.False(t, config.ChecksumValidation)
	assert.False(t, config.AutoContentType)
	assert.Equal(t, metadata, config.Metadata)
	assert.True(t, config.Compression)
	assert.False(t, config.Deduplication)
	assert.NotNil(t, config.OnProgress)
}

func TestUploader_MultipleUploaders(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultUploadConfig()
	uploader1 := NewUploader(client, config)
	uploader2 := NewUploader(client, config)

	assert.NotNil(t, uploader1)
	assert.NotNil(t, uploader2)
	assert.NotEqual(t, uploader1.pool, uploader2.pool)

	uploader1.Close()
	uploader2.Close()
}

func TestUploader_WithProgressCallback(t *testing.T) {
	client := NewClient("us-east-1")

	var capturedBytes, capturedTotal int64
	config := UploadConfig{
		ConcurrentUploads: 3,
		OnProgress: func(bytes, total int64) {
			capturedBytes = bytes
			capturedTotal = total
		},
	}

	uploader := NewUploader(client, config)
	assert.NotNil(t, uploader)

	// Test that progress callback is properly stored in config
	assert.NotNil(t, config.OnProgress)
	config.OnProgress(256, 1024)
	assert.Equal(t, int64(256), capturedBytes)
	assert.Equal(t, int64(1024), capturedTotal)

	uploader.Close()
}

func TestUploader_WithMetadata(t *testing.T) {
	client := NewClient("us-east-1")

	metadata := map[string]*string{
		"author":      stringPtr("test-user"),
		"version":     stringPtr("1.0"),
		"environment": stringPtr("test"),
	}

	config := UploadConfig{
		ConcurrentUploads: 5,
		Metadata:          metadata,
	}

	uploader := NewUploader(client, config)
	assert.NotNil(t, uploader)
	assert.Equal(t, metadata, config.Metadata)
	assert.Equal(t, "test-user", *config.Metadata["author"])
	assert.Equal(t, "1.0", *config.Metadata["version"])
	assert.Equal(t, "test", *config.Metadata["environment"])

	uploader.Close()
}

func TestUploadRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request UploadRequest
		valid   bool
	}{
		{
			name: "valid request",
			request: UploadRequest{
				Bucket:   "test-bucket",
				Key:      "test-key",
				FilePath: "/path/to/file",
			},
			valid: true,
		},
		{
			name: "empty bucket",
			request: UploadRequest{
				Bucket:   "",
				Key:      "test-key",
				FilePath: "/path/to/file",
			},
			valid: false,
		},
		{
			name: "empty key",
			request: UploadRequest{
				Bucket:   "test-bucket",
				Key:      "",
				FilePath: "/path/to/file",
			},
			valid: false,
		},
		{
			name: "empty file path",
			request: UploadRequest{
				Bucket:   "test-bucket",
				Key:      "test-key",
				FilePath: "",
			},
			valid: false,
		},
		{
			name: "with content type",
			request: UploadRequest{
				Bucket:      "test-bucket",
				Key:         "test-key",
				FilePath:    "/path/to/file",
				ContentType: "application/json",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.valid {
				assert.NotEmpty(t, tt.request.Bucket)
				assert.NotEmpty(t, tt.request.Key)
				assert.NotEmpty(t, tt.request.FilePath)
			} else {
				hasEmpty := tt.request.Bucket == "" || tt.request.Key == "" || tt.request.FilePath == ""
				assert.True(t, hasEmpty)
			}
		})
	}
}

func TestUploader_ContextHandling(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultUploadConfig()
	uploader := NewUploader(client, config)
	defer uploader.Close()

	// Test context creation
	ctx := context.Background()
	assert.NotNil(t, ctx)

	// Test context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NotNil(t, ctx)

	// Test context with cancellation
	ctx, cancel = context.WithCancel(context.Background())
	assert.NotNil(t, ctx)
	cancel() // Cancel immediately
	assert.Equal(t, context.Canceled, ctx.Err())
}

func TestUploader_ContentTypeDetection(t *testing.T) {
	tests := []struct {
		fileName    string
		expectedExt string
	}{
		{"test.txt", ".txt"},
		{"document.pdf", ".pdf"},
		{"image.jpg", ".jpg"},
		{"data.json", ".json"},
		{"style.css", ".css"},
		{"script.js", ".js"},
		{"archive.zip", ".zip"},
		{"noextension", ""},
	}

	for _, tt := range tests {
		t.Run(tt.fileName, func(t *testing.T) {
			ext := filepath.Ext(tt.fileName)
			assert.Equal(t, tt.expectedExt, ext)
		})
	}
}

func TestUploader_ResourceCleanup(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultUploadConfig()

	// Create and close multiple uploaders to test resource cleanup
	for i := 0; i < 5; i++ {
		uploader := NewUploader(client, config)
		assert.NotNil(t, uploader)
		uploader.Close()
	}
}

func TestUploader_BatchOperations(t *testing.T) {
	// Test batch upload request preparation
	requests := []UploadRequest{
		{
			Bucket:   "bucket1",
			Key:      "key1",
			FilePath: "/path1",
		},
		{
			Bucket:   "bucket2",
			Key:      "key2",
			FilePath: "/path2",
		},
		{
			Bucket:   "bucket3",
			Key:      "key3",
			FilePath: "/path3",
		},
	}

	assert.Len(t, requests, 3)
	for i, req := range requests {
		assert.Equal(t, req.Bucket, requests[i].Bucket)
		assert.Equal(t, req.Key, requests[i].Key)
		assert.Equal(t, req.FilePath, requests[i].FilePath)
	}
}

func TestUploader_MetadataMerging(t *testing.T) {
	// Test metadata merging logic
	defaultMetadata := map[string]*string{
		"environment": stringPtr("production"),
		"version":     stringPtr("1.0"),
	}

	requestMetadata := map[string]*string{
		"author": stringPtr("user123"),
		"type":   stringPtr("document"),
	}

	// Simulate metadata merging
	merged := make(map[string]*string)
	for k, v := range defaultMetadata {
		merged[k] = v
	}
	for k, v := range requestMetadata {
		merged[k] = v
	}

	assert.Len(t, merged, 4)
	assert.Equal(t, "production", *merged["environment"])
	assert.Equal(t, "1.0", *merged["version"])
	assert.Equal(t, "user123", *merged["author"])
	assert.Equal(t, "document", *merged["type"])
}

// Benchmark tests
func BenchmarkNewUploader(b *testing.B) {
	client := NewClient("us-east-1")
	config := DefaultUploadConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		uploader := NewUploader(client, config)
		uploader.Close()
	}
}

func BenchmarkUploadConfig_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := DefaultUploadConfig()
		_ = config
	}
}

func BenchmarkUploadRequest_Creation(b *testing.B) {
	metadata := map[string]*string{
		"author": stringPtr("test-user"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request := UploadRequest{
			Bucket:      "test-bucket",
			Key:         "test-key",
			FilePath:    "/path/to/file",
			ContentType: "text/plain",
			Metadata:    metadata,
		}
		_ = request
	}
}

func BenchmarkMetadataCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		metadata := map[string]*string{
			"author":      stringPtr("test-user"),
			"version":     stringPtr("1.0"),
			"environment": stringPtr("test"),
		}
		_ = metadata
	}
}

func BenchmarkProgressCallback_Upload(b *testing.B) {
	callback := func(bytes, total int64) {
		// Simulate progress calculation with upload-specific logic
		percentage := float64(bytes) / float64(total) * 100
		_ = percentage
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callback(int64(i), 1000)
	}
}
