// Package interfaces defines common interfaces to avoid circular dependencies
package interfaces

import (
	"context"

	"github.com/seike460/s3ry/pkg/types"
)

// Job represents a unit of work that can be executed by a worker
type Job interface {
	// Execute performs the job and returns an error
	Execute(ctx context.Context) error
}

// Result represents the result of a job execution
type Result struct {
	Job   Job
	Error error
}

// WorkerPool manages a pool of workers that can execute jobs
type WorkerPool interface {
	// Start begins processing jobs
	Start()
	// Stop gracefully shuts down the worker pool
	Stop()
	// Submit adds a job to the worker pool
	Submit(job Job) error
	// Results returns a channel of job results
	Results() <-chan Result
}

// PoolStats provides statistics about the worker pool
type PoolStats struct {
	WorkerCount   int   `json:"worker_count"`
	ActiveJobs    int   `json:"active_jobs"`
	QueuedJobs    int   `json:"queued_jobs"`
	CompletedJobs int64 `json:"completed_jobs"`
	FailedJobs    int64 `json:"failed_jobs"`
}

// TransferJob represents a file transfer job
type TransferJob interface {
	Job
	// Source returns the source path/key
	Source() string
	// Target returns the target path/key
	Target() string
	// Size returns the expected transfer size
	Size() int64
	// Progress returns the current transfer progress
	Progress() int64
	// TransferType returns the type of transfer
	TransferType() types.TransferType
}
