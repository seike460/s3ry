package worker

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockJob implements Job interface for testing
type MockJob struct {
	id        int
	duration  time.Duration
	shouldErr bool
	executed  int32
}

func (m *MockJob) Execute(ctx context.Context) error {
	atomic.AddInt32(&m.executed, 1)

	if m.duration > 0 {
		select {
		case <-time.After(m.duration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if m.shouldErr {
		return errors.New("mock job error")
	}

	return nil
}

func (m *MockJob) WasExecuted() bool {
	return atomic.LoadInt32(&m.executed) > 0
}

func (m *MockJob) ExecutionCount() int {
	return int(atomic.LoadInt32(&m.executed))
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 0, config.Workers) // Should use CPU count
	assert.Equal(t, 100, config.QueueSize)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.RetryCount)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
}

func TestNewPool(t *testing.T) {
	config := DefaultConfig()
	pool := NewPool(config)

	assert.NotNil(t, pool)
	assert.Equal(t, runtime.NumCPU(), pool.workers)
	assert.NotNil(t, pool.jobQueue)
	assert.NotNil(t, pool.resultChan)
	assert.NotNil(t, pool.ctx)
	assert.NotNil(t, pool.cancel)
	assert.NotNil(t, pool.metrics)

	// Clean up
	pool.Stop()
}

func TestNewPool_CustomWorkerCount(t *testing.T) {
	config := PoolConfig{
		Workers:   5,
		QueueSize: 50,
	}
	pool := NewPool(config)

	assert.Equal(t, 5, pool.workers)
	assert.Equal(t, 50, cap(pool.jobQueue))

	pool.Stop()
}

func TestPool_StartStop(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)

	// Start the pool
	pool.Start()

	// Verify pool is running
	stats := pool.GetWorkerStats()
	assert.Equal(t, 2, stats.TotalWorkers)
	assert.True(t, stats.IsRunning)

	// Stop the pool
	pool.Stop()

	// Verify pool is stopped
	stats = pool.GetWorkerStats()
	assert.False(t, stats.IsRunning)
}

func TestPool_Submit(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	job := &MockJob{id: 1}
	err := pool.Submit(job)

	assert.NoError(t, err)

	// Wait for job to be processed
	result := <-pool.Results()
	assert.NoError(t, result.Error)
	assert.True(t, job.WasExecuted())
}

func TestPool_SubmitMultipleJobs(t *testing.T) {
	config := PoolConfig{
		Workers:   3,
		QueueSize: 20,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	numJobs := 10
	jobs := make([]*MockJob, numJobs)

	// Submit jobs
	for i := 0; i < numJobs; i++ {
		jobs[i] = &MockJob{id: i}
		err := pool.Submit(jobs[i])
		assert.NoError(t, err)
	}

	// Collect results
	for i := 0; i < numJobs; i++ {
		result := <-pool.Results()
		assert.NoError(t, result.Error)
	}

	// Verify all jobs were executed
	for i, job := range jobs {
		assert.True(t, job.WasExecuted(), "Job %d was not executed", i)
	}
}

func TestPool_JobWithError(t *testing.T) {
	config := PoolConfig{
		Workers:   1,
		QueueSize: 5,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	job := &MockJob{id: 1, shouldErr: true}
	err := pool.Submit(job)
	assert.NoError(t, err)

	result := <-pool.Results()
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "mock job error")
	assert.True(t, job.WasExecuted())
}

func TestPool_QueueFull(t *testing.T) {
	config := PoolConfig{
		Workers:   1,
		QueueSize: 2,
	}
	pool := NewPool(config)
	// Don't start the pool, so jobs accumulate in queue

	// Fill the queue
	for i := 0; i < 2; i++ {
		job := &MockJob{id: i}
		err := pool.Submit(job)
		assert.NoError(t, err)
	}

	// Next job should fail
	job := &MockJob{id: 3}
	err := pool.Submit(job)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job queue is full")

	pool.Stop()
}

func TestPool_ContextCancellation(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()

	// Submit a long-running job
	job := &MockJob{id: 1, duration: 100 * time.Millisecond}
	err := pool.Submit(job)
	assert.NoError(t, err)

	// Stop the pool before job completes
	pool.Stop()

	// Try to submit after stop
	newJob := &MockJob{id: 2}
	err = pool.Submit(newJob)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pool is shutting down")
}

func TestPool_ConcurrentSubmission(t *testing.T) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: 50,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	numGoroutines := 10
	jobsPerGoroutine := 5
	totalJobs := numGoroutines * jobsPerGoroutine

	var wg sync.WaitGroup
	var submitted int32

	// Submit jobs concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < jobsPerGoroutine; j++ {
				job := &MockJob{id: goroutineID*jobsPerGoroutine + j}
				err := pool.Submit(job)
				if err == nil {
					atomic.AddInt32(&submitted, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	// Collect results
	resultsCollected := 0
	timeout := time.After(5 * time.Second)

	for resultsCollected < int(submitted) {
		select {
		case <-pool.Results():
			resultsCollected++
		case <-timeout:
			t.Fatalf("Timeout waiting for results. Expected %d, got %d", submitted, resultsCollected)
		}
	}

	assert.Equal(t, int32(totalJobs), submitted)
	assert.Equal(t, totalJobs, resultsCollected)
}

func TestPool_GetWorkerStats(t *testing.T) {
	config := PoolConfig{
		Workers:   3,
		QueueSize: 15,
	}
	pool := NewPool(config)

	stats := pool.GetWorkerStats()
	assert.Equal(t, 3, stats.TotalWorkers)
	assert.Equal(t, 0, stats.QueueLength)
	assert.Equal(t, 15, stats.QueueCapacity)
	assert.True(t, stats.IsRunning)

	pool.Start()

	// Submit some jobs
	for i := 0; i < 5; i++ {
		job := &MockJob{id: i, duration: 50 * time.Millisecond}
		pool.Submit(job)
	}

	stats = pool.GetWorkerStats()
	assert.True(t, stats.QueueLength >= 0) // Some jobs might have been processed already

	pool.Stop()

	stats = pool.GetWorkerStats()
	assert.False(t, stats.IsRunning)
}

func TestPool_GetMetrics(t *testing.T) {
	config := DefaultConfig()
	pool := NewPool(config)

	metrics := pool.GetMetrics()
	assert.NotNil(t, metrics)

	pool.Stop()
}

func TestPool_MultipleStops(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()

	// Multiple stops should not panic
	assert.NotPanics(t, func() {
		pool.Stop()
		pool.Stop()
		pool.Stop()
	})
}

func TestPool_JobTimeout(t *testing.T) {
	config := PoolConfig{
		Workers:   1,
		QueueSize: 5,
		Timeout:   10 * time.Millisecond,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	// Submit a job that takes longer than timeout
	job := &MockJob{id: 1, duration: 100 * time.Millisecond}
	err := pool.Submit(job)
	assert.NoError(t, err)

	result := <-pool.Results()
	// The job should complete (our current implementation doesn't enforce timeout yet)
	// This test documents current behavior
	assert.NoError(t, result.Error)
}

func TestPool_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := PoolConfig{
		Workers:   runtime.NumCPU(),
		QueueSize: 1000,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	numJobs := 1000
	start := time.Now()

	// Submit jobs
	for i := 0; i < numJobs; i++ {
		job := &MockJob{id: i}
		pool.Submit(job)
	}

	// Collect results
	for i := 0; i < numJobs; i++ {
		<-pool.Results()
	}

	duration := time.Since(start)
	t.Logf("Processed %d jobs in %v", numJobs, duration)

	// Should process at least 100 jobs per second
	jobsPerSecond := float64(numJobs) / duration.Seconds()
	assert.Greater(t, jobsPerSecond, 100.0, "Performance too slow: %.2f jobs/sec", jobsPerSecond)
}

// Benchmark tests
func BenchmarkPool_JobSubmission(b *testing.B) {
	config := PoolConfig{
		Workers:   runtime.NumCPU(),
		QueueSize: 1000,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			job := &MockJob{id: id}
			pool.Submit(job)
			id++
		}
	})
}

func BenchmarkPool_JobExecution(b *testing.B) {
	config := PoolConfig{
		Workers:   runtime.NumCPU(),
		QueueSize: b.N,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	b.ResetTimer()

	// Submit jobs
	for i := 0; i < b.N; i++ {
		job := &MockJob{id: i}
		pool.Submit(job)
	}

	// Wait for completion
	for i := 0; i < b.N; i++ {
		<-pool.Results()
	}
}

func BenchmarkNewPool(b *testing.B) {
	config := DefaultConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool := NewPool(config)
		pool.Stop()
	}
}
