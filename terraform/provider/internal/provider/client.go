package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// S3ryClient wraps the s3ry CLI for Terraform operations
type S3ryClient struct {
	config *S3ryConfig
}

// S3Object represents an S3 object
type S3Object struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
	StorageClass string    `json:"storage_class"`
}

// S3Bucket represents an S3 bucket
type S3Bucket struct {
	Name         string    `json:"name"`
	Region       string    `json:"region"`
	CreationDate time.Time `json:"creation_date"`
	Objects      int64     `json:"objects"`
	Size         int64     `json:"size"`
}

// PerformanceMetrics represents S3ry performance data
type PerformanceMetrics struct {
	Throughput          float64 `json:"throughput_mbps"`
	OperationsPerSecond int64   `json:"operations_per_second"`
	ActiveWorkers       int     `json:"active_workers"`
	MemoryUsage         int64   `json:"memory_usage_bytes"`
	CPUUsage            float64 `json:"cpu_usage_percent"`
	UploadSpeed         float64 `json:"upload_speed_mbps"`
	DownloadSpeed       float64 `json:"download_speed_mbps"`
}

// UploadResult represents the result of an upload operation
type UploadResult struct {
	Success     bool    `json:"success"`
	Key         string  `json:"key"`
	Size        int64   `json:"size"`
	ETag        string  `json:"etag"`
	Throughput  float64 `json:"throughput_mbps"`
	Duration    int64   `json:"duration_ms"`
	WorkersUsed int     `json:"workers_used"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	Success       bool    `json:"success"`
	FilesUploaded int64   `json:"files_uploaded"`
	FilesSkipped  int64   `json:"files_skipped"`
	BytesUploaded int64   `json:"bytes_uploaded"`
	Duration      int64   `json:"duration_ms"`
	Throughput    float64 `json:"throughput_mbps"`
}

// NewS3ryClient creates a new S3ry client with the given configuration
func NewS3ryClient(config *S3ryConfig) (*S3ryClient, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	return &S3ryClient{
		config: config,
	}, nil
}

// TestConnection verifies that the s3ry binary is available and working
func (c *S3ryClient) TestConnection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.config.S3ryPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to execute s3ry: %w", err)
	}

	if !strings.Contains(string(output), "s3ry") {
		return fmt.Errorf("unexpected output from s3ry --version: %s", string(output))
	}

	return nil
}

// ListBuckets retrieves all accessible S3 buckets
func (c *S3ryClient) ListBuckets(ctx context.Context) ([]S3Bucket, error) {
	args := c.buildBaseArgs()
	args = append(args, "list", "--format", "json")

	output, err := c.executeCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	var buckets []S3Bucket
	if err := json.Unmarshal(output, &buckets); err != nil {
		return nil, fmt.Errorf("failed to parse bucket list: %w", err)
	}

	return buckets, nil
}

// ListObjects retrieves objects in a specific bucket
func (c *S3ryClient) ListObjects(ctx context.Context, bucket, prefix string, maxKeys int) ([]S3Object, error) {
	args := c.buildBaseArgs()
	args = append(args, "list", bucket, "--format", "json")

	if prefix != "" {
		args = append(args, "--prefix", prefix)
	}

	if maxKeys > 0 {
		args = append(args, "--max-keys", strconv.Itoa(maxKeys))
	}

	output, err := c.executeCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects in bucket %s: %w", bucket, err)
	}

	var objects []S3Object
	if err := json.Unmarshal(output, &objects); err != nil {
		return nil, fmt.Errorf("failed to parse object list: %w", err)
	}

	return objects, nil
}

// GetBucketInfo retrieves detailed information about a specific bucket
func (c *S3ryClient) GetBucketInfo(ctx context.Context, bucket string) (*S3Bucket, error) {
	args := c.buildBaseArgs()
	args = append(args, "info", bucket, "--format", "json")

	output, err := c.executeCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get bucket info for %s: %w", bucket, err)
	}

	var bucketInfo S3Bucket
	if err := json.Unmarshal(output, &bucketInfo); err != nil {
		return nil, fmt.Errorf("failed to parse bucket info: %w", err)
	}

	return &bucketInfo, nil
}

// UploadFile uploads a file to S3
func (c *S3ryClient) UploadFile(ctx context.Context, localPath, bucket, key string) (*UploadResult, error) {
	args := c.buildBaseArgs()
	args = append(args, "upload", localPath, fmt.Sprintf("s3://%s/%s", bucket, key), "--format", "json")

	output, err := c.executeCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to upload %s to s3://%s/%s: %w", localPath, bucket, key, err)
	}

	var result UploadResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse upload result: %w", err)
	}

	return &result, nil
}

// SyncDirectory synchronizes a local directory with an S3 prefix
func (c *S3ryClient) SyncDirectory(ctx context.Context, localPath, bucket, prefix string, deleteExisting bool) (*SyncResult, error) {
	args := c.buildBaseArgs()
	s3Path := fmt.Sprintf("s3://%s", bucket)
	if prefix != "" {
		s3Path = fmt.Sprintf("s3://%s/%s", bucket, prefix)
	}

	args = append(args, "sync", localPath, s3Path, "--format", "json")

	if deleteExisting {
		args = append(args, "--delete")
	}

	output, err := c.executeCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to sync %s to %s: %w", localPath, s3Path, err)
	}

	var result SyncResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse sync result: %w", err)
	}

	return &result, nil
}

// GetPerformanceMetrics retrieves current performance metrics
func (c *S3ryClient) GetPerformanceMetrics(ctx context.Context) (*PerformanceMetrics, error) {
	args := c.buildBaseArgs()
	args = append(args, "metrics", "--format", "json")

	output, err := c.executeCommand(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get performance metrics: %w", err)
	}

	var metrics PerformanceMetrics
	if err := json.Unmarshal(output, &metrics); err != nil {
		return nil, fmt.Errorf("failed to parse performance metrics: %w", err)
	}

	return &metrics, nil
}

// DeleteObject deletes an object from S3
func (c *S3ryClient) DeleteObject(ctx context.Context, bucket, key string) error {
	args := c.buildBaseArgs()
	args = append(args, "delete", fmt.Sprintf("s3://%s/%s", bucket, key))

	_, err := c.executeCommand(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to delete s3://%s/%s: %w", bucket, key, err)
	}

	return nil
}

// SetBucketPolicy sets a bucket policy (if supported by s3ry)
func (c *S3ryClient) SetBucketPolicy(ctx context.Context, bucket, policy string) error {
	args := c.buildBaseArgs()
	args = append(args, "policy", "set", bucket, policy)

	_, err := c.executeCommand(ctx, args...)
	if err != nil {
		return fmt.Errorf("failed to set bucket policy for %s: %w", bucket, err)
	}

	return nil
}

// GetBucketPolicy retrieves a bucket policy
func (c *S3ryClient) GetBucketPolicy(ctx context.Context, bucket string) (string, error) {
	args := c.buildBaseArgs()
	args = append(args, "policy", "get", bucket)

	output, err := c.executeCommand(ctx, args...)
	if err != nil {
		return "", fmt.Errorf("failed to get bucket policy for %s: %w", bucket, err)
	}

	return string(output), nil
}

// buildBaseArgs constructs the base arguments for s3ry commands
func (c *S3ryClient) buildBaseArgs() []string {
	args := []string{}

	if c.config.AWSRegion != "" {
		args = append(args, "--region", c.config.AWSRegion)
	}

	if c.config.WorkerPoolSize > 0 {
		args = append(args, "--workers", strconv.Itoa(c.config.WorkerPoolSize))
	}

	if c.config.ChunkSize != "" {
		args = append(args, "--chunk-size", c.config.ChunkSize)
	}

	if c.config.Timeout > 0 {
		args = append(args, "--timeout", strconv.Itoa(c.config.Timeout))
	}

	if c.config.MaxRetries > 0 {
		args = append(args, "--retries", strconv.Itoa(c.config.MaxRetries))
	}

	switch c.config.PerformanceMode {
	case "high":
		args = append(args, "--performance", "high")
	case "maximum":
		args = append(args, "--performance", "maximum")
	}

	if c.config.EnableTelemetry {
		args = append(args, "--telemetry")
	}

	return args
}

// executeCommand executes an s3ry command with the given arguments
func (c *S3ryClient) executeCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.config.S3ryPath, args...)

	output, err := cmd.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("s3ry command failed with exit code %d: %s", 
				exitError.ExitCode(), string(exitError.Stderr))
		}
		return nil, err
	}

	return output, nil
}