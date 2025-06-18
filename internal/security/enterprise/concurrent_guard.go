package enterprise

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// ConcurrentGuard provides security guards for concurrent processing
type ConcurrentGuard struct {
	monitor      *SecurityMonitor
	raceDetector *RaceConditionDetector
	leakDetector *ResourceLeakDetector
	config       *ConcurrentGuardConfig
	mutex        sync.RWMutex
	guardedOps   int64
	violations   int64
}

// ConcurrentGuardConfig holds concurrent security configuration
type ConcurrentGuardConfig struct {
	Enabled              bool          `json:"enabled"`
	RaceDetectionEnabled bool          `json:"race_detection_enabled"`
	LeakDetectionEnabled bool          `json:"leak_detection_enabled"`
	MaxGoroutines        int           `json:"max_goroutines"`
	MemoryThresholdMB    int64         `json:"memory_threshold_mb"`
	CheckInterval        time.Duration `json:"check_interval"`
	ViolationThreshold   int64         `json:"violation_threshold"`
}

// NewConcurrentGuard creates a new concurrent security guard
func NewConcurrentGuard(monitor *SecurityMonitor, config *ConcurrentGuardConfig) *ConcurrentGuard {
	if config == nil {
		config = DefaultConcurrentGuardConfig()
	}

	return &ConcurrentGuard{
		monitor:      monitor,
		raceDetector: NewRaceConditionDetector(),
		leakDetector: NewResourceLeakDetector(),
		config:       config,
	}
}

// DefaultConcurrentGuardConfig returns default concurrent guard configuration
func DefaultConcurrentGuardConfig() *ConcurrentGuardConfig {
	return &ConcurrentGuardConfig{
		Enabled:              true,
		RaceDetectionEnabled: true,
		LeakDetectionEnabled: true,
		MaxGoroutines:        10000,
		MemoryThresholdMB:    1000, // 1GB
		CheckInterval:        time.Second * 10,
		ViolationThreshold:   5,
	}
}

// GuardWorkerExecution guards worker pool execution with security monitoring
func (cg *ConcurrentGuard) GuardWorkerExecution(ctx context.Context, workerID int, fn func() error) error {
	if !cg.config.Enabled {
		return fn()
	}

	atomic.AddInt64(&cg.guardedOps, 1)

	// Pre-execution checks
	if err := cg.preExecutionCheck(ctx, workerID); err != nil {
		atomic.AddInt64(&cg.violations, 1)
		return err
	}

	// Execute with monitoring
	startTime := time.Now()
	var execErr error

	// Detect potential race conditions
	if cg.config.RaceDetectionEnabled {
		cg.raceDetector.BeginOperation(workerID)
		defer cg.raceDetector.EndOperation(workerID)
	}

	// Execute function
	execErr = fn()

	// Post-execution checks
	duration := time.Since(startTime)
	if err := cg.postExecutionCheck(ctx, workerID, duration, execErr); err != nil {
		atomic.AddInt64(&cg.violations, 1)
		cg.monitor.RecordWorkerPoolAnomaly(runtime.NumGoroutine(), cg.config.MaxGoroutines)
	}

	return execErr
}

// GuardSliceOperation guards concurrent slice operations
func (cg *ConcurrentGuard) GuardSliceOperation(slicePtr interface{}, operation func() error) error {
	if !cg.config.Enabled {
		return operation()
	}

	// Monitor for slice reallocation race conditions
	cg.raceDetector.MonitorSliceOperation(slicePtr)
	defer cg.raceDetector.CompleteSliceOperation(slicePtr)

	return operation()
}

// GuardContextOperation guards context-based operations
func (cg *ConcurrentGuard) GuardContextOperation(ctx context.Context, operation func(context.Context) error) error {
	if !cg.config.Enabled {
		return operation(ctx)
	}

	// Create monitored context
	monitoredCtx := cg.createMonitoredContext(ctx)
	defer cg.cleanupMonitoredContext(monitoredCtx)

	return operation(monitoredCtx)
}

// preExecutionCheck performs pre-execution security checks
func (cg *ConcurrentGuard) preExecutionCheck(ctx context.Context, workerID int) error {
	// Check goroutine count
	if runtime.NumGoroutine() > cg.config.MaxGoroutines {
		cg.monitor.RecordWorkerPoolAnomaly(runtime.NumGoroutine(), cg.config.MaxGoroutines)
		return ErrTooManyGoroutines
	}

	// Check memory usage
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	currentMemMB := int64(m.Alloc / 1024 / 1024)

	if currentMemMB > cg.config.MemoryThresholdMB {
		cg.monitor.RecordMemoryPressure(currentMemMB)
		return ErrMemoryThresholdExceeded
	}

	// Check for context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return nil
}

// postExecutionCheck performs post-execution security checks
func (cg *ConcurrentGuard) postExecutionCheck(ctx context.Context, workerID int, duration time.Duration, execErr error) error {
	// Check for excessive execution time
	if duration > time.Minute*5 {
		atomic.AddInt64(&cg.monitor.metrics.TimeoutAnomalies, 1)
		cg.monitor.escalateThreatLevel(ThreatLevelMedium)
	}

	// Check for resource leaks
	if cg.config.LeakDetectionEnabled {
		if leak := cg.leakDetector.CheckForLeaks(workerID); leak != nil {
			cg.monitor.RecordResourceLeak(leak.ResourceType)
			return ErrResourceLeakDetected
		}
	}

	// Check error patterns
	if execErr != nil {
		cg.analyzeErrorPattern(execErr)
	}

	return nil
}

// createMonitoredContext creates a context with security monitoring
func (cg *ConcurrentGuard) createMonitoredContext(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)

	// Add security monitoring metadata
	monitoredCtx := context.WithValue(ctx, "security_monitor", cg.monitor)
	monitoredCtx = context.WithValue(monitoredCtx, "guard_start_time", time.Now())
	monitoredCtx = context.WithValue(monitoredCtx, "cancel_func", cancel)

	return monitoredCtx
}

// cleanupMonitoredContext cleans up monitored context
func (cg *ConcurrentGuard) cleanupMonitoredContext(ctx context.Context) {
	if cancelFunc, ok := ctx.Value("cancel_func").(context.CancelFunc); ok {
		cancelFunc()
	}
}

// analyzeErrorPattern analyzes error patterns for security implications
func (cg *ConcurrentGuard) analyzeErrorPattern(err error) {
	// Analyze error message for security indicators
	errorMsg := err.Error()

	// Check for potential attack patterns
	suspiciousPatterns := []string{
		"too many connections",
		"resource exhausted",
		"timeout exceeded",
		"memory allocation failed",
	}

	for _, pattern := range suspiciousPatterns {
		if contains(errorMsg, pattern) {
			atomic.AddInt64(&cg.violations, 1)
			break
		}
	}
}

// GetStatistics returns guard statistics
func (cg *ConcurrentGuard) GetStatistics() *ConcurrentGuardStats {
	return &ConcurrentGuardStats{
		GuardedOperations: atomic.LoadInt64(&cg.guardedOps),
		Violations:        atomic.LoadInt64(&cg.violations),
		CurrentGoroutines: int64(runtime.NumGoroutine()),
		RaceDetections:    cg.raceDetector.GetDetectionCount(),
		LeakDetections:    cg.leakDetector.GetLeakCount(),
	}
}

// ConcurrentGuardStats holds guard statistics
type ConcurrentGuardStats struct {
	GuardedOperations int64 `json:"guarded_operations"`
	Violations        int64 `json:"violations"`
	CurrentGoroutines int64 `json:"current_goroutines"`
	RaceDetections    int64 `json:"race_detections"`
	LeakDetections    int64 `json:"leak_detections"`
}

// RaceConditionDetector detects potential race conditions
type RaceConditionDetector struct {
	activeOperations map[int]time.Time
	sliceOperations  map[interface{}]time.Time
	detectionCount   int64
	mutex            sync.RWMutex
}

// NewRaceConditionDetector creates a new race condition detector
func NewRaceConditionDetector() *RaceConditionDetector {
	return &RaceConditionDetector{
		activeOperations: make(map[int]time.Time),
		sliceOperations:  make(map[interface{}]time.Time),
	}
}

// BeginOperation begins monitoring an operation for race conditions
func (rcd *RaceConditionDetector) BeginOperation(operationID int) {
	rcd.mutex.Lock()
	defer rcd.mutex.Unlock()

	if _, exists := rcd.activeOperations[operationID]; exists {
		// Potential race condition detected
		atomic.AddInt64(&rcd.detectionCount, 1)
	}

	rcd.activeOperations[operationID] = time.Now()
}

// EndOperation ends monitoring an operation
func (rcd *RaceConditionDetector) EndOperation(operationID int) {
	rcd.mutex.Lock()
	defer rcd.mutex.Unlock()

	delete(rcd.activeOperations, operationID)
}

// MonitorSliceOperation monitors slice operations for race conditions
func (rcd *RaceConditionDetector) MonitorSliceOperation(slicePtr interface{}) {
	rcd.mutex.Lock()
	defer rcd.mutex.Unlock()

	if _, exists := rcd.sliceOperations[slicePtr]; exists {
		// Concurrent slice operation detected
		atomic.AddInt64(&rcd.detectionCount, 1)
	}

	rcd.sliceOperations[slicePtr] = time.Now()
}

// CompleteSliceOperation completes slice operation monitoring
func (rcd *RaceConditionDetector) CompleteSliceOperation(slicePtr interface{}) {
	rcd.mutex.Lock()
	defer rcd.mutex.Unlock()

	delete(rcd.sliceOperations, slicePtr)
}

// GetDetectionCount returns the number of race conditions detected
func (rcd *RaceConditionDetector) GetDetectionCount() int64 {
	return atomic.LoadInt64(&rcd.detectionCount)
}

// ResourceLeakDetector detects resource leaks
type ResourceLeakDetector struct {
	trackedResources map[int]*TrackedResource
	leakCount        int64
	mutex            sync.RWMutex
}

// TrackedResource represents a tracked resource
type TrackedResource struct {
	ResourceType string
	CreatedAt    time.Time
	LastAccess   time.Time
	WorkerID     int
}

// ResourceLeak represents a detected resource leak
type ResourceLeak struct {
	ResourceType string
	WorkerID     int
	Duration     time.Duration
}

// NewResourceLeakDetector creates a new resource leak detector
func NewResourceLeakDetector() *ResourceLeakDetector {
	return &ResourceLeakDetector{
		trackedResources: make(map[int]*TrackedResource),
	}
}

// CheckForLeaks checks for resource leaks for a specific worker
func (rld *ResourceLeakDetector) CheckForLeaks(workerID int) *ResourceLeak {
	rld.mutex.RLock()
	defer rld.mutex.RUnlock()

	if resource, exists := rld.trackedResources[workerID]; exists {
		// Check if resource has been idle for too long
		if time.Since(resource.LastAccess) > time.Minute*10 {
			atomic.AddInt64(&rld.leakCount, 1)
			return &ResourceLeak{
				ResourceType: resource.ResourceType,
				WorkerID:     workerID,
				Duration:     time.Since(resource.CreatedAt),
			}
		}
	}

	return nil
}

// GetLeakCount returns the number of leaks detected
func (rld *ResourceLeakDetector) GetLeakCount() int64 {
	return atomic.LoadInt64(&rld.leakCount)
}

// Security error definitions
var (
	ErrTooManyGoroutines       = &SecurityError{Code: "GOROUTINE_LIMIT", Message: "Too many goroutines"}
	ErrMemoryThresholdExceeded = &SecurityError{Code: "MEMORY_LIMIT", Message: "Memory threshold exceeded"}
	ErrResourceLeakDetected    = &SecurityError{Code: "RESOURCE_LEAK", Message: "Resource leak detected"}
)

// SecurityError represents a security-related error
type SecurityError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *SecurityError) Error() string {
	return e.Message
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || (len(s) > len(substr) &&
			(s[0:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsInMiddle(s, substr))))
}

func containsInMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
