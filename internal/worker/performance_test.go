package worker

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// PerformanceJob simulates CPU-intensive work for benchmarking
type PerformanceJob struct {
	id       string
	duration time.Duration
}

func (j *PerformanceJob) Execute(ctx context.Context) error {
	// Simulate work by sleeping
	time.Sleep(j.duration)
	return nil
}

// BenchmarkSequentialVsParallel compares sequential vs parallel execution
func BenchmarkSequentialVsParallel(b *testing.B) {
	jobCount := 100
	jobDuration := 10 * time.Millisecond
	
	b.Run("Sequential", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Sequential execution
			start := time.Now()
			for j := 0; j < jobCount; j++ {
				job := &PerformanceJob{
					id:       fmt.Sprintf("job-%d", j),
					duration: jobDuration,
				}
				_ = job.Execute(context.Background())
			}
			b.Logf("Sequential execution took: %v", time.Since(start))
		}
	})

	b.Run("Parallel-5Workers", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkParallelExecution(b, jobCount, jobDuration, 5)
		}
	})

	b.Run("Parallel-10Workers", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			benchmarkParallelExecution(b, jobCount, jobDuration, 10)
		}
	})
}

func benchmarkParallelExecution(b *testing.B, jobCount int, jobDuration time.Duration, workers int) {
	config := PoolConfig{
		Workers:   workers,
		QueueSize: jobCount * 2,
		Timeout:   30 * time.Second,
	}
	
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	start := time.Now()
	
	// Submit all jobs
	for j := 0; j < jobCount; j++ {
		job := &PerformanceJob{
			id:       fmt.Sprintf("job-%d", j),
			duration: jobDuration,
		}
		err := pool.Submit(job)
		if err != nil {
			b.Fatalf("Failed to submit job: %v", err)
		}
	}
	
	// Wait for all results
	for j := 0; j < jobCount; j++ {
		<-pool.Results()
	}
	
	elapsed := time.Since(start)
	b.Logf("Parallel execution (%d workers) took: %v", workers, elapsed)
}

// BenchmarkWorkerPoolPerformance tests the worker pool performance characteristics
func BenchmarkWorkerPoolPerformance(b *testing.B) {
	workerCounts := []int{1, 2, 5, 10, 20}
	jobCounts := []int{50, 100, 200}
	
	for _, workers := range workerCounts {
		for _, jobs := range jobCounts {
			b.Run(fmt.Sprintf("Workers%d-Jobs%d", workers, jobs), func(b *testing.B) {
				config := PoolConfig{
					Workers:   workers,
					QueueSize: jobs * 2,
					Timeout:   30 * time.Second,
				}
				
				b.ResetTimer()
				
				for i := 0; i < b.N; i++ {
					pool := NewPool(config)
					pool.Start()
					
					// Submit jobs
					for j := 0; j < jobs; j++ {
						job := &PerformanceJob{
							id:       fmt.Sprintf("job-%d", j),
							duration: 5 * time.Millisecond,
						}
						err := pool.Submit(job)
						if err != nil {
							b.Fatalf("Failed to submit job: %v", err)
						}
					}
					
					// Wait for all results
					for j := 0; j < jobs; j++ {
						<-pool.Results()
					}
					
					pool.Stop()
				}
			})
		}
	}
}

// BenchmarkMemoryEfficiency tests memory usage patterns
func BenchmarkMemoryEfficiency(b *testing.B) {
	b.Run("LargeJobQueue", func(b *testing.B) {
		config := PoolConfig{
			Workers:   10,
			QueueSize: 10000, // Large queue
			Timeout:   30 * time.Second,
		}
		
		b.ResetTimer()
		
		for i := 0; i < b.N; i++ {
			pool := NewPool(config)
			pool.Start()
			
			// Fill up the queue
			jobCount := 1000
			for j := 0; j < jobCount; j++ {
				job := &PerformanceJob{
					id:       fmt.Sprintf("job-%d", j),
					duration: 1 * time.Millisecond,
				}
				err := pool.Submit(job)
				if err != nil {
					b.Fatalf("Failed to submit job: %v", err)
				}
			}
			
			// Process all jobs
			for j := 0; j < jobCount; j++ {
				<-pool.Results()
			}
			
			pool.Stop()
		}
	})
}

// TestPerformanceImprovement tests the actual performance improvement
func TestPerformanceImprovement(t *testing.T) {
	jobCount := 50
	jobDuration := 20 * time.Millisecond
	
	// Sequential execution
	start := time.Now()
	for j := 0; j < jobCount; j++ {
		job := &PerformanceJob{
			id:       fmt.Sprintf("job-%d", j),
			duration: jobDuration,
		}
		_ = job.Execute(context.Background())
	}
	sequentialTime := time.Since(start)
	
	// Parallel execution with 10 workers
	config := PoolConfig{
		Workers:   10,
		QueueSize: jobCount * 2,
		Timeout:   30 * time.Second,
	}
	
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	start = time.Now()
	
	// Submit all jobs
	for j := 0; j < jobCount; j++ {
		job := &PerformanceJob{
			id:       fmt.Sprintf("job-%d", j),
			duration: jobDuration,
		}
		err := pool.Submit(job)
		if err != nil {
			t.Fatalf("Failed to submit job: %v", err)
		}
	}
	
	// Wait for all results
	for j := 0; j < jobCount; j++ {
		<-pool.Results()
	}
	
	parallelTime := time.Since(start)
	
	// Calculate improvement
	improvement := float64(sequentialTime) / float64(parallelTime)
	
	t.Logf("Sequential time: %v", sequentialTime)
	t.Logf("Parallel time (10 workers): %v", parallelTime)
	t.Logf("Performance improvement: %.2fx", improvement)
	
	// Expect at least 3x improvement (conservative for 10 workers due to overhead)
	if improvement < 3.0 {
		t.Errorf("Expected at least 3x improvement, got %.2fx", improvement)
	}
	
	// Log success if we achieved 5x or better
	if improvement >= 5.0 {
		t.Logf("✅ Achieved 5x+ performance improvement: %.2fx", improvement)
	}
}

// TestConcurrentDownloads simulates concurrent S3 downloads
func TestConcurrentDownloads(t *testing.T) {
	// Simulate download jobs with varying sizes
	type DownloadJob struct {
		filename string
		size     int64
		duration time.Duration
	}
	
	downloads := []DownloadJob{
		{"file1.txt", 1024 * 1024, 100 * time.Millisecond},
		{"file2.jpg", 5 * 1024 * 1024, 200 * time.Millisecond},
		{"file3.pdf", 10 * 1024 * 1024, 300 * time.Millisecond},
		{"file4.zip", 50 * 1024 * 1024, 500 * time.Millisecond},
		{"file5.mp4", 100 * 1024 * 1024, 800 * time.Millisecond},
	}
	
	// Sequential downloads
	start := time.Now()
	for _, dl := range downloads {
		job := &PerformanceJob{
			id:       dl.filename,
			duration: dl.duration,
		}
		_ = job.Execute(context.Background())
	}
	sequentialTime := time.Since(start)
	
	// Concurrent downloads
	config := PoolConfig{
		Workers:   5, // One worker per download
		QueueSize: len(downloads) * 2,
		Timeout:   30 * time.Second,
	}
	
	pool := NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	start = time.Now()
	
	// Submit all download jobs
	for _, dl := range downloads {
		job := &PerformanceJob{
			id:       dl.filename,
			duration: dl.duration,
		}
		err := pool.Submit(job)
		if err != nil {
			t.Fatalf("Failed to submit download job: %v", err)
		}
	}
	
	// Wait for all downloads
	for range downloads {
		<-pool.Results()
	}
	
	parallelTime := time.Since(start)
	
	improvement := float64(sequentialTime) / float64(parallelTime)
	
	t.Logf("Sequential downloads: %v", sequentialTime)
	t.Logf("Concurrent downloads: %v", parallelTime)
	t.Logf("Download improvement: %.2fx", improvement)
	
	// For concurrent downloads, we expect at least 2x improvement in realistic test environments
	expectedMin := 2.0 // Realistic expectation for test environment
	if improvement < expectedMin {
		t.Errorf("Expected at least %.1fx improvement, got %.2fx", expectedMin, improvement)
	}
	
	// Log the performance achievement
	if improvement >= 2.0 {
		t.Logf("✅ Performance test passed - achieved %.2fx improvement", improvement)
	}
}

// TestMemoryUsageComparison tests memory efficiency
func TestMemoryUsageComparison(t *testing.T) {
	jobCount := 1000
	
	// Test with small queue vs large queue
	configs := []struct {
		name      string
		workers   int
		queueSize int
	}{
		{"SmallQueue", 5, 10},
		{"MediumQueue", 10, 100},
		{"LargeQueue", 10, 1000},
	}
	
	for _, config := range configs {
		t.Run(config.name, func(t *testing.T) {
			poolConfig := PoolConfig{
				Workers:   config.workers,
				QueueSize: config.queueSize,
				Timeout:   30 * time.Second,
			}
			
			pool := NewPool(poolConfig)
			pool.Start()
			defer pool.Stop()
			
			start := time.Now()
			
			// Submit jobs in batches to test queue efficiency
			batchSize := config.queueSize / 2
			if batchSize <= 0 {
				batchSize = 1
			}
			
			var wg sync.WaitGroup
			
			// Producer goroutine
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < jobCount; j++ {
					job := &PerformanceJob{
						id:       fmt.Sprintf("job-%d", j),
						duration: 5 * time.Millisecond,
					}
					
					for {
						err := pool.Submit(job)
						if err == nil {
							break
						}
						// If queue is full, wait a bit
						time.Sleep(1 * time.Millisecond)
					}
				}
			}()
			
			// Consumer - collect results
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < jobCount; j++ {
					<-pool.Results()
				}
			}()
			
			wg.Wait()
			
			elapsed := time.Since(start)
			t.Logf("%s completed %d jobs in %v", config.name, jobCount, elapsed)
		})
	}
}