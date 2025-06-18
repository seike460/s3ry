package worker

import (
	"context"
	"fmt"
	"os"

	"github.com/seike460/s3ry/internal/errors"
	"github.com/seike460/s3ry/internal/metrics"
	"github.com/seike460/s3ry/pkg/interfaces"
	"github.com/seike460/s3ry/pkg/types"
)

// S3DownloadJob represents a S3 download job
type S3DownloadJob struct {
	Client   interfaces.S3Client
	Request  types.DownloadRequest
	Progress types.ProgressCallback
}

// Execute implements the Job interface for S3DownloadJob
func (j *S3DownloadJob) Execute(ctx context.Context) error {
	// Get metrics instance
	m := metrics.GetGlobalMetrics()
	timer := m.StartTimer("s3_download")
	defer timer.Stop()

	// For MVP: Use basic download without head request
	// Real file size will be determined during download
	fileSize := int64(0)

	// Use interface download method for MVP
	err := j.Client.DownloadFile(ctx, j.Request.Bucket, j.Request.Key, j.Request.FilePath)
	if err != nil {
		return j.wrapError(err, "download_file", map[string]interface{}{
			"bucket":    j.Request.Bucket,
			"key":       j.Request.Key,
			"file_path": j.Request.FilePath,
		})
	}

	// Get file size for progress and metrics
	fileInfo, err := os.Stat(j.Request.FilePath)
	if err == nil && fileInfo != nil {
		fileSize = fileInfo.Size()
		// Call progress callback if provided
		if j.Progress != nil {
			j.Progress(fileSize, fileSize)
		}
		// Update metrics on successful download
		m.IncrementS3Downloads()
		m.AddBytesTransferred(fileSize)
	}

	return nil
}

// S3UploadJob represents a S3 upload job
type S3UploadJob struct {
	Client   interfaces.S3Client
	Request  types.UploadRequest
	Progress types.ProgressCallback
}

// Execute implements the Job interface for S3UploadJob
func (j *S3UploadJob) Execute(ctx context.Context) error {
	// Get metrics instance
	m := metrics.GetGlobalMetrics()
	timer := m.StartTimer("s3_upload")
	defer timer.Stop()

	// Get file info for size and progress
	fileInfo, err := os.Stat(j.Request.FilePath)
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", j.Request.FilePath, err)
	}

	// Upload using interface method for MVP
	err = j.Client.UploadFile(ctx, j.Request.FilePath, j.Request.Bucket, j.Request.Key)
	if err != nil {
		return fmt.Errorf("failed to upload %s: %w", j.Request.FilePath, err)
	}

	// Update metrics on successful upload
	m.IncrementS3Uploads()
	m.AddBytesTransferred(fileInfo.Size())

	// Report completion
	if j.Progress != nil {
		j.Progress(fileInfo.Size(), fileInfo.Size())
	}

	return nil
}

// S3DeleteJob represents a S3 delete job
type S3DeleteJob struct {
	Client interfaces.S3Client
	Bucket string
	Key    string
}

// Execute implements the Job interface for S3DeleteJob
func (j *S3DeleteJob) Execute(ctx context.Context) error {
	// Get metrics instance
	m := metrics.GetGlobalMetrics()
	timer := m.StartTimer("s3_delete")
	defer timer.Stop()

	err := j.Client.DeleteObject(ctx, j.Bucket, j.Key)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", j.Key, err)
	}

	// Update metrics on successful delete
	m.IncrementS3Deletes()

	return nil
}

// S3ListJob represents a S3 list job
type S3ListJob struct {
	Client  interfaces.S3Client
	Request types.ListRequest
	Results chan<- []types.Object
}

// Execute implements the Job interface for S3ListJob
func (j *S3ListJob) Execute(ctx context.Context) error {
	// Get metrics instance
	m := metrics.GetGlobalMetrics()
	timer := m.StartTimer("s3_list")
	defer timer.Stop()

	var objects []types.Object

	// Use S3Client interface for listing
	objectList, err := j.Client.ListObjects(ctx, j.Request.Bucket, j.Request.Prefix, "")

	if err != nil {
		return fmt.Errorf("failed to list objects in %s: %w", j.Request.Bucket, err)
	}

	// Convert interface objects to types.Object
	for _, obj := range objectList {
		objects = append(objects, types.Object{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
		})
	}

	// Update metrics on successful list operation
	m.IncrementS3Lists()

	// Send results to the channel
	select {
	case j.Results <- objects:
	case <-ctx.Done():
		return ctx.Err()
	}

	return nil
}

// wrapError wraps errors with S3ryError for standardized error handling
func (j *S3DownloadJob) wrapError(err error, operation string, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	// Check if already wrapped
	if _, ok := err.(*errors.S3ryError); ok {
		return err
	}

	// Determine error code based on operation
	var errorCode errors.ErrorCode
	switch operation {
	case "head_object", "get_object":
		errorCode = errors.ErrCodeS3Connection
	case "create_file", "copy_data":
		errorCode = errors.ErrCodeFileSystem
	default:
		errorCode = errors.ErrCodeUnknown
	}

	// Add job-specific context
	if context == nil {
		context = make(map[string]interface{})
	}
	context["job_type"] = "s3_download"
	context["operation"] = operation

	return errors.Wrap(err, errorCode, "s3_download_job", err.Error()).WithContext("job_context", context)
}

// wrapError wraps errors with S3ryError for standardized error handling
func (j *S3UploadJob) wrapError(err error, operation string, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	// Check if already wrapped
	if _, ok := err.(*errors.S3ryError); ok {
		return err
	}

	// Determine error code based on operation
	var errorCode errors.ErrorCode
	switch operation {
	case "put_object":
		errorCode = errors.ErrCodeS3Connection
	case "open_file", "stat_file":
		errorCode = errors.ErrCodeFileSystem
	default:
		errorCode = errors.ErrCodeUnknown
	}

	// Add job-specific context
	if context == nil {
		context = make(map[string]interface{})
	}
	context["job_type"] = "s3_upload"
	context["operation"] = operation

	return errors.Wrap(err, errorCode, "s3_upload_job", err.Error()).WithContext("job_context", context)
}

// wrapError wraps errors with S3ryError for standardized error handling
func (j *S3DeleteJob) wrapError(err error, operation string, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	// Check if already wrapped
	if _, ok := err.(*errors.S3ryError); ok {
		return err
	}

	// Add job-specific context
	if context == nil {
		context = make(map[string]interface{})
	}
	context["job_type"] = "s3_delete"
	context["operation"] = operation

	return errors.Wrap(err, errors.ErrCodeS3Connection, "s3_delete_job", err.Error()).WithContext("job_context", context)
}

// wrapError wraps errors with S3ryError for standardized error handling
func (j *S3ListJob) wrapError(err error, operation string, context map[string]interface{}) error {
	if err == nil {
		return nil
	}

	// Check if already wrapped
	if _, ok := err.(*errors.S3ryError); ok {
		return err
	}

	// Add job-specific context
	if context == nil {
		context = make(map[string]interface{})
	}
	context["job_type"] = "s3_list"
	context["operation"] = operation

	return errors.Wrap(err, errors.ErrCodeS3Connection, "s3_list_job", err.Error()).WithContext("job_context", context)
}
