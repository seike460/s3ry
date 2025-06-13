package performance

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/seike460/s3ry/internal/metrics"
	"github.com/seike460/s3ry/internal/worker"
)

// PerformanceBenchmark represents a baseline performance measurement
type PerformanceBenchmark struct {
	Name           string
	BaselineNS     int64   // Baseline time in nanoseconds
	ToleranceRatio float64 // Acceptable degradation ratio (1.2 = 20% slower is OK)
}

// Performance regression thresholds
var performanceBaselines = []PerformanceBenchmark{
	{
		Name:           "WorkerPool_Creation",
		BaselineNS:     1000000,  // 1ms baseline
		ToleranceRatio: 2.0,      // Allow 2x degradation
	},
	{
		Name:           "WorkerPool_JobSubmission", 
		BaselineNS:     100000,   // 0.1ms baseline
		ToleranceRatio: 3.0,      // Allow 3x degradation for job submission
	},
	{
		Name:           "Metrics_GlobalAccess",
		BaselineNS:     50000,    // 0.05ms baseline
		ToleranceRatio: 2.0,      // Allow 2x degradation
	},
	{
		Name:           "Metrics_SnapshotGeneration",
		BaselineNS:     500000,   // 0.5ms baseline
		ToleranceRatio: 2.0,      // Allow 2x degradation
	},
}

// TestPerformanceRegression runs automated performance regression tests
func TestPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression tests in short mode")
	}

	for _, baseline := range performanceBaselines {
		t.Run(baseline.Name, func(t *testing.T) {
			var actualNS int64
			
			switch baseline.Name {
			case "WorkerPool_Creation":
				actualNS = benchmarkWorkerPoolCreation(t)
			case "WorkerPool_JobSubmission":
				actualNS = benchmarkWorkerPoolJobSubmission(t)
			case "Metrics_GlobalAccess":
				actualNS = benchmarkMetricsGlobalAccess(t)
			case "Metrics_SnapshotGeneration":
				actualNS = benchmarkMetricsSnapshotGeneration(t)
			default:
				t.Fatalf("Unknown benchmark: %s", baseline.Name)
			}
			
			// Check if performance is within acceptable range
			maxAllowed := int64(float64(baseline.BaselineNS) * baseline.ToleranceRatio)
			
			if actualNS > maxAllowed {
				t.Errorf("Performance regression detected in %s: actual %dns > max allowed %dns (baseline: %dns, tolerance: %.1fx)",
					baseline.Name, actualNS, maxAllowed, baseline.BaselineNS, baseline.ToleranceRatio)
			} else {
				t.Logf("✅ Performance OK for %s: %dns <= %dns (baseline: %dns)", 
					baseline.Name, actualNS, maxAllowed, baseline.BaselineNS)
			}
		})
	}
}

// benchmarkWorkerPoolCreation measures worker pool creation time
func benchmarkWorkerPoolCreation(t *testing.T) int64 {
	const iterations = 10
	
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		config := worker.DefaultConfig()
		config.Workers = 4
		pool := worker.NewPool(config)
		pool.Start()
		pool.Stop()
	}
	
	elapsed := time.Since(start)
	avgNS := elapsed.Nanoseconds() / iterations
	
	t.Logf("Worker pool creation: %dns avg (%d iterations)", avgNS, iterations)
	return avgNS
}

// benchmarkWorkerPoolJobSubmission measures job submission performance
func benchmarkWorkerPoolJobSubmission(t *testing.T) int64 {
	config := worker.DefaultConfig()
	config.Workers = 2
	pool := worker.NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	// Simple mock job
	job := &mockJob{}
	
	const iterations = 100
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		pool.Submit(job)
	}
	
	elapsed := time.Since(start)
	avgNS := elapsed.Nanoseconds() / iterations
	
	t.Logf("Job submission: %dns avg (%d iterations)", avgNS, iterations)
	return avgNS
}

// benchmarkMetricsGlobalAccess measures global metrics access time
func benchmarkMetricsGlobalAccess(t *testing.T) int64 {
	const iterations = 1000
	
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		m := metrics.GetGlobalMetrics()
		_ = m
	}
	
	elapsed := time.Since(start)
	avgNS := elapsed.Nanoseconds() / iterations
	
	t.Logf("Global metrics access: %dns avg (%d iterations)", avgNS, iterations)
	return avgNS
}

// benchmarkMetricsSnapshotGeneration measures snapshot generation time
func benchmarkMetricsSnapshotGeneration(t *testing.T) int64 {
	m := metrics.GetGlobalMetrics()
	
	// Add some metrics data
	for i := 0; i < 10; i++ {
		m.IncrementS3Downloads()
		m.AddBytesTransferred(1024)
	}
	
	const iterations = 100
	start := time.Now()
	
	for i := 0; i < iterations; i++ {
		snapshot := m.GetSnapshot()
		_ = snapshot
	}
	
	elapsed := time.Since(start)
	avgNS := elapsed.Nanoseconds() / iterations
	
	t.Logf("Snapshot generation: %dns avg (%d iterations)", avgNS, iterations)
	return avgNS
}

// TestMemoryUsageRegression checks for memory usage regressions
func TestMemoryUsageRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory regression tests in short mode")
	}
	
	// Force GC to get accurate baseline
	runtime.GC()
	runtime.GC()
	
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	
	// Create worker pool and submit jobs
	config := worker.DefaultConfig()
	config.Workers = 4
	pool := worker.NewPool(config)
	pool.Start()
	
	// Submit jobs to test memory usage
	job := &mockJob{}
	for i := 0; i < 100; i++ {
		pool.Submit(job)
	}
	
	// Wait for jobs to complete
	time.Sleep(50 * time.Millisecond)
	pool.Stop()
	
	// Force GC and measure memory
	runtime.GC()
	runtime.GC()
	
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	
	// Check memory increase
	memIncreaseBytes := after.Alloc - baseline.Alloc
	maxAllowedBytes := int64(10 * 1024 * 1024) // 10MB max increase
	
	if int64(memIncreaseBytes) > maxAllowedBytes {
		t.Errorf("Memory usage regression: increased by %d bytes (max allowed: %d bytes)", 
			memIncreaseBytes, maxAllowedBytes)
	} else {
		t.Logf("✅ Memory usage OK: increased by %d bytes (<= %d bytes)", 
			memIncreaseBytes, maxAllowedBytes)
	}
}

// TestConcurrentPerformance verifies concurrent operation performance
func TestConcurrentPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent performance tests in short mode")
	}
	
	// Test concurrent metrics access
	const numGoroutines = 10
	const operationsPerGoroutine = 100
	
	start := time.Now()
	done := make(chan bool, numGoroutines)
	
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()
			
			m := metrics.GetGlobalMetrics()
			for j := 0; j < operationsPerGoroutine; j++ {
				m.IncrementS3Downloads()
				_ = m.GetSnapshot()
			}
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
	
	elapsed := time.Since(start)
	totalOps := numGoroutines * operationsPerGoroutine
	opsPerSecond := float64(totalOps) / elapsed.Seconds()
	
	// Expect at least 1000 operations per second under concurrent load
	minOpsPerSec := 1000.0
	
	if opsPerSecond < minOpsPerSec {
		t.Errorf("Concurrent performance regression: %.2f ops/sec < %.2f ops/sec required", 
			opsPerSecond, minOpsPerSec)
	} else {
		t.Logf("✅ Concurrent performance OK: %.2f ops/sec (>= %.2f ops/sec)", 
			opsPerSecond, minOpsPerSec)
	}
}

// mockJob implements the Job interface for testing
type mockJob struct{}

func (j *mockJob) Execute(ctx context.Context) error {
	// Simulate minimal work without sleep
	return nil
}

// BenchmarkWorkerPoolThroughput benchmarks overall worker pool throughput
func BenchmarkWorkerPoolThroughput(b *testing.B) {
	config := worker.DefaultConfig()
	config.Workers = 4
	config.QueueSize = b.N + 100 // Ensure queue is large enough
	pool := worker.NewPool(config)
	pool.Start()
	defer pool.Stop()
	
	job := &mockJob{}
	
	// Start a result consumer
	go func() {
		for i := 0; i < b.N; i++ {
			<-pool.Results()
		}
	}()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(job)
	}
}

// BenchmarkMetricsPerformance benchmarks metrics operations
func BenchmarkMetricsPerformance(b *testing.B) {
	m := metrics.GetGlobalMetrics()
	
	b.Run("IncrementOperations", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				m.IncrementS3Downloads()
			}
		})
	})
	
	b.Run("GetSnapshot", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = m.GetSnapshot()
		}
	})
	
	b.Run("AddBytesTransferred", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				m.AddBytesTransferred(1024)
			}
		})
	})
}