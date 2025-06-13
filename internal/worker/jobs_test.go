package worker

import (
	"context"
	"testing"

	"github.com/seike460/s3ry/pkg/types"
	"github.com/stretchr/testify/assert"
)

// Basic structure tests for S3 job types

func TestS3DownloadJob_BasicStructure(t *testing.T) {
	job := &S3DownloadJob{
		Request: types.DownloadRequest{
			Bucket:   "test-bucket",
			Key:      "test-key",
			FilePath: "/tmp/test-file",
		},
	}
	
	assert.Equal(t, "test-bucket", job.Request.Bucket)
	assert.Equal(t, "test-key", job.Request.Key)
	assert.Equal(t, "/tmp/test-file", job.Request.FilePath)
}

func TestS3UploadJob_BasicStructure(t *testing.T) {
	job := &S3UploadJob{
		Request: types.UploadRequest{
			Bucket:      "test-bucket",
			Key:         "test-key",
			FilePath:    "/tmp/test-file",
			ContentType: "text/plain",
		},
	}
	
	assert.Equal(t, "test-bucket", job.Request.Bucket)
	assert.Equal(t, "test-key", job.Request.Key)
	assert.Equal(t, "/tmp/test-file", job.Request.FilePath)
	assert.Equal(t, "text/plain", job.Request.ContentType)
}

func TestS3DeleteJob_BasicStructure(t *testing.T) {
	job := &S3DeleteJob{
		Bucket: "test-bucket",
		Key:    "test-key",
	}
	
	assert.Equal(t, "test-bucket", job.Bucket)
	assert.Equal(t, "test-key", job.Key)
}

func TestS3ListJob_BasicStructure(t *testing.T) {
	resultChan := make(chan []types.Object, 1)
	
	job := &S3ListJob{
		Request: types.ListRequest{
			Bucket: "test-bucket",
			Prefix: "documents/",
		},
		Results: resultChan,
	}
	
	assert.Equal(t, "test-bucket", job.Request.Bucket)
	assert.Equal(t, "documents/", job.Request.Prefix)
	assert.NotNil(t, job.Results)
}

func TestJobInterfaceCompliance(t *testing.T) {
	// Test that all S3 job types implement the Job interface
	var jobs []Job
	
	jobs = append(jobs, &S3DownloadJob{})
	jobs = append(jobs, &S3UploadJob{})
	jobs = append(jobs, &S3DeleteJob{})
	jobs = append(jobs, &S3ListJob{})
	
	assert.Len(t, jobs, 4)
	
	// Test interface compliance by type assertion
	for _, job := range jobs {
		assert.NotNil(t, job)
		// If this compiles, the interface is implemented correctly
		_ = job.(Job)
	}
}

func TestProgressCallback_Types(t *testing.T) {
	// Test progress callback type compatibility
	var callback types.ProgressCallback
	
	callback = func(bytes, total int64) {
		// Test callback
		assert.True(t, bytes >= 0)
		assert.True(t, total >= 0)
	}
	
	// Test callback execution
	callback(100, 1000)
	callback(1000, 1000)
}

// Test job execution with mock clients
func TestS3DownloadJob_ExecuteWithMockError(t *testing.T) {
	// Test that jobs properly handle nil clients with panic recovery
	job := &S3DownloadJob{
		Client: nil, // Nil client should cause panic
		Request: types.DownloadRequest{
			Bucket:   "test-bucket",
			Key:      "test-key", 
			FilePath: "/tmp/test-download",
		},
	}
	
	ctx := context.Background()
	
	// Test that Execute panics with nil client
	assert.Panics(t, func() {
		job.Execute(ctx)
	})
}

func TestS3UploadJob_ExecuteWithMockError(t *testing.T) {
	// Test with non-existent file first
	job := &S3UploadJob{
		Client: nil,
		Request: types.UploadRequest{
			Bucket:   "test-bucket",
			Key:      "test-key",
			FilePath: "/tmp/nonexistent-file-for-upload-test",
		},
	}
	
	ctx := context.Background()
	err := job.Execute(ctx)
	
	// Should error due to file not found, before client is used
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestS3DeleteJob_ExecuteWithMockError(t *testing.T) {
	job := &S3DeleteJob{
		Client: nil, // Nil client should cause panic
		Bucket: "test-bucket",
		Key:    "test-key",
	}
	
	ctx := context.Background()
	
	// Test that Execute panics with nil client
	assert.Panics(t, func() {
		job.Execute(ctx)
	})
}

func TestS3ListJob_ExecuteWithMockError(t *testing.T) {
	resultChan := make(chan []types.Object, 1)
	defer close(resultChan)
	
	job := &S3ListJob{
		Client: nil, // Nil client should cause panic
		Request: types.ListRequest{
			Bucket: "test-bucket",
			Prefix: "test-prefix/",
		},
		Results: resultChan,
	}
	
	ctx := context.Background()
	
	// Test that Execute panics with nil client
	assert.Panics(t, func() {
		job.Execute(ctx)
	})
}

func TestJobsWithValidStructure(t *testing.T) {
	// Test that jobs can be created with valid structure
	jobs := []Job{
		&S3DownloadJob{Request: types.DownloadRequest{Bucket: "test", Key: "test", FilePath: "/tmp/test"}},
		&S3UploadJob{Request: types.UploadRequest{Bucket: "test", Key: "test", FilePath: "/tmp/test"}},
		&S3DeleteJob{Bucket: "test", Key: "test"},
		&S3ListJob{Request: types.ListRequest{Bucket: "test"}, Results: make(chan []types.Object, 1)},
	}
	
	// Test that all jobs implement the interface correctly
	for i, job := range jobs {
		assert.NotNil(t, job, "Job %d should not be nil", i)
		// Verifying interface compliance by type assertion
		_, ok := job.(Job)
		assert.True(t, ok, "Job %d should implement Job interface", i)
	}
}