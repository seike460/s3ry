package worker

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBatchProcessor(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	var progressCalled bool
	onProgress := func(completed, total int) {
		progressCalled = true
	}
	
	bp := NewBatchProcessor(pool, onProgress)
	
	assert.NotNil(t, bp)
	assert.Equal(t, pool, bp.pool)
	assert.NotNil(t, bp.onProgress)
	
	// Test that progress callback is set
	bp.onProgress(1, 10)
	assert.True(t, progressCalled)
}

func TestNewBatchProcessor_NilProgress(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	bp := NewBatchProcessor(pool, nil)
	
	assert.NotNil(t, bp)
	assert.Equal(t, pool, bp.pool)
	assert.Nil(t, bp.onProgress)
}

func TestBatchProcessor_ProcessBatch(t *testing.T) {
	config := PoolConfig{
		Workers:   3,
		QueueSize: 20,
	}
	pool := NewPool(config)
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
	
	bp := NewBatchProcessor(pool, onProgress)
	
	// Create batch of jobs
	numJobs := 5
	jobs := make([]Job, numJobs)
	for i := 0; i < numJobs; i++ {
		jobs[i] = &MockJob{id: i}
	}
	
	// Process batch
	results := bp.ProcessBatch(jobs)
	
	// Verify results
	assert.Len(t, results, numJobs)
	for _, result := range results {
		assert.NoError(t, result.Error)
		// Don't check job order since concurrent execution may reorder them
		assert.NotNil(t, result.Job)
	}
	
	// Verify progress tracking
	completed, total := bp.Progress()
	assert.Equal(t, numJobs, completed)
	assert.Equal(t, numJobs, total)
	
	// Verify progress callbacks were called
	progressMutex.Lock()
	assert.True(t, len(progressUpdates) >= 1)
	lastProgress := progressUpdates[len(progressUpdates)-1]
	assert.Equal(t, numJobs, lastProgress.completed)
	assert.Equal(t, numJobs, lastProgress.total)
	progressMutex.Unlock()
}

func TestBatchProcessor_ProcessBatch_WithErrors(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	bp := NewBatchProcessor(pool, nil)
	
	// Create batch with some failing jobs
	jobs := []Job{
		&MockJob{id: 1, shouldErr: false},
		&MockJob{id: 2, shouldErr: true},
		&MockJob{id: 3, shouldErr: false},
		&MockJob{id: 4, shouldErr: true},
	}
	
	results := bp.ProcessBatch(jobs)
	
	assert.Len(t, results, 4)
	
	// Count errors and successes (order may vary due to concurrency)
	var errorCount, successCount int
	for _, result := range results {
		if result.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}
	assert.Equal(t, 2, errorCount)
	assert.Equal(t, 2, successCount)
	
	// Progress should still track all jobs
	completed, total := bp.Progress()
	assert.Equal(t, 4, completed)
	assert.Equal(t, 4, total)
}

func TestBatchProcessor_ProcessBatch_Empty(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	bp := NewBatchProcessor(pool, nil)
	
	results := bp.ProcessBatch([]Job{})
	
	assert.Len(t, results, 0)
	
	completed, total := bp.Progress()
	assert.Equal(t, 0, completed)
	assert.Equal(t, 0, total)
}

func TestBatchProcessor_ProcessBatch_SubmissionError(t *testing.T) {
	config := PoolConfig{
		Workers:   1,
		QueueSize: 1,
	}
	pool := NewPool(config)
	// Don't start the pool, so submission will fail when queue is full
	
	bp := NewBatchProcessor(pool, nil)
	
	// Fill the queue and try to submit more
	jobs := []Job{
		&MockJob{id: 1},
		&MockJob{id: 2},
		&MockJob{id: 3}, // This should fail
	}
	
	results := bp.ProcessBatch(jobs)
	
	// Should return error result for failed submission
	assert.Len(t, results, 1)
	assert.Error(t, results[0].Error)
	assert.Contains(t, results[0].Error.Error(), "job queue is full")
	
	pool.Stop()
}

func TestBatchProcessor_ConcurrentProcessing(t *testing.T) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: 100,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	numBatches := 3
	jobsPerBatch := 10
	
	var wg sync.WaitGroup
	allResults := make([][]Result, numBatches)
	
	// Process multiple batches concurrently
	for i := 0; i < numBatches; i++ {
		wg.Add(1)
		go func(batchID int) {
			defer wg.Done()
			
			bp := NewBatchProcessor(pool, nil)
			
			jobs := make([]Job, jobsPerBatch)
			for j := 0; j < jobsPerBatch; j++ {
				jobs[j] = &MockJob{id: batchID*jobsPerBatch + j}
			}
			
			results := bp.ProcessBatch(jobs)
			allResults[batchID] = results
		}(i)
	}
	
	wg.Wait()
	
	// Verify all batches completed successfully
	for i, results := range allResults {
		assert.Len(t, results, jobsPerBatch, "Batch %d has wrong number of results", i)
		for j, result := range results {
			assert.NoError(t, result.Error, "Batch %d, job %d failed", i, j)
		}
	}
}

func TestBatchProcessor_ProgressTracking(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 20,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	var progressHistory []struct {
		completed int
		total     int
	}
	var mutex sync.Mutex
	
	onProgress := func(completed, total int) {
		mutex.Lock()
		progressHistory = append(progressHistory, struct {
			completed int
			total     int
		}{completed, total})
		mutex.Unlock()
	}
	
	bp := NewBatchProcessor(pool, onProgress)
	
	// Create jobs with small delays to see progress updates
	numJobs := 8
	jobs := make([]Job, numJobs)
	for i := 0; i < numJobs; i++ {
		jobs[i] = &MockJob{id: i, duration: 10 * time.Millisecond}
	}
	
	results := bp.ProcessBatch(jobs)
	
	assert.Len(t, results, numJobs)
	
	// Check that progress was tracked
	mutex.Lock()
	assert.True(t, len(progressHistory) >= 1, "Expected at least one progress update")
	
	// Verify progress values are reasonable
	for _, progress := range progressHistory {
		assert.True(t, progress.completed >= 0)
		assert.True(t, progress.completed <= progress.total)
		assert.Equal(t, numJobs, progress.total)
	}
	
	// Last progress should show completion
	lastProgress := progressHistory[len(progressHistory)-1]
	assert.Equal(t, numJobs, lastProgress.completed)
	assert.Equal(t, numJobs, lastProgress.total)
	mutex.Unlock()
}

func TestBatchProcessor_Progress_InitialState(t *testing.T) {
	config := PoolConfig{
		Workers:   2,
		QueueSize: 10,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	bp := NewBatchProcessor(pool, nil)
	
	completed, total := bp.Progress()
	assert.Equal(t, 0, completed)
	assert.Equal(t, 0, total)
}

func TestBatchProcessor_MultipleProcessBatches(t *testing.T) {
	config := PoolConfig{
		Workers:   3,
		QueueSize: 30,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	bp := NewBatchProcessor(pool, nil)
	
	// Process first batch
	jobs1 := []Job{
		&MockJob{id: 1},
		&MockJob{id: 2},
	}
	results1 := bp.ProcessBatch(jobs1)
	assert.Len(t, results1, 2)
	
	completed, total := bp.Progress()
	assert.Equal(t, 2, completed)
	assert.Equal(t, 2, total)
	
	// Process second batch (should reset progress)
	jobs2 := []Job{
		&MockJob{id: 3},
		&MockJob{id: 4},
		&MockJob{id: 5},
	}
	results2 := bp.ProcessBatch(jobs2)
	assert.Len(t, results2, 3)
	
	completed, total = bp.Progress()
	assert.Equal(t, 3, completed)
	assert.Equal(t, 3, total)
}

func TestBatchProcessor_LargeBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large batch test in short mode")
	}
	
	config := PoolConfig{
		Workers:   8,
		QueueSize: 500,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	bp := NewBatchProcessor(pool, nil)
	
	// Create large batch
	numJobs := 200
	jobs := make([]Job, numJobs)
	for i := 0; i < numJobs; i++ {
		jobs[i] = &MockJob{id: i}
	}
	
	start := time.Now()
	results := bp.ProcessBatch(jobs)
	duration := time.Since(start)
	
	assert.Len(t, results, numJobs)
	for i, result := range results {
		assert.NoError(t, result.Error, "Job %d failed", i)
	}
	
	completed, total := bp.Progress()
	assert.Equal(t, numJobs, completed)
	assert.Equal(t, numJobs, total)
	
	t.Logf("Processed %d jobs in %v", numJobs, duration)
	
	// Should complete reasonably quickly with multiple workers
	assert.Less(t, duration.Seconds(), 5.0, "Batch processing took too long")
}

// Benchmark tests
func BenchmarkBatchProcessor_ProcessBatch(b *testing.B) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: b.N,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	bp := NewBatchProcessor(pool, nil)
	
	jobs := make([]Job, b.N)
	for i := 0; i < b.N; i++ {
		jobs[i] = &MockJob{id: i}
	}
	
	b.ResetTimer()
	results := bp.ProcessBatch(jobs)
	
	// Verify results were returned
	assert.Len(b, results, b.N)
}

func BenchmarkBatchProcessor_SmallBatches(b *testing.B) {
	config := PoolConfig{
		Workers:   4,
		QueueSize: 100,
	}
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	bp := NewBatchProcessor(pool, nil)
	batchSize := 10
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		jobs := make([]Job, batchSize)
		for j := 0; j < batchSize; j++ {
			jobs[j] = &MockJob{id: i*batchSize + j}
		}
		
		results := bp.ProcessBatch(jobs)
		if len(results) != batchSize {
			b.Fatalf("Expected %d results, got %d", batchSize, len(results))
		}
	}
}