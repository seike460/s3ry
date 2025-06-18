package s3

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/seike460/s3ry/internal/worker"
)

// Downloader provides concurrent S3 download capabilities
type Downloader struct {
	client *Client
	pool   *worker.Pool
}

// DownloadConfig configures download behavior
type DownloadConfig struct {
	ConcurrentDownloads int              // Number of concurrent downloads
	PartSize            int64            // Size of each download part
	Concurrency         int              // Number of concurrent parts per file
	ResumeDownloads     bool             // Whether to resume partial downloads
	VerifyChecksum      bool             // Whether to verify checksums
	OnProgress          ProgressCallback // Progress callback
}

// DefaultDownloadConfig returns default download configuration
func DefaultDownloadConfig() DownloadConfig {
	return DownloadConfig{
		ConcurrentDownloads: 5,
		PartSize:            5 * 1024 * 1024, // 5MB
		Concurrency:         3,
		ResumeDownloads:     true,
		VerifyChecksum:      true,
	}
}

// NewDownloader creates a new concurrent S3 downloader
func NewDownloader(client *Client, config DownloadConfig) *Downloader {
	poolConfig := worker.DefaultConfig()
	poolConfig.Workers = config.ConcurrentDownloads
	pool := worker.NewPool(poolConfig)
	pool.Start()

	return &Downloader{
		client: client,
		pool:   pool,
	}
}

// Close stops the downloader and cleans up resources
func (d *Downloader) Close() {
	d.pool.Stop()
}

// Download downloads a single file from S3
func (d *Downloader) Download(ctx context.Context, request DownloadRequest, config DownloadConfig) error {
	// Check if file already exists
	if config.ResumeDownloads {
		if _, err := os.Stat(request.FilePath); err == nil {
			// File exists, check if it's complete
			if d.isDownloadComplete(ctx, request) {
				return nil // Already downloaded
			}
		}
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(request.FilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create progress tracker if callback provided
	var progressFunc ProgressCallback
	if config.OnProgress != nil {
		// Get object size first
		headInput := &s3.HeadObjectInput{
			Bucket: aws.String(request.Bucket),
			Key:    aws.String(request.Key),
		}

		headOutput, err := d.client.s3Client.HeadObjectWithContext(ctx, headInput)
		if err != nil {
			return fmt.Errorf("failed to get object metadata: %w", err)
		}

		totalSize := *headOutput.ContentLength
		var downloadedBytes int64
		var mutex sync.Mutex

		progressFunc = func(bytes, _ int64) {
			mutex.Lock()
			downloadedBytes += bytes
			config.OnProgress(downloadedBytes, totalSize)
			mutex.Unlock()
		}
	}

	// Submit download job
	job := &worker.S3DownloadJob{
		Client:   d.client,
		Request:  ToTypesDownloadRequest(request),
		Progress: ToTypesProgressCallback(progressFunc),
	}

	if err := d.pool.Submit(job); err != nil {
		return fmt.Errorf("failed to submit download job: %w", err)
	}

	// Wait for result with timeout safety
	select {
	case result := <-d.pool.Results():
		return result.Error
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(30 * time.Second):
		return fmt.Errorf("download timed out after 30 seconds - worker pool may be deadlocked")
	}
}

// DownloadBatch downloads multiple files concurrently
func (d *Downloader) DownloadBatch(ctx context.Context, requests []DownloadRequest, config DownloadConfig) []BatchOperation {
	jobs := make([]worker.Job, len(requests))

	for i, request := range requests {
		// Create progress tracker for this specific download
		var progressFunc ProgressCallback
		if config.OnProgress != nil {
			progressFunc = func(bytes, total int64) {
				config.OnProgress(bytes, total)
			}
		}

		jobs[i] = &worker.S3DownloadJob{
			Client:   d.client,
			Request:  ToTypesDownloadRequest(request),
			Progress: ToTypesProgressCallback(progressFunc),
		}
	}

	// Process batch with progress tracking
	batchProcessor := worker.NewBatchProcessor(d.pool, func(completed, total int) {
		// Optional: additional batch progress tracking
	})

	results := batchProcessor.ProcessBatch(jobs)

	// Convert worker results to batch operations
	operations := make([]BatchOperation, len(results))
	for i, result := range results {
		operations[i] = BatchOperation{
			Operation: "download",
			Key:       requests[i].Key,
			Success:   result.Error == nil,
			Error:     result.Error,
		}
	}

	return operations
}

// DownloadDirectory downloads all objects with a given prefix (simulating directory download)
func (d *Downloader) DownloadDirectory(ctx context.Context, bucket, prefix, localDir string, config DownloadConfig) error {
	// List all objects with the prefix
	lister := NewLister(d.client)
	defer lister.Close()

	objects, err := lister.ListObjects(ctx, ListRequest{
		Bucket: bucket,
		Prefix: prefix,
	})
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	// Create download requests
	requests := make([]DownloadRequest, 0, len(objects))
	for _, obj := range objects {
		// Skip directory markers
		if obj.Key[len(obj.Key)-1] == '/' {
			continue
		}

		// Calculate local file path
		relativePath := obj.Key
		if prefix != "" && len(obj.Key) > len(prefix) {
			relativePath = obj.Key[len(prefix):]
		}

		localPath := filepath.Join(localDir, relativePath)

		requests = append(requests, DownloadRequest{
			Bucket:   bucket,
			Key:      obj.Key,
			FilePath: localPath,
		})
	}

	// Download all files
	operations := d.DownloadBatch(ctx, requests, config)

	// Check for any errors
	for _, op := range operations {
		if !op.Success {
			return fmt.Errorf("download failed for %s: %w", op.Key, op.Error)
		}
	}

	return nil
}

// DownloadWithRetry downloads a file with automatic retry on failure
func (d *Downloader) DownloadWithRetry(ctx context.Context, request DownloadRequest, config DownloadConfig, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := d.Download(ctx, request, config)
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Don't retry if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Clean up partial file before retry
		os.Remove(request.FilePath)
	}

	return fmt.Errorf("download failed after %d attempts: %w", maxRetries+1, lastErr)
}

// GetDownloadURL generates a presigned URL for download (useful for large files or sharing)
func (d *Downloader) GetDownloadURL(ctx context.Context, bucket, key string, expiration int64) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	req, _ := d.client.s3Client.GetObjectRequest(input)
	url, err := req.Presign(time.Hour) // 1 hour expiration
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url, nil
}

// isDownloadComplete checks if a file has been completely downloaded
func (d *Downloader) isDownloadComplete(ctx context.Context, request DownloadRequest) bool {
	// Get local file size
	localInfo, err := os.Stat(request.FilePath)
	if err != nil {
		return false
	}

	// Get remote file size
	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	}

	headOutput, err := d.client.s3Client.HeadObjectWithContext(ctx, headInput)
	if err != nil {
		return false
	}

	// Compare sizes
	return localInfo.Size() == *headOutput.ContentLength
}
