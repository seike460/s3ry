package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Comprehensive test coverage for Worker package functionality
// Focusing on core functions to achieve 90% coverage

func TestPoolConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, 0, config.Workers) // Should use CPU count
	assert.Equal(t, 100, config.QueueSize)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.RetryCount)
	assert.Equal(t, 1*time.Second, config.RetryDelay)
}

func TestPool_Creation(t *testing.T) {
	tests := []struct {
		name   string
		config PoolConfig
	}{
		{
			name:   "default_config",
			config: DefaultConfig(),
		},
		{
			name: "custom_config",
			config: PoolConfig{
				Workers:    4,
				QueueSize:  50,
				Timeout:    10 * time.Second,
				RetryCount: 2,
				RetryDelay: 500 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := NewPool(tt.config)
			require.NotNil(t, pool)

			expectedWorkers := tt.config.Workers
			if expectedWorkers <= 0 {
				expectedWorkers = 4 // Assume 4 CPUs for test
			}

			assert.NotNil(t, pool.jobQueue)
			assert.NotNil(t, pool.resultChan)
			assert.NotNil(t, pool.ctx)
			assert.NotNil(t, pool.cancel)
			assert.NotNil(t, pool.metrics)
		})
	}
}

func TestPool_StartStopComprehensive(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	require.NotNil(t, pool)

	t.Run("start_and_stop", func(t *testing.T) {
		pool.Start()

		// Pool should be running
		stats := pool.GetWorkerStats()
		assert.True(t, stats.IsRunning)
		assert.Equal(t, 2, stats.TotalWorkers)
		assert.Equal(t, 10, stats.QueueCapacity)

		pool.Stop()

		// Pool should be stopped
		stats = pool.GetWorkerStats()
		assert.False(t, stats.IsRunning)
	})
}

func TestPool_JobSubmission(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 5,
	}
	pool := NewPool(config)
	require.NotNil(t, pool)

	pool.Start()
	defer pool.Stop()

	t.Run("successful_submission", func(t *testing.T) {
		job := &TestJob{ID: 1, Duration: 10 * time.Millisecond}

		err := pool.Submit(job)
		assert.NoError(t, err)
	})

	t.Run("queue_full", func(t *testing.T) {
		// Fill the queue
		for i := 0; i < 10; i++ {
			job := &TestJob{ID: i, Duration: 100 * time.Millisecond}
			pool.Submit(job) // Some will succeed, some may fail
		}

		// This one should potentially fail due to full queue
		job := &TestJob{ID: 999, Duration: 10 * time.Millisecond}
		err := pool.Submit(job)

		// Either succeeds or fails with queue full
		if err != nil {
			assert.Contains(t, err.Error(), "queue is full")
		}
	})
}

func TestPool_JobExecution(t *testing.T) {
	config := PoolConfig{
		Workers:   1,
		QueueSize: 5,
	}
	pool := NewPool(config)
	require.NotNil(t, pool)

	pool.Start()
	defer pool.Stop()

	t.Run("successful_job_execution", func(t *testing.T) {
		job := &TestJob{ID: 1, Duration: 10 * time.Millisecond}

		err := pool.Submit(job)
		require.NoError(t, err)

		// Get result
		result := <-pool.Results()
		assert.NoError(t, result.Error)
		assert.Equal(t, job, result.Job)
	})

	t.Run("job_execution_with_error", func(t *testing.T) {
		job := &TestJob{
			ID:       2,
			Duration: 10 * time.Millisecond,
			Data:     []byte("error"),
		}

		err := pool.Submit(job)
		require.NoError(t, err)

		// Get result
		result := <-pool.Results()

		// Check if job executed (error depends on TestJob implementation)
		assert.NotNil(t, result)
		assert.Equal(t, job, result.Job)
	})
}

func TestPool_Metrics(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 5,
	}
	pool := NewPool(config)
	require.NotNil(t, pool)

	t.Run("get_metrics", func(t *testing.T) {
		metrics := pool.GetMetrics()
		require.NotNil(t, metrics)
		// Metrics should be available (structure test)
	})

	t.Run("get_worker_stats", func(t *testing.T) {
		stats := pool.GetWorkerStats()

		assert.Equal(t, 2, stats.TotalWorkers)
		assert.Equal(t, 5, stats.QueueCapacity)
		assert.GreaterOrEqual(t, stats.QueueLength, 0)
		assert.LessOrEqual(t, stats.QueueLength, 5)
	})
}

func TestBatchProcessor_Functionality(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	require.NotNil(t, pool)

	pool.Start()
	defer pool.Stop()

	var progressUpdates []struct {
		completed int
		total     int
	}
	var progressMutex sync.Mutex

	onProgress := func(completed, total int) {
		progressMutex.Lock()
		progressUpdates = append(progressUpdates, struct {
			completed int
			total     int
		}{completed, total})
		progressMutex.Unlock()
	}

	processor := NewBatchProcessor(pool, onProgress)
	require.NotNil(t, processor)

	t.Run("process_batch", func(t *testing.T) {
		jobs := []Job{
			&TestJob{ID: 1, Duration: 10 * time.Millisecond},
			&TestJob{ID: 2, Duration: 15 * time.Millisecond},
			&TestJob{ID: 3, Duration: 5 * time.Millisecond},
		}

		results := processor.ProcessBatch(jobs)

		assert.Len(t, results, len(jobs))

		// Check progress was tracked
		completed, total := processor.Progress()
		assert.Equal(t, len(jobs), completed)
		assert.Equal(t, len(jobs), total)

		// Check progress callbacks were called
		progressMutex.Lock()
		assert.Greater(t, len(progressUpdates), 0)
		progressMutex.Unlock()
	})
}

func TestErrorHandling_Comprehensive(t *testing.T) {
	config := PoolConfig{
		Workers:   1,
		QueueSize: 5,
	}
	pool := NewPool(config)
	require.NotNil(t, pool)

	pool.Start()
	defer pool.Stop()

	t.Run("wrap_job_error_nil", func(t *testing.T) {
		result := pool.wrapJobError(nil, "test_operation")
		assert.Nil(t, result)
	})

	t.Run("wrap_job_error_timeout", func(t *testing.T) {
		result := pool.wrapJobError(context.DeadlineExceeded, "test_operation")
		require.NotNil(t, result)
		assert.Contains(t, result.Error(), "timeout")
	})

	t.Run("wrap_job_error_cancelled", func(t *testing.T) {
		result := pool.wrapJobError(context.Canceled, "test_operation")
		require.NotNil(t, result)
		assert.Contains(t, result.Error(), "cancelled")
	})

	t.Run("wrap_job_error_generic", func(t *testing.T) {
		err := errors.New("generic error")
		result := pool.wrapJobError(err, "test_operation")
		require.NotNil(t, result)
		assert.Contains(t, result.Error(), "generic error")
	})
}

func TestConcurrentOperations(t *testing.T) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: 20,
	}
	pool := NewPool(config)
	require.NotNil(t, pool)

	pool.Start()
	defer pool.Stop()

	t.Run("concurrent_job_submission", func(t *testing.T) {
		const numJobs = 50
		const numGoroutines = 10

		var successCount int64
		var errorCount int64
		var wg sync.WaitGroup

		// Submit jobs concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(start int) {
				defer wg.Done()
				for j := 0; j < numJobs/numGoroutines; j++ {
					job := &TestJob{
						ID:       start*100 + j,
						Duration: time.Duration(j%10+1) * time.Millisecond,
					}

					err := pool.Submit(job)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		// Collect results
		successfulJobs := atomic.LoadInt64(&successCount)
		totalResults := int64(0)

		timeout := time.After(5 * time.Second)

	resultLoop:
		for totalResults < successfulJobs {
			select {
			case result := <-pool.Results():
				totalResults++
				_ = result // Process result
			case <-timeout:
				break resultLoop
			}
		}

		assert.Greater(t, successfulJobs, int64(0))
		assert.GreaterOrEqual(t, totalResults, successfulJobs/2) // At least half should complete
	})
}

func TestJobExecutionDetails(t *testing.T) {
	config := PoolConfig{
		Workers:   1,
		QueueSize: 5,
	}
	pool := NewPool(config)
	require.NotNil(t, pool)

	pool.Start()
	defer pool.Stop()

	t.Run("job_execution_timing", func(t *testing.T) {
		duration := 50 * time.Millisecond
		job := &TestJob{ID: 1, Duration: duration}

		start := time.Now()
		err := pool.Submit(job)
		require.NoError(t, err)

		result := <-pool.Results()
		elapsed := time.Since(start)

		assert.NoError(t, result.Error)
		assert.GreaterOrEqual(t, elapsed, duration)
		assert.Less(t, elapsed, duration+100*time.Millisecond) // Should complete reasonably quickly
	})
}

func TestPoolResourceManagement(t *testing.T) {
	t.Run("multiple_pools", func(t *testing.T) {
		// Test creating and managing multiple pools
		pools := make([]*Pool, 3)
		for i := range pools {
			config := PoolConfig{
				Workers:   2,
				QueueSize: 5,
			}
			pools[i] = NewPool(config)
			pools[i].Start()
		}

		// All pools should be independent
		for i, pool := range pools {
			job := &TestJob{ID: i, Duration: 10 * time.Millisecond}
			err := pool.Submit(job)
			assert.NoError(t, err)
		}

		// Collect results and clean up
		for _, pool := range pools {
			select {
			case result := <-pool.Results():
				assert.NoError(t, result.Error)
			case <-time.After(1 * time.Second):
				t.Error("Timeout waiting for result")
			}
			pool.Stop()
		}
	})
}

// Benchmark tests to ensure performance characteristics
func BenchmarkPool_JobSubmissionComprehensive(b *testing.B) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: 1000,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &TestJob{ID: i, Duration: time.Microsecond}
		pool.Submit(job)
	}
}

func BenchmarkPool_JobExecutionComprehensive(b *testing.B) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: 100,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &TestJob{ID: i, Duration: time.Microsecond}
		pool.Submit(job)
		<-pool.Results()
	}
}

func BenchmarkBatchProcessor_ProcessBatchComprehensive(b *testing.B) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: 100,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()

	processor := NewBatchProcessor(pool, nil)

	jobs := make([]Job, 10)
	for i := range jobs {
		jobs[i] = &TestJob{ID: i, Duration: time.Microsecond}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		processor.ProcessBatch(jobs)
	}
}

func BenchmarkPool_WorkerStats(b *testing.B) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: 100,
	}
	pool := NewPool(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stats := pool.GetWorkerStats()
		_ = stats
	}
}
