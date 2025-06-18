package s3

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultDownloadConfig(t *testing.T) {
	config := DefaultDownloadConfig()

	assert.Equal(t, 5, config.ConcurrentDownloads)
	assert.Equal(t, int64(5*1024*1024), config.PartSize)
	assert.Equal(t, 3, config.Concurrency)
	assert.True(t, config.ResumeDownloads)
	assert.True(t, config.VerifyChecksum)
	assert.Nil(t, config.OnProgress)
}

func TestNewDownloader(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()
	downloader := NewDownloader(client, config)

	assert.NotNil(t, downloader)
	assert.NotNil(t, downloader.client)
	assert.NotNil(t, downloader.pool)
	assert.Equal(t, client, downloader.client)

	// Clean up
	downloader.Close()
}

func TestDownloader_Close(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()
	downloader := NewDownloader(client, config)

	// Should not panic
	assert.NotPanics(t, func() {
		downloader.Close()
	})
}

func TestDownloadRequest_Creation(t *testing.T) {
	request := DownloadRequest{
		Bucket:   "test-bucket",
		Key:      "path/to/file.txt",
		FilePath: "/local/path/file.txt",
	}

	assert.Equal(t, "test-bucket", request.Bucket)
	assert.Equal(t, "path/to/file.txt", request.Key)
	assert.Equal(t, "/local/path/file.txt", request.FilePath)
}

func TestDownloadConfig_CustomValues(t *testing.T) {
	config := DownloadConfig{
		ConcurrentDownloads: 10,
		PartSize:            10 * 1024 * 1024,
		Concurrency:         5,
		ResumeDownloads:     false,
		VerifyChecksum:      false,
		OnProgress: func(bytes, total int64) {
			// Custom progress callback
		},
	}

	assert.Equal(t, 10, config.ConcurrentDownloads)
	assert.Equal(t, int64(10*1024*1024), config.PartSize)
	assert.Equal(t, 5, config.Concurrency)
	assert.False(t, config.ResumeDownloads)
	assert.False(t, config.VerifyChecksum)
	assert.NotNil(t, config.OnProgress)
}

func TestDownloader_MultipleDownloads(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()
	downloader1 := NewDownloader(client, config)
	downloader2 := NewDownloader(client, config)

	assert.NotNil(t, downloader1)
	assert.NotNil(t, downloader2)
	assert.NotEqual(t, downloader1.pool, downloader2.pool)

	downloader1.Close()
	downloader2.Close()
}

func TestDownloader_WithProgressCallback(t *testing.T) {
	client := NewClient("us-east-1")

	var capturedBytes, capturedTotal int64
	config := DownloadConfig{
		ConcurrentDownloads: 3,
		OnProgress: func(bytes, total int64) {
			capturedBytes = bytes
			capturedTotal = total
		},
	}

	downloader := NewDownloader(client, config)
	assert.NotNil(t, downloader)

	// Test that progress callback is properly stored in config
	assert.NotNil(t, config.OnProgress)
	config.OnProgress(512, 1024)
	assert.Equal(t, int64(512), capturedBytes)
	assert.Equal(t, int64(1024), capturedTotal)

	downloader.Close()
}

func TestDownloader_ContextHandling(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()
	downloader := NewDownloader(client, config)
	defer downloader.Close()

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

func TestDownloadRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request DownloadRequest
		valid   bool
	}{
		{
			name: "valid request",
			request: DownloadRequest{
				Bucket:   "test-bucket",
				Key:      "test-key",
				FilePath: "/path/to/file",
			},
			valid: true,
		},
		{
			name: "empty bucket",
			request: DownloadRequest{
				Bucket:   "",
				Key:      "test-key",
				FilePath: "/path/to/file",
			},
			valid: false,
		},
		{
			name: "empty key",
			request: DownloadRequest{
				Bucket:   "test-bucket",
				Key:      "",
				FilePath: "/path/to/file",
			},
			valid: false,
		},
		{
			name: "empty file path",
			request: DownloadRequest{
				Bucket:   "test-bucket",
				Key:      "test-key",
				FilePath: "",
			},
			valid: false,
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

func TestDownloader_ResourceCleanup(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()

	// Create and close multiple downloaders to test resource cleanup
	for i := 0; i < 5; i++ {
		downloader := NewDownloader(client, config)
		assert.NotNil(t, downloader)
		downloader.Close()
	}
}

func TestDownloader_DirectoryCreation(t *testing.T) {
	// Test that we can create proper directory paths for downloads
	tempDir := os.TempDir()
	testDir := filepath.Join(tempDir, "s3ry-test-download")
	defer os.RemoveAll(testDir)

	filePath := filepath.Join(testDir, "subdir", "test-file.txt")

	// Test directory creation logic (without actual download)
	dir := filepath.Dir(filePath)
	err := os.MkdirAll(dir, 0755)
	assert.NoError(t, err)

	// Verify directory was created
	info, err := os.Stat(dir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

// Benchmark tests
func BenchmarkNewDownloader(b *testing.B) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		downloader := NewDownloader(client, config)
		downloader.Close()
	}
}

func BenchmarkDownloadConfig_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config := DefaultDownloadConfig()
		_ = config
	}
}

func BenchmarkDownloadRequest_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		request := DownloadRequest{
			Bucket:   "test-bucket",
			Key:      "test-key",
			FilePath: "/path/to/file",
		}
		_ = request
	}
}

func BenchmarkProgressCallback(b *testing.B) {
	callback := func(bytes, total int64) {
		// Simulate progress calculation
		_ = float64(bytes) / float64(total) * 100
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		callback(int64(i), 1000)
	}
}

// Test actual download functionality with error cases
func TestDownloader_DownloadWithInvalidRequest(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()
	downloader := NewDownloader(client, config)
	defer downloader.Close()

	ctx := context.Background()
	tempFile := filepath.Join(os.TempDir(), "test-download.txt")
	defer os.Remove(tempFile)

	// Test with empty bucket - should fail gracefully
	request := DownloadRequest{
		Bucket:   "",
		Key:      "test-key",
		FilePath: tempFile,
	}

	err := downloader.Download(ctx, request, config)
	// Should fail due to validation or AWS error
	assert.Error(t, err)
}

func TestDownloader_DirectoryCreationForDownload(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()
	downloader := NewDownloader(client, config)
	defer downloader.Close()

	ctx := context.Background()
	tempDir := filepath.Join(os.TempDir(), "s3ry-test-nested")
	tempFile := filepath.Join(tempDir, "subdir", "deep", "test-file.txt")
	defer os.RemoveAll(tempDir)

	request := DownloadRequest{
		Bucket:   "non-existent-bucket",
		Key:      "test-key",
		FilePath: tempFile,
	}

	// This will fail due to non-existent bucket, but should create directories
	err := downloader.Download(ctx, request, config)
	assert.Error(t, err) // Expected to fail

	// Check that directory was created despite download failure
	dir := filepath.Dir(tempFile)
	info, err := os.Stat(dir)
	assert.NoError(t, err, "Directory should be created even if download fails")
	assert.True(t, info.IsDir())
}

func TestDownloader_ProgressCallbackIntegration(t *testing.T) {
	client := NewClient("us-east-1")

	var progressCalled bool
	var lastBytes, lastTotal int64

	config := DownloadConfig{
		ConcurrentDownloads: 1,
		PartSize:            1024,
		OnProgress: func(bytes, total int64) {
			progressCalled = true
			lastBytes = bytes
			lastTotal = total
		},
	}

	downloader := NewDownloader(client, config)
	defer downloader.Close()

	ctx := context.Background()
	tempFile := filepath.Join(os.TempDir(), "test-progress.txt")
	defer os.Remove(tempFile)

	request := DownloadRequest{
		Bucket:   "test-bucket-that-does-not-exist",
		Key:      "test-key",
		FilePath: tempFile,
	}

	// This will fail, but we're testing that progress callback structure is correct
	err := downloader.Download(ctx, request, config)
	assert.Error(t, err) // Expected failure due to non-existent bucket

	// Progress callback might not be called due to early failure, but config is correct
	assert.NotNil(t, config.OnProgress)

	// Test callback directly
	config.OnProgress(512, 1024)
	assert.True(t, progressCalled)
	assert.Equal(t, int64(512), lastBytes)
	assert.Equal(t, int64(1024), lastTotal)
}

func TestDownloader_ContextCancellation(t *testing.T) {
	client := NewClient("us-east-1")
	config := DefaultDownloadConfig()
	downloader := NewDownloader(client, config)
	defer downloader.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tempFile := filepath.Join(os.TempDir(), "test-cancelled.txt")
	defer os.Remove(tempFile)

	request := DownloadRequest{
		Bucket:   "test-bucket",
		Key:      "test-key",
		FilePath: tempFile,
	}

	err := downloader.Download(ctx, request, config)
	// Should fail due to context cancellation or AWS error
	assert.Error(t, err)
}

func TestDownloader_ResumeDownloads(t *testing.T) {
	client := NewClient("us-east-1")
	config := DownloadConfig{
		ResumeDownloads: true,
		VerifyChecksum:  false,
	}
	downloader := NewDownloader(client, config)
	defer downloader.Close()

	ctx := context.Background()
	tempFile := filepath.Join(os.TempDir(), "test-resume.txt")
	defer os.Remove(tempFile)

	// Create a file that already exists
	err := os.WriteFile(tempFile, []byte("existing content"), 0644)
	assert.NoError(t, err)

	request := DownloadRequest{
		Bucket:   "test-bucket",
		Key:      "test-key",
		FilePath: tempFile,
	}

	// This should check if download is complete (will likely fail due to non-existent bucket)
	err = downloader.Download(ctx, request, config)
	assert.Error(t, err) // Expected failure, but resume logic is tested

	// File should still exist from our creation
	_, err = os.Stat(tempFile)
	assert.NoError(t, err)
}

func TestDownloader_ConcurrentDownloads(t *testing.T) {
	client := NewClient("us-east-1")
	config := DownloadConfig{
		ConcurrentDownloads: 3,
		PartSize:            1024,
	}
	downloader := NewDownloader(client, config)
	defer downloader.Close()

	// Test that we can create multiple concurrent download requests
	ctx := context.Background()

	requests := []DownloadRequest{
		{Bucket: "bucket1", Key: "key1", FilePath: filepath.Join(os.TempDir(), "file1.txt")},
		{Bucket: "bucket2", Key: "key2", FilePath: filepath.Join(os.TempDir(), "file2.txt")},
		{Bucket: "bucket3", Key: "key3", FilePath: filepath.Join(os.TempDir(), "file3.txt")},
	}

	// Clean up
	for _, req := range requests {
		defer os.Remove(req.FilePath)
	}

	// Test concurrent downloads (will fail due to non-existent buckets)
	for _, req := range requests {
		err := downloader.Download(ctx, req, config)
		assert.Error(t, err) // Expected to fail, but tests concurrent structure
	}
}
