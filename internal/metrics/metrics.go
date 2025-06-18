package metrics

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Metrics holds performance metrics
type Metrics struct {
	mu                sync.RWMutex
	StartTime         time.Time
	TotalOperations   int64
	S3Operations      S3Metrics
	MemoryUsage       MemoryMetrics
	PerformanceTimers map[string]time.Duration
}

// S3Metrics holds S3-specific metrics
type S3Metrics struct {
	Downloads        int64
	Uploads          int64
	Deletes          int64
	ListOperations   int64
	TotalBytes       int64
	FailedOperations int64
}

// MemoryMetrics holds memory usage information
type MemoryMetrics struct {
	AllocatedBytes   uint64
	TotalAllocations uint64
	GCRuns           uint32
	HeapSize         uint64
}

// Timer represents a performance timer
type Timer struct {
	name      string
	startTime time.Time
	metrics   *Metrics
}

var (
	globalMetrics *Metrics
	once          sync.Once
)

// GetGlobalMetrics returns the global metrics instance
func GetGlobalMetrics() *Metrics {
	once.Do(func() {
		globalMetrics = NewMetrics()
	})
	return globalMetrics
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		StartTime:         time.Now(),
		PerformanceTimers: make(map[string]time.Duration),
	}
}

// IncrementS3Downloads increments the download counter
func (m *Metrics) IncrementS3Downloads() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.S3Operations.Downloads++
	m.TotalOperations++
}

// IncrementS3Uploads increments the upload counter
func (m *Metrics) IncrementS3Uploads() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.S3Operations.Uploads++
	m.TotalOperations++
}

// IncrementS3Deletes increments the delete counter
func (m *Metrics) IncrementS3Deletes() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.S3Operations.Deletes++
	m.TotalOperations++
}

// IncrementS3Lists increments the list operations counter
func (m *Metrics) IncrementS3Lists() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.S3Operations.ListOperations++
	m.TotalOperations++
}

// AddBytesTransferred adds to the total bytes transferred
func (m *Metrics) AddBytesTransferred(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.S3Operations.TotalBytes += bytes
}

// IncrementFailedOperations increments the failed operations counter
func (m *Metrics) IncrementFailedOperations() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.S3Operations.FailedOperations++
}

// UpdateMemoryMetrics updates the memory usage metrics
func (m *Metrics) UpdateMemoryMetrics() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.MemoryUsage.AllocatedBytes = memStats.Alloc
	m.MemoryUsage.TotalAllocations = memStats.TotalAlloc
	m.MemoryUsage.GCRuns = memStats.NumGC
	m.MemoryUsage.HeapSize = memStats.HeapAlloc
}

// StartTimer starts a named performance timer
func (m *Metrics) StartTimer(name string) *Timer {
	return &Timer{
		name:      name,
		startTime: time.Now(),
		metrics:   m,
	}
}

// Stop stops the timer and records the duration
func (t *Timer) Stop() time.Duration {
	duration := time.Since(t.startTime)

	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()

	t.metrics.PerformanceTimers[t.name] = duration

	return duration
}

// GetUptime returns the uptime since metrics started
func (m *Metrics) GetUptime() time.Duration {
	return time.Since(m.StartTime)
}

// GetOperationsPerSecond calculates operations per second
func (m *Metrics) GetOperationsPerSecond() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.StartTime)
	if uptime.Seconds() == 0 {
		return 0
	}

	return float64(m.TotalOperations) / uptime.Seconds()
}

// GetBytesPerSecond calculates bytes transferred per second
func (m *Metrics) GetBytesPerSecond() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uptime := time.Since(m.StartTime)
	if uptime.Seconds() == 0 {
		return 0
	}

	return float64(m.S3Operations.TotalBytes) / uptime.Seconds()
}

// GetFailureRate calculates the failure rate as a percentage
func (m *Metrics) GetFailureRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.TotalOperations == 0 {
		return 0
	}

	return (float64(m.S3Operations.FailedOperations) / float64(m.TotalOperations)) * 100
}

// GetSnapshot returns a snapshot of current metrics
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	// Update memory metrics first without holding any locks
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	m.mu.Lock()
	// Update memory metrics while holding write lock
	m.MemoryUsage.AllocatedBytes = memStats.Alloc
	m.MemoryUsage.TotalAllocations = memStats.TotalAlloc
	m.MemoryUsage.GCRuns = memStats.NumGC
	m.MemoryUsage.HeapSize = memStats.HeapAlloc

	// Create snapshot while holding write lock
	timers := make(map[string]time.Duration)
	for k, v := range m.PerformanceTimers {
		timers[k] = v
	}

	uptime := time.Since(m.StartTime)
	var operationsPerSec, bytesPerSec, failureRate float64

	if uptime.Seconds() > 0 {
		operationsPerSec = float64(m.TotalOperations) / uptime.Seconds()
		bytesPerSec = float64(m.S3Operations.TotalBytes) / uptime.Seconds()
	}

	if m.TotalOperations > 0 {
		failureRate = (float64(m.S3Operations.FailedOperations) / float64(m.TotalOperations)) * 100
	}

	snapshot := MetricsSnapshot{
		Timestamp:         time.Now(),
		Uptime:            uptime,
		TotalOperations:   m.TotalOperations,
		S3Operations:      m.S3Operations,
		MemoryUsage:       m.MemoryUsage,
		PerformanceTimers: timers,
		OperationsPerSec:  operationsPerSec,
		BytesPerSec:       bytesPerSec,
		FailureRate:       failureRate,
	}

	m.mu.Unlock()
	return snapshot
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	Timestamp         time.Time
	Uptime            time.Duration
	TotalOperations   int64
	S3Operations      S3Metrics
	MemoryUsage       MemoryMetrics
	PerformanceTimers map[string]time.Duration
	OperationsPerSec  float64
	BytesPerSec       float64
	FailureRate       float64
}

// String returns a formatted string representation of the metrics
func (ms MetricsSnapshot) String() string {
	return fmt.Sprintf(`Metrics Snapshot (%s)
===================
Uptime: %v
Total Operations: %d
Operations/sec: %.2f
Failure Rate: %.2f%%

S3 Operations:
  Downloads: %d
  Uploads: %d
  Deletes: %d
  List Operations: %d
  Total Bytes: %d
  Failed Operations: %d
  Bytes/sec: %.2f

Memory Usage:
  Allocated: %d bytes
  Total Allocations: %d bytes
  GC Runs: %d
  Heap Size: %d bytes
`,
		ms.Timestamp.Format(time.RFC3339),
		ms.Uptime,
		ms.TotalOperations,
		ms.OperationsPerSec,
		ms.FailureRate,
		ms.S3Operations.Downloads,
		ms.S3Operations.Uploads,
		ms.S3Operations.Deletes,
		ms.S3Operations.ListOperations,
		ms.S3Operations.TotalBytes,
		ms.S3Operations.FailedOperations,
		ms.BytesPerSec,
		ms.MemoryUsage.AllocatedBytes,
		ms.MemoryUsage.TotalAllocations,
		ms.MemoryUsage.GCRuns,
		ms.MemoryUsage.HeapSize,
	)
}

// Reset resets all metrics to zero
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StartTime = time.Now()
	m.TotalOperations = 0
	m.S3Operations = S3Metrics{}
	m.MemoryUsage = MemoryMetrics{}
	m.PerformanceTimers = make(map[string]time.Duration)
}
