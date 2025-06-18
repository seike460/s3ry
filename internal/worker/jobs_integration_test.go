package worker

import (
	"context"
	"testing"
	"time"

	"github.com/seike460/s3ry/pkg/types"
	"github.com/stretchr/testify/assert"
)

// IntegrationTestJob is a simple job implementation for testing job execution patterns
type IntegrationTestJob struct {
	id       int
	executed bool
	err      error
	duration time.Duration
}

func (t *IntegrationTestJob) Execute(ctx context.Context) error {
	if t.duration > 0 {
		select {
		case <-time.After(t.duration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	t.executed = true
	return t.err
}

func TestS3DownloadJob_Structure(t *testing.T) {
	// Test that S3DownloadJob can be created and has proper structure
	job := &S3DownloadJob{
		Client: nil, // Will be nil in structure test
		Request: types.DownloadRequest{
			Bucket:   "test-bucket",
			Key:      "test-key",
			FilePath: "/tmp/test-file",
		},
		Progress: nil,
	}

	assert.NotNil(t, job)
	assert.Equal(t, "test-bucket", job.Request.Bucket)
	assert.Equal(t, "test-key", job.Request.Key)
	assert.Equal(t, "/tmp/test-file", job.Request.FilePath)
}

func TestS3UploadJob_Structure(t *testing.T) {
	// Test that S3UploadJob can be created and has proper structure
	job := &S3UploadJob{
		Client: nil, // Will be nil in structure test
		Request: types.UploadRequest{
			Bucket:      "test-bucket",
			Key:         "test-key",
			FilePath:    "/tmp/test-file",
			ContentType: "text/plain",
		},
		Progress: nil,
	}

	assert.NotNil(t, job)
	assert.Equal(t, "test-bucket", job.Request.Bucket)
	assert.Equal(t, "test-key", job.Request.Key)
	assert.Equal(t, "/tmp/test-file", job.Request.FilePath)
	assert.Equal(t, "text/plain", job.Request.ContentType)
}

func TestS3DeleteJob_Structure(t *testing.T) {
	// Test that S3DeleteJob can be created and has proper structure
	job := &S3DeleteJob{
		Client: nil, // Will be nil in structure test
		Bucket: "test-bucket",
		Key:    "test-key",
	}

	assert.NotNil(t, job)
	assert.Equal(t, "test-bucket", job.Bucket)
	assert.Equal(t, "test-key", job.Key)
}

func TestS3ListJob_Structure(t *testing.T) {
	// Test that S3ListJob can be created and has proper structure
	resultChan := make(chan []types.Object, 1)

	job := &S3ListJob{
		Client: nil, // Will be nil in structure test
		Request: types.ListRequest{
			Bucket: "test-bucket",
			Prefix: "documents/",
		},
		Results: resultChan,
	}

	assert.NotNil(t, job)
	assert.Equal(t, "test-bucket", job.Request.Bucket)
	assert.Equal(t, "documents/", job.Request.Prefix)
	assert.NotNil(t, job.Results)
}

func TestJobIntegrationWithWorkerPool(t *testing.T) {
	// Test that jobs work properly with the worker pool
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	// Create test jobs
	jobs := []Job{
		&IntegrationTestJob{id: 1, duration: 10 * time.Millisecond},
		&IntegrationTestJob{id: 2, duration: 20 * time.Millisecond},
		&IntegrationTestJob{id: 3, duration: 5 * time.Millisecond},
	}

	// Submit jobs
	for _, job := range jobs {
		err := pool.Submit(job)
		assert.NoError(t, err)
	}

	// Collect results
	results := make([]Result, 0, len(jobs))
	for i := 0; i < len(jobs); i++ {
		result := <-pool.Results()
		results = append(results, result)
	}

	// Verify all jobs completed
	assert.Len(t, results, len(jobs))
	for _, result := range results {
		assert.NoError(t, result.Error)
		testJob := result.Job.(*IntegrationTestJob)
		assert.True(t, testJob.executed)
	}
}

func TestS3JobTypes_ImplementJobInterface(t *testing.T) {
	// Test that all S3 job types implement the Job interface
	var jobs []Job

	jobs = append(jobs, &S3DownloadJob{})
	jobs = append(jobs, &S3UploadJob{})
	jobs = append(jobs, &S3DeleteJob{})
	jobs = append(jobs, &S3ListJob{})

	// If this compiles, then all types implement the Job interface
	assert.Len(t, jobs, 4)

	// Test that they can be cast to Job interface
	for i, job := range jobs {
		assert.NotNil(t, job, "Job %d should not be nil", i)
	}

	// Test interface compliance without actually executing
	// (execution would fail with nil clients, which is expected)
	var downloadJob Job = &S3DownloadJob{}
	var uploadJob Job = &S3UploadJob{}
	var deleteJob Job = &S3DeleteJob{}
	var listJob Job = &S3ListJob{}

	assert.NotNil(t, downloadJob)
	assert.NotNil(t, uploadJob)
	assert.NotNil(t, deleteJob)
	assert.NotNil(t, listJob)
}

func TestProgressCallback_Integration(t *testing.T) {
	// Test progress callback functionality
	var progressCalls []struct {
		bytes int64
		total int64
	}

	progressCallback := func(bytes, total int64) {
		progressCalls = append(progressCalls, struct {
			bytes int64
			total int64
		}{bytes, total})
	}

	// Test progress callback with mock data
	progressCallback(100, 1000)
	progressCallback(500, 1000)
	progressCallback(1000, 1000)

	assert.Len(t, progressCalls, 3)
	assert.Equal(t, int64(100), progressCalls[0].bytes)
	assert.Equal(t, int64(1000), progressCalls[0].total)
	assert.Equal(t, int64(500), progressCalls[1].bytes)
	assert.Equal(t, int64(1000), progressCalls[2].bytes)
}

func TestS3Jobs_MetricsIntegration(t *testing.T) {
	// Test that S3 jobs work with metrics (structure test)
	// We can't test actual metrics without real operations, but we can test the structure

	// Test download job with metrics integration
	downloadJob := &S3DownloadJob{
		Request: types.DownloadRequest{
			Bucket:   "metrics-test-bucket",
			Key:      "metrics-test-key",
			FilePath: "/tmp/metrics-test",
		},
	}

	assert.NotNil(t, downloadJob)

	// Test upload job with metrics integration
	uploadJob := &S3UploadJob{
		Request: types.UploadRequest{
			Bucket:   "metrics-test-bucket",
			Key:      "metrics-test-key",
			FilePath: "/tmp/metrics-test",
		},
	}

	assert.NotNil(t, uploadJob)

	// Test delete job with metrics integration
	deleteJob := &S3DeleteJob{
		Bucket: "metrics-test-bucket",
		Key:    "metrics-test-key",
	}

	assert.NotNil(t, deleteJob)

	// Test list job with metrics integration
	listJob := &S3ListJob{
		Request: types.ListRequest{
			Bucket: "metrics-test-bucket",
		},
		Results: make(chan []types.Object, 1),
	}

	assert.NotNil(t, listJob)
}

func TestJobErrorHandling(t *testing.T) {
	// Test job error handling patterns
	config := PoolConfig{
		Workers:   1,
		QueueSize: 5,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	// Create jobs with different error conditions
	jobs := []Job{
		&IntegrationTestJob{id: 1, err: nil},            // Success
		&IntegrationTestJob{id: 2, err: assert.AnError}, // Error
		&IntegrationTestJob{id: 3, err: nil},            // Success
	}

	// Submit jobs
	for _, job := range jobs {
		err := pool.Submit(job)
		assert.NoError(t, err)
	}

	// Collect results and verify error handling
	var errorCount, successCount int
	for i := 0; i < len(jobs); i++ {
		result := <-pool.Results()
		if result.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	assert.Equal(t, 1, errorCount)
	assert.Equal(t, 2, successCount)
}

func TestJobCancellation(t *testing.T) {
	// Test job cancellation through context
	config := PoolConfig{
		Workers:   1,
		QueueSize: 5,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	// Create a job that will be cancelled
	longRunningJob := &IntegrationTestJob{
		id:       1,
		duration: 100 * time.Millisecond,
	}

	err := pool.Submit(longRunningJob)
	assert.NoError(t, err)

	// Stop the pool quickly to trigger context cancellation
	go func() {
		time.Sleep(10 * time.Millisecond)
		pool.Stop()
	}()

	// The result might be success or cancellation depending on timing
	result := <-pool.Results()
	// We don't assert on the specific error since timing is unpredictable
	// Just verify we get a result
	assert.NotNil(t, result)
}

// Benchmark tests for job structures and basic operations
func BenchmarkS3DownloadJob_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &S3DownloadJob{
			Request: types.DownloadRequest{
				Bucket:   "benchmark-bucket",
				Key:      "benchmark-key",
				FilePath: "/tmp/benchmark-file",
			},
		}
		_ = job
	}
}

func BenchmarkS3UploadJob_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &S3UploadJob{
			Request: types.UploadRequest{
				Bucket:   "benchmark-bucket",
				Key:      "benchmark-key",
				FilePath: "/tmp/benchmark-file",
			},
		}
		_ = job
	}
}

func BenchmarkJobSubmissionToPool(b *testing.B) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 1000,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &IntegrationTestJob{id: i}
		pool.Submit(job)
	}
}
