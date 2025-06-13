package worker

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
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

	// Get file size for metrics
	headInput := &s3.HeadObjectInput{
		Bucket: aws.String(j.Request.Bucket),
		Key:    aws.String(j.Request.Key),
	}
	
	headOutput, err := j.Client.S3().HeadObjectWithContext(ctx, headInput)
	if err != nil {
		return fmt.Errorf("failed to get object metadata: %w", err)
	}
	
	fileSize := *headOutput.ContentLength

	// Create the file
	file, err := os.Create(j.Request.FilePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", j.Request.FilePath, err)
	}
	defer file.Close()

	// Create download input
	input := &s3.GetObjectInput{
		Bucket: aws.String(j.Request.Bucket),
		Key:    aws.String(j.Request.Key),
	}

	// Download with progress tracking
	downloader := j.Client.Downloader()
	if j.Progress != nil {
		downloader.Concurrency = 5 // Concurrent download parts
		downloader.PartSize = 5 * 1024 * 1024 // 5MB parts
	}

	_, err = downloader.DownloadWithContext(ctx, file, input)
	if err != nil {
		// Clean up the file if download failed
		os.Remove(j.Request.FilePath)
		return fmt.Errorf("failed to download %s: %w", j.Request.Key, err)
	}

	// Update metrics on successful download
	m.IncrementS3Downloads()
	m.AddBytesTransferred(fileSize)

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

	// Open the file
	file, err := os.Open(j.Request.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", j.Request.FilePath, err)
	}
	defer file.Close()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", j.Request.FilePath, err)
	}

	// Create upload input
	input := &s3manager.UploadInput{
		Bucket: aws.String(j.Request.Bucket),
		Key:    aws.String(j.Request.Key),
		Body:   file,
	}

	// Set content type if provided
	if j.Request.ContentType != "" {
		input.ContentType = aws.String(j.Request.ContentType)
	}

	// Set metadata if provided
	if j.Request.Metadata != nil {
		input.Metadata = j.Request.Metadata
	}

	// Upload with progress tracking
	uploader := j.Client.Uploader()
	if j.Progress != nil {
		uploader.Concurrency = 5 // Concurrent upload parts
		uploader.PartSize = 5 * 1024 * 1024 // 5MB parts
	}

	_, err = uploader.UploadWithContext(ctx, input)
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

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(j.Bucket),
		Key:    aws.String(j.Key),
	}

	_, err := j.Client.S3().DeleteObjectWithContext(ctx, input)
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

	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(j.Request.Bucket),
	}

	if j.Request.Prefix != "" {
		input.Prefix = aws.String(j.Request.Prefix)
	}
	if j.Request.Delimiter != "" {
		input.Delimiter = aws.String(j.Request.Delimiter)
	}
	if j.Request.MaxKeys > 0 {
		input.MaxKeys = aws.Int64(j.Request.MaxKeys)
	}
	if j.Request.StartAfter != "" {
		input.StartAfter = aws.String(j.Request.StartAfter)
	}

	err := j.Client.S3().ListObjectsV2PagesWithContext(ctx, input,
		func(page *s3.ListObjectsV2Output, lastPage bool) bool {
			for _, obj := range page.Contents {
				if obj.Key != nil && !strings.HasSuffix(*obj.Key, "/") {
					objects = append(objects, types.Object{
						Key:          *obj.Key,
						Size:         *obj.Size,
						LastModified: *obj.LastModified,
						ETag:         *obj.ETag,
						StorageClass: *obj.StorageClass,
					})
				}
			}
			return !lastPage
		})

	if err != nil {
		return fmt.Errorf("failed to list objects in %s: %w", j.Request.Bucket, err)
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