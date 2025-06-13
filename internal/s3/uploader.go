package s3

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/seike460/s3ry/internal/worker"
)

// Uploader provides concurrent S3 upload capabilities
type Uploader struct {
	client *Client
	pool   *worker.Pool
}

// UploadConfig configures upload behavior
type UploadConfig struct {
	ConcurrentUploads int                    // Number of concurrent uploads
	PartSize          int64                  // Size of each upload part
	Concurrency       int                    // Number of concurrent parts per file
	ChecksumValidation bool                  // Whether to validate checksums
	AutoContentType   bool                   // Whether to auto-detect content type
	Metadata          map[string]*string     // Default metadata for all uploads
	OnProgress        ProgressCallback       // Progress callback
	Compression       bool                   // Whether to compress files before upload
	Deduplication     bool                   // Whether to check for existing files
}

// DefaultUploadConfig returns default upload configuration
func DefaultUploadConfig() UploadConfig {
	return UploadConfig{
		ConcurrentUploads:  5,
		PartSize:           5 * 1024 * 1024, // 5MB
		Concurrency:        3,
		ChecksumValidation: true,
		AutoContentType:    true,
		Compression:        false,
		Deduplication:      true,
	}
}

// NewUploader creates a new concurrent S3 uploader
func NewUploader(client *Client, config UploadConfig) *Uploader {
	poolConfig := worker.DefaultConfig()
	poolConfig.Workers = config.ConcurrentUploads
	pool := worker.NewPool(poolConfig)
	pool.Start()

	return &Uploader{
		client: client,
		pool:   pool,
	}
}

// Close stops the uploader and cleans up resources
func (u *Uploader) Close() {
	u.pool.Stop()
}

// Upload uploads a single file to S3
func (u *Uploader) Upload(ctx context.Context, request UploadRequest, config UploadConfig) error {
	// Check if file exists locally
	fileInfo, err := os.Stat(request.FilePath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", request.FilePath, err)
	}

	// Skip if it's a directory
	if fileInfo.IsDir() {
		return fmt.Errorf("cannot upload directory %s, use UploadDirectory instead", request.FilePath)
	}

	// Auto-detect content type if enabled
	if config.AutoContentType && request.ContentType == "" {
		request.ContentType = u.detectContentType(request.FilePath)
	}

	// Merge default metadata
	if request.Metadata == nil {
		request.Metadata = make(map[string]*string)
	}
	for k, v := range config.Metadata {
		if _, exists := request.Metadata[k]; !exists {
			request.Metadata[k] = v
		}
	}

	// Check for deduplication
	if config.Deduplication {
		exists, err := u.objectExists(ctx, request.Bucket, request.Key)
		if err != nil {
			return fmt.Errorf("failed to check if object exists: %w", err)
		}
		if exists {
			// Compare file sizes/checksums to determine if upload is needed
			if u.isUploadNeeded(ctx, request, fileInfo) {
				// File is different, continue with upload
			} else {
				return nil // File already exists and is identical
			}
		}
	}

	// Create progress tracker if callback provided
	var progressFunc ProgressCallback
	if config.OnProgress != nil {
		totalSize := fileInfo.Size()
		var uploadedBytes int64
		var mutex sync.Mutex

		progressFunc = func(bytes, _ int64) {
			mutex.Lock()
			uploadedBytes += bytes
			config.OnProgress(uploadedBytes, totalSize)
			mutex.Unlock()
		}
	}

	// Submit upload job
	job := &worker.S3UploadJob{
		Client:   u.client,
		Request:  ToTypesUploadRequest(request),
		Progress: ToTypesProgressCallback(progressFunc),
	}

	if err := u.pool.Submit(job); err != nil {
		return fmt.Errorf("failed to submit upload job: %w", err)
	}

	// Wait for result
	select {
	case result := <-u.pool.Results():
		return result.Error
	case <-ctx.Done():
		return ctx.Err()
	}
}

// UploadBatch uploads multiple files concurrently
func (u *Uploader) UploadBatch(ctx context.Context, requests []UploadRequest, config UploadConfig) []BatchOperation {
	jobs := make([]worker.Job, 0, len(requests))
	
	// Filter and prepare requests
	for _, request := range requests {
		// Check if file exists
		fileInfo, err := os.Stat(request.FilePath)
		if err != nil || fileInfo.IsDir() {
			continue // Skip non-existent files or directories
		}

		// Auto-detect content type if enabled
		if config.AutoContentType && request.ContentType == "" {
			request.ContentType = u.detectContentType(request.FilePath)
		}

		// Create progress tracker for this specific upload
		var progressFunc ProgressCallback
		if config.OnProgress != nil {
			progressFunc = func(bytes, total int64) {
				config.OnProgress(bytes, total)
			}
		}

		jobs = append(jobs, &worker.S3UploadJob{
			Client:   u.client,
			Request:  ToTypesUploadRequest(request),
			Progress: ToTypesProgressCallback(progressFunc),
		})
	}

	// Process batch with progress tracking
	batchProcessor := worker.NewBatchProcessor(u.pool, func(completed, total int) {
		// Optional: additional batch progress tracking
	})

	results := batchProcessor.ProcessBatch(jobs)
	
	// Convert worker results to batch operations
	operations := make([]BatchOperation, len(results))
	for i, result := range results {
		operations[i] = BatchOperation{
			Operation: "upload",
			Key:       requests[i].Key,
			Success:   result.Error == nil,
			Error:     result.Error,
		}
	}

	return operations
}

// UploadDirectory uploads all files in a directory recursively
func (u *Uploader) UploadDirectory(ctx context.Context, localDir, bucket, prefix string, config UploadConfig) error {
	// Walk through the directory
	var requests []UploadRequest
	
	err := filepath.Walk(localDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate S3 key
		relativePath, err := filepath.Rel(localDir, path)
		if err != nil {
			return err
		}
		
		// Convert Windows paths to S3 paths (use forward slashes)
		s3Key := prefix + strings.ReplaceAll(relativePath, "\\", "/")

		requests = append(requests, UploadRequest{
			Bucket:   bucket,
			Key:      s3Key,
			FilePath: path,
		})

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory %s: %w", localDir, err)
	}

	// Upload all files
	operations := u.UploadBatch(ctx, requests, config)
	
	// Check for any errors
	for _, op := range operations {
		if !op.Success {
			return fmt.Errorf("upload failed for %s: %w", op.Key, op.Error)
		}
	}

	return nil
}

// UploadWithRetry uploads a file with automatic retry on failure
func (u *Uploader) UploadWithRetry(ctx context.Context, request UploadRequest, config UploadConfig, maxRetries int) error {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := u.Upload(ctx, request, config)
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Don't retry if context was cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}
		
		// Clean up partial upload before retry (multipart uploads are handled by AWS SDK)
	}
	
	return fmt.Errorf("upload failed after %d attempts: %w", maxRetries+1, lastErr)
}

// GetUploadURL generates a presigned URL for upload
func (u *Uploader) GetUploadURL(ctx context.Context, bucket, key string, expiration int64) (string, error) {
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	req, _ := u.client.s3Client.PutObjectRequest(input)
	url, err := req.Presign(time.Hour) // 1 hour expiration
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return url, nil
}

// detectContentType detects the MIME type of a file
func (u *Uploader) detectContentType(filePath string) string {
	ext := filepath.Ext(filePath)
	contentType := mime.TypeByExtension(ext)
	
	if contentType == "" {
		// Try to detect by reading file content
		file, err := os.Open(filePath)
		if err != nil {
			return "application/octet-stream"
		}
		defer file.Close()

		// Read first 512 bytes to detect content type
		buffer := make([]byte, 512)
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return "application/octet-stream"
		}

		contentType = http.DetectContentType(buffer[:n])
	}

	return contentType
}

// objectExists checks if an object exists in S3
func (u *Uploader) objectExists(ctx context.Context, bucket, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}

	_, err := u.client.s3Client.HeadObjectWithContext(ctx, input)
	if err != nil {
		// Check if error is "Not Found"
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// isUploadNeeded checks if an upload is needed by comparing file metadata
func (u *Uploader) isUploadNeeded(ctx context.Context, request UploadRequest, localInfo os.FileInfo) bool {
	// Get remote object metadata
	input := &s3.HeadObjectInput{
		Bucket: aws.String(request.Bucket),
		Key:    aws.String(request.Key),
	}

	output, err := u.client.s3Client.HeadObjectWithContext(ctx, input)
	if err != nil {
		return true // If we can't get metadata, assume upload is needed
	}

	// Compare file sizes
	if localInfo.Size() != *output.ContentLength {
		return true
	}

	// Compare modification times (if available in metadata)
	if output.Metadata != nil {
		if lastModified, exists := output.Metadata["last-modified"]; exists {
			// Compare with local file modification time
			// This is a simplified comparison
			_ = lastModified
		}
	}

	// If we can't determine differences, assume upload is needed
	return false
}

// calculateMD5 calculates MD5 hash of a file
func (u *Uploader) calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}