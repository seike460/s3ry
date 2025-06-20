package worker

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/seike460/s3ry/internal/errors"
	"github.com/seike460/s3ry/internal/metrics"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// Job represents a unit of work
type Job = interfaces.Job

// Result represents the result of a job execution
type Result = interfaces.Result

// Pool represents a worker pool
type Pool struct {
	workers    int
	jobQueue   chan Job
	resultChan chan Result
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	once       sync.Once
	metrics    *metrics.Metrics
}

// PoolConfig configures the worker pool
type PoolConfig struct {
	Workers    int           // Number of workers (0 = number of CPUs)
	QueueSize  int           // Size of job queue (0 = unbuffered)
	Timeout    time.Duration // Timeout for individual jobs
	RetryCount int           // Number of retries for failed jobs
	RetryDelay time.Duration // Delay between retries
}

// DefaultConfig returns a default pool configuration
func DefaultConfig() PoolConfig {
	return PoolConfig{
		Workers:    0, // Use number of CPUs
		QueueSize:  100,
		Timeout:    30 * time.Second,
		RetryCount: 3,
		RetryDelay: 1 * time.Second,
	}
}

// NewPool creates a new worker pool with the given configuration
func NewPool(config PoolConfig) *Pool {
	if config.Workers <= 0 {
		config.Workers = runtime.NumCPU()
	}

	ctx, cancel := context.WithCancel(context.Background())

	pool := &Pool{
		workers:    config.Workers,
		jobQueue:   make(chan Job, config.QueueSize),
		resultChan: make(chan Result, config.QueueSize),
		ctx:        ctx,
		cancel:     cancel,
		metrics:    metrics.GetGlobalMetrics(),
	}

	return pool
}

// Start starts the worker pool
func (p *Pool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the worker pool gracefully
func (p *Pool) Stop() {
	p.once.Do(func() {
		close(p.jobQueue)
		p.wg.Wait()
		p.cancel()
		close(p.resultChan)
	})
}

// Submit submits a job to the pool
func (p *Pool) Submit(job Job) error {
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("pool is shutting down")
	default:
	}

	select {
	case p.jobQueue <- job:
		return nil
	case <-p.ctx.Done():
		return fmt.Errorf("pool is shutting down")
	default:
		return fmt.Errorf("job queue is full")
	}
}

// Results returns the result channel
func (p *Pool) Results() <-chan Result {
	return p.resultChan
}

// worker is the main worker goroutine
func (p *Pool) worker(id int) {
	defer p.wg.Done()

	for {
		select {
		case job, ok := <-p.jobQueue:
			if !ok {
				// Job queue is closed
				return
			}

			// Execute job with timeout
			result := p.executeJob(job)

			// Send result
			select {
			case p.resultChan <- result:
			case <-p.ctx.Done():
				return
			}

		case <-p.ctx.Done():
			return
		}
	}
}

// executeJob executes a job with timeout, retry logic, and error recovery
func (p *Pool) executeJob(job Job) Result {
	// Start performance timer
	timer := p.metrics.StartTimer("job_execution")
	defer timer.Stop()

	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()

	var lastErr error
	maxRetries := 3

	// Execute with retry logic for recoverable errors
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := job.Execute(ctx)

		if err == nil {
			// Success
			return Result{Job: job, Error: nil}
		}

		lastErr = err

		// Check if error is recoverable
		if s3ryErr, ok := err.(*errors.S3ryError); ok {
			if recoverable, ok := s3ryErr.Context["recoverable"].(bool); ok && !recoverable {
				// Non-recoverable error, don't retry
				break
			}

			// Check if we should retry
			if attempt < maxRetries {
				// Apply backoff delay
				var delay time.Duration
				if retryDelay, ok := s3ryErr.Context["retry_delay"].(string); ok && retryDelay == "exponential" {
					delay = time.Duration(1<<uint(attempt)) * time.Second
				} else {
					delay = time.Second * time.Duration(attempt+1)
				}

				// Wait before retry
				select {
				case <-ctx.Done():
					return Result{Job: job, Error: ctx.Err()}
				case <-time.After(delay):
					// Continue to next retry
				}
			}
		} else {
			// Non-S3ry error, wrap it and don't retry
			lastErr = p.wrapJobError(err, "job_execution")
			break
		}
	}

	// Update metrics based on result
	if lastErr != nil {
		p.metrics.IncrementFailedOperations()
	}

	return Result{
		Job:   job,
		Error: lastErr,
	}
}

// wrapJobError wraps generic job errors in S3ryError
func (p *Pool) wrapJobError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Check if already wrapped
	if _, ok := err.(*errors.S3ryError); ok {
		return err
	}

	// Determine error code based on error type
	var errorCode errors.ErrorCode
	if err == context.DeadlineExceeded {
		errorCode = errors.ErrCodeTimeout
	} else if err == context.Canceled {
		errorCode = errors.ErrCodeCancelled
	} else {
		errorCode = errors.ErrCodeJobExecution
	}

	return errors.Wrap(err, errorCode, "worker_pool", err.Error()).WithContext("worker_pool_context", map[string]interface{}{
		"operation":   operation,
		"recoverable": errorCode == errors.ErrCodeTimeout || errorCode == errors.ErrCodeJobExecution,
	})
}

// BatchProcessor handles batch processing with progress tracking
type BatchProcessor struct {
	pool          interfaces.WorkerPool
	totalJobs     int
	completedJobs int
	mutex         sync.RWMutex
	onProgress    func(completed, total int)
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(pool interfaces.WorkerPool, onProgress func(completed, total int)) *BatchProcessor {
	return &BatchProcessor{
		pool:       pool,
		onProgress: onProgress,
	}
}

// ProcessBatch processes a batch of jobs and waits for completion
func (bp *BatchProcessor) ProcessBatch(jobs []Job) []Result {
	bp.mutex.Lock()
	bp.totalJobs = len(jobs)
	bp.completedJobs = 0
	bp.mutex.Unlock()

	// Submit all jobs
	for _, job := range jobs {
		if err := bp.pool.Submit(job); err != nil {
			// If submission fails, return error result
			return []Result{{Job: job, Error: err}}
		}
	}

	// Collect results
	results := make([]Result, 0, len(jobs))
	for i := 0; i < len(jobs); i++ {
		result := <-bp.pool.Results()
		results = append(results, result)

		bp.mutex.Lock()
		bp.completedJobs++
		if bp.onProgress != nil {
			bp.onProgress(bp.completedJobs, bp.totalJobs)
		}
		bp.mutex.Unlock()
	}

	return results
}

// Progress returns the current progress
func (bp *BatchProcessor) Progress() (completed, total int) {
	bp.mutex.RLock()
	defer bp.mutex.RUnlock()
	return bp.completedJobs, bp.totalJobs
}

// GetMetrics returns the pool's metrics
func (p *Pool) GetMetrics() *metrics.Metrics {
	return p.metrics
}

// GetWorkerStats returns worker pool statistics
func (p *Pool) GetWorkerStats() WorkerStats {
	return WorkerStats{
		TotalWorkers:  p.workers,
		QueueLength:   len(p.jobQueue),
		QueueCapacity: cap(p.jobQueue),
		IsRunning:     p.ctx.Err() == nil,
	}
}

// WorkerStats represents worker pool statistics
type WorkerStats struct {
	TotalWorkers  int
	QueueLength   int
	QueueCapacity int
	IsRunning     bool
}
